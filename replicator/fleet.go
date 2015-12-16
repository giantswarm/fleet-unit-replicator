package replicator

import (
	"time"

	"github.com/coreos/fleet/schema"
)

type StatesFun func() ([]*schema.UnitState, error)

func waitForSystemdState(statesFun StatesFun, unit string, allowedStates []string) error {
	seenDesiredState := 0
	for {
		states, err := statesFun()
		if err != nil {
			return maskAny(err)
		}

		found := false
		seenDesired := false
		for _, state := range states {
			if state.Name == unit {
				found = true
				for _, allowedState := range allowedStates {
					if allowedState == state.SystemdActiveState {
						seenDesired = true
					}
				}
			}
		}

		if found && !seenDesired {
			seenDesiredState = 0
		} else {
			seenDesiredState++
		}

		if seenDesiredState > 5 {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func waitForDeadUnit(statesFun StatesFun, unit string) error {
	return waitForSystemdState(statesFun, unit, []string{"failed", "dead", "inactive"})
}

func waitForActiveUnit(statesFun StatesFun, unit string) error {
	return waitForSystemdState(statesFun, unit, []string{"active"})
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
