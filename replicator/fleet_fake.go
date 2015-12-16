package replicator

import (
	"time"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/schema"
	"github.com/giantswarm/retry-go"
	"github.com/golang/glog"
)

const (
	fleetStateLaunched = "launched"
	fleetStateLoaded   = "loaded"
)

var (
	writeRetryOptions = []retry.RetryOption{
		retry.Sleep(500 * time.Millisecond),
		retry.MaxTries(10),
	}
	readRetryOptions = []retry.RetryOption{
		retry.Sleep(500 * time.Millisecond),
		retry.MaxTries(10),
	}
)

type FleetOperator interface {
	CreateUnit(unit string, options []*schema.UnitOption) error
	DestroyUnit(unit string) error
}

type FleetRWOperator struct {
	API client.API
}

func (ff *FleetRWOperator) fetchUnitStates() ([]*schema.UnitState, error) {
	var states []*schema.UnitState
	err := retry.Do(func() (err error) {
		states, err = ff.API.UnitStates()
		return maskAny(err)
	}, readRetryOptions...)
	return states, maskAny(err)
}

func (ff *FleetRWOperator) CreateUnit(unit string, options []*schema.UnitOption) error {
	fleetUnit := schema.Unit{
		Name:         unit,
		Options:      options,
		DesiredState: fleetStateLaunched,
	}

	err := retry.Do(func() error {
		return ff.API.CreateUnit(&fleetUnit)
	}, writeRetryOptions...)
	if err != nil {
		return maskAny(err)
	}

	glog.Infof("Waiting for %s to come up.", unit)

	err = retry.Do(func() error {
		return waitForActiveUnit(ff.fetchUnitStates, unit)
	}, writeRetryOptions...)
	if err != nil {
		return maskAny(err)
	}

	return nil
}
func (ff *FleetRWOperator) DestroyUnit(unit string) error {
	err := retry.Do(func() error {
		return ff.API.SetUnitTargetState(unit, fleetStateLoaded)
	}, writeRetryOptions...)
	if err != nil {
		return maskAny(err)
	}

	glog.Infof("Waiting for %s to be stopped.", unit)
	if err := waitForDeadUnit(ff.fetchUnitStates, unit); err != nil {
		return maskAny(err)
	}

	err = retry.Do(func() error {
		return ff.API.DestroyUnit(unit)
	}, writeRetryOptions...)
	if err != nil {
		return maskAny(err)
	}

	return nil
}

type FleetROOperator struct {
	client.API
}

func (ff *FleetROOperator) CreateUnit(unit string, options []*schema.UnitOption) error {
	glog.Infof("Fleet::CreateUnit(%s, %v) ignored - DryRun\n", unit, options)
	return nil
}
func (ff *FleetROOperator) DestroyUnit(unit string) error {
	glog.Infof("Fleet::DestroyUnit(%s) ignored - DryRun\n", unit)
	return nil
}
