package main

import (
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestEnvironmentVariables тестирует чтение переменных окружения
// func TestEnvironmentVariables(t *testing.T) {
// 	// Вспомогательная функция для тестирования с заданными переменными окружения
// 	testWithEnv := func(t *testing.T, address, report, poll string,
// 		expectedURL string, expectedReport, expectedPoll time.Duration) {

// 		// Устанавливаем переменные окружения
// 		if address != "" {
// 			t.Setenv("ADDRESS", address)
// 		} else {
// 			os.Unsetenv("ADDRESS")
// 		}

// 		if report != "" {
// 			t.Setenv("REPORT_INTERVAL", report)
// 		} else {
// 			os.Unsetenv("REPORT_INTERVAL")
// 		}

// 		if poll != "" {
// 			t.Setenv("POLL_INTERVAL", poll)
// 		} else {
// 			os.Unsetenv("POLL_INTERVAL")
// 		}

// 		// Сбрасываем состояние флагов
// 		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

// 		// Вызываем parseAgentFlags
// 		cfg := parseAgentFlags()

// 		if cfg.ServerURL != expectedURL {
// 			t.Errorf("Expected server URL %s, got %s", expectedURL, cfg.ServerURL)
// 		}

// 		if cfg.ReportInterval != expectedReport {
// 			t.Errorf("Expected report interval %v, got %v", expectedReport, cfg.ReportInterval)
// 		}

// 		if cfg.PollInterval != expectedPoll {
// 			t.Errorf("Expected poll interval %v, got %v", expectedPoll, cfg.PollInterval)
// 		}
// 	}

// 	// // Тест 1: Все переменные окружения установлены
// 	// t.Run("All environment variables set", func(t *testing.T) {
// 	// 	testWithEnv(t, "example.com:9090", "5", "1",
// 	// 		"http://example.com:9090", 5*time.Second, 1*time.Second)
// 	// })

// 	// // Тест 2: Частично установленные переменные окружения
// 	// t.Run("Partially set environment variables", func(t *testing.T) {
// 	// 	testWithEnv(t, "", "15", "",
// 	// 		"http://localhost:8080", 15*time.Second, 2*time.Second)
// 	// })

// 	// // Тест 3: Некорректные значения в переменных окружения
// 	t.Run("Invalid environment values", func(t *testing.T) {
// 		testWithEnv(t, "", "invalid", "-5",
// 			"http://localhost:8080", 10*time.Second, 2*time.Second)
// 	})
// }

// TestEnvironmentVariablesWithFlags тестирует приоритет флагов над переменными окружения
func TestEnvironmentVariablesWithFlags(t *testing.T) {
	// Вспомогательная функция
	testCase := func(t *testing.T, envAddress, envReport, envPoll string,
		args []string, expectedURL string, expectedReport, expectedPoll time.Duration) {
		
		// Сохраняем оригинальные args
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()
		
		// Устанавливаем переменные окружения
		if envAddress != "" {
			t.Setenv("ADDRESS", envAddress)
		}
		if envReport != "" {
			t.Setenv("REPORT_INTERVAL", envReport)
		}
		if envPoll != "" {
			t.Setenv("POLL_INTERVAL", envPoll)
		}
		
		// Устанавливаем аргументы
		os.Args = args
		
		// Сбрасываем состояние флагов
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		
		cfg := parseAgentFlags()
		
		if cfg.ServerURL != expectedURL {
			t.Errorf("Expected server URL %s, got %s", expectedURL, cfg.ServerURL)
		}
		
		if cfg.ReportInterval != expectedReport {
			t.Errorf("Expected report interval %v, got %v", expectedReport, cfg.ReportInterval)
		}
		
		if cfg.PollInterval != expectedPoll {
			t.Errorf("Expected poll interval %v, got %v", expectedPoll, cfg.PollInterval)
		}
	}
	
	// Тест: Флаги переопределяют переменные окружения
	t.Run("Flags override environment", func(t *testing.T) {
		testCase(t, "env.example.com:8080", "20", "5",
			[]string{"agent", "-a=flag.example.com:9090", "-r=30", "-p=3"},
			"http://flag.example.com:9090", 30*time.Second, 3*time.Second)
	})
}

// TestNormalizeURL тестирует нормализацию URL
func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"With port", "localhost:8080", "http://localhost:8080"},
		{"IP with port", "127.0.0.1:9090", "http://127.0.0.1:9090"},
		{"Without port", "example.com", "http://example.com:8080"},
		{"With http scheme", "http://localhost:8080", "http://localhost:8080"},
		{"With https scheme", "https://example.com", "https://example.com"},
		{"Empty", "", "http://localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeURL(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestAgentWithEnvironmentVariables тестирует создание агента с переменными окружения
func TestAgentWithEnvironmentVariables(t *testing.T) {
	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	// Сохраняем оригинальные args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	
	// Устанавливаем переменные окружения
	serverURL := strings.TrimPrefix(server.URL, "http://")
	t.Setenv("ADDRESS", serverURL)
	t.Setenv("REPORT_INTERVAL", "1")
	t.Setenv("POLL_INTERVAL", "1")
	
	// Устанавливаем аргументы
	os.Args = []string{"agent"}
	
	// Сбрасываем состояние флагов
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	
	cfg := parseAgentFlags()
	agent := NewAgent(cfg)
	
	// Проверяем, что конфигурация загружена из переменных окружения
	if !strings.Contains(agent.config.ServerURL, serverURL) {
		t.Errorf("Expected server URL to contain %s, got %s", serverURL, agent.config.ServerURL)
	}
	if agent.config.ReportInterval != 1*time.Second {
		t.Errorf("Expected report interval 1s from env, got %v", agent.config.ReportInterval)
	}
	if agent.config.PollInterval != 1*time.Second {
		t.Errorf("Expected poll interval 1s from env, got %v", agent.config.PollInterval)
	}
}

// TestAgentFlagsDefaultValues тестирует значения по умолчанию
func TestAgentFlagsDefaultValues(t *testing.T) {
	// Сохраняем оригинальные args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	
	// Очищаем переменные окружения
	os.Unsetenv("ADDRESS")
	os.Unsetenv("REPORT_INTERVAL")
	os.Unsetenv("POLL_INTERVAL")
	
	// Устанавливаем аргументы без флагов
	os.Args = []string{"agent"}
	
	// Сбрасываем состояние флагов
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	
	cfg := parseAgentFlags()
	
	// Проверяем значения по умолчанию
	if cfg.ServerURL != "http://localhost:8080" {
		t.Errorf("Expected default server URL http://localhost:8080, got %s", cfg.ServerURL)
	}
	if cfg.ReportInterval != 10*time.Second {
		t.Errorf("Expected default report interval 10s, got %v", cfg.ReportInterval)
	}
	if cfg.PollInterval != 2*time.Second {
		t.Errorf("Expected default poll interval 2s, got %v", cfg.PollInterval)
	}
}

// TestAgentSendMetric тестирует отправку метрик
func TestAgentSendMetric(t *testing.T) {
	// Создаем тестовый сервер
	var receivedRequests []Metrics
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.URL.Path != "/update" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Читаем тело запроса
		var metric Metrics
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := json.Unmarshal(body, &metric); err != nil {
			t.Errorf("Failed to decode request JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Сохраняем полученную метрику
		receivedRequests = append(receivedRequests, metric)

		// Отправляем корректный JSON ответ
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		
		// Возвращаем ту же метрику (как это делает реальный сервер)
		if err := json.NewEncoder(w).Encode(metric); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	cfg := Config{
		ServerURL:      server.URL,
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
	}

	agent := NewAgent(cfg)

	// Тестируем отправку gauge метрики
	err := agent.sendMetric("gauge", "test_metric", 123.456)
	if err != nil {
		t.Errorf("Failed to send gauge metric: %v", err)
	}

	// Проверяем полученную метрику
	if len(receivedRequests) != 1 {
		t.Errorf("Expected 1 request, got %d", len(receivedRequests))
	} else {
		metric := receivedRequests[0]
		if metric.ID != "test_metric" {
			t.Errorf("Expected metric ID 'test_metric', got %s", metric.ID)
		}
		if metric.MType != "gauge" {
			t.Errorf("Expected metric type 'gauge', got %s", metric.MType)
		}
		if metric.Value == nil || *metric.Value != 123.456 {
			t.Errorf("Expected metric value 123.456, got %v", metric.Value)
		}
	}

	// Тестируем отправку counter метрики
	err = agent.sendMetric("counter", "test_counter", int64(42))
	if err != nil {
		t.Errorf("Failed to send counter metric: %v", err)
	}

	// Проверяем полученную метрику
	if len(receivedRequests) != 2 {
		t.Errorf("Expected 2 requests, got %d", len(receivedRequests))
	} else {
		metric := receivedRequests[1]
		if metric.ID != "test_counter" {
			t.Errorf("Expected metric ID 'test_counter', got %s", metric.ID)
		}
		if metric.MType != "counter" {
			t.Errorf("Expected metric type 'counter', got %s", metric.MType)
		}
		if metric.Delta == nil || *metric.Delta != 42 {
			t.Errorf("Expected metric delta 42, got %v", metric.Delta)
		}
	}
}

// TestAgentCollectMetrics тестирует сбор метрик
func TestAgentCollectMetrics(t *testing.T) {
	cfg := Config{
		ServerURL:      "http://localhost:8080",
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
	}
	
	agent := NewAgent(cfg)
	agent.collectMetrics()
	
	// Проверяем, что метрики собраны
	if _, exists := agent.gauges["Alloc"]; !exists {
		t.Error("Expected Alloc metric to be collected")
	}
	
	if _, exists := agent.counters["PollCount"]; !exists {
		t.Error("Expected PollCount metric to be collected")
	}
	
	if _, exists := agent.gauges["RandomValue"]; !exists {
		t.Error("Expected RandomValue metric to be collected")
	}
	
	// Проверяем тип PollCount
	if count, ok := agent.counters["PollCount"]; !ok || count != 1 {
		t.Errorf("Expected PollCount to be 1, got %v", count)
	}
}

// TestAgentCreation тестирует создание агента
func TestAgentCreation(t *testing.T) {
	cfg := Config{
		ServerURL:      "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
	}
	
	agent := NewAgent(cfg)
	
	if agent.config.ServerURL != cfg.ServerURL {
		t.Error("Agent config not set correctly")
	}
	if agent.client == nil {
		t.Error("Agent HTTP client not initialized")
	}
	if agent.counters == nil || agent.gauges == nil {
		t.Error("Agent metrics map not initialized")
	}
}

// TestURLNormalization тестирует нормализацию URL из переменных окружения
func TestURLNormalization(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{"Address with port", "example.com:9090", "http://example.com:9090"},
		{"Address without port", "example.com", "http://example.com:8080"},
		{"Address with http", "http://example.com", "http://example.com"},
		{"Address with https", "https://secure.example.com", "https://secure.example.com"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Сохраняем оригинальные args
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			
			// Устанавливаем переменную окружения
			t.Setenv("ADDRESS", tt.envValue)
			
			// Устанавливаем аргументы
			os.Args = []string{"agent"}
			
			// Сбрасываем состояние флагов
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			
			cfg := parseAgentFlags()
			
			if cfg.ServerURL != tt.expected {
				t.Errorf("For env value %s, expected %s, got %s", tt.envValue, tt.expected, cfg.ServerURL)
			}
		})
	}
}

// TestInvalidIntervalValues тестирует некорректные значения интервалов
func TestInvalidIntervalValues(t *testing.T) {
	// Сохраняем оригинальные args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	
	// Устанавливаем некорректные переменные окружения
	t.Setenv("REPORT_INTERVAL", "not-a-number")
	t.Setenv("POLL_INTERVAL", "0") // 0 - невалидное значение
	
	// Устанавливаем аргументы
	os.Args = []string{"agent"}
	
	// Сбрасываем состояние флагов
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	
	cfg := parseAgentFlags()
	
	// Должны использоваться значения по умолчанию
	if cfg.ReportInterval != 10*time.Second {
		t.Errorf("Expected default report interval 10s for invalid value, got %v", cfg.ReportInterval)
	}
	if cfg.PollInterval != 2*time.Second {
		t.Errorf("Expected default poll interval 2s for invalid value, got %v", cfg.PollInterval)
	}
}

// TestFlagPriorityOverEnv тестирует приоритет флагов
func TestFlagPriorityOverEnv(t *testing.T) {
	// Сохраняем оригинальные args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	
	// Устанавливаем переменные окружения
	t.Setenv("ADDRESS", "env.example.com:8080")
	t.Setenv("REPORT_INTERVAL", "20")
	t.Setenv("POLL_INTERVAL", "5")
	
	// Устанавливаем флаги командной строки
	os.Args = []string{"agent", "-a=flag.example.com:9090"}
	
	// Сбрасываем состояние флагов
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	
	cfg := parseAgentFlags()
	
	// Адрес должен быть из флага
	if cfg.ServerURL != "http://flag.example.com:9090" {
		t.Errorf("Expected flag server URL, got %s", cfg.ServerURL)
	}
	// Интервалы должны быть из переменных окружения (флаги не установлены)
	if cfg.ReportInterval != 20*time.Second {
		t.Errorf("Expected env report interval 20s, got %v", cfg.ReportInterval)
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("Expected env poll interval 5s, got %v", cfg.PollInterval)
	}
}

// TestSendMetrics тестирует отправку всех метрик
func TestSendMetrics(t *testing.T) {
	// Создаем тестовый сервер для подсчета запросов
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	cfg := Config{
		ServerURL:      server.URL,
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
	}
	
	agent := NewAgent(cfg)
	
	// Добавляем тестовые метрики
	agent.gauges["test_gauge"] = 99.99
	agent.counters["test_counter"] = int64(50)
	
	agent.sendMetrics()
	
	// Проверяем количество отправленных запросов
	// Ожидаем 2 запроса (gauge и counter), invalid_type не должен отправиться
	expectedRequests := 2
	if requestCount != expectedRequests {
		t.Errorf("Expected %d requests, got %d", expectedRequests, requestCount)
	}
}

// TestSendMetricErrorHandling тестирует обработку ошибок при отправке метрик
func TestSendMetricErrorHandling(t *testing.T) {
	// Сервер возвращающий ошибку
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	
	cfg := Config{
		ServerURL:      server.URL,
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
	}
	
	agent := NewAgent(cfg)
	
	// Тест ошибки сервера
	err := agent.sendMetric("gauge", "test_metric", 123.456)
	if err == nil {
		t.Error("Expected error for server returning 400")
	}
	
	// Тест неверного типа значения
	err = agent.sendMetric("gauge", "test_metric", "invalid")
	if err == nil {
		t.Error("Expected error for unsupported metric type")
	}
}

// TestCollectRuntimeMetrics тестирует сбор метрик runtime
func TestCollectRuntimeMetrics(t *testing.T) {
	cfg := Config{
		ServerURL:      "http://localhost:8080",
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
	}
	
	agent := NewAgent(cfg)
	agent.collectRuntimeMetrics()
	
	// Проверяем, что основные метрики собраны
	expectedMetrics := []string{
		"Alloc", "HeapAlloc", "HeapSys", "HeapObjects",
		"NumGC", "GCCPUFraction", "TotalAlloc", "Sys",
	}
	
	for _, metric := range expectedMetrics {
		if _, exists := agent.gauges[metric]; !exists {
			t.Errorf("Expected metric %s to be collected", metric)
		}
	}
}

// TestAgentJSONSend тестирует отправку метрик в формате JSON
func TestAgentJSONSend(t *testing.T) {
	// Создаем тестовый сервер
	receivedMetrics := make([]Metrics, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		
		if r.URL.Path != "/update" {
			t.Errorf("Expected path /update, got %s", r.URL.Path)
		}
		
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		
		var metric Metrics
		if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
			t.Errorf("Failed to decode request JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		receivedMetrics = append(receivedMetrics, metric)
		
		// Отправляем успешный ответ
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(metric)
	}))
	defer server.Close()
	
	cfg := Config{
		ServerURL:      server.URL,
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
	}
	
	agent := NewAgent(cfg)
	
	// Тестируем отправку gauge метрики
	err := agent.sendMetricJSON("gauge", "test_gauge", 123.456)
	if err != nil {
		t.Errorf("Failed to send gauge metric: %v", err)
	}
	
	// Тестируем отправку counter метрики
	err = agent.sendMetricJSON("counter", "test_counter", int64(42))
	if err != nil {
		t.Errorf("Failed to send counter metric: %v", err)
	}
	
	// Проверяем полученные метрики
	if len(receivedMetrics) != 2 {
		t.Errorf("Expected 2 metrics received, got %d", len(receivedMetrics))
	}
	
	if receivedMetrics[0].ID != "test_gauge" || receivedMetrics[0].MType != "gauge" || 
		receivedMetrics[0].Value == nil || *receivedMetrics[0].Value != 123.456 {
		t.Errorf("First metric not as expected: %+v", receivedMetrics[0])
	}
	
	if receivedMetrics[1].ID != "test_counter" || receivedMetrics[1].MType != "counter" || 
		receivedMetrics[1].Delta == nil || *receivedMetrics[1].Delta != 42 {
		t.Errorf("Second metric not as expected: %+v", receivedMetrics[1])
	}
}