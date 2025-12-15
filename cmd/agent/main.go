package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Config конфигурация агента
type Config struct {
	ServerURL      string
	PollInterval   time.Duration
	ReportInterval time.Duration
}

// parseAgentFlags парсит флаги и переменные окружения агента
func parseAgentFlags() Config {
	// Значения по умолчанию
	cfg := Config{
		ServerURL:      "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
	}

	// Читаем значения из переменных окружения
	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		cfg.ServerURL = normalizeURL(envAddr)
		log.Printf("Using ADDRESS from environment: %s", cfg.ServerURL)
	}

	if envReport := os.Getenv("REPORT_INTERVAL"); envReport != "" {
		if val, err := strconv.Atoi(envReport); err == nil && val > 0 {
			cfg.ReportInterval = time.Duration(val) * time.Second
			log.Printf("Using REPORT_INTERVAL from environment: %d seconds", val)
		} else {
			log.Printf("Invalid REPORT_INTERVAL environment variable: %s, using default", envReport)
		}
	}

	if envPoll := os.Getenv("POLL_INTERVAL"); envPoll != "" {
		if val, err := strconv.Atoi(envPoll); err == nil && val > 0 {
			cfg.PollInterval = time.Duration(val) * time.Second
			log.Printf("Using POLL_INTERVAL from environment: %d seconds", val)
		} else {
			log.Printf("Invalid POLL_INTERVAL environment variable: %s, using default", envPoll)
		}
	}

	// Парсим флаги командной строки (они имеют приоритет над переменными окружения)
	var pollIntervalSec, reportIntervalSec int
	var serverURL string

	// Устанавливаем текущие значения как значения по умолчанию для флагов
	flag.StringVar(&serverURL, "a", "", "HTTP server endpoint address (overrides ADDRESS env var)")
	flag.IntVar(&reportIntervalSec, "r", 0, "Report interval in seconds (overrides REPORT_INTERVAL env var)")
	flag.IntVar(&pollIntervalSec, "p", 0, "Poll interval in seconds (overrides POLL_INTERVAL env var)")

	// Проверяем неизвестные флаги
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Environment variables:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  ADDRESS          HTTP server endpoint address (default: http://localhost:8080)\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  REPORT_INTERVAL  Report interval in seconds (default: 10)\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  POLL_INTERVAL    Poll interval in seconds (default: 2)\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\nCommand line flags (override environment variables):\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nUnknown flags will cause the application to exit with an error.\n")
	}

	flag.Parse()

	// Проверяем наличие неизвестных аргументов
	if len(flag.Args()) > 0 {
		fmt.Fprintf(flag.CommandLine.Output(), "Error: unknown flags or arguments: %v\n", flag.Args())
		flag.Usage()
		os.Exit(1)
	}

	// Применяем значения флагов (если они были установлены)
	if serverURL != "" {
		cfg.ServerURL = normalizeURL(serverURL)
		log.Printf("Using ADDRESS from command line flag: %s", cfg.ServerURL)
	}

	if reportIntervalSec > 0 {
		cfg.ReportInterval = time.Duration(reportIntervalSec) * time.Second
		log.Printf("Using REPORT_INTERVAL from command line flag: %d seconds", reportIntervalSec)
	}

	if pollIntervalSec > 0 {
		cfg.PollInterval = time.Duration(pollIntervalSec) * time.Second
		log.Printf("Using POLL_INTERVAL from command line flag: %d seconds", pollIntervalSec)
	}

	return cfg
}

// normalizeURL добавляет схему http:// если она отсутствует
func normalizeURL(rawURL string) string {
	if rawURL == "" {
		return "http://localhost:8080"
	}

	// Если URL уже содержит схему, возвращаем как есть
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}

	// Добавляем схему по умолчанию
	// Если содержит порт (например, localhost:8080), добавляем http://
	if strings.Contains(rawURL, ":") {
		return "http://" + rawURL
	}

	// Для простых имен хостов добавляем порт по умолчанию
	return "http://" + rawURL + ":8080"
}

// validateURL проверяет валидность URL
func validateURL(rawURL string) error {
	_, err := url.ParseRequestURI(rawURL)
	return err
}

// Agent структура агента
type Agent struct {
	config  Config
	gauges   map[string]float64
	counters map[string]int64
	client  *http.Client
}

