package collector

import (
	"math/rand"
	"runtime"
)

type MetricCollector struct {
	pollCount int64
}

// NewCollector создает новый коллектор
func NewCollector() *MetricCollector {
	return &MetricCollector{}
}

func (mc *MetricCollector) CollectRuntimeMetrics() map[string]float64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	gauges := make(map[string]float64)

	gauges["Alloc"] = float64(memStats.Alloc)
	gauges["BuckHashSys"] = float64(memStats.BuckHashSys)
	gauges["Frees"] = float64(memStats.Frees)
	gauges["GCCPUFraction"] = memStats.GCCPUFraction
	gauges["GCSys"] = float64(memStats.GCSys)
	gauges["HeapAlloc"] = float64(memStats.HeapAlloc)
	gauges["HeapIdle"] = float64(memStats.HeapIdle)
	gauges["HeapInuse"] = float64(memStats.HeapInuse)
	gauges["HeapObjects"] = float64(memStats.HeapObjects)
	gauges["HeapReleased"] = float64(memStats.HeapReleased)
	gauges["HeapSys"] = float64(memStats.HeapSys)
	gauges["LastGC"] = float64(memStats.LastGC)
	gauges["Lookups"] = float64(memStats.Lookups)
	gauges["MCacheInuse"] = float64(memStats.MCacheInuse)
	gauges["MCacheSys"] = float64(memStats.MCacheSys)
	gauges["MSpanInuse"] = float64(memStats.MSpanInuse)
	gauges["MSpanSys"] = float64(memStats.MSpanSys)
	gauges["Mallocs"] = float64(memStats.Mallocs)
	gauges["NextGC"] = float64(memStats.NextGC)
	gauges["NumForcedGC"] = float64(memStats.NumForcedGC)
	gauges["NumGC"] = float64(memStats.NumGC)
	gauges["OtherSys"] = float64(memStats.OtherSys)
	gauges["PauseTotalNs"] = float64(memStats.PauseTotalNs)
	gauges["StackInuse"] = float64(memStats.StackInuse)
	gauges["StackSys"] = float64(memStats.StackSys)
	gauges["Sys"] = float64(memStats.Sys)
	gauges["TotalAlloc"] = float64(memStats.TotalAlloc)

	// RandomValue - случайное значение
	gauges["RandomValue"] = rand.Float64()

	return gauges
}

func (mc *MetricCollector) CollectCustomMetrics() map[string]int64 {
	counters := make(map[string]int64)

	mc.pollCount++
	counters["PollCount"] = mc.pollCount
	return counters
}