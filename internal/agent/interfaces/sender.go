package interfaces

type Sender interface {
	SendMetric(metricType, name string, value interface{}) error
	SendMetrics(gauges map[string]float64, counters map[string]int64, serverURL string, threshold int)
}