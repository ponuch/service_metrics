package main

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
)

// TestAgentCollectMetrics тестирует сбор метрик
func TestAgentCollectMetrics(t *testing.T) {
    cfg := Config{
        PollInterval:   1 * time.Second,
        ReportInterval: 5 * time.Second,
        ServerURL:      "http://localhost:8080",
    }
    
    agent := NewAgent(cfg)
    agent.collectMetrics()
    
    // Проверяем, что метрики собраны
    if _, exists := agent.metrics["Alloc"]; !exists {
        t.Error("Expected Alloc metric to be collected")
    }
    
    if _, exists := agent.metrics["PollCount"]; !exists {
        t.Error("Expected PollCount metric to be collected")
    }
    
    if _, exists := agent.metrics["RandomValue"]; !exists {
        t.Error("Expected RandomValue metric to be collected")
    }
    
    // Проверяем тип PollCount
    if count, ok := agent.metrics["PollCount"].(int64); !ok || count != 1 {
        t.Errorf("Expected PollCount to be 1, got %v", count)
    }
}

// TestAgentSendMetric тестирует отправку метрики
func TestAgentSendMetric(t *testing.T) {
    // Создаем тестовый сервер
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            t.Errorf("Expected POST request, got %s", r.Method)
        }

		validURL := (r.URL.Path == "/update/gauge/testMetric/123.456") || (r.URL.Path == "/update/counter/testCounter/42")

		if validURL == false {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
        
        
        if r.Header.Get("Content-Type") != "text/plain" {
            t.Errorf("Unexpected Content-Type: %s", r.Header.Get("Content-Type"))
        }
        
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()
    
    cfg := Config{
        PollInterval:   1 * time.Second,
        ReportInterval: 5 * time.Second,
        ServerURL:      server.URL,
    }
    
    agent := NewAgent(cfg)
    
    // Тестируем отправку gauge метрики
    err := agent.sendMetric("gauge", "testMetric", 123.456)
    if err != nil {
        t.Errorf("Failed to send gauge metric: %v", err)
    }
    
    // Тестируем отправку counter метрики
    err = agent.sendMetric("counter", "testCounter", int64(42))
    if err != nil {
        t.Errorf("Failed to send counter metric: %v", err)
    }
}

// TestAgentSendMetrics тестирует отправку всех метрик
func TestAgentSendMetrics(t *testing.T) {
    requestCount := 0
    expectedRequests := 2 // PollCount + RandomValue
    
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestCount++
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()
    
    cfg := Config{
        PollInterval:   1 * time.Second,
        ReportInterval: 5 * time.Second,
        ServerURL:      server.URL,
    }
    
    agent := NewAgent(cfg)
    agent.collectCustomMetrics() // Собираем только кастомные метрики для теста
    
    agent.sendMetrics()
    
    if requestCount != expectedRequests {
        t.Errorf("Expected %d requests, got %d", expectedRequests, requestCount)
    }
}

// TestAgentConfig тестирует конфигурацию агента
func TestAgentConfig(t *testing.T) {
    cfg := Config{
        PollInterval:   2 * time.Second,
        ReportInterval: 10 * time.Second,
        ServerURL:      "http://test:8080",
    }
    
    if cfg.PollInterval != 2*time.Second {
        t.Errorf("Expected poll interval 2s, got %v", cfg.PollInterval)
    }
    
    if cfg.ReportInterval != 10*time.Second {
        t.Errorf("Expected report interval 10s, got %v", cfg.ReportInterval)
    }
    
    if cfg.ServerURL != "http://test:8080" {
        t.Errorf("Expected server URL http://test:8080, got %s", cfg.ServerURL)
    }
}