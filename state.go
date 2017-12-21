package winminer

import (
	"sync"
	"time"

	"github.com/pkg/errors"
)

// LiveState is a helper struct to keep track of Live API updates.
type LiveState struct {
	machines           []MachineEntry
	devicesLastUpdated map[string]time.Time
	l                  sync.Mutex
}

// NewLiveState returns a new LiveState.
func NewLiveState() *LiveState {
	return &LiveState{
		devicesLastUpdated: make(map[string]time.Time),
	}
}

// SetSystemInfo clears the current state and sets it to the state received.
// The machine entries are kept as a reference, do not modify them later on.
func (s *LiveState) SetSystemInfo(entries []MachineEntry) {
	s.l.Lock()
	defer s.l.Unlock()

	s.machines = entries
	s.devicesLastUpdated = make(map[string]time.Time)
}

// AddMachine adds a machine entry if it's not present already.
// If it is, the entry is overwritten.
func (s *LiveState) AddMachine(entry MachineEntry) {
	s.l.Lock()
	defer s.l.Unlock()
	for i, m := range s.machines {
		if m.SID == entry.SID {
			s.machines[i] = entry
			return
		}
	}

	s.machines = append(s.machines, entry)
}

// Update updates the state with the given status change.
// Returns an error if the device or machine was not found.
// If that happens, the state got out of sync somehow.
// Best close and re-open the websocket connection and rebuild the state.
func (s *LiveState) Update(container StatusChangeContainer) error {
	s.l.Lock()
	defer s.l.Unlock()
	for i, m := range s.machines {
		if m.SID == container.MachineSID {
			for j, d := range m.Devices {
				if d.ID == container.DeviceID {
					d.Status = container.Status

					m.Devices[j] = d
					s.machines[i] = m
					s.devicesLastUpdated[container.DeviceID] = time.Now()

					return nil
				}
			}
			return errors.New("device not found")
		}
	}
	return errors.New("machine not found")
}
