package main

import (
	"net/http"
	"net/http/httptest"
	"os"
    "flag"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
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
	
	// Тест 4: Ничего не задано (значение по умолчанию)
	t.Run("Default value", func(t *testing.T) {
		testWithEnv(t, "", []string{"server"}, "localhost:8080")
	})
	
	// Тест 5: Пустая переменная окружения и флаг
	t.Run("Empty env variable and flag", func(t *testing.T) {
		testWithEnv(t, "", []string{"server", "-a=127.0.0.1:8080"}, "127.0.0.1:8080")
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
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg)
	
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
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg)
	
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
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg)
	
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
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg)
	
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
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg)
	
	if server.storage != storage {
		t.Error("Server storage not set correctly")
	}
	if server.config.Addr != cfg.Addr {
		t.Error("Server config not set correctly")
	}
	if server.router == nil {
		t.Error("Server router not initialized")
	}
}

// Mock для os.Exit
var osExit = os.Exit

// TestMethodNotAllowed тестирует обработку неподдерживаемых методов
func TestMethodNotAllowed(t *testing.T) {
	storage := NewMemStorage()
	cfg := Config{Addr: "localhost:8080"}
	server := NewServer(storage, cfg)
	
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
	
	// Тест 5: Пустая переменная окружения, нет флага
	t.Run("Empty env, no flag", func(t *testing.T) {
		testPriority(t, "", "", "localhost:8080")
	})
}