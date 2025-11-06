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

// parseAgentFlags парсит флаги агента
func parseAgentFlags() Config {
	cfg := Config{
		ServerURL:      "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
	}

	var pollIntervalSec, reportIntervalSec int

	flag.StringVar(&cfg.ServerURL, "a", cfg.ServerURL, "HTTP server endpoint address")
	flag.IntVar(&reportIntervalSec, "r", 10, "Report interval in seconds")
	flag.IntVar(&pollIntervalSec, "p", 2, "Poll interval in seconds")

	// Проверяем неизвестные флаги
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
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

	// Преобразуем секунды в Duration
	cfg.PollInterval = time.Duration(pollIntervalSec) * time.Second
	cfg.ReportInterval = time.Duration(reportIntervalSec) * time.Second

	// Нормализуем URL - добавляем схему если отсутствует
	cfg.ServerURL = normalizeURL(cfg.ServerURL)

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
	metrics map[string]interface{}
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
		metrics: make(map[string]interface{}),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// collectRuntimeMetrics собирает метрики из пакета runtime
func (a *Agent) collectRuntimeMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	a.metrics["Alloc"] = float64(memStats.Alloc)
	a.metrics["BuckHashSys"] = float64(memStats.BuckHashSys)
	a.metrics["Frees"] = float64(memStats.Frees)
	a.metrics["GCCPUFraction"] = memStats.GCCPUFraction
	a.metrics["GCSys"] = float64(memStats.GCSys)
	a.metrics["HeapAlloc"] = float64(memStats.HeapAlloc)
	a.metrics["HeapIdle"] = float64(memStats.HeapIdle)
	a.metrics["HeapInuse"] = float64(memStats.HeapInuse)
	a.metrics["HeapObjects"] = float64(memStats.HeapObjects)
	a.metrics["HeapReleased"] = float64(memStats.HeapReleased)
	a.metrics["HeapSys"] = float64(memStats.HeapSys)
	a.metrics["LastGC"] = float64(memStats.LastGC)
	a.metrics["Lookups"] = float64(memStats.Lookups)
	a.metrics["MCacheInuse"] = float64(memStats.MCacheInuse)
	a.metrics["MCacheSys"] = float64(memStats.MCacheSys)
	a.metrics["MSpanInuse"] = float64(memStats.MSpanInuse)
	a.metrics["MSpanSys"] = float64(memStats.MSpanSys)
	a.metrics["Mallocs"] = float64(memStats.Mallocs)
	a.metrics["NextGC"] = float64(memStats.NextGC)
	a.metrics["NumForcedGC"] = float64(memStats.NumForcedGC)
	a.metrics["NumGC"] = float64(memStats.NumGC)
	a.metrics["OtherSys"] = float64(memStats.OtherSys)
	a.metrics["PauseTotalNs"] = float64(memStats.PauseTotalNs)
	a.metrics["StackInuse"] = float64(memStats.StackInuse)
	a.metrics["StackSys"] = float64(memStats.StackSys)
	a.metrics["Sys"] = float64(memStats.Sys)
	a.metrics["TotalAlloc"] = float64(memStats.TotalAlloc)
}

// collectCustomMetrics собирает кастомные метрики
func (a *Agent) collectCustomMetrics() {
	// PollCount - счетчик обновлений
	if count, ok := a.metrics["PollCount"].(int64); ok {
		a.metrics["PollCount"] = count + 1
	} else {
		a.metrics["PollCount"] = int64(1)
	}
	
	// RandomValue - случайное значение
	a.metrics["RandomValue"] = rand.Float64()
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
	log.Printf("Sending %d metrics to %s", len(a.metrics), a.config.ServerURL)
	
	sentCount := 0
	for name, value := range a.metrics {
		var metricType string
		
		switch value.(type) {
		case float64:
			metricType = "gauge"
		case int64:
			metricType = "counter"
		default:
			log.Printf("Unknown type for metric %s: %T", name, value)
			continue
		}
		
		if err := a.sendMetric(metricType, name, value); err != nil {
			log.Printf("Failed to send metric %s: %v", name, err)
		} else {
			sentCount++
		}
	}
	
	log.Printf("Successfully sent %d metrics", sentCount)
}

// Run запускает агент
func (a *Agent) Run() {
	pollTicker := time.NewTicker(a.config.PollInterval)
	reportTicker := time.NewTicker(a.config.ReportInterval)
	
	defer pollTicker.Stop()
	defer reportTicker.Stop()
	
	log.Printf("Agent started with:")
	log.Printf("  Server URL: %s", a.config.ServerURL)
	log.Printf("  Poll interval: %v", a.config.PollInterval)
	log.Printf("  Report interval: %v", a.config.ReportInterval)
	
	// Собираем метрики сразу при старте
	a.collectMetrics()
	
	for {
		select {
		case <-pollTicker.C:
			a.collectMetrics()
			log.Printf("Collected %d metrics at %v", len(a.metrics), time.Now().Format("15:04:05"))
			
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