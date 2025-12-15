package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
    "flag"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
    "go.uber.org/zap/zapcore"
)

// TestServerEnvironmentVariables тестирует чтение переменных окружения сервера
func TestServerEnvironmentVariables(t *testing.T) {
	// Вспомогательная функция для тестирования с заданными переменными окружения
	testWithEnv := func(t *testing.T, envAddress string, args []string, expectedAddr string) {
		// Сохраняем оригинальные args
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()
		
		// Устанавливаем переменную окружения
		if envAddress != "" {
			t.Setenv("ADDRESS", envAddress)
		} else {
			os.Unsetenv("ADDRESS")
		}
		
		// Устанавливаем аргументы
		os.Args = args
		
		// Сбрасываем состояние флагов
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		
		// Вызываем parseServerFlags
		cfg := parseServerFlags()
		
		if cfg.Addr != expectedAddr {
			t.Errorf("Expected address %s, got %s", expectedAddr, cfg.Addr)
		}
	}
	
	// Тест 1: Только переменная окружения
	t.Run("Environment variable only", func(t *testing.T) {
		testWithEnv(t, "localhost:9090", []string{"server"}, "localhost:9090")
	})
	
	// Тест 2: Переменная окружения и флаг (флаг должен иметь приоритет)
	t.Run("Environment variable and flag", func(t *testing.T) {
		testWithEnv(t, "localhost:9090", []string{"server", "-a=127.0.0.1:8080"}, "127.0.0.1:8080")
	})
	
	// Тест 3: Только флаг (без переменной окружения)
	t.Run("Flag only", func(t *testing.T) {
		testWithEnv(t, "", []string{"server", "-a=127.0.0.1:8080"}, "127.0.0.1:8080")
	})
	
	// Тест 4: Ничего не задано (значение по умолчания)
	t.Run("Default value", func(t *testing.T) {
		testWithEnv(t, "", []string{"server"}, "localhost:8080")
	})
}


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

// TestMemStorageGetAllMetrics тестирует получение всех метрик
func TestMemStorageGetAllMetrics(t *testing.T) {
	storage := NewMemStorage()
	
	// Добавляем тестовые метрики
	storage.UpdateGauge("gauge1", 1.1)
	storage.UpdateGauge("gauge2", 2.2)
	storage.UpdateCounter("counter1", 10)
	storage.UpdateCounter("counter2", 20)
	
	gauges, counters := storage.GetAllMetrics()
	
	if len(gauges) != 2 {
		t.Errorf("Expected 2 gauge metrics, got %d", len(gauges))
	}
	if len(counters) != 2 {
		t.Errorf("Expected 2 counter metrics, got %d", len(counters))
	}
	
	if gauges["gauge1"] != 1.1 {
		t.Errorf("Expected gauge1 value 1.1, got %f", gauges["gauge1"])
	}
	if counters["counter1"] != 10 {
		t.Errorf("Expected counter1 value 10, got %d", counters["counter1"])
	}
}

