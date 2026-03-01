package interfaces

type Sender interface {
	sendMetricJSON(threshold int, serverURL, metricType, name string, value interface{}) error
	sendMetricLegacy(serverURL, metricType, name string, value interface{}) error 
	SendMetric(threshold int, serverURL, metricType, name string, value interface{}) error
	SendMetrics(gauges map[string]float64, counters map[string]int64, serverURL string, threshold int)
}