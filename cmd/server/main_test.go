package main

import (
    "errors"
    "net/http"
    "net/http/httptest"
    "strconv"
    "strings"
    "testing"
)

// TestMemStorageGauge тестирует работу с gauge метриками
func TestMemStorageGauge(t *testing.T) {
    storage := NewMemStorage()
    
    // Тест обновления значения
    storage.UpdateGauge("testGauge", 123.45)
    value, err := storage.GetGauge("testGauge")
    if err != nil {
        t.Errorf("Failed to get gauge value: %v", err)
    }
    if value != 123.45 {
        t.Errorf("Expected gauge value 123.45, got %f", value)
    }
    
    // Тест перезаписи значения
    storage.UpdateGauge("testGauge", 678.90)
    value, err = storage.GetGauge("testGauge")
    if err != nil {
        t.Errorf("Failed to get gauge value after update: %v", err)
    }
    if value != 678.90 {
        t.Errorf("Expected gauge value 678.90 after update, got %f", value)
    }
    
    // Тест получения несуществующей метрики
    _, err = storage.GetGauge("nonExistent")
    if err == nil {
        t.Error("Expected error for non-existent gauge metric")
    }
}

// TestMemStorageCounter тестирует работу с counter метриками
func TestMemStorageCounter(t *testing.T) {
    storage := NewMemStorage()
    
    // Тест добавления значения
    storage.UpdateCounter("testCounter", 10)
    value, err := storage.GetCounter("testCounter")
    if err != nil {
        t.Errorf("Failed to get counter value: %v", err)
    }
    if value != 10 {
        t.Errorf("Expected counter value 10, got %d", value)
    }
    
    // Тест инкремента значения
    storage.UpdateCounter("testCounter", 5)
    value, err = storage.GetCounter("testCounter")
    if err != nil {
        t.Errorf("Failed to get counter value after increment: %v", err)
    }
    if value != 15 {
        t.Errorf("Expected counter value 15 after increment, got %d", value)
    }
    
    // Тест получения несуществующей метрики
    _, err = storage.GetCounter("nonExistent")
    if err == nil {
        t.Error("Expected error for non-existent counter metric")
    }
}

// TestServerHandlers тестирует HTTP обработчики
func TestServerHandlers(t *testing.T) {
    storage := NewMemStorage()
    handler := makeUpdateHandler(storage)
    
    // Тест успешного обновления gauge метрики
    req := httptest.NewRequest("POST", "/update/gauge/testMetric/123.45", nil)
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    
    if rr.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rr.Code)
    }
    
    value, err := storage.GetGauge("testMetric")
    if err != nil || value != 123.45 {
        t.Errorf("Metric not stored correctly: %v", err)
    }
    
    // Тест успешного обновления counter метрики
    req = httptest.NewRequest("POST", "/update/counter/testCounter/42", nil)
    rr = httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    
    if rr.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rr.Code)
    }
    
    count, err := storage.GetCounter("testCounter")
    if err != nil || count != 42 {
        t.Errorf("Counter not stored correctly: %v", err)
    }
    
    // Тест неверного типа метрики
    req = httptest.NewRequest("POST", "/update/invalid/testMetric/123", nil)
    rr = httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    
    if rr.Code != http.StatusBadRequest {
        t.Errorf("Expected status 400 for invalid metric type, got %d", rr.Code)
    }
    
    // Тест отсутствия имени метрики
    req = httptest.NewRequest("POST", "/update/gauge//123", nil)
    rr = httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    
    if rr.Code != http.StatusNotFound {
        t.Errorf("Expected status 404 for empty metric name, got %d", rr.Code)
    }
    
    // Тест неверного значения gauge
    req = httptest.NewRequest("POST", "/update/gauge/testMetric/invalid", nil)
    rr = httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    
    if rr.Code != http.StatusBadRequest {
        t.Errorf("Expected status 400 for invalid gauge value, got %d", rr.Code)
    }
    
    // Тест неверного значения counter
    req = httptest.NewRequest("POST", "/update/counter/testCounter/invalid", nil)
    rr = httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    
    if rr.Code != http.StatusBadRequest {
        t.Errorf("Expected status 400 for invalid counter value, got %d", rr.Code)
    }
}

// TestURLParsing тестирует парсинг URL
func TestURLParsing(t *testing.T) {
    tests := []struct {
        path     string
        expected []string
        valid    bool
    }{
        {"/update/gauge/metric/123.45", []string{"update", "gauge", "metric", "123.45"}, true},
        {"/update/counter/metric/42", []string{"update", "counter", "metric", "42"}, true},
        {"/update/invalid/metric/123", []string{"update", "invalid", "metric", "123"}, false},
        {"/update/gauge//123", []string{"update", "gauge", "", "123"}, false},
        {"/update/gauge/metric/", []string{"update", "gauge", "metric", ""}, false},
        {"/invalid/path", nil, false},
    }
    
    for _, test := range tests {
        segments := strings.Split(strings.Trim(test.path, "/"), "/")
        
        if test.valid {
            if len(segments) != 4 {
                t.Errorf("Expected 4 segments for %s, got %d", test.path, len(segments))
            }
            for i, seg := range test.expected {
                if segments[i] != seg {
                    t.Errorf("Segment %d mismatch for %s: expected %s, got %s", i, test.path, seg, segments[i])
                }
            }
        }
    }
}

// makeUpdateHandler создает HTTP обработчик для тестов
func makeUpdateHandler(storage Storage) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
            return
        }

        segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
        if len(segments) != 4 {
            http.Error(w, "Invalid path format", http.StatusNotFound)
            return
        }

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
        w.Write([]byte("OK"))
    }
}