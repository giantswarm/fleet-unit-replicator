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
func (stats *Stats) MarkUndesiredUnitAlive(unit Unit) {
	logStats("MarkUndesiredUnitAlive", 1)
}
func (stats *Stats) DeleteUndesiredUnit(unit Unit) {
	logStats("DeleteUnit", 1)
}
