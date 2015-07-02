package replicator

import (
	"fmt"
	"strings"
	"time"

	"github.com/coreos/fleet/client"
	"github.com/juju/errgo"
)

var maskAny = errgo.MaskFunc(errgo.Any)
var mask = errgo.MaskFunc()

type Config struct {
	TickerTime time.Duration
	DeleteTime time.Duration

	MachineTag   string
	UnitPrefix   string
	UnitTemplate string
}

type Dependencies struct {
	Fleet client.API
}

type Service struct {
	Config
	Dependencies

	ticker *time.Ticker
	stats  Stats
}

func New(cfg Config, deps Dependencies) *Service {
	return &Service{
		Config:       cfg,
		Dependencies: deps,
		stats:        Stats{},
	}
}

func (srv *Service) Run() {
	srv.ticker = time.NewTicker(srv.TickerTime)

	for range srv.ticker.C {
		fmt.Println("*tick*")

		if err := srv.Reconcile(); err != nil {
			fmt.Printf("ERROR: %v\n", err)
		}
	}
}

func (srv *Service) Reconcile() error {
	// Get machines and transform them to the desired-units
	machines, err := srv.getMachines()
	if err != nil {
		return maskAny(err)
	}

	if len(machines) == 0 {
		return errgo.Newf("No machines found to act on. Skipping reconcile.")
	}

	desiredUnits := srv.transformMachinesToDesiredUnits(machines)

	// Get managed fleet units
	managedUnits, err := srv.getManagedFleetUnits()
	if err != nil {
		return maskAny(err)
	}

	newDesiredUnits, activeUnits, undesiredUnits := srv.Diff(desiredUnits, managedUnits)

	return nil

}

func (srv *Service) Diff(desiredUnits, managedUnits []string) (newDesiredUnits, activeUnits, undesiredUnits []string) {

	for _, i := range desiredUnits {
		found := false
		for _, j := range managedUnits {
			if i == j {
				found = true
				break
			}
		}

		if found {
			activeUnits = append(activeUnits, i)
		} else {
			newDesiredUnits = append(newDesiredUnits, i)
		}
	}

	for _, i := range managedUnits {
		found := false
		for _, j := range desiredUnits {
			if i == j {
				found = true
				break
			}
		}

		// NOTE: No else, since the found case is already handled above in the first loop
		if !found {
			undesiredUnits = append(undesiredUnits, i)
		}
	}

	return
}
func (srv *Service) transformMachinesToDesiredUnits(machines []string) []string {
	desiredUnits := []string{}

	for m := range machines {
		unit := fmt.Sprintf("%s-%s.service", srv.UnitPrefix, m)
		desiredUnits = append(desiredUnits, unit)
	}
	return desiredUnits
}

func (srv *Service) getMachines() ([]string, error) {
	fleetMachines, err := srv.Fleet.Machines()
	if err != nil {
		return nil, maskAny(err)
	}
	srv.stats.SeenMachinesTotal(len(fleetMachines))

	machines := []string{}
	for _, m := range fleetMachines {
		// NOTE: At GiantSwarm we are only interested on the left side of the tag. The right is always "true" for us.
		if _, ok := m.Metadata[srv.MachineTag]; !ok {
			continue
		}
		machines = append(machines, m.ShortID())
	}
	srv.stats.SeenMachinesActive(len(machines))
	return machines, nil
}

func (srv *Service) getManagedFleetUnits() ([]string, error) {
	units, err := srv.Fleet.Units()
	if err != nil {
		return nil, maskAny(err)
	}

	managedUnits := []string{}
	for _, u := range units {
		if !strings.HasPrefix(u.Name, srv.UnitPrefix) {
			continue
		}
		managedUnits = append(managedUnits, u.Name)
	}
	srv.stats.SeenManagedUnits(len(managedUnits))
	return managedUnits, nil
}
