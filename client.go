package winminer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

// API URLs.
const (
	apiBaseURL         = "https://api.winminer.com"
	loginURL           = apiBaseURL + "/user/login"
	statsURL           = apiBaseURL + "/user/stats"
	withdrawHistoryURL = apiBaseURL + "/user/withdraw-history"
	exchangeURL        = apiBaseURL + "/coin/exchange"
	withdrawDataURL    = apiBaseURL + "/withdraw/data"
	machinesURL        = apiBaseURL + "/hub/machines"
	hubAuth2URL        = apiBaseURL + "/hub/auth2"
)

// JSON content type.
const (
	jsonContentType = "application/json; charset=utf-8"
)

type lowLevelClient struct {
	c             *http.Client
	userToken     string
	userTokenLock sync.RWMutex
	debug         bool
}

func (c *lowLevelClient) connect(auth2Token, hubBaseURL, connectionToken string) (*websocket.Conn, error) {
	// this does not need to be a method of lowLevelClient, but we'll leave it like that for now

	v := url.Values{}
	v.Set("clientProtocol", "1.5")
	v.Set("connectionData", "[{\"name\":\"reportinghub\"}]")
	v.Set("token", auth2Token)
	v.Set("transport", "webSockets")
	v.Set("tid", "10")
	v.Set("connectionToken", connectionToken)

	d := websocket.DefaultDialer
	wsUrl := "wss:" + strings.Split(hubBaseURL, ":")[1]
	conn, _, err := d.Dial(wsUrl+"/signalr/connect?"+v.Encode(), http.Header{})
	if err != nil {
		return nil, errors.Wrap(err, "unable to open WebSockets connection")
	}

	return conn, nil
}

// A GenericSignalrResponse is used for both the /start and /ping endpoint of
// the live API "signalr" endpoint.
type GenericSignalrResponse struct {
	Response string `json:"Response"`
}

func (c *lowLevelClient) start(nonce int64, auth2Token, hubBaseURL, connectionToken string) error {
	v := url.Values{}
	v.Set("clientProtocol", "1.5")
	v.Set("connectionData", "[{\"name\":\"reportinghub\"}]")
	v.Set("connectionToken", connectionToken)
	v.Set("token", auth2Token)
	v.Set("_", fmt.Sprint(nonce))
	v.Set("transport", "webSockets")
	var resp GenericSignalrResponse

	err := c.do(http.MethodGet, false, hubBaseURL+"/signalr/start", v, nil, &resp)
	if err != nil {
		return errors.Wrap(err, "unable to start")
	}

	if resp.Response != "started" {
		return errors.New("did not receive a started response")
	}

	return nil
}

func (c *lowLevelClient) ping(nonce int64, auth2Token, hubBaseURL string) error {
	v := url.Values{}
	v.Set("token", auth2Token)
	v.Set("_", fmt.Sprint(nonce))
	var resp GenericSignalrResponse

	err := c.do(http.MethodGet, false, hubBaseURL+"/signalr/ping", v, nil, &resp)
	if err != nil {
		return errors.Wrap(err, "unable to ping")
	}

	if resp.Response != "pong" {
		return errors.New("did not receive a pong response")
	}

	return nil
}

// A NegotiateResponse is the response to a negotiate request for a websocket
// connection.
type NegotiateResponse struct {
	URL                        string          `json:"Url"`
	ConnectionToken            string          `json:"ConnectionToken"`
	ConnectionID               string          `json:"ConnectionId"`
	KeepAliveTimeout           decimal.Decimal `json:"KeepAliveTimeout"`
	DisconnectTimeout          decimal.Decimal `json:"DisconnectTimeout"`
	ConnectionTimeout          decimal.Decimal `json:"ConnectionTimeout"`
	TryWebSockets              bool            `json:"TryWebSockets"`
	ProtocolVersion            string          `json:"ProtocolVersion"`
	TransportConnectionTimeout decimal.Decimal `json:"TransportConnectionTimeout"`
	LongPollDelay              decimal.Decimal `json:"LongPollDelay"`
}

