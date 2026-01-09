package storage

import (
	"maps"
	"sync"
)

type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
	mu       sync.RWMutex
}

// NewMemStorage создает новое хранилище в памяти
func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (storage *MemStorage) StoreGaugesMetrics(gauges map[string]float64) {
	storage.gauges = gauges
}

func (storage *MemStorage) StoreCountersMetrics(counters map[string]int64) {
	storage.counters = counters
}

// GetGauges возвращает все gauge метрики
func (storage *MemStorage) GetGauges() map[string]float64 {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	
	gauges := make(map[string]float64, len(storage.gauges))
	maps.Copy(gauges, storage.gauges)
	
	return gauges
}

// GetCounters возвращает все counter метрики
func (storage *MemStorage) GetCounters() map[string]int64 {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	
	counters := make(map[string]int64, len(storage.counters))
	maps.Copy(counters, storage.counters)
	
	return counters
}

// Clear очищает хранилище
func (s *MemStorage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.gauges = make(map[string]float64)
	s.counters = make(map[string]int64)
}