package winminer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// Mining status constants.
const (
	StatusMining      = 8
	StatusStoppingToo = 10
	StatusStopping    = 0 // maybe
	StatusStarting1   = 2
	StatusStarting2   = 1
	StatusStarting3   = 5
	StatusStarting4   = 6
)

// A WebsocketClient is a client for the Winminer Live API.
type WebsocketClient struct {
	ws     *websocket.Conn
	wsLock sync.Mutex

	wg     sync.WaitGroup
	closed chan struct{}
	err    chan error

	debug bool
}

func newWebsocketClient(c *lowLevelClient) (*WebsocketClient, error) {
	nonce := time.Now().UnixNano() / 1000000
	client := WebsocketClient{
		closed: make(chan struct{}),
		err:    make(chan error),
	}

	auth2Resp, err := c.auth2()
	if err != nil {
		return nil, errors.Wrap(err, "unable to auth2")
	}
	hubBaseURL := auth2Resp.Host
	auth2Token := auth2Resp.Token

	negResp, err := c.negotiate(nonce, auth2Token, hubBaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "unable to negotiate")
	}
	connectionToken := negResp.ConnectionToken
	nonce++

	conn, err := c.connect(auth2Token, hubBaseURL, connectionToken)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect")
	}
	client.ws = conn

	err = c.start(nonce, auth2Token, hubBaseURL, connectionToken)
	if err != nil {
		return nil, errors.Wrap(err, "unable to start")
	}

	client.wg.Add(1)
	go func() {
		defer client.wg.Done()

		t := time.NewTicker(1 * time.Minute)

		for {
			select {
			case <-client.closed:
				t.Stop()
				return
			case <-t.C:
				nonce++
				err = c.ping(nonce, auth2Token, hubBaseURL)
				if err != nil {
					log.WithField("err", err).Errorln("unable to ping signalr")
					client.err <- errors.Wrap(err, "unable to ping signalr")
				}
			}
		}
	}()

	client.wg.Add(1)
	go func() {
		defer client.wg.Done()
		t := time.NewTicker(1 * time.Minute)
		currentNonce := 1

		for {
			select {
			case <-client.closed:
				t.Stop()
				return
			case <-t.C:
				client.wsLock.Lock()
				err := client.ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("{\"H\":\"reportinghub\",\"M\":\"KeepAlive\",\"A\":[],\"I\":%d}", currentNonce)))
				client.wsLock.Unlock()

				if err != nil {
					log.WithField("err", err).Errorln("unable to ping WSS")
					client.err <- errors.Wrap(err, "unable to ping WSS")
				}

				currentNonce++
			}
		}
	}()

	return &client, nil
}

func (c *WebsocketClient) close() {
	log.Debugln("websocket close() called")
	select {
	case <-c.closed:
		return
	default:
	}
	close(c.closed)
	c.wg.Wait()

	c.wsLock.Lock()
	c.ws.Close()
	c.wsLock.Unlock()

	close(c.err)
}

// A RawMessageContainer contains RawMessages from the Live API.
type RawMessageContainer struct {
	Channel  string       `json:"C"`
	Messages []RawMessage `json:"M"`
}

func (c RawMessageContainer) isInteresting() bool {
	split := strings.Split(c.Channel, ",")
	if len(split) != 5 {
		return false
	}

	id1, err := strconv.Atoi(split[2][:1])
	if err != nil {
		log.WithField("err", err).Warnln("unable to parse message ID 1")
		return false
	}

	id2, err := strconv.Atoi(split[3][:1])
	if err != nil {
		log.WithField("err", err).Warnln("unable to parse message ID 2")
		return false
	}

	return id1 == 2 && id2 == 2
}

// A RawMessage is an unparsed message from the Live API.
// Use the Method field to determine the type of the message and the parse it
// with ParseStatusChangedMessage or ParseSystemInfoMessage.
type RawMessage struct {
	Host      string            `json:"H"`
	Method    string            `json:"M"`
	Arguments []json.RawMessage `json:"A"`
}

func parseMessage(b []byte) (*RawMessageContainer, error) {
	if len(b) < 10 {
		return nil, errors.New("message too short")
	}

	var r RawMessageContainer
	err := json.Unmarshal(b, &r)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse message")
	}

	return &r, nil
}

func isInterestingChannel(b []byte) bool {
	if len(b) < 10 {
		return false
	}

	var r RawMessageContainer
	err := json.Unmarshal(b, &r)
	if err != nil {
		log.WithField("err", err).Warnln("unable to parse message")
		return false
	}

	return r.isInteresting()
}

// Read reads a message off the websocket.
// Use ReadNextInterestingMessage instead.
//
// This method returns all kinds of errors that concurrently occurred since the
// last call to Read.
// If it does return an error, close and re-open the websocket connection.
func (c *WebsocketClient) Read() (messageType int, b []byte, err error) {
	select {
	case <-c.closed:
		return 0, nil, errors.New("ws closed")
	case err := <-c.err:
		return 0, nil, errors.Wrap(err, "connection broken")
	default:
	}

	c.wsLock.Lock()
	messageType, b, err = c.ws.ReadMessage()
	c.wsLock.Unlock()

	if c.debug {
		log.WithFields(log.Fields{"messageType": messageType, "b": string(b), "err": err}).Debugln("websocket read")
	}

	return
}

// ReadNextInterestingMessages reads messages off the websocket until an
// interesting message comes by.
func (c *WebsocketClient) ReadNextInterestingMessages() (*RawMessageContainer, error) {
	for {
		mType, b, err := c.Read()
		if err != nil {
			return nil, errors.Wrap(err, "read failed")
		}
		if mType != websocket.TextMessage {
			continue
		}
		if !isInterestingChannel(b) {
			continue
		}

		parsed, err := parseMessage(b)
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse message")
		}

		return parsed, nil
	}
}
