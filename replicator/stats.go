package replicator

import (
	"github.com/giantswarm/metrics"
)

type Stats struct {
	metrics metrics.Collector
}

func (stats *Stats) SeenMachinesTotal(count int) {
	stats.metrics.Gauge(int64(count), "fleet", "machines", "total", "count")
}
func (stats *Stats) SeenMachinesActive(count int) {
	stats.metrics.Gauge(int64(count), "fleet", "machines", "active", "count")
}

func (stats *Stats) SeenUnitsTotal(count int) {
	stats.metrics.Gauge(int64(count), "fleet", "units", "all", "count")
}
func (stats *Stats) SeenUnitsManaged(count int) {
	stats.metrics.Gauge(int64(count), "fleet", "units", "all", "active")
}

func (stats *Stats) MarkNewUndesiredUnit(unit Unit) {
	stats.metrics.Counter(1, "fleet", "units", "undesired", "new")
}
func (stats *Stats) MarkUndesiredUnitBackToDesired(unit Unit) {
	stats.metrics.Counter(1, "fleet", "units", "undesired", "revived")
}
func (stats *Stats) DeleteUndesiredUnit(unit Unit) {
	stats.metrics.Counter(1, "fleet", "units", "undesired", "delete_unit")
}
func (stats *Stats) UndesiredUnitsGauge(g int) {
	stats.metrics.Gauge(int64(g), "fleet", "units", "undesired", "count")
}

func (stats *Stats) DesiredUnitsGauge(g int) {
	stats.metrics.Gauge(int64(g), "fleet", "units", "desired", "count")
}

func (stats *Stats) ActiveUnitsSeen(g int) {
	stats.metrics.Gauge(int64(g), "fleet", "units", "active", "count")
}

func (stats *Stats) ActiveUnitsNoUpdateRequired(g int64) {
	stats.metrics.Gauge(g, "fleet", "units", "active", "no_update_required")
}

func (stats *Stats) ActiveUnitsUpdateRequired(g int64) {
	stats.metrics.Gauge(g, "fleet", "units", "active", "update_required")
}

func (stats *Stats) UpdateUnitIgnoredCooldown(unit Unit) {
	stats.metrics.Counter(1, "fleet", "units", "active", "update_skipped_cooldown")
}
