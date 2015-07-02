package replicator

import "fmt"

func logStats(s string, n int) {
	fmt.Printf("[STATS] %s = %d\n", s, n)
}

type Stats struct{}

func (stats *Stats) SeenMachinesTotal(count int) {
	logStats("seenMachinesTotal", count)
}
func (stats *Stats) SeenMachinesActive(count int) {
	logStats("seenMachinesActive", count)
}

func (stats *Stats) SeenManagedUnits(count int) {
	logStats("ManagedFleetUnits", count)
}

func (stats *Stats) MarkNewUndesiredUnit(unit Unit) {
	logStats("MarkNewUndesiredUnit", 1)
}
func (stats *Stats) MarkUndesiredUnitBackToDesired(unit Unit) {
	logStats("MarkUndesiredUnitBackToDesired", 1)
}
func (stats *Stats) DeleteUndesiredUnit(unit Unit) {
	logStats("DeleteUnit", 1)
}

func (stats *Stats) MarkActiveUnitNoUpdateRequired(unit Unit) {
	logStats("MarkActiveUnitNoUpdateRequired", 1)
}

func (stats *Stats) MarkActiveUnitUpdateRequired(unit Unit) {
	logStats("MarkActiveUnitUpdateRequired", 1)
}

func (stats *Stats) UpdateUnitIgnoredCooldown(unit Unit) {
	logStats("UpdateUnitIgnoredCooldown", 1)
}