// TestUpdateMetricHandler тестирует обработчик обновления метрик
func TestUpdateMetricHandler(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg, logger)
	
	// Тест успешного обновления gauge метрики
	req := httptest.NewRequest("POST", "/update/gauge/test_metric/123.45", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	
	value, err := storage.GetGauge("test_metric")
	if err != nil || value != 123.45 {
		t.Errorf("Metric not stored correctly: %v", err)
	}
	
	// Тест успешного обновления counter метрики
	req = httptest.NewRequest("POST", "/update/counter/test_counter/42", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	
	count, err := storage.GetCounter("test_counter")
	if err != nil || count != 42 {
		t.Errorf("Counter not stored correctly: %v", err)
	}
	
	// Тест неверного типа метрики
	req = httptest.NewRequest("POST", "/update/invalid/test_metric/123", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid metric type, got %d", rr.Code)
	}
	
	// Тест отсутствия имени метрики
	req = httptest.NewRequest("POST", "/update/gauge//123", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for empty metric name, got %d", rr.Code)
	}
	
	// Тест неверного значения gauge
	req = httptest.NewRequest("POST", "/update/gauge/test_metric/invalid", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid gauge value, got %d", rr.Code)
	}
	
	// Тест неверного значения counter
	req = httptest.NewRequest("POST", "/update/counter/test_counter/invalid", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid counter value, got %d", rr.Code)
	}
}

// TestGetMetricValueHandler тестирует обработчик получения значения метрики
func TestGetMetricValueHandler(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg, logger)
	
	// Подготавливаем данные
	storage.UpdateGauge("test_gauge", 99.99)
	storage.UpdateCounter("test_counter", 100)
	
	// Тест получения значения gauge метрики
	req := httptest.NewRequest("GET", "/value/gauge/test_gauge", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "99.99" {
		t.Errorf("Expected value 99.99, got %s", rr.Body.String())
	}
	
	// Тест получения значения counter метрики
	req = httptest.NewRequest("GET", "/value/counter/test_counter", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "100" {
		t.Errorf("Expected value 100, got %s", rr.Body.String())
	}
	
	// Тест получения несуществующей метрики
	req = httptest.NewRequest("GET", "/value/gauge/nonexistent", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
	
	// Тест неверного типа метрики
	req = httptest.NewRequest("GET", "/value/invalid/test_metric", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid metric type, got %d", rr.Code)
	}
}

// TestGetAllMetricsHandler тестирует обработчик получения всех метрик
func TestGetAllMetricsHandler(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg, logger)
	
	// Добавляем тестовые метрики
	storage.UpdateGauge("cpu_usage", 75.5)
	storage.UpdateCounter("request_count", 42)
	
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	
	body := rr.Body.String()
	if !strings.Contains(body, "Metrics") {
		t.Errorf("Expected HTML page with 'Metrics' title")
	}
	if !strings.Contains(body, "cpu_usage") {
		t.Errorf("Expected gauge metric 'cpu_usage' in response")
	}
	if !strings.Contains(body, "request_count") {
		t.Errorf("Expected counter metric 'request_count' in response")
	}
	if !strings.Contains(body, "75.5") {
		t.Errorf("Expected gauge value 75.5 in response")
	}
	if !strings.Contains(body, "42") {
		t.Errorf("Expected counter value 42 in response")
	}
}

// TestRouterConfiguration тестирует конфигурацию маршрутизатора
func TestRouterConfiguration(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg, logger)
	
	tests := []struct {
		method string
		path   string
		exists bool
	}{
		{"POST", "/update/gauge/test/1.0", true},
		{"POST", "/update/counter/test/1", true},
		{"GET", "/value/gauge/test", true},
		{"GET", "/value/counter/test", true},
		{"GET", "/", true},
		{"POST", "/invalid/path", false},
		{"GET", "/unknown", false},
	}
	
	for _, test := range tests {
		httptest.NewRequest(test.method, test.path, nil)
		routeContext := chi.NewRouteContext()
		
		matched := server.router.Match(routeContext, test.method, test.path)
		if matched != test.exists {
			t.Errorf("Route %s %s: expected %v, got %v", test.method, test.path, test.exists, matched)
		}
	}
}

// TestServerCreation тестирует создание сервера
func TestServerCreation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg, logger)
	
	if server.storage != storage {
		t.Error("Server storage not set correctly")
	}
	if server.config.Addr != cfg.Addr {
		t.Error("Server config not set correctly")
	}
	if server.router == nil {
		t.Error("Server router not initialized")
	}
	if server.logger != logger {
		t.Error("Server logger not set correctly")
	}
}

// Mock для os.Exit
var osExit = os.Exit

// TestMethodNotAllowed тестирует обработку неподдерживаемых методов
func TestMethodNotAllowed(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg, logger)
	
	// Тест GET для update (должен быть только POST)
	req := httptest.NewRequest("GET", "/update/gauge/test/1.0", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for GET on update endpoint, got %d", rr.Code)
	}
	
	// Тест POST для value (должен быть только GET)
	req = httptest.NewRequest("POST", "/value/gauge/test", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for POST on value endpoint, got %d", rr.Code)
	}
}

// TestServerConfigPriority тестирует приоритет конфигурации
func TestServerConfigPriority(t *testing.T) {
	// Тест приоритета: флаг > переменная окружения > значение по умолчанию
	
	// Вспомогательная функция
	testPriority := func(t *testing.T, envValue string, flagValue string, expected string) {
		// Сохраняем оригинальные args
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()
		
		// Устанавливаем переменную окружения
		if envValue != "" {
			t.Setenv("ADDRESS", envValue)
		} else {
			os.Unsetenv("ADDRESS")
		}
		
		// Устанавливаем аргументы командной строки
		if flagValue != "" {
			os.Args = []string{"server", "-a=" + flagValue}
		} else {
			os.Args = []string{"server"}
		}
		
		// Сбрасываем состояние флагов
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		
		cfg := parseServerFlags()
		
		if cfg.Addr != expected {
			t.Errorf("Expected address %s, got %s (env=%s, flag=%s)", 
				expected, cfg.Addr, envValue, flagValue)
		}
	}
	
	// Тест 1: Только переменная окружения
	t.Run("Environment only", func(t *testing.T) {
		testPriority(t, "env.example.com:9090", "", "env.example.com:9090")
	})
	
	// Тест 2: Только флаг
	t.Run("Flag only", func(t *testing.T) {
		testPriority(t, "", "flag.example.com:8080", "flag.example.com:8080")
	})
	
	// Тест 3: Флаг имеет приоритет над переменной окружения
	t.Run("Flag overrides env", func(t *testing.T) {
		testPriority(t, "env.example.com:9090", "flag.example.com:8080", "flag.example.com:8080")
	})
	
	// Тест 4: Ничего не задано - значение по умолчанию
	t.Run("Default value", func(t *testing.T) {
		testPriority(t, "", "", "localhost:8080")
	})
}

