package winminer

import (
	"sync"
	"time"

	"github.com/pkg/errors"
)

// LiveState is a helper struct to keep track of Live API updates.
// Access must be protected with the embedded mutex.
type LiveState struct {
	Machines           []MachineEntry
	DevicesLastUpdated map[string]time.Time
	sync.Mutex
}

// NewLiveState returns a new LiveState.
func NewLiveState() *LiveState {
	return &LiveState{
		DevicesLastUpdated: make(map[string]time.Time),
	}
}

// SetSystemInfo clears the current state and sets it to the state received.
// The machine entries are kept as a reference, do not modify them later on.
func (s *LiveState) SetSystemInfo(entries []MachineEntry) {
	s.Lock()
	defer s.Unlock()

	s.Machines = entries
	s.DevicesLastUpdated = make(map[string]time.Time)
}

// AddMachine adds a machine entry if it's not present already.
// If it is, the entry is overwritten.
func (s *LiveState) AddMachine(entry MachineEntry) {
	s.Lock()
	defer s.Unlock()
	for i, m := range s.Machines {
		if m.SID == entry.SID {
			s.Machines[i] = entry
			return
		}
	}

	s.Machines = append(s.Machines, entry)
}

// Update updates the state with the given status change.
// Returns an error if the device or machine was not found.
// If that happens, the state got out of sync somehow.
// Best close and re-open the websocket connection and rebuild the state.
func (s *LiveState) Update(container StatusChangeContainer) error {
	s.Lock()
	defer s.Unlock()
	for i, m := range s.Machines {
		if m.SID == container.MachineSID {
			for j, d := range m.Devices {
				if d.ID == container.DeviceID {
					d.Status = container.Status

					m.Devices[j] = d
					s.Machines[i] = m
					s.DevicesLastUpdated[container.DeviceID] = time.Now()

					return nil
				}
			}
			return errors.New("device not found")
		}
	}
	return errors.New("machine not found")
}
