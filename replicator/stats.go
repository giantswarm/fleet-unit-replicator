package replicator

import "github.com/golang/glog"

func logStatsCounter(s string, delta int) {
	if delta >= 0 {
		glog.Infof("[STATS] %s: +%d\n", s, delta)
	} else {
		glog.Infof("[STATS] %s: %d\n", s, delta)
	}

}
func logStatsGauge(s string, n int) {
	glog.Infof("[STATS] %s: %d\n", s, n)
}

type Stats struct{}

func (stats *Stats) SeenMachinesTotal(count int) {
	logStatsGauge("seenMachinesTotal", count)
}
func (stats *Stats) SeenMachinesActive(count int) {
	logStatsGauge("seenMachinesActive", count)
}

func (stats *Stats) SeenManagedUnits(count int) {
	logStatsGauge("ManagedFleetUnits", count)
}

func (stats *Stats) MarkNewUndesiredUnit(unit Unit) {
	logStatsCounter("MarkNewUndesiredUnit", 1)
}
func (stats *Stats) MarkUndesiredUnitBackToDesired(unit Unit) {
	logStatsCounter("MarkUndesiredUnitBackToDesired", 1)
}
func (stats *Stats) DeleteUndesiredUnit(unit Unit) {
	logStatsCounter("DeleteUnit", 1)
}
func (stats *Stats) UndesiredUnitsGauge(g int) {
	logStatsGauge("UndesiredUnits", g)
}

func (stats *Stats) MarkActiveUnitNoUpdateRequired(unit Unit) {
	logStatsCounter("MarkActiveUnitNoUpdateRequired", 1)
}

func (stats *Stats) MarkActiveUnitUpdateRequired(unit Unit) {
	logStatsCounter("MarkActiveUnitUpdateRequired", 1)
}

func (stats *Stats) UpdateUnitIgnoredCooldown(unit Unit) {
	logStatsCounter("UpdateUnitIgnoredCooldown", 1)
}