func (c *lowLevelClient) negotiate(nonce int64, auth2Token, hubBaseURL string) (*NegotiateResponse, error) {
	v := url.Values{}
	v.Set("clientProtocol", "1.5")
	v.Set("connectionData", "[{\"name\":\"reportinghub\"}]")
	v.Set("token", auth2Token)
	v.Set("_", fmt.Sprint(nonce))
	var resp NegotiateResponse

	err := c.do(http.MethodGet, false, hubBaseURL+"/signalr/negotiate", v, nil, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "unable to negotiate")
	}

	return &resp, nil
}

// A WithdrawHistoryResponse is the resopnse to a withdraw history request.
type WithdrawHistoryResponse struct {
	Balance      decimal.Decimal    `json:"balance"`
	Transactions []TransactionEntry `json:"transactions"`
}

// A LitecoinTransaction holds information about a litecoin transaction.
type LitecoinTransaction struct {
	WalletAddress string   `json:"WalletAddress"`
	WithdrawType  int      `json:"WithdrawType"`
	JWT           JWTEntry `json:"jwt"`
}

// A JWTEntry holds signed(?) information about litecoin(?) transaction
// specifics.
type JWTEntry struct {
	Data                string          `json:"data"` // never seen, no idea what type
	BaseCurrency        string          `json:"baseCurrency"`
	BaseAmount          decimal.Decimal `json:"baseAmount"`
	WithholdingTax      decimal.Decimal `json:"withholdingTax"`
	WinminerFee         decimal.Decimal `json:"winminerFee"`
	ProviderFee         decimal.Decimal `json:"providerFee"`
	NetAmount           decimal.Decimal `json:"netAmount"`
	Exchange            decimal.Decimal `json:"exchange"`
	ProviderName        string          `json:"providerName"`
	FriendlyAmount      string          `json:"fAmount"`
	FriendlyWinminerFee string          `json:"fWinminerFee"`
	FriendlyProviderFee string          `json:"fProviderFee"`
	FriendlyNetAmount   string          `json:"fNetAmount"`
	// JWT stuff
	ExpirationTime float64 `json:"exp"`
	JWTID          string  `json:"jti"`
	IssuedAt       float64 `json:"iat"`
	Issuer         string  `json:"iss"`
}

// A TransactionEntry holds information about one withdrawal.
// After determining the type of the transaction, parse the TransactionData
// using e.g. ParseAsLitecoinTransaction.
type TransactionEntry struct {
	TransactionID           string `json:"transactionId"`
	IsCompleted             bool   `json:"isCompleted"`
	CompletedDate           string `json:"completedDate"` // never seen, but probably string
	RequestDate             string `json:"requestDate"`
	TransactionType         int    `json:"transactionType"`
	Status                  int    `json:"status"`
	TransactionData         string `json:"transactionData"`
	FriendlyStatus          string `json:"friendlyStatus"`
	FriendlyTransactionType string `json:"friendlyTransactionType"`
	Data                    string `json:"data"`
	FriendlyTotalAmount     string `json:"friendlyTotalAmount"`
	FriendlyNetAmount       string `json:"friendlyNetAmount"`
	FriendlyWinMinerFees    string `json:"friendlyWinMinerFees"`
	FriendlyProviderFees    string `json:"friendlyProviderFees"`
	ProviderName            string `json:"providerName"`
	ExternalTransactionID   string `json:"externalTransactionId"`
	IP                      string `json:"ip"`
}

// ParseDataAsLitecoinTransaction parses the TransactionData as a
// LitcoinTransaction.
func (e TransactionEntry) ParseDataAsLitecoinTransaction() (*LitecoinTransaction, error) {
	var t LitecoinTransaction
	err := json.Unmarshal([]byte(e.TransactionData), &t)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse as litecoin transaction")
	}

	return &t, nil
}

func (c *lowLevelClient) getWithdrawHistory() (*WithdrawHistoryResponse, error) {
	var resp WithdrawHistoryResponse

	err := c.do(http.MethodGet, true, withdrawHistoryURL, nil, nil, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get withdraw history")
	}

	return &resp, nil
}

// A WithdrawDataResponse is the response to a WithdrawData request.
type WithdrawDataResponse struct {
	AppleGiftCards  []GiftCardEntry `json:"appleGiftCards"`
	AmazonGiftCards []GiftCardEntry `json:"amazonGiftCards"`
	Fees            []FeeEntry      `json:"fees"`
	Exchange        ExchangeRates   `json:"exchange"`
	Balance         decimal.Decimal `json:"balance"`
}

