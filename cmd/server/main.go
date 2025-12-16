package main

import (
    "compress/gzip"
    "context"
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "html/template"
    "io"
    "log"
    "net/http"
    "os"
    "os/signal"
    "path/filepath"
    "strconv"
    "strings"
    "sync"
    "syscall"
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
    Addr            string
    StoreInterval   time.Duration // Интервал сохранения на диск (секунды)
    FileStoragePath string        // Путь к файлу сохранения
    Restore         bool          // Загружать ранее сохраненные значения
}

// Metrics структура для JSON API
type Metrics struct {
	ID    string   `json:"id"`              // имя метрики
	MType string   `json:"type"`            // параметр, принимающий значение gauge или counter
	Delta *int64   `json:"delta,omitempty"` // значение метрики в случае передачи counter
	Value *float64 `json:"value,omitempty"` // значение метрики в случае передачи gauge
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
    defaultStoreInterval := 300 * time.Second
    defaultFileStoragePath := "/tmp/metrics-db.json"
    defaultRestore := true

    // Читаем значения из переменных окружения
    if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
        defaultAddr = envAddr
        log.Printf("Using ADDRESS from environment: %s", defaultAddr)
    }
    
    if envStoreInterval := os.Getenv("STORE_INTERVAL"); envStoreInterval != "" {
        if val, err := strconv.Atoi(envStoreInterval); err == nil && val >= 0 {
            defaultStoreInterval = time.Duration(val) * time.Second
            log.Printf("Using STORE_INTERVAL from environment: %d seconds", val)
        } else {
            log.Printf("Invalid STORE_INTERVAL environment variable: %s, using default", envStoreInterval)
        }
    }
    
    if envFileStoragePath := os.Getenv("FILE_STORAGE_PATH"); envFileStoragePath != "" {
        defaultFileStoragePath = envFileStoragePath
        log.Printf("Using FILE_STORAGE_PATH from environment: %s", defaultFileStoragePath)
    }
    
    if envRestore := os.Getenv("RESTORE"); envRestore != "" {
        if val, err := strconv.ParseBool(envRestore); err == nil {
            defaultRestore = val
            log.Printf("Using RESTORE from environment: %t", defaultRestore)
        } else {
            log.Printf("Invalid RESTORE environment variable: %s, using default", envRestore)
        }
    }

    cfg := Config{
        Addr:            defaultAddr,
        StoreInterval:   defaultStoreInterval,
        FileStoragePath: defaultFileStoragePath,
        Restore:         defaultRestore,
    }

    // Объявляем переменные для флагов
    var (
        flagAddr            string
        flagStoreInterval   int
        flagFileStoragePath string
        flagRestore         bool
    )
    
    // Устанавливаем значения по умолчанию для флагов
    flag.StringVar(&flagAddr, "a", "", "HTTP server endpoint address (overrides ADDRESS env var)")
    flag.IntVar(&flagStoreInterval, "i", 0, "Store interval in seconds (overrides STORE_INTERVAL env var)")
    flag.StringVar(&flagFileStoragePath, "f", "", "File storage path (overrides FILE_STORAGE_PATH env var)")
    flag.BoolVar(&flagRestore, "r", true, "Restore metrics from file on start (overrides RESTORE env var)")

    // Проверяем неизвестные флаги
    flag.Usage = func() {
        fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
        fmt.Fprintf(flag.CommandLine.Output(), "Environment variables:\n")
        fmt.Fprintf(flag.CommandLine.Output(), "  ADDRESS           HTTP server endpoint address (default: localhost:8080)\n")
        fmt.Fprintf(flag.CommandLine.Output(), "  STORE_INTERVAL    Store interval in seconds (default: 300, 0 - sync save)\n")
        fmt.Fprintf(flag.CommandLine.Output(), "  FILE_STORAGE_PATH File storage path (default: /tmp/metrics-db.json)\n")
        fmt.Fprintf(flag.CommandLine.Output(), "  RESTORE           Restore metrics from file (default: true)\n")
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

    // Собираем информацию о том, какие флаги были установлены
    flagsSet := make(map[string]bool)
    flag.Visit(func(f *flag.Flag) {
        flagsSet[f.Name] = true
    })

    // Если флаги были переданы, используем их значения
    if flagsSet["a"] {
        cfg.Addr = flagAddr
        log.Printf("Using ADDRESS from command line flag: %s", cfg.Addr)
    }
    
    if flagsSet["i"] {
        cfg.StoreInterval = time.Duration(flagStoreInterval) * time.Second
        log.Printf("Using STORE_INTERVAL from command line flag: %d seconds", flagStoreInterval)
    }
    
    if flagsSet["f"] {
        cfg.FileStoragePath = flagFileStoragePath
        log.Printf("Using FILE_STORAGE_PATH from command line flag: %s", cfg.FileStoragePath)
    }
    
    if flagsSet["r"] {
        cfg.Restore = flagRestore
        log.Printf("Using RESTORE from command line flag: %t", cfg.Restore)
    }

    return cfg
}

