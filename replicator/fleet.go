package replicator

import (
	"time"

	"github.com/coreos/fleet/client"
)

func waitForSystemdActiveState(client client.API, unit string, allowedStates []string) error {
	seenDesiredState := 0
	for {
		states, err := client.UnitStates()
		if err != nil {
			return maskAny(err)
		}

		found := false
		for _, state := range states {
			if state.Name == unit {
				found = true
				for _, allowedState := range allowedStates {
					if allowedState == state.SystemdActiveState {
						seenDesiredState++
					}
				}
			}
		}

		if !found {
			seenDesiredState++
		}

		if seenDesiredState > 5 {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func waitForDeadUnit(client client.API, unit string) error {
	return waitForSystemdActiveState(client, unit, []string{"failed", "dead", "inactive"})
}

func waitForActiveUnit(client client.API, unit string) error {
	return waitForSystemdActiveState(client, unit, []string{"active"})
}

func unitOptionsEqual(left, right []*schema.UnitOption) bool {
	if len(left) != len(right) {
		return false
	}

	for _, i := range left {
		found := false
		for _, j := range right {
			if i.Name == j.Name && i.Section == j.Section && i.Value == j.Value {
				found = true
			}
		}

		if !found {
			return false
		}
	}
	return true
}