// TestLoggingMiddleware тестирует middleware логирования
func TestLoggingMiddleware(t *testing.T) {
	// Создаем тестовый логгер с буфером для проверки вывода
	var logOutput bytes.Buffer
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	
	// Создаем кодировщик, который пишет в буфер для тестов
	encoder := zap.NewProductionEncoderConfig()
	encoder.TimeKey = "timestamp"
	encoder.EncodeTime = zapcore.ISO8601TimeEncoder
	
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoder),
		zapcore.AddSync(&logOutput),
		zap.InfoLevel,
	)
	
	logger := zap.New(core)
	
	// Создаем тестовый обработчик
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	// Обертываем обработчик middleware
	middleware := loggingMiddleware(logger)
	wrappedHandler := middleware(handler)
	
	// Создаем тестовый запрос
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	
	// Выполняем запрос
	wrappedHandler.ServeHTTP(rr, req)
	
	// Проверяем ответ
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	
	if rr.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %s", rr.Body.String())
	}
	
	// Проверяем, что лог был записан
	logStr := logOutput.String()
	if !strings.Contains(logStr, "HTTP request") {
		t.Error("Expected log to contain 'HTTP request'")
	}
	if !strings.Contains(logStr, `"method":"GET"`) {
		t.Error("Expected log to contain method GET")
	}
	if !strings.Contains(logStr, `"uri":"/test"`) {
		t.Error("Expected log to contain uri /test")
	}
	if !strings.Contains(logStr, `"status":200`) {
		t.Error("Expected log to contain status 200")
	}
}

// TestResponseWriterWrapper тестирует обертку ResponseWriter
func TestResponseWriterWrapper(t *testing.T) {
	// Создаем базовый ResponseWriter
	baseWriter := httptest.NewRecorder()
	
	// Создаем обертку
	wrappedWriter := &responseWriter{
		ResponseWriter: baseWriter,
		status:         http.StatusOK,
	}
	
	// Тестируем WriteHeader
	wrappedWriter.WriteHeader(http.StatusCreated)
	
	if wrappedWriter.status != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", wrappedWriter.status)
	}
	
	if !wrappedWriter.wroteHeader {
		t.Error("Expected wroteHeader to be true")
	}
	
	// Тестируем Write
	data := []byte("Hello, World!")
	n, err := wrappedWriter.Write(data)
	
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}
	
	if wrappedWriter.size != len(data) {
		t.Errorf("Expected size %d, got %d", len(data), wrappedWriter.size)
	}
	
	// Проверяем, что данные записались в базовый writer
	if baseWriter.Body.String() != "Hello, World!" {
		t.Errorf("Expected body 'Hello, World!', got %s", baseWriter.Body.String())
	}
	
	// Проверяем повторный вызов WriteHeader (должен игнорироваться)
	wrappedWriter.WriteHeader(http.StatusBadRequest)
	if wrappedWriter.status != http.StatusCreated {
		t.Errorf("Expected status to remain 201 after second WriteHeader, got %d", wrappedWriter.status)
	}
}


// TestServerWithLogger тестирует создание сервера с логгером
func TestServerWithLogger(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg, logger)
	
	if server.logger != logger {
		t.Error("Server logger not set correctly")
	}
}

// TestUnwrapMethod тестирует метод Unwrap у responseWriter
func TestUnwrapMethod(t *testing.T) {
	baseWriter := httptest.NewRecorder()
	wrappedWriter := &responseWriter{
		ResponseWriter: baseWriter,
	}
	
	// Проверяем, что Unwrap возвращает оригинальный writer
	unwrapped := wrappedWriter.Unwrap()
	if unwrapped != baseWriter {
		t.Error("Unwrap should return the original ResponseWriter")
	}
}

