package config

import (
	"os"
	"fmt"
	"time"
	"log"
	"strings"
	"strconv"
	"flag"
)

type AgentConfig struct {
	ServerURL      string
	PollInterval   time.Duration
	ReportInterval time.Duration
	CompressThreshold int // Порог для сжатия в байтах
}

func New() *AgentConfig {
	config := parseAgentFlags()
	return config
}

// parseAgentFlags парсит флаги и переменные окружения агента
func parseAgentFlags() *AgentConfig {
	// Значения по умолчанию
	cfg := AgentConfig{
		ServerURL:      "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		CompressThreshold: 1024, // 1KB порог для сжатия
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

	return &cfg
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