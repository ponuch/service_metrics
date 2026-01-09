package interfaces

type Storage interface {
	StoreGaugesMetrics(gauges map[string]float64)
	StoreCountersMetrics(counters map[string]int64)
	GetGauges() map[string]float64
	GetCounters() map[string]int64
	Clear()
}