// TestMiddlewareWithDifferentHTTPMethods тестирует middleware с разными HTTP методами
func TestMiddlewareWithDifferentHTTPMethods(t *testing.T) {
	var logOutput bytes.Buffer
	encoder := zap.NewProductionEncoderConfig()
	encoder.TimeKey = "timestamp"
	encoder.EncodeTime = zapcore.ISO8601TimeEncoder
	
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoder),
		zapcore.AddSync(&logOutput),
		zap.InfoLevel,
	)
	
	logger := zap.New(core)
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response"))
	})
	
	middleware := loggingMiddleware(logger)
	wrappedHandler := middleware(handler)
	
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			logOutput.Reset()
			
			req := httptest.NewRequest(method, "/test", nil)
			rr := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rr, req)
			
			logStr := logOutput.String()
			if !strings.Contains(logStr, fmt.Sprintf(`"method":"%s"`, method)) {
				t.Errorf("Expected log to contain method %s", method)
			}
		})
	}
}

// TestMiddlewareWithDifferentStatusCodes тестирует middleware с разными кодами статусов
func TestMiddlewareWithDifferentStatusCodes(t *testing.T) {
	var logOutput bytes.Buffer
	encoder := zap.NewProductionEncoderConfig()
	encoder.TimeKey = "timestamp"
	encoder.EncodeTime = zapcore.ISO8601TimeEncoder
	
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoder),
		zapcore.AddSync(&logOutput),
		zap.InfoLevel,
	)
	
	logger := zap.New(core)
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusCode, _ := strconv.Atoi(r.URL.Query().Get("code"))
		if statusCode == 0 {
			statusCode = 200
		}
		w.WriteHeader(statusCode)
		w.Write([]byte(fmt.Sprintf("Status %d", statusCode)))
	})
	
	middleware := loggingMiddleware(logger)
	wrappedHandler := middleware(handler)
	
	statusCodes := []int{200, 201, 204, 301, 400, 401, 403, 404, 500, 503}
	
	for _, code := range statusCodes {
		t.Run(fmt.Sprintf("Status%d", code), func(t *testing.T) {
			logOutput.Reset()
			
			req := httptest.NewRequest("GET", fmt.Sprintf("/test?code=%d", code), nil)
			rr := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rr, req)
			
			if rr.Code != code {
				t.Errorf("Expected status %d, got %d", code, rr.Code)
			}
			
			logStr := logOutput.String()
			if !strings.Contains(logStr, fmt.Sprintf(`"status":%d`, code)) {
				t.Errorf("Expected log to contain status %d", code)
			}
		})
	}
}

// TestMiddlewareDoesNotAffectResponse тестирует, что middleware не влияет на ответ
func TestMiddlewareDoesNotAffectResponse(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	// Создаем обработчик, который устанавливает заголовки и пишет тело
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Header", "CustomValue")
		w.WriteHeader(http.StatusCreated)
		
		response := map[string]string{"message": "Hello, World!"}
		json.NewEncoder(w).Encode(response)
	})
	
	middleware := loggingMiddleware(logger)
	wrappedHandler := middleware(handler)
	
	req := httptest.NewRequest("POST", "/api/test", nil)
	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)
	
	// Проверяем статус
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", rr.Code)
	}
	
	// Проверяем заголовки
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}
	
	if custom := rr.Header().Get("X-Custom-Header"); custom != "CustomValue" {
		t.Errorf("Expected X-Custom-Header CustomValue, got %s", custom)
	}
	
	// Проверяем тело ответа
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Errorf("Failed to decode response body: %v", err)
	}
	
	if body["message"] != "Hello, World!" {
		t.Errorf("Expected message 'Hello, World!', got %s", body["message"])
	}
}

// TestTemplateInitialization тестирует инициализацию шаблона
func TestTemplateInitialization(t *testing.T) {
	if t == nil {
		t.Error("Template should be initialized")
	}
	
	if templateError != nil {
		t.Errorf("Template should be parsed without error, got: %v", templateError)
	}
}

