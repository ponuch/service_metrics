package agent

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ponuch/service_metrics/internal/agent/config"
	models "github.com/ponuch/service_metrics/internal/model"
)

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


	cfg := config.New()

	agent := NewAgent(*cfg)
	
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
	
	cfg := config.New()
	
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
	var receivedRequests []models.Metrics
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

		// Читаем тело запроса с учетом возможного сжатия
		var reader io.Reader = r.Body
		contentEncoding := r.Header.Get("Content-Encoding")
		if strings.Contains(contentEncoding, "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				t.Errorf("Failed to create gzip reader: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer gz.Close()
			reader = gz
		}

		body, err := io.ReadAll(reader)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Проверяем, что данные являются валидным JSON
		if !json.Valid(body) {
			t.Errorf("Received invalid JSON: %s", string(body))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var metric models.Metrics
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

	cfg := config.AgentConfig{
		ServerURL:        server.URL,
		PollInterval:     1 * time.Second,
		ReportInterval:   5 * time.Second,
		CompressThreshold: 1024, // Устанавливаем порог, чтобы данные не сжимались
	}

	agent := NewAgent(cfg)

	// Тестируем отправку gauge метрики
	err := agent.sender.SendMetric("gauge", "test_metric", 123.456)
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

	err = agent.sender.SendMetric("counter", "test_counter", int64(42))
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

// // TestAgentCollectMetrics тестирует сбор метрик
func TestAgentCollectMetrics(t *testing.T) {
	cfg := config.AgentConfig{
		ServerURL:      "http://localhost:8080",
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
	}
	
	agent := NewAgent(cfg)
	agent.collectMetrics()

	gauges := agent.collector.CollectRuntimeMetrics()
	counters := agent.collector.CollectCustomMetrics()
	
	
	// Проверяем, что метрики собраны
	if _, exists := gauges["Alloc"]; !exists {
		t.Error("Expected Alloc metric to be collected")
	}
	
	if _, exists := counters["PollCount"]; !exists {
		t.Error("Expected PollCount metric to be collected")
	}
	
	if _, exists := gauges["RandomValue"]; !exists {
		t.Error("Expected RandomValue metric to be collected")
	}
	
	// Проверяем тип PollCount
	if count, ok := counters["PollCount"]; !ok || count != 1 {
		t.Errorf("Expected PollCount to be 1, got %v", count)
	}
}

// // TestAgentCreation тестирует создание агента
func TestAgentCreation(t *testing.T) {
	cfg := config.AgentConfig{
		ServerURL:      "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
	}
	
	agent := NewAgent(cfg)
	
	if agent.config.ServerURL != cfg.ServerURL {
		t.Error("Agent config not set correctly")
	}
	if agent.sender.Client == nil {
		t.Error("Agent HTTP client not initialized")
	}
	if agent.storage.GetCounters() == nil || agent.storage.GetGauges() == nil {
		t.Error("Agent metrics map not initialized")
	}
}

// // TestURLNormalization тестирует нормализацию URL из переменных окружения
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
			
			cfg := config.New()
			
			if cfg.ServerURL != tt.expected {
				t.Errorf("For env value %s, expected %s, got %s", tt.envValue, tt.expected, cfg.ServerURL)
			}
		})
	}
}

// // TestInvalidIntervalValues тестирует некорректные значения интервалов
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
	
	cfg := config.New()
	
	// Должны использоваться значения по умолчанию
	if cfg.ReportInterval != 10*time.Second {
		t.Errorf("Expected default report interval 10s for invalid value, got %v", cfg.ReportInterval)
	}
	if cfg.PollInterval != 2*time.Second {
		t.Errorf("Expected default poll interval 2s for invalid value, got %v", cfg.PollInterval)
	}
}

// // TestFlagPriorityOverEnv тестирует приоритет флагов
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
	
	cfg := config.New()
	
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

// // TestSendMetrics тестирует отправку всех метрик
func TestSendMetrics(t *testing.T) {
	// Создаем тестовый сервер для подсчета запросов
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	cfg := config.AgentConfig{
		ServerURL:      server.URL,
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
	}
	
	agent := NewAgent(cfg)
	
	// Добавляем тестовые метрики
	customMetrics := map[string]int64 {"test_counter": int64(50)} 
	runtimeMetrics := map[string]float64 {"test_gauge": 99.99}
	
	agent.sender.SendMetrics(runtimeMetrics, customMetrics, cfg.ServerURL, cfg.CompressThreshold)
	
	// Проверяем количество отправленных запросов
	// Ожидаем 2 запроса (gauge и counter), invalid_type не должен отправиться
	expectedRequests := 2
	if requestCount != expectedRequests {
		t.Errorf("Expected %d requests, got %d", expectedRequests, requestCount)
	}
}

// // TestSendMetricErrorHandling тестирует обработку ошибок при отправке метрик
func TestSendMetricErrorHandling(t *testing.T) {
	// Сервер возвращающий ошибку
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	
	cfg := config.AgentConfig{
		ServerURL:      server.URL,
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
	}
	
	agent := NewAgent(cfg)
	
	// Тест ошибки сервера
	err := agent.sender.SendMetric("gauge", "test_metric", 123.456)
	if err == nil {
		t.Error("Expected error for server returning 400")
	}
	
	// Тест неверного типа значения
	err = agent.sender.SendMetric("gauge", "test_metric", "invalid")
	if err == nil {
		t.Error("Expected error for unsupported metric type")
	}
}