// MemStorage хранит метрики в памяти с возможностью сохранения на диск
type MemStorage struct {
    gauges   map[string]float64
    counters map[string]int64
    mu       sync.RWMutex
    filePath string
    saveChan chan struct{}
    logger   *zap.Logger
}

// NewMemStorage создаёт новый экземпляр MemStorage
func NewMemStorage(filePath string, logger *zap.Logger) *MemStorage {
    return &MemStorage{
        gauges:   make(map[string]float64),
        counters: make(map[string]int64),
        filePath: filePath,
        saveChan: make(chan struct{}, 1),
        logger:   logger,
    }
}

// Save сохраняет метрики в файл
func (m *MemStorage) Save() error {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    // Создаем директорию если она не существует
    if err := os.MkdirAll(filepath.Dir(m.filePath), 0755); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }
    
    // Подготавливаем данные для сохранения
    var metrics []Metrics
    for name, value := range m.gauges {
        v := value
        metrics = append(metrics, Metrics{
            ID:    name,
            MType: Gauge,
            Value: &v,
        })
    }
    
    for name, value := range m.counters {
        v := value
        metrics = append(metrics, Metrics{
            ID:    name,
            MType: Counter,
            Delta: &v,
        })
    }
    
    // Сериализуем в JSON
    data, err := json.MarshalIndent(metrics, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal metrics: %w", err)
    }
    
    // Записываем во временный файл
    tmpFile := m.filePath + ".tmp"
    if err := os.WriteFile(tmpFile, data, 0644); err != nil {
        return fmt.Errorf("failed to write temporary file: %w", err)
    }
    
    // Атомарно заменяем старый файл новым
    if err := os.Rename(tmpFile, m.filePath); err != nil {
        return fmt.Errorf("failed to rename temporary file: %w", err)
    }
    
    m.logger.Info("Metrics saved to file",
        zap.String("file", m.filePath),
        zap.Int("gauges", len(m.gauges)),
        zap.Int("counters", len(m.counters)))
    
    return nil
}

// Load загружает метрики из файла
func (m *MemStorage) Load() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Проверяем существование файла
    if _, err := os.Stat(m.filePath); os.IsNotExist(err) {
        m.logger.Info("No existing metrics file found, starting fresh")
        return nil
    }
    
    // Читаем файл
    data, err := os.ReadFile(m.filePath)
    if err != nil {
        return fmt.Errorf("failed to read file: %w", err)
    }
    
    // Декодируем JSON
    var metrics []Metrics
    if err := json.Unmarshal(data, &metrics); err != nil {
        // Если файл поврежден, создаем новый
        m.logger.Warn("Failed to parse metrics file, starting fresh", zap.Error(err))
        return nil
    }
    
    // Загружаем метрики в память
    gaugesLoaded := 0
    countersLoaded := 0
    
    for _, metric := range metrics {
        switch metric.MType {
        case Gauge:
            if metric.Value != nil {
                m.gauges[metric.ID] = *metric.Value
                gaugesLoaded++
            }
        case Counter:
            if metric.Delta != nil {
                m.counters[metric.ID] = *metric.Delta
                countersLoaded++
            }
        }
    }
    
    m.logger.Info("Metrics loaded from file",
        zap.String("file", m.filePath),
        zap.Int("gauges", gaugesLoaded),
        zap.Int("counters", countersLoaded),
        zap.Int("total", len(metrics)))
    
    return nil
}