// A WithdrawOption describes one withdraw option.
// Lots of the fields are just required for rendering, but kept here for
// completeness.
type WithdrawOption struct {
	Logo                             string          `json:"logo"`
	Description                      string          `json:"description"`
	DescriptionAdd                   string          `json:"descriptionAdd"`
	TemplateURL                      string          `json:"templateUrl"`
	Height                           int             `json:"height"`
	TypeID                           int             `json:"typeId"`
	Path                             string          `json:"path"`
	MinimumToWithdraw                decimal.Decimal `json:"minimumToWithdraw"`
	MaximumToWithdraw                decimal.Decimal `json:"maximumToWithdraw"`
	NoCheckout                       bool            `json:"noCheckout"`
	ConfirmMessage                   []string        `json:"confirmMessage"`
	ConfirmMessageTokenValueProperty string          `json:"confirmMessageTokenValueProperty"`
	Disabled                         bool            `json:"disabled"`
	Message                          string          `json:"message"` // never seen, no idea what type
	AllowHighFee                     bool            `json:"allowHighFee"`
}

// ExchangeRates holds information about exchange rates to crypto currencies.
type ExchangeRates struct {
	BTC decimal.Decimal `json:"btc"`
	ETH decimal.Decimal `json:"eth"`
	LTC decimal.Decimal `json:"ltc"`
}

// A FeeEntry holds information about fees applied on withdrawal.
type FeeEntry struct {
	Type             int             `json:"type"`
	ProviderLowFee   decimal.Decimal `json:"providerLowFee"`
	ProviderFee      decimal.Decimal `json:"providerFee"`
	ProviderHighFee  decimal.Decimal `json:"providerHighFee"`
	ProviderFixedFee bool            `json:"providerFixedFee"`
	WithholdingTax   decimal.Decimal `json:"withholdingTax"`
	WinMinerFee      decimal.Decimal `json:"winMinerFee"`
}

// A GiftCardEntry holds information about withdrawal to gift cards.
type GiftCardEntry struct {
	ID          int    `json:"id"`
	Country     string `json:"country"`
	LocalAmount int    `json:"localAmount"`
	Amount      int    `json:"amount"`
	Symbol      string `json:"symbol"`
}

func (c *lowLevelClient) getWithdrawData() (*WithdrawDataResponse, error) {
	var resp WithdrawDataResponse

	err := c.do(http.MethodGet, true, withdrawDataURL, nil, nil, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get withdraw data")
	}

	return &resp, nil
}

// An Auth2Request is used to authenticate for the Websocket API.
type Auth2Request struct {
	ClientType int    `json:"clientType"`
	LoginToken string `json:"loginToken"`
}

// An Auth2Response is the response to an Auth2Request.
type Auth2Response struct {
	Host  string `json:"host"`
	Token string `json:"token"`
}

func (c *lowLevelClient) auth2() (*Auth2Response, error) {
	c.userTokenLock.RLock()
	userToken := c.userToken
	c.userTokenLock.RUnlock()

	req := Auth2Request{
		ClientType: 200,
		LoginToken: userToken,
	}
	var resp Auth2Response

	err := c.do(http.MethodPost, true, hubAuth2URL, nil, req, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "unable to auth2")
	}

	return &resp, nil
}

// A MachinesResponse holds a bunch of MachineEntries.
type MachinesResponse []MachineEntry

// A MachineEntry holds information about one machine.
// This is used by both the HTTP and Websocket API.
type MachineEntry struct {
	MachineName   string        `json:"machineName"`
	SID           string        `json:"sid"`
	ClientVersion string        `json:"clientVersion"`
	IsAdmin       bool          `json:"isAdmin"`
	IsPortable    bool          `json:"isPortable"`
	Devices       []DeviceEntry `json:"devices"`
	Key           string        `json:"key"`
}

// A DeviceEntry holds information about one device.
// This is used for both the HTTP and the Websocket API.
type DeviceEntry struct {
	ID      string       `json:"id"` // probably string
	Enabled bool         `json:"enabled"`
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	Status  DeviceStatus `json:"status"`
}

// DeviceStatus is the status of one device.
// This is used for both the HTTP and the Websocket API.
type DeviceStatus struct {
	Status    int               `json:"status"`
	Tags      []string          `json:"tags"`
	Hashrates []decimal.Decimal `json:"hashrates"`
	Profits   []decimal.Decimal `json:"profits"`
	Currency  string            `json:"currency"`
	ExtraData string            `json:"extraData"` // never seen, no idea what type
}