// // TestCollectRuntimeMetrics тестирует сбор метрик runtime
func TestCollectRuntimeMetrics(t *testing.T) {
	cfg := config.AgentConfig{
		ServerURL:      "http://localhost:8080",
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
	}
	
	agent := NewAgent(cfg)
	runtimeMetrics := agent.collector.CollectRuntimeMetrics()
	
	// Проверяем, что основные метрики собраны
	expectedMetrics := []string{
		"Alloc", "HeapAlloc", "HeapSys", "HeapObjects",
		"NumGC", "GCCPUFraction", "TotalAlloc", "Sys",
	}
	
	for _, metric := range expectedMetrics {
		if _, exists := runtimeMetrics[metric]; !exists {
			t.Errorf("Expected metric %s to be collected", metric)
		}
	}
}

// // TestAgentJSONSend тестирует отправку метрик в формате JSON
func TestAgentJSONSend(t *testing.T) {
	// Создаем тестовый сервер
	receivedMetrics := make([]models.Metrics, 0)
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
		
		// Обрабатываем сжатые данные
		var reader io.Reader = r.Body
		contentEncoding := r.Header.Get("Content-Encoding")
		if strings.Contains(contentEncoding, "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				t.Errorf("Failed to create gzip reader: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer gz.Close()
			reader = gz
		}
		
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		// Проверяем валидность JSON
		if !json.Valid(body) {
			t.Errorf("Invalid JSON received: %s", string(body))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		var metric models.Metrics
		if err := json.Unmarshal(body, &metric); err != nil {
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
	
	cfg := config.AgentConfig{
		ServerURL:        server.URL,
		PollInterval:     1 * time.Second,
		ReportInterval:   5 * time.Second,
		CompressThreshold: 1024, // Устанавливаем порог, чтобы данные не сжимались
	}
	
	agent := NewAgent(cfg)
	
	// Тестируем отправку gauge метрики
	err := agent.sender.SendMetric("gauge", "test_gauge", 123.456)
	if err != nil {
		t.Errorf("Failed to send gauge metric: %v", err)
	}
	
	// Тестируем отправку counter метрики
	err = agent.sender.SendMetric("counter", "test_counter", int64(42))
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


// // TestAgentGzipCompression тестирует сжатие gzip в агенте
func TestAgentGzipCompression(t *testing.T) {
	// Создаем тестовый сервер для проверки сжатых запросов
	var receivedCompressed bool
	var receivedContentEncoding string
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentEncoding = r.Header.Get("Content-Encoding")
		receivedCompressed = strings.Contains(receivedContentEncoding, "gzip")
		
		// Читаем тело запроса
		var reader io.Reader = r.Body
		if receivedCompressed {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				t.Errorf("Failed to create gzip reader: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer gz.Close()
			reader = gz
		}
		
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		// Проверяем JSON
		var metric models.Metrics
		if err := json.Unmarshal(body, &metric); err != nil {
			t.Errorf("Failed to decode JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		// Отправляем ответ (сжимаем если клиент поддерживает)
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")
		
		w.Header().Set("Content-Type", "application/json")
		if supportsGzip {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			json.NewEncoder(gz).Encode(metric)
		} else {
			json.NewEncoder(w).Encode(metric)
		}
	}))
	defer server.Close()
	
	// Тест 1: Данные меньше порога сжатия
	cfg1 := config.AgentConfig{
		ServerURL:        server.URL,
		PollInterval:     1 * time.Second,
		ReportInterval:   5 * time.Second,
		CompressThreshold: 1000, // Большой порог
	}
	
	agent1 := NewAgent(cfg1)
	
	// Сбрасываем флаги
	receivedCompressed = false
	receivedContentEncoding = ""
	
	// Отправляем маленькие данные (JSON около 50 байт)
	err := agent1.sender.SendMetric("gauge", "test", 123.456)
	if err != nil {
		t.Errorf("Failed to send metric: %v", err)
	}
	
	// Не должно быть сжато, так как данные меньше порога
	if receivedCompressed {
		t.Error("Small data should not be compressed")
	}
	
	// Тест 2: Данные больше порога сжатия
	cfg2 := config.AgentConfig{
		ServerURL:        server.URL,
		PollInterval:     1 * time.Second,
		ReportInterval:   5 * time.Second,
		CompressThreshold: 10, // Маленький порог
	}
	
	agent2 := NewAgent(cfg2)
	
	// Сбрасываем флаги
	receivedCompressed = false
	receivedContentEncoding = ""
	
	// Отправляем те же данные
	err = agent2.sender.SendMetric("gauge", "test", 123.456)
	if err != nil {
		t.Errorf("Failed to send metric: %v", err)
	}
	
	// Должно быть сжато, так как данные больше порога
	if !receivedCompressed {
		t.Error("Large data should be compressed")
	}
	
	if !strings.Contains(receivedContentEncoding, "gzip") {
		t.Errorf("Expected Content-Encoding: gzip, got %s", receivedContentEncoding)
	}
}




