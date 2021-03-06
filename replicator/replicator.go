package replicator

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/unit"
	"github.com/giantswarm/metrics"
	"github.com/golang/glog"
	"github.com/juju/errgo"
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
	Fleet    client.API
	Operator FleetOperator
	Metrics  metrics.Collector
}

type Service struct {
	Config
	Dependencies

	ticker            *time.Ticker
	stats             Stats
	undesiredState    map[string]time.Time
	resetCooldowntime chan struct{}
	lastUpdate        *ExpiringBool
	shutdownWG        sync.WaitGroup
}

func New(cfg Config, deps Dependencies) *Service {
	return &Service{
		Config:            cfg,
		Dependencies:      deps,
		stats:             Stats{deps.Metrics},
		undesiredState:    map[string]time.Time{},
		resetCooldowntime: make(chan struct{}),
		lastUpdate:        NewExpiringBool(cfg.UpdateCooldownTime),
		ticker:            nil,
	}
}

type Unit struct {
	Name      string
	MachineID string
}

func (srv *Service) Stop() {
	srv.ticker.Stop()
	srv.shutdownWG.Wait()
}

func (srv *Service) Serve() {
	srv.ticker = time.NewTicker(srv.TickerTime)

	r := func() error {
		glog.Info("*tick*")
		glog.Infof("Time until lastUpdate resets: %s", srv.lastUpdate.RemainingTime())
		srv.shutdownWG.Add(1)
		defer func() {
			srv.shutdownWG.Done()
		}()

		return srv.Reconcile()
	}

	if err := r(); err != nil {
		glog.Fatalf("%v", err)
	}

	for {
		select {
		case <-srv.ticker.C:
			if err := r(); err != nil {
				glog.Fatalf("%v", err)
			}
		case <-srv.resetCooldowntime:
			glog.Infof("Resetting cooldown time. Was at %s", srv.lastUpdate.RemainingTime())
			srv.lastUpdate.SetFalse()
		}
	}
}

func (srv *Service) ResetCooldowntime() {
	srv.resetCooldowntime <- struct{}{}
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
	newDesiredUnits, activeUnits, undesiredUnits := diffUnits(desiredUnits, managedUnits)

	srv.stats.DesiredUnitsGauge(len(newDesiredUnits))
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
	uf, err := unit.NewUnitFile(srv.UnitTemplate)
	if err != nil {
		return nil, mask(err)
	}
	options := schema.MapUnitFileToSchemaUnitOptions(uf)
	options = append(options, &schema.UnitOption{
		Section: "X-Fleet",
		Name:    "MachineID",
		Value:   desiredUnit.MachineID,
	})
	return options, nil
}

func (srv *Service) createNewFleetUnit(desiredUnit Unit) error {
	options, err := srv.unitToOptions(desiredUnit)
	if err != nil {
		return maskAny(err)
	}

	if err := srv.Operator.CreateUnit(desiredUnit.Name, options); err != nil {
		return maskAny(err)
	}

	return nil
}

func (srv *Service) checkActiveUnitsForTemplateUpdate(units []Unit) error {

	var (
		noUpdateRequired int64 = 0
		updateRequired   int64 = 0
	)

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
			noUpdateRequired++
		} else {
			updateRequired++

			if err := srv.updateUnit(unit, desiredOptions); err != nil {
				return maskAny(err)
			}
		}
	}

	srv.stats.ActiveUnitsSeen(len(units))
	srv.stats.ActiveUnitsUpdateRequired(updateRequired)
	srv.stats.ActiveUnitsNoUpdateRequired(noUpdateRequired)
	return nil
}

func (srv *Service) updateUnit(unit Unit, options []*schema.UnitOption) error {
	if srv.lastUpdate.State() == true {
		glog.Info("Ignoring update due to cooldown time.")
		srv.stats.UpdateUnitIgnoredCooldown(unit)
		return nil
	}

	if err := srv.destroyUnit(unit); err != nil {
		return maskAny(err)
	}
	// We suspect we might need to wait here for the unit to actually be deleted.
	// Until we implement that, we just sleep as a workaround.
	time.Sleep(15 * time.Second)

	srv.lastUpdate.SetTrue()

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

	srv.stats.UndesiredUnitsGauge(len(srv.undesiredState))

	return nil
}

func (srv *Service) destroyUnit(unit Unit) error {
	if err := srv.Operator.DestroyUnit(unit.Name); err != nil {
		return maskAny(err)
	}
	return nil
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

	// Build Machinelist
	machines := []string{}

	for _, m := range fleetMachines {
		if srv.MachineTag == "" {
			goto done
		}

		// NOTE: At GiantSwarm we are only interested on the left side of the tag. The right is always "true" for us.
		if _, ok := m.Metadata[srv.MachineTag]; !ok {
			continue
		}
	done:
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

	srv.stats.SeenUnitsTotal(len(units))
	srv.stats.SeenUnitsManaged(len(managedUnits))
	return managedUnits, nil
}

func diffUnits(desiredUnits, managedUnits []Unit) (newDesiredUnits, activeUnits, undesiredUnits []Unit) {
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

func NewExpiringBool(cooldown time.Duration) *ExpiringBool {
	return &ExpiringBool{
		CooldownTime: cooldown,
		LastTouch:    nil,
		now:          time.Now,
	}
}

// ExpiringBool represents kindof a boolean that keeps its 'true' state only for a certain
// amount of time.
type ExpiringBool struct {
	CooldownTime time.Duration
	LastTouch    *time.Time
	now          func() time.Time
}

func (c *ExpiringBool) State() bool {
	if c.LastTouch != nil && c.LastTouch.After(c.now().Add(-c.CooldownTime)) {
		return true
	}
	return false
}

// Touch updates the internal timestamp, so State() returns true until CooldownTime is over
func (c *ExpiringBool) SetTrue() {
	t := c.now()
	c.LastTouch = &t
}

// SetFalse resets the internal state.
func (c *ExpiringBool) SetFalse() {
	c.LastTouch = nil
}

func (c *ExpiringBool) RemainingTime() time.Duration {
	if c.LastTouch == nil {
		return 0
	}
	return c.LastTouch.Add(c.CooldownTime).Sub(c.now())
}
