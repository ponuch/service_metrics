package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Типы метрик
const (
	Gauge   = "gauge"
	Counter = "counter"
)

// Config конфигурация сервера
type Config struct {
	Addr string
}

var t *template.Template
var templateError error

// инициализация темплейта
func init() {
	// HTML шаблон
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Metrics</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        h2 { color: #333; }
    </style>
</head>
<body>
    <h1>Metrics</h1>
    
    <h2>Gauge Metrics</h2>
    <table>
        <tr>
            <th>Name</th>
            <th>Value</th>
        </tr>
        {{range $name, $value := .Gauges}}
        <tr>
            <td>{{$name}}</td>
            <td>{{$value}}</td>
        </tr>
        {{end}}
    </table>

    <h2>Counter Metrics</h2>
    <table>
        <tr>
            <th>Name</th>
            <th>Value</th>
        </tr>
        {{range $name, $value := .Counters}}
        <tr>
            <td>{{$name}}</td>
            <td>{{$value}}</td>
        </tr>
        {{end}}
    </table>
</body>
</html>
`
	// Парсинг и выполнение шаблона
	t, templateError = template.New("metrics").Parse(tmpl)
	if templateError != nil {
		log.Fatal("Error generating page")
		return
	}
}

// parseServerFlags парсит флаги и переменные окружения сервера
func parseServerFlags() Config {
	// Значения по умолчанию
	defaultAddr := "localhost:8080"

	// Читаем значение из переменной окружения ADDRESS
	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		defaultAddr = envAddr
		log.Printf("Using ADDRESS from environment: %s", defaultAddr)
	}

	cfg := Config{
		Addr: defaultAddr,
	}

	// Объявляем переменную для флага
	var flagAddr string
	
	// Устанавливаем текущее значение как значение по умолчанию для флага
	flag.StringVar(&flagAddr, "a", "", "HTTP server endpoint address (overrides ADDRESS env var)")

	// Проверяем неизвестные флаги
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Environment variables:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  ADDRESS          HTTP server endpoint address (default: localhost:8080)\n")
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

	// Если флаг был передан, используем его значение
	if flagAddr != "" {
		cfg.Addr = flagAddr
		log.Printf("Using ADDRESS from command line flag: %s", cfg.Addr)
	}

	return cfg
}

// MemStorage хранит метрики в памяти
type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
}

// NewMemStorage создаёт новый экземпляр MemStorage
func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

// UpdateGauge обновляет значение метрики типа gauge
func (m *MemStorage) UpdateGauge(name string, value float64) {
	m.gauges[name] = value
}

// UpdateCounter обновляет значение метрики типа counter
func (m *MemStorage) UpdateCounter(name string, value int64) {
	m.counters[name] += value
}

// GetGauge возвращает значение метрики типа gauge
func (m *MemStorage) GetGauge(name string) (float64, error) {
	value, ok := m.gauges[name]
	if !ok {
		return 0, errors.New("metric not found")
	}
	return value, nil
}

// GetCounter возвращает значение метрики типа counter
func (m *MemStorage) GetCounter(name string) (int64, error) {
	value, ok := m.counters[name]
	if !ok {
		return 0, errors.New("metric not found")
	}
	return value, nil
}

// GetAllMetrics возвращает все метрики
func (m *MemStorage) GetAllMetrics() (map[string]float64, map[string]int64) {
	return m.gauges, m.counters
}

// Storage определяет интерфейс для работы с хранилищем метрик
type Storage interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
	GetGauge(name string) (float64, error)
	GetCounter(name string) (int64, error)
	GetAllMetrics() (map[string]float64, map[string]int64)
}

// responseWriter обертка для ResponseWriter для захвата статуса и размера
type responseWriter struct {
	http.ResponseWriter
	status      int
	size        int
	wroteHeader bool
}

// WriteHeader перехватывает статус ответа
func (rw *responseWriter) WriteHeader(status int) {
	if rw.wroteHeader {
		return
	}
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
	rw.wroteHeader = true
}

// Write перехватывает размер ответа
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// Unwrap возвращает оригинальный ResponseWriter (для совместимости)
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// loggingMiddleware middleware для логирования запросов и ответов
func loggingMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Создаем обертку для ResponseWriter
			wrappedWriter := &responseWriter{
				ResponseWriter: w,
				status:         http.StatusOK, // Значение по умолчанию
			}
			
			// Обрабатываем запрос
			next.ServeHTTP(wrappedWriter, r)
			
			// Вычисляем длительность
			duration := time.Since(start)
			
			// Логируем информацию о запросе и ответе
			logger.Info("HTTP request",
				zap.String("method", r.Method),
				zap.String("uri", r.RequestURI),
				zap.Int("status", wrappedWriter.status),
				zap.Int("size", wrappedWriter.size),
				zap.Duration("duration", duration),
			)
		})
	}
}

// Server структура сервера
type Server struct {
	storage Storage
	router  *chi.Mux
	config  Config
	logger  *zap.Logger
}

// NewServer создает новый экземпляр сервера
func NewServer(storage Storage, cfg Config, logger *zap.Logger) *Server {
	s := &Server{
		storage: storage,
		router:  chi.NewRouter(),
		config:  cfg,
		logger:  logger,
	}
	s.configureRouter()
	return s
}

// configureRouter настраивает маршруты
func (s *Server) configureRouter() {
	// Добавляем middleware логирования
	s.router.Use(loggingMiddleware(s.logger))
	
	s.router.Route("/update", func(r chi.Router) {
		r.Post("/{metricType}/{metricName}/{metricValue}", s.updateMetricHandler)
	})

	s.router.Route("/value", func(r chi.Router) {
		r.Get("/{metricType}/{metricName}", s.getMetricValueHandler)
	})

	s.router.Get("/", s.getAllMetricsHandler)
}

// updateMetricHandler обрабатывает запрос на обновление метрики
func (s *Server) updateMetricHandler(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "metricType")
	metricName := chi.URLParam(r, "metricName")
	metricValue := chi.URLParam(r, "metricValue")

	if metricName == "" {
		http.Error(w, "Metric name cannot be empty", http.StatusNotFound)
		return
	}

	switch metricType {
	case Gauge:
		value, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			http.Error(w, "Invalid gauge value", http.StatusBadRequest)
			return
		}
		s.storage.UpdateGauge(metricName, value)
	case Counter:
		value, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			http.Error(w, "Invalid counter value", http.StatusBadRequest)
			return
		}
		s.storage.UpdateCounter(metricName, value)
	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// getMetricValueHandler обрабатывает запрос на получение значения метрики
func (s *Server) getMetricValueHandler(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "metricType")
	metricName := chi.URLParam(r, "metricName")

	if metricName == "" {
		http.Error(w, "Metric name cannot be empty", http.StatusNotFound)
		return
	}

	var value string

	switch metricType {
	case Gauge:
		val, err := s.storage.GetGauge(metricName)
		if err != nil {
			http.Error(w, "Metric not found", http.StatusNotFound)
			return
		}
		value = strconv.FormatFloat(val, 'f', -1, 64)
	case Counter:
		val, err := s.storage.GetCounter(metricName)
		if err != nil {
			http.Error(w, "Metric not found", http.StatusNotFound)
			return
		}
		value = strconv.FormatInt(val, 10)
	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(value))
}

// getAllMetricsHandler обрабатывает запрос на получение всех метрик
func (s *Server) getAllMetricsHandler(w http.ResponseWriter, r *http.Request) {
	gauges, counters := s.storage.GetAllMetrics()

	// Данные для шаблона
	data := struct {
		Gauges   map[string]float64
		Counters map[string]int64
	}{
		Gauges:   gauges,
		Counters: counters,
	}

	w.Header().Set("Content-Type", "text/html")
	err := t.Execute(w, data)
	if err != nil {
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
}

// ServeHTTP реализует интерфейс http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func main() {
	// Инициализация логгера zap
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()
	
	// Заменяем стандартный логгер на zap
	zap.ReplaceGlobals(logger)
	
	cfg := parseServerFlags()
	storage := NewMemStorage()
	server := NewServer(storage, cfg, logger)

	logger.Info("Server starting", zap.String("address", cfg.Addr))
	
	if err := http.ListenAndServe(cfg.Addr, server); err != nil {
		logger.Fatal("Server failed to start", zap.Error(err))
	}
}