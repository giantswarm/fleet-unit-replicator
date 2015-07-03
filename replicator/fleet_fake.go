package replicator

import (
	"fmt"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/schema"
	"github.com/golang/glog"
)

const (
	fleetStateLaunched = "launched"
	fleetStateLoaded   = "loaded"
)

type FleetOperator interface {
	CreateUnit(unit string, options []*schema.UnitOption) error
	DestroyUnit(unit string) error
}

type FleetRWOperator struct {
	API client.API
}

func (ff *FleetRWOperator) CreateUnit(unit string, options []*schema.UnitOption) error {
	fleetUnit := schema.Unit{
		Name:         unit,
		Options:      options,
		DesiredState: fleetStateLaunched,
	}
	if err := ff.API.CreateUnit(&fleetUnit); err != nil {
		return maskAny(err)
	}

	glog.Infof("Waiting for %s to come up.", unit)
	if err := waitForActiveUnit(ff.API, unit); err != nil {
		return maskAny(err)
	}

	return nil
}
func (ff *FleetRWOperator) DestroyUnit(unit string) error {

	if err := ff.API.SetUnitTargetState(unit, fleetStateLoaded); err != nil {
		return maskAny(err)
	}

	glog.Infof("Waiting for %s to be stopped.", unit)
	if err := waitForDeadUnit(ff.API, unit); err != nil {
		return maskAny(err)
	}

	if err := ff.API.DestroyUnit(unit); err != nil {
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