// RequestSave запрашивает сохранение метрик
func (m *MemStorage) RequestSave() {
    select {
    case m.saveChan <- struct{}{}:
        // Запрос отправлен
    default:
        // Уже есть ожидающий запрос
    }
}


// UpdateGauge обновляет значение метрики типа gauge
func (m *MemStorage) UpdateGauge(name string, value float64) {
    m.mu.Lock()
    m.gauges[name] = value
    m.mu.Unlock()
}

// UpdateCounter обновляет значение метрики типа counter
func (m *MemStorage) UpdateCounter(name string, value int64) {
    m.mu.Lock()
    m.counters[name] += value
    m.mu.Unlock()
}

// GetGauge возвращает значение метрики типа gauge
func (m *MemStorage) GetGauge(name string) (float64, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    value, ok := m.gauges[name]
    if !ok {
        return 0, errors.New("metric not found")
    }
    return value, nil
}

// GetCounter возвращает значение метрики типа counter
func (m *MemStorage) GetCounter(name string) (int64, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    value, ok := m.counters[name]
    if !ok {
        return 0, errors.New("metric not found")
    }
    return value, nil
}

// GetAllMetrics возвращает все метрики
func (m *MemStorage) GetAllMetrics() (map[string]float64, map[string]int64) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    // Создаем копии для безопасного доступа
    gaugesCopy := make(map[string]float64, len(m.gauges))
    countersCopy := make(map[string]int64, len(m.counters))
    
    for k, v := range m.gauges {
        gaugesCopy[k] = v
    }
    
    for k, v := range m.counters {
        countersCopy[k] = v
    }
    
    return gaugesCopy, countersCopy
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

// compressableContentType проверяет, можно ли сжимать данный тип контента
func compressableContentType(contentType string) bool {
	return strings.Contains(contentType, "application/json") || 
	       strings.Contains(contentType, "text/html") ||
	       strings.Contains(contentType, "text/plain")
}

// gzipReader обертка для чтения сжатых данных
type gzipReader struct {
	*gzip.Reader
	io.Closer
}

func (gz gzipReader) Close() error {
	return gz.Closer.Close()
}

// gzipMiddleware middleware для обработки gzip сжатия запросов и ответов
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Обработка входящего сжатого запроса
		contentEncoding := r.Header.Get("Content-Encoding")
		if strings.Contains(contentEncoding, "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Invalid gzip data", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			r.Body = gzipReader{gz, r.Body}
			r.Header.Del("Content-Encoding")
		}

		// Проверяем, поддерживает ли клиент gzip
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")

		if supportsGzip {
			// Используем gzipWriter для сжатия ответа
			gw := &gzipWriter{
				ResponseWriter: w,
				Writer:         gzip.NewWriter(w),
			}
			defer gw.Close()
			
			// Устанавливаем заголовок сжатия
			gw.Header().Set("Content-Encoding", "gzip")
			w = gw
		}

		next.ServeHTTP(w, r)
	})
}

// gzipWriter обертка для записи сжатых данных
type gzipWriter struct {
	http.ResponseWriter
	Writer *gzip.Writer
}

// Write записывает данные со сжатием
func (gw *gzipWriter) Write(b []byte) (int, error) {
	// Проверяем тип контента
	contentType := gw.Header().Get("Content-Type")
	if contentType == "" {
		// Если Content-Type не установлен, пытаемся определить
		if http.DetectContentType(b) == "text/plain; charset=utf-8" {
			contentType = "text/plain"
		}
	}

	// Сжимаем только если тип контента поддерживает сжатие
	if compressableContentType(contentType) {
		gw.Header().Set("Content-Encoding", "gzip")
		return gw.Writer.Write(b)
	}
	
	// Иначе пишем без сжатия
	return gw.ResponseWriter.Write(b)
}

// WriteHeader устанавливает заголовок ответа
func (gw *gzipWriter) WriteHeader(statusCode int) {
	// Удаляем Content-Length, так как размер после сжатия изменится
	gw.Header().Del("Content-Length")
	gw.ResponseWriter.WriteHeader(statusCode)
}