// TestServerIntegration тестирует интеграцию всех компонентов сервера
func TestServerIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg, logger)
	
	// Сценарий: добавляем метрики, получаем их значения, смотрим все метрики
	
	// 1. Добавляем gauge метрику
	req := httptest.NewRequest("POST", "/update/gauge/cpu_usage/75.5", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Failed to update gauge: status %d", rr.Code)
	}
	
	// 2. Добавляем counter метрику
	req = httptest.NewRequest("POST", "/update/counter/requests/1", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Failed to update counter: status %d", rr.Code)
	}
	
	// 3. Инкрементируем counter
	req = httptest.NewRequest("POST", "/update/counter/requests/2", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Failed to increment counter: status %d", rr.Code)
	}
	
	// 4. Получаем значение gauge
	req = httptest.NewRequest("GET", "/value/gauge/cpu_usage", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Failed to get gauge value: status %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "75.5" {
		t.Errorf("Expected gauge value 75.5, got %s", rr.Body.String())
	}
	
	// 5. Получаем значение counter (должно быть 1+2=3)
	req = httptest.NewRequest("GET", "/value/counter/requests", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Failed to get counter value: status %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "3" {
		t.Errorf("Expected counter value 3, got %s", rr.Body.String())
	}
	
	// 6. Получаем все метрики
	req = httptest.NewRequest("GET", "/", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Failed to get all metrics: status %d", rr.Code)
	}
	
	body := rr.Body.String()
	if !strings.Contains(body, "cpu_usage") || !strings.Contains(body, "75.5") {
		t.Error("Expected gauge metric in HTML response")
	}
	if !strings.Contains(body, "requests") || !strings.Contains(body, "3") {
		t.Error("Expected counter metric in HTML response")
	}
}

// TestResponseWriterImplicitWriteHeader тестирует неявный вызов WriteHeader через Write
func TestResponseWriterImplicitWriteHeader(t *testing.T) {
	baseWriter := httptest.NewRecorder()
	wrappedWriter := &responseWriter{
		ResponseWriter: baseWriter,
		status:         http.StatusOK,
	}
	
	// Пишем без явного вызова WriteHeader
	data := []byte("Hello")
	n, err := wrappedWriter.Write(data)
	
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}
	
	// Write должен был вызвать WriteHeader с кодом по умолчанию (200)
	if !wrappedWriter.wroteHeader {
		t.Error("Write should have triggered WriteHeader")
	}
	
	if wrappedWriter.status != http.StatusOK {
		t.Errorf("Expected implicit status 200, got %d", wrappedWriter.status)
	}
}

// TestServerErrorHandling тестирует обработку ошибок сервером
func TestServerErrorHandling(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg, logger)
	
	tests := []struct {
		name     string
		method   string
		path     string
		body     io.Reader
		expected int
	}{
		{"Invalid gauge value", "POST", "/update/gauge/metric/not-a-number", nil, http.StatusBadRequest},
		{"Invalid counter value", "POST", "/update/counter/metric/not-a-number", nil, http.StatusBadRequest},
		{"Negative counter", "POST", "/update/counter/metric/-10", nil, http.StatusOK}, // Отрицательные значения допустимы
		{"Very large number", "POST", "/update/gauge/metric/1.7976931348623157e+308", nil, http.StatusOK},
		{"Scientific notation", "POST", "/update/gauge/metric/1.23e4", nil, http.StatusOK},
		{"Metric with spaces", "POST", "/update/gauge/metric%20name/123", nil, http.StatusOK},
		{"Metric with special chars", "POST", "/update/gauge/metric-name_123/456", nil, http.StatusOK},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, tt.body)
			rr := httptest.NewRecorder()
			server.ServeHTTP(rr, req)
			
			if rr.Code != tt.expected {
				t.Errorf("%s: Expected status %d, got %d", tt.name, tt.expected, rr.Code)
			}
		})
	}
}

// TestMiddlewarePreservesOriginalWriter тестирует, что middleware сохраняет оригинальный ResponseWriter
func TestMiddlewarePreservesOriginalWriter(t *testing.T) {
	// Создаем кастомный ResponseWriter для тестирования
	type customWriter struct {
		http.ResponseWriter
		customField string
	}
	
	var logOutput bytes.Buffer
	encoder := zap.NewProductionEncoderConfig()
	encoder.TimeKey = "timestamp"
	encoder.EncodeTime = zapcore.ISO8601TimeEncoder
	
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoder),
		zapcore.AddSync(&logOutput),
		zap.InfoLevel,
	)
	
	logger := zap.New(core)
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем, что w - это наш обернутый writer
		if rw, ok := w.(*responseWriter); ok {
			// Проверяем, что оригинальный writer доступен через Unwrap
			orig := rw.Unwrap()
			if cw, ok := orig.(*customWriter); ok && cw.customField == "test" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
				return
			}
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	
	middleware := loggingMiddleware(logger)
	
	req := httptest.NewRequest("GET", "/test", nil)
	
	// Создаем кастомный writer
	customW := &customWriter{
		ResponseWriter: httptest.NewRecorder(),
		customField:    "test",
	}
	
	// Создаем handler с middleware
	handlerWithMiddleware := middleware(handler)
	
	// Выполняем запрос
	handlerWithMiddleware.ServeHTTP(customW, req)
	
	// Проверяем, что запрос обработан успешно
	if rr, ok := customW.ResponseWriter.(*httptest.ResponseRecorder); ok {
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	}
}