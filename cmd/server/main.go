package main

import (
    "errors"
    "fmt"
    "net/http"
    "strconv"
    "strings"
)

// Типы метрик
const (
    Gauge   = "gauge"
    Counter = "counter"
)

// MemStorage хранит метрики в памяти.
type MemStorage struct {
    gauges   map[string]float64
    counters map[string]int64
}

// NewMemStorage создаёт новый экземпляр MemStorage.
func NewMemStorage() *MemStorage {
    return &MemStorage{
        gauges:   make(map[string]float64),
        counters: make(map[string]int64),
    }
}

// UpdateGauge обновляет значение метрики типа gauge.
func (m *MemStorage) UpdateGauge(name string, value float64) {
    m.gauges[name] = value
}

// UpdateCounter обновляет значение метрики типа counter.
func (m *MemStorage) UpdateCounter(name string, value int64) {
    m.counters[name] += value
}

// GetGauge возвращает значение метрики типа gauge.
func (m *MemStorage) GetGauge(name string) (float64, error) {
    value, ok := m.gauges[name]
    if !ok {
        return 0, errors.New("metric not found")
    }
    return value, nil
}

// GetCounter возвращает значение метрики типа counter.
func (m *MemStorage) GetCounter(name string) (int64, error) {
    value, ok := m.counters[name]
    if !ok {
        return 0, errors.New("metric not found")
    }
    return value, nil
}

// Storage определяет интерфейс для работы с хранилищем метрик.
type Storage interface {
    UpdateGauge(name string, value float64)
    UpdateCounter(name string, value int64)
    GetGauge(name string) (float64, error)
    GetCounter(name string) (int64, error)
}

func main() {
    storage := NewMemStorage()
    http.HandleFunc("/update/", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
            return
        }

        // Разбор пути запроса
        segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
        if len(segments) != 4 {
            http.Error(w, "Invalid path format", http.StatusNotFound)
            return
        }

        // segments: ["update", "type", "name", "value"]
        metricType := segments[1]
        metricName := segments[2]
        metricValue := segments[3]

        if metricName == "" {
            http.Error(w, "Metric name cannot be empty", http.StatusNotFound)
            return
        }

        switch metricType {
        case Gauge:
            value, err := strconv.ParseFloat(metricValue, 64)
            if err != nil {
                http.Error(w, "Invalid gauge value", http.StatusBadRequest)
                return
            }
            storage.UpdateGauge(metricName, value)
        case Counter:
            value, err := strconv.ParseInt(metricValue, 10, 64)
            if err != nil {
                http.Error(w, "Invalid counter value", http.StatusBadRequest)
                return
            }
            storage.UpdateCounter(metricName, value)
        default:
            http.Error(w, "Invalid metric type", http.StatusBadRequest)
            return
        }

        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Metric updated"))
    })

    fmt.Println("Server starting on :8080")
    http.ListenAndServe(":8080", nil)
}