// Close закрывает writer gzip
func (gw *gzipWriter) Close() error {
	if gw.Writer != nil {
		return gw.Writer.Close()
	}
	return nil
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
				zap.String("content_encoding", r.Header.Get("Content-Encoding")),
				zap.String("accept_encoding", r.Header.Get("Accept-Encoding")),
			)
		})
	}
}

// Server структура сервера
type Server struct {
    storage      Storage
    storageFile  *MemStorage // Для доступа к методам сохранения
    router       *chi.Mux
    config       Config
    logger       *zap.Logger
    saveTicker   *time.Ticker
    stopChan     chan struct{}
    wg           sync.WaitGroup
}

// NewServer создает новый экземпляр сервера
func NewServer(storage Storage, storageFile *MemStorage, cfg Config, logger *zap.Logger) *Server {
    s := &Server{
        storage:     storage,
        storageFile: storageFile,
        router:      chi.NewRouter(),
        config:      cfg,
        logger:      logger,
        stopChan:    make(chan struct{}),
    }
    s.configureRouter()
    return s
}

// configureRouter настраивает маршруты
func (s *Server) configureRouter() {
	// Добавляем middleware gzip перед логированием
	s.router.Use(gzipMiddleware)
	
	// Добавляем middleware логирования
	s.router.Use(loggingMiddleware(s.logger))
	
	// Старые эндпоинты (оставляем для обратной совместимости)
	s.router.Route("/update", func(r chi.Router) {
		r.Post("/{metricType}/{metricName}/{metricValue}", s.updateMetricHandler)
		r.Post("/", s.updateJSONHandler) // Новый JSON эндпоинт
	})

	s.router.Route("/value", func(r chi.Router) {
		r.Get("/{metricType}/{metricName}", s.getMetricValueHandler)
		r.Post("/", s.getValueJSONHandler) // Новый JSON эндпоинт
	})

	s.router.Get("/", s.getAllMetricsHandler)
}

// updateMetricHandler обрабатывает запрос на обновление метрики (старый формат)
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

    // Если синхронное сохранение, сохраняем на диск
    if s.config.StoreInterval == 0 {
        s.storageFile.RequestSave()
    }

    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

// getMetricValueHandler обрабатывает запрос на получение значения метрики (старый формат)
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

// updateJSONHandler обрабатывает запрос на обновление метрики в формате JSON
func (s *Server) updateJSONHandler(w http.ResponseWriter, r *http.Request) {
    // Проверяем Content-Type
    contentType := r.Header.Get("Content-Type")
    if contentType != "application/json" {
        http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
        return
    }

    // Читаем тело запроса
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read request body", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    // Декодируем JSON
    var metric Metrics
    if err := json.Unmarshal(body, &metric); err != nil {
        http.Error(w, "Invalid JSON format", http.StatusBadRequest)
        return
    }

    // Валидируем данные
    if metric.ID == "" {
        http.Error(w, "Metric ID cannot be empty", http.StatusBadRequest)
        return
    }

    if metric.MType != Gauge && metric.MType != Counter {
        http.Error(w, "Invalid metric type", http.StatusBadRequest)
        return
    }

    // Обновляем метрику
    switch metric.MType {
    case Gauge:
        if metric.Value == nil {
            http.Error(w, "Value field is required for gauge metric", http.StatusBadRequest)
            return
        }
        s.storage.UpdateGauge(metric.ID, *metric.Value)
    case Counter:
        if metric.Delta == nil {
            http.Error(w, "Delta field is required for counter metric", http.StatusBadRequest)
            return
        }
        s.storage.UpdateCounter(metric.ID, *metric.Delta)
    }

    // Если синхронное сохранение, сохраняем на диск
    if s.config.StoreInterval == 0 {
        s.storageFile.RequestSave()
    }

    // Подготавливаем успешный ответ
    response := metric
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    
    if err := json.NewEncoder(w).Encode(response); err != nil {
        s.logger.Error("Failed to encode response", zap.Error(err))
        http.Error(w, "Internal server error", http.StatusInternalServerError)
    }
}

