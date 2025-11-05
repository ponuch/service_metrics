package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestServerHandlers(t *testing.T) {
	storage := NewMemStorage()
	server := NewServer(storage)

	// Тест обновления gauge метрики
	req := httptest.NewRequest("POST", "/update/gauge/test_metric/123.45", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Тест обновления counter метрики
	req = httptest.NewRequest("POST", "/update/counter/test_counter/10", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Тест получения значения gauge метрики
	req = httptest.NewRequest("GET", "/value/gauge/test_metric", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "123.45" {
		t.Errorf("Expected value 123.45, got %s", rr.Body.String())
	}

	// Тест получения значения counter метрики
	req = httptest.NewRequest("GET", "/value/counter/test_counter", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "10" {
		t.Errorf("Expected value 10, got %s", rr.Body.String())
	}

	// Тест получения несуществующей метрики
	req = httptest.NewRequest("GET", "/value/gauge/nonexistent", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}

	// Тест получения главной страницы
	req = httptest.NewRequest("GET", "/", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Metrics") {
		t.Errorf("Expected HTML page with metrics, got %s", rr.Body.String())
	}
}

func TestRouterConfiguration(t *testing.T) {
	storage := NewMemStorage()
	server := NewServer(storage)

	// Проверка маршрутов
	tests := []struct {
		method string
		path   string
	}{
		{"POST", "/update/gauge/test/1.0"},
		{"POST", "/update/counter/test/1"},
		{"GET", "/value/gauge/test"},
		{"GET", "/value/counter/test"},
		{"GET", "/"},
	}

	for _, test := range tests {
		httptest.NewRequest(test.method, test.path, nil)
		routeContext := chi.NewRouteContext()
		
		// Проверяем, что маршрут существует
		if !server.router.Match(routeContext, test.method, test.path) {
			t.Errorf("Route not found: %s %s", test.method, test.path)
		}
	}
}