// NewAgent создает новый экземпляр агента
func NewAgent(cfg Config) *Agent {
	// Валидируем URL
	if err := validateURL(cfg.ServerURL); err != nil {
		log.Printf("Warning: Invalid server URL %s: %v", cfg.ServerURL, err)
	}

	return &Agent{
		config:  cfg,
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// collectRuntimeMetrics собирает метрики из пакета runtime
func (a *Agent) collectRuntimeMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	a.gauges["Alloc"] = float64(memStats.Alloc)
	a.gauges["BuckHashSys"] = float64(memStats.BuckHashSys)
	a.gauges["Frees"] = float64(memStats.Frees)
	a.gauges["GCCPUFraction"] = memStats.GCCPUFraction
	a.gauges["GCSys"] = float64(memStats.GCSys)
	a.gauges["HeapAlloc"] = float64(memStats.HeapAlloc)
	a.gauges["HeapIdle"] = float64(memStats.HeapIdle)
	a.gauges["HeapInuse"] = float64(memStats.HeapInuse)
	a.gauges["HeapObjects"] = float64(memStats.HeapObjects)
	a.gauges["HeapReleased"] = float64(memStats.HeapReleased)
	a.gauges["HeapSys"] = float64(memStats.HeapSys)
	a.gauges["LastGC"] = float64(memStats.LastGC)
	a.gauges["Lookups"] = float64(memStats.Lookups)
	a.gauges["MCacheInuse"] = float64(memStats.MCacheInuse)
	a.gauges["MCacheSys"] = float64(memStats.MCacheSys)
	a.gauges["MSpanInuse"] = float64(memStats.MSpanInuse)
	a.gauges["MSpanSys"] = float64(memStats.MSpanSys)
	a.gauges["Mallocs"] = float64(memStats.Mallocs)
	a.gauges["NextGC"] = float64(memStats.NextGC)
	a.gauges["NumForcedGC"] = float64(memStats.NumForcedGC)
	a.gauges["NumGC"] = float64(memStats.NumGC)
	a.gauges["OtherSys"] = float64(memStats.OtherSys)
	a.gauges["PauseTotalNs"] = float64(memStats.PauseTotalNs)
	a.gauges["StackInuse"] = float64(memStats.StackInuse)
	a.gauges["StackSys"] = float64(memStats.StackSys)
	a.gauges["Sys"] = float64(memStats.Sys)
	a.gauges["TotalAlloc"] = float64(memStats.TotalAlloc)
}

// collectCustomMetrics собирает кастомные метрики
func (a *Agent) collectCustomMetrics() {
	// PollCount - счетчик обновлений
	if count, ok := a.counters["PollCount"]; ok {
		a.counters["PollCount"] = count + 1
	} else {
		a.counters["PollCount"] = int64(1)
	}
	
	// RandomValue - случайное значение
	a.gauges["RandomValue"] = rand.Float64()
}

// collectMetrics собирает все метрики
func (a *Agent) collectMetrics() {
	a.collectRuntimeMetrics()
	a.collectCustomMetrics()
}

// sendMetric отправляет одну метрику на сервер
func (a *Agent) sendMetric(metricType, name string, value interface{}) error {
	var valueStr string
	
	switch v := value.(type) {
	case float64:
		valueStr = strconv.FormatFloat(v, 'f', -1, 64)
	case int64:
		valueStr = strconv.FormatInt(v, 10)
	default:
		return fmt.Errorf("unsupported metric type: %T", value)
	}
	
	url := fmt.Sprintf("%s/update/%s/%s/%s", 
		a.config.ServerURL, metricType, name, valueStr)
	
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}
	
	return nil
}

// sendMetrics отправляет все метрики на сервер
func (a *Agent) sendMetrics() {
	allMetrics := len(a.counters) + len(a.gauges)
	log.Printf("Sending %d metrics to %s", allMetrics, a.config.ServerURL)
	
	sentCount := 0

	for name, value := range a.gauges {
		if err := a.sendMetric("gauge", name, value); err != nil {
			log.Printf("Failed to send metric %s: %v", name, err)
		} else {
			sentCount++
		}
	}
	
	for name, value := range a.counters {
		if err := a.sendMetric("counter", name, value); err != nil {
			log.Printf("Failed to send metric %s: %v", name, err)
		} else {
			sentCount++
		}
	}
	
	log.Printf("Successfully sent %d metrics", sentCount)
}

// Run запускает агента
func (a *Agent) Run() {
	pollTicker := time.NewTicker(a.config.PollInterval)
	reportTicker := time.NewTicker(a.config.ReportInterval)
	
	defer pollTicker.Stop()
	defer reportTicker.Stop()
	
	log.Printf("Agent started with configuration:")
	log.Printf("  Server URL: %s", a.config.ServerURL)
	log.Printf("  Poll interval: %v", a.config.PollInterval)
	log.Printf("  Report interval: %v", a.config.ReportInterval)
	log.Printf("  Source: environment variables and command line flags")
	
	// Собираем метрики сразу при старте
	a.collectMetrics()
	
	for {
		select {
		case <-pollTicker.C:
			a.collectMetrics()
			allMetrics := len(a.counters) + len(a.gauges)
			log.Printf("Collected %d metrics at %v", allMetrics, time.Now().Format("15:04:05"))
			
		case <-reportTicker.C:
			a.sendMetrics()
		}
	}
}

func main() {
	cfg := parseAgentFlags()
	agent := NewAgent(cfg)
	agent.Run()
}