// getValueJSONHandler обрабатывает запрос на получение значения метрики в формате JSON
func (s *Server) getValueJSONHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Декодируем JSON
	var requestMetric Metrics
	if err := json.Unmarshal(body, &requestMetric); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// Валидируем данные
	if requestMetric.ID == "" {
		http.Error(w, "Metric ID cannot be empty", http.StatusBadRequest)
		return
	}

	if requestMetric.MType != Gauge && requestMetric.MType != Counter {
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}

	// Получаем значение метрики
	responseMetric := Metrics{
		ID:    requestMetric.ID,
		MType: requestMetric.MType,
	}

	switch requestMetric.MType {
	case Gauge:
		value, err := s.storage.GetGauge(requestMetric.ID)
		if err != nil {
			http.Error(w, "Metric not found", http.StatusNotFound)
			return
		}
		responseMetric.Value = &value
	case Counter:
		value, err := s.storage.GetCounter(requestMetric.ID)
		if err != nil {
			http.Error(w, "Metric not found", http.StatusNotFound)
			return
		}
		responseMetric.Delta = &value
	}

	// Отправляем ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(responseMetric); err != nil {
		s.logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
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
    
    // Создаем хранилище
    storage := NewMemStorage(cfg.FileStoragePath, logger)
    
    // Загружаем метрики из файла если нужно
    if cfg.Restore {
        if err := storage.Load(); err != nil {
            logger.Error("Failed to load metrics from file", 
                zap.String("file", cfg.FileStoragePath), 
                zap.Error(err))
        }
    } else {
        logger.Info("Skipping restore from file as per configuration")
    }
    
    server := NewServer(storage, storage, cfg, logger)
    
    // Запускаем горутину для сохранения метрик
    server.startSaver()
    
    // Обработка сигналов для graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    
    // Создаем HTTP сервер с таймаутами
    httpServer := &http.Server{
        Addr:         cfg.Addr,
        Handler:      server,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    // Запускаем сервер в горутине
    serverErr := make(chan error, 1)
    go func() {
        logger.Info("Server starting", 
            zap.String("address", cfg.Addr),
            zap.Duration("store_interval", cfg.StoreInterval),
            zap.String("file_storage_path", cfg.FileStoragePath),
            zap.Bool("restore", cfg.Restore))
        
        if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            serverErr <- err
        }
    }()
    
    // Ожидаем сигнала завершения или ошибки сервера
    select {
    case err := <-serverErr:
        logger.Fatal("Server failed to start", zap.Error(err))
    case sig := <-quit:
        logger.Info("Shutting down server", zap.String("signal", sig.String()))
        
        // Даем время на завершение обработки запросов
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        // Останавливаем сервер
        server.Stop()
        
        if err := httpServer.Shutdown(ctx); err != nil {
            logger.Error("Server forced to shutdown", zap.Error(err))
        }
        
        logger.Info("Server exited properly")
    }
}

// startSaver запускает горутину для периодического сохранения метрик
func (s *Server) startSaver() {
    // Если интервал сохранения 0, то синхронное сохранение уже обрабатывается в хендлерах
    if s.config.StoreInterval <= 0 {
        s.logger.Info("Synchronous save mode enabled")
        return
    }
    
    s.saveTicker = time.NewTicker(s.config.StoreInterval)
    s.wg.Add(1)
    
    go func() {
        defer s.wg.Done()
        defer s.saveTicker.Stop()
        
        s.logger.Info("Starting periodic saver", 
            zap.Duration("interval", s.config.StoreInterval))
        
        for {
            select {
            case <-s.saveTicker.C:
                if err := s.storageFile.Save(); err != nil {
                    s.logger.Error("Failed to save metrics", zap.Error(err))
                }
            case <-s.storageFile.saveChan:
                // Запрос на сохранение из хендлера (для синхронного режима)
                if err := s.storageFile.Save(); err != nil {
                    s.logger.Error("Failed to save metrics", zap.Error(err))
                }
            case <-s.stopChan:
                // Сохраняем перед выходом
                s.logger.Info("Saving metrics before shutdown")
                if err := s.storageFile.Save(); err != nil {
                    s.logger.Error("Failed to save metrics on shutdown", zap.Error(err))
                }
                return
            }
        }
    }()
}

// Stop останавливает сервер
func (s *Server) Stop() {
    close(s.stopChan)
    s.wg.Wait()
    s.logger.Info("Server stopped")
}