func (c *lowLevelClient) getMachines() (*MachinesResponse, error) {
	var resp MachinesResponse

	err := c.do(http.MethodGet, true, machinesURL, nil, nil, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get machines")
	}

	return &resp, nil
}

// An ExchangeRequest holds the data necessary to get coin exchange information.
// Note that this is only used by the miner, not the website.
type ExchangeRequest struct {
	MiningToken  string `json:"MiningToken"`
	BalanceToken string `json:"BalanceToken"`
}

// An ExchangeResponse is the response to an ExchangeRequest.
type ExchangeResponse struct {
	UserBalance decimal.Decimal `json:"userBalance"`
}

func (c *lowLevelClient) getExchangeBalance(miningToken, balanceToken string) (*ExchangeResponse, error) {
	req := ExchangeRequest{
		MiningToken:  miningToken,
		BalanceToken: balanceToken,
	}
	var resp ExchangeResponse

	err := c.do(http.MethodPost, false, exchangeURL, nil, req, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get exchange balance")
	}

	return &resp, nil
}

// A StatsResponse is the response to a stats query.
type StatsResponse struct {
	Stats   []StatEntry     `json:"stats"`
	Balance decimal.Decimal `json:"balance"`
	Cache   decimal.Decimal `json:"cache"`
}

// A StatEntry is one entry with stats.
type StatEntry struct {
	ClientID  int             `json:"clientId"`
	Date      string          `json:"date"`
	Currency  string          `json:"currency"`
	MachineID string          `json:"machineId"`
	RewardUSD decimal.Decimal `json:"rewardUSD"`
	HashSec   int             `json:"hashSec"`
}

// ParseDate parses a date from the winminer string-encoding to a time.Time.
func ParseDate(date string) (time.Time, error) {
	return time.Parse(time.RFC3339, date)
}

func (c *lowLevelClient) getStats() (*StatsResponse, error) {
	var resp StatsResponse

	err := c.do(http.MethodGet, true, statsURL, nil, nil, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get stats")
	}

	return &resp, nil
}

// A LoginRequest holds the data used to log in.
type LoginRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	HubClientType int    `json:"hubClientType"`
}

// A LoginResponse is the response to a login request.
type LoginResponse struct {
	UserToken string `json:"userToken"`
	HubToken  string `json:"hubToken"`
	HubHost   string `json:"hubHost"`
}

func (c *lowLevelClient) postLogin(email, password string) (*LoginResponse, error) {
	req := LoginRequest{
		Email:         email,
		Password:      password,
		HubClientType: 200,
	}
	var resp LoginResponse

	err := c.do(http.MethodPost, false, loginURL, nil, req, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "unable to login")
	}

	c.userTokenLock.Lock()
	c.userToken = resp.UserToken
	c.userTokenLock.Unlock()

	return &resp, nil
}

func (c *lowLevelClient) do(method string, withAuth bool, url string, params url.Values, request, response interface{}) error {
	var body io.Reader
	if request != nil {
		b, err := json.Marshal(request)
		if err != nil {
			return errors.Wrap(err, "unable to encode request data")
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return errors.Wrap(err, "unable to construct request")
	}
	if withAuth {
		c.userTokenLock.RLock()
		userToken := c.userToken
		c.userTokenLock.RUnlock()
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", userToken))
	}
	if body != nil {
		req.Header.Set("Content-Type", jsonContentType)
	}
	if params != nil {
		req.URL.RawQuery = params.Encode()
	}

	if c.debug {
		log.WithFields(log.Fields{"method": method, "withAuth": withAuth, "url": url, "params": params, "request": request}).Debugln("performing request")
	}

	resp, err := c.c.Do(req)
	if err != nil {
		return errors.Wrap(err, "unable to perform request")
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "unable to read response body")
	}
	if c.debug {
		log.WithFields(log.Fields{"statusCode": resp.StatusCode, "status": resp.Status, "body": string(b)}).Debugln("got response")
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("server returned status %d: %s, body %s", resp.StatusCode, resp.Status, string(b))
	}

	err = json.Unmarshal(b, response)
	if err != nil {
		return errors.Wrapf(err, "unable to decode response (raw: %s)", string(b))
	}

	return nil
}
