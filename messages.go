package winminer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// Websocket method constants.
const (
	MethodSetSystemInfo   = "SetSystemInfo"   // A: Client ID, Machine SID, Machine Info
	MethodStatusChanged   = "StatusChanged"   // A: Machine SID, Device ID, Device Status
	MethodStateChanged    = "StateChanged"    // A: Machine SID, Device ID, Enabled (bool)
	MethodAppClosed       = "AppClosed"       // A: Machine SID, Client ID?
	MethodClientConnected = "ClientConnected" // A: Client ID?
	MethodRemoveMessage   = "RemoveMessage"   // A: Machine SID, Message
	MethodAddMessage      = "AddMessage"      // A: Machine SID, Message
	MethodMiningStarted   = "MiningStarted"   // A: Client ID?
	MethodMiningStopped   = "MiningStopped"   // A: Client ID?
)

func parseString(m json.RawMessage) (string, error) {
	bb, err := m.MarshalJSON()
	if err != nil {
		return "", errors.Wrap(err, "unable to marshal JSON")
	}

	var s string
	err = json.Unmarshal(bb, &s)
	if err != nil {
		return "", errors.Wrap(err, "unable to unmarshal as string")
	}

	return s, nil
}

func parseBool(m json.RawMessage) (bool, error) {
	bb, err := m.MarshalJSON()
	if err != nil {
		return false, errors.Wrap(err, "unable to marshal JSON")
	}

	var b bool
	err = json.Unmarshal(bb, &b)
	if err != nil {
		return false, errors.Wrap(err, "unable to unmarshal as bool")
	}

	return b, nil
}

func checkMethodAndArgCount(msg RawMessage, method string, argCount int) error {
	if msg.Method != method {
		return fmt.Errorf("not a %s message", method)
	}

	if len(msg.Arguments) != argCount {
		log.WithField("args", msg.Arguments).Errorf("%s message didn't have %d args", method, argCount)
		return fmt.Errorf("expected %d arguments, got %d", argCount, len(msg.Arguments))
	}

	return nil
}

type ClientConnectedMessage struct {
	ClientID string
}

func ParseClientConnectedMessage(message RawMessage) (*ClientConnectedMessage, error) {
	err := checkMethodAndArgCount(message, MethodClientConnected, 1)
	if err != nil {
		return nil, errors.Wrap(err, "invalid message")
	}

	clientID, err := parseString(message.Arguments[0])
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode")
	}

	return &ClientConnectedMessage{ClientID: clientID}, nil
}

// An AppClosedMessage holds the arguments of a MethodAppClosed call.
type AppClosedMessage struct {
	MachineSID string
	ClientID   string
}

// ParseAppClosedMessage parses a given RawMessage as an AppClosedMessage.
func ParseAppClosedMessage(message RawMessage) (*AppClosedMessage, error) {
	err := checkMethodAndArgCount(message, MethodAppClosed, 2)
	if err != nil {
		return nil, errors.Wrap(err, "invalid message")
	}

	machineSID, err := parseString(message.Arguments[0])
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode")
	}

	clientID, err := parseString(message.Arguments[1])
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode")
	}

	return &AppClosedMessage{MachineSID: machineSID, ClientID: clientID}, nil
}

// A StateChangedMessage holds the arguments of a MethodStateChanged call.
type StateChangedMessage struct {
	MachineSID string
	DeviceID   string
	Enabled    bool
}

// ParseStateChangedMessage parses a given RawMessage as a StateChangedMessage.
func ParseStateChangedMessage(message RawMessage) (*StateChangedMessage, error) {
	err := checkMethodAndArgCount(message, MethodStateChanged, 3)
	if err != nil {
		return nil, errors.Wrap(err, "invalid message")
	}

	machineSID, err := parseString(message.Arguments[0])
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode")
	}

	deviceID, err := parseString(message.Arguments[1])
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode")
	}

	enabled, err := parseBool(message.Arguments[2])
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode")
	}

	return &StateChangedMessage{
		MachineSID: machineSID,
		DeviceID:   deviceID,
		Enabled:    enabled,
	}, nil
}

// A StatusChangedMessage holds the arguments of a MethodStatusChanged call.
type StatusChangedMessage struct {
	MachineSID string
	DeviceID   string
	Status     DeviceStatus
}

// ParseStatusChangedMessage parses a StatusChanged message.
func ParseStatusChangedMessage(message RawMessage) (*StatusChangedMessage, error) {
	err := checkMethodAndArgCount(message, MethodStatusChanged, 3)
	if err != nil {
		return nil, errors.Wrap(err, "invalid message")
	}

	machineSID, err := parseString(message.Arguments[0])
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode")
	}

	deviceID, err := parseString(message.Arguments[1])
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode")
	}

	bb3, err := message.Arguments[2].MarshalJSON()
	if err != nil {
		return nil, errors.Wrap(err, "unable to marshal JSON")
	}

	var s DeviceStatus
	err = json.Unmarshal(bb3, &s)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode")
	}

	return &StatusChangedMessage{MachineSID: machineSID, DeviceID: deviceID, Status: s}, nil
}

// ParseSystemInfoMessage parses a SystemInfo message.
func ParseSystemInfoMessage(message RawMessage) ([]MachineEntry, error) {
	if message.Method != MethodSetSystemInfo {
		return nil, errors.New("not a SystemInfo message")
	}

	if len(message.Arguments) != 3 {
		log.WithField("args", message.Arguments).Errorln("SystemInfo message didn't have 3 args")
		return nil, errors.New("expected 3 arguments")
	}

	// Arg 1 is Client ID
	// Arg 2 is Machine SID
	bb, _ := message.Arguments[2].MarshalJSON()
	var m MachineEntry
	err := json.Unmarshal(bb, &m)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode")
	}

	return []MachineEntry{m}, nil
}
