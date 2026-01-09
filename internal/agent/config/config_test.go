package config

import (
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

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

// TestAgentWithEnvironmentVariables тестирует переменные окружения
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
	
	agentConfig := New()
	
	// Проверяем, что конфигурация загружена из переменных окружения
	if !strings.Contains(agentConfig.ServerURL, serverURL) {
		t.Errorf("Expected server URL to contain %s, got %s", serverURL, agentConfig.ServerURL)
	}
	if agentConfig.ReportInterval != 1*time.Second {
		t.Errorf("Expected report interval 1s from env, got %v", agentConfig.ReportInterval)
	}
	if agentConfig.PollInterval != 1*time.Second {
		t.Errorf("Expected poll interval 1s from env, got %v", agentConfig.PollInterval)
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
