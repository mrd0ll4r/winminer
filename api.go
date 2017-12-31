package winminer

import (
	"net/http"
	"sync"

	"github.com/pkg/errors"
)

// An APIClient is a client for the WinMiner API.
type APIClient struct {
	c *lowLevelClient

	ws     *WebsocketClient
	wsLock sync.Mutex
}

// NewAPIClient constructs a new API client and attempts to log in.
func NewAPIClient(email, password string, debug bool) (*APIClient, error) {
	c := &lowLevelClient{
		c:             &http.Client{},
		debug:         debug,
		userTokenLock: sync.RWMutex{},
	}

	_, err := c.postLogin(email, password)
	if err != nil {
		return nil, errors.Wrap(err, "unable to login")
	}

	return &APIClient{
		c: c,
	}, nil
}

func (c *APIClient) connectWebsocket() (*WebsocketClient, error) {
	if c.ws != nil {
		return c.ws, nil
	}

	ws, err := newWebsocketClient(c.c)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect websocket")
	}

	ws.debug = c.c.debug

	c.ws = ws
	return ws, nil
}

// ConnectWebsocket connects a websocket connection for the Live API.
// Note that, if there is already a connection established, that connection
// will be returned instead.
// Close the connection with CloseWebsocket.
func (c *APIClient) ConnectWebsocket() (*WebsocketClient, error) {
	c.wsLock.Lock()
	defer c.wsLock.Unlock()

	return c.connectWebsocket()
}

func (c *APIClient) closeWebsocket() error {
	if c.ws == nil {
		return errors.New("websocket not connected")
	}

	c.ws.close()
	c.ws = nil
	return nil
}

// CloseWebsocket closes the websocket connection.
func (c *APIClient) CloseWebsocket() error {
	c.wsLock.Lock()
	defer c.wsLock.Unlock()

	return c.closeWebsocket()
}

// ReconnectWebsocket closes and re-opens the websocket connection.
// Use this in case of any errors with the websocket connection.
func (c *APIClient) ReconnectWebsocket() (*WebsocketClient, error) {
	c.wsLock.Lock()
	defer c.wsLock.Unlock()

	c.closeWebsocket() // ignore the "not connected" error

	return c.connectWebsocket()
}

// GetWithdrawHistory retrieves the withdraw history.
func (c *APIClient) GetWithdrawHistory() (*WithdrawHistoryResponse, error) {
	return c.c.getWithdrawHistory()
}

// GetWithdrawData retrieves information about current withdraw options.
func (c *APIClient) GetWithdrawData() (*WithdrawDataResponse, error) {
	return c.c.getWithdrawData()
}

// GetMachines gets information about current machines.
// Please note that the live information contained in this can not be trusted,
// e.g. the Enabled field will always be set to true, even if a device is not
// actually enabled.
func (c *APIClient) GetMachines() (*MachinesResponse, error) {
	return c.c.getMachines()
}

// GetStats returns historical statistics.
func (c *APIClient) GetStats() (*StatsResponse, error) {
	return c.c.getStats()
}
