package interfaces

type Collector interface {
	CollectRuntimeMetrics() map[string]float64
	CollectCustomMetrics() map[string]int64
}