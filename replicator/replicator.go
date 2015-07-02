package replicator

import (
	"fmt"
	"strings"
	"time"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/unit"
	"github.com/juju/errgo"
)

const (
	fleetStateLaunched = "launched"
	fleetStateLoaded   = "loaded"
)

var maskAny = errgo.MaskFunc(errgo.Any)
var mask = errgo.MaskFunc()

type Config struct {
	TickerTime         time.Duration
	DeleteTime         time.Duration
	UpdateCooldownTime time.Duration

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

	ticker         *time.Ticker
	stats          Stats
	undesiredState map[string]time.Time
	lastUpdate     *time.Time
}

func New(cfg Config, deps Dependencies) *Service {
	return &Service{
		Config:         cfg,
		Dependencies:   deps,
		stats:          Stats{},
		undesiredState: map[string]time.Time{},
		lastUpdate:     nil,
		ticker:         nil,
	}
}

type Unit struct {
	Name      string
	MachineID string
}

func (srv *Service) Run() {
	srv.ticker = time.NewTicker(srv.TickerTime)

	if err := srv.Reconcile(); err != nil {
		fmt.Printf("ERROR: %v\n", err)
	}

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

	// Now identify what needs to be done
	newDesiredUnits, activeUnits, undesiredUnits := srv.Diff(desiredUnits, managedUnits)

	for _, newUnit := range newDesiredUnits {
		if err := srv.createNewFleetUnit(newUnit); err != nil {
			return maskAny(err)
		}
	}

	if err := srv.checkActiveUnitsForTemplateUpdate(activeUnits); err != nil {
		return maskAny(err)
	}

	if err := srv.updateUndesiredState(desiredUnits, undesiredUnits); err != nil {
		return maskAny(err)
	}

	return nil
}

func (srv *Service) unitToOptions(desiredUnit Unit) ([]*schema.UnitOption, error) {
	unitContent := srv.UnitTemplate +
		`
		[X-Fleet]
		MachineID=` + desiredUnit.MachineID + `
		`

	uf, err := unit.NewUnitFile(unitContent)
	if err != nil {
		return nil, mask(err)
	}
	return schema.MapUnitFileToSchemaUnitOptions(uf), nil
}

func (srv *Service) createNewFleetUnit(desiredUnit Unit) error {
	options, err := srv.unitToOptions(desiredUnit)
	if err != nil {
		return maskAny(err)
	}

	fleetUnit := schema.Unit{
		Name:         desiredUnit.Name,
		Options:      options,
		DesiredState: fleetStateLaunched,
	}
	if err := srv.Fleet.CreateUnit(&fleetUnit); err != nil {
		return maskAny(err)
	}

	fmt.Println("Waiting for " + desiredUnit.Name + " to come up.")
	if err := waitForActiveUnit(srv.Fleet, desiredUnit.Name); err != nil {
		return maskAny(err)
	}
	return nil
}

func (srv *Service) checkActiveUnitsForTemplateUpdate(units []Unit) error {
	for _, unit := range units {
		desiredOptions, err := srv.unitToOptions(unit)
		if err != nil {
			return maskAny(err)
		}
		fleetUnit, err := srv.Fleet.Unit(unit.Name)
		if err != nil {
			return maskAny(err)
		}

		if unitOptionsEqual(desiredOptions, fleetUnit.Options) {
			srv.stats.MarkActiveUnitNoUpdateRequired(unit)
		} else {
			srv.stats.MarkActiveUnitUpdateRequired(unit)

			if err := srv.updateUnit(unit, desiredOptions); err != nil {
				return maskAny(err)
			}
		}
	}
	return nil
}

func (srv *Service) updateUnit(unit Unit, options []*schema.UnitOption) error {
	if srv.lastUpdate != nil && srv.lastUpdate.After(time.Now().Add(-srv.UpdateCooldownTime)) {
		fmt.Println("Ignoring update due to cooldown time.")
		srv.stats.UpdateUnitIgnoredCooldown(unit)
		return nil
	}

	if err := srv.destroyUnit(unit); err != nil {
		return maskAny(err)
	}

	t := time.Now()
	srv.lastUpdate = &t

	if err := srv.createNewFleetUnit(unit); err != nil {
		return maskAny(err)
	}

	return nil
}

func (srv *Service) updateUndesiredState(desiredUnits, undesiredUnits []Unit) error {
	for _, du := range desiredUnits {
		if _, ok := srv.undesiredState[du.Name]; ok {
			srv.stats.MarkUndesiredUnitBackToDesired(du)
			delete(srv.undesiredState, du.Name)
		}
	}

	for _, udu := range undesiredUnits {
		firstUndesired, ok := srv.undesiredState[udu.Name]

		if !ok {
			srv.stats.MarkNewUndesiredUnit(udu)
			srv.undesiredState[udu.Name] = time.Now()
		} else {
			if firstUndesired.Before(time.Now().Add(-srv.DeleteTime)) {
				srv.stats.DeleteUndesiredUnit(udu)
				if err := srv.destroyUnit(udu); err != nil {
					return maskAny(err)
				}
				delete(srv.undesiredState, udu.Name)
			}
		}
	}
	return nil
}

func (srv *Service) destroyUnit(unit Unit) error {

	if err := srv.Fleet.SetUnitTargetState(unit.Name, fleetStateLoaded); err != nil {
		return maskAny(err)
	}

	fmt.Println("Waiting for " + unit.Name + " to be stopped.")
	if err := waitForDeadUnit(srv.Fleet, unit.Name); err != nil {
		return maskAny(err)
	}

	if err := srv.Fleet.DestroyUnit(unit.Name); err != nil {
		return maskAny(err)
	}
	return nil
}

func (srv *Service) Diff(desiredUnits, managedUnits []Unit) (newDesiredUnits, activeUnits, undesiredUnits []Unit) {
	for _, i := range desiredUnits {
		found := false
		for _, j := range managedUnits {
			if i.Name == j.Name {
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
func (srv *Service) transformMachinesToDesiredUnits(machines []string) []Unit {
	desiredUnits := []Unit{}

	for _, m := range machines {
		unit := fmt.Sprintf("%s-%s.service", srv.UnitPrefix, m)
		desiredUnits = append(desiredUnits, Unit{
			Name:      unit,
			MachineID: m,
		})
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
		machines = append(machines, m.ID)
	}
	srv.stats.SeenMachinesActive(len(machines))
	return machines, nil
}

func (srv *Service) getManagedFleetUnits() ([]Unit, error) {
	units, err := srv.Fleet.Units()
	if err != nil {
		return nil, maskAny(err)
	}

	managedUnits := []Unit{}
	for _, u := range units {
		if !strings.HasPrefix(u.Name, srv.UnitPrefix) {
			continue
		}
		managedUnits = append(managedUnits, Unit{
			Name:      u.Name,
			MachineID: u.MachineID,
		})
	}
	srv.stats.SeenManagedUnits(len(managedUnits))
	return managedUnits, nil
}
