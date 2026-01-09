package sender

import (
	"bytes"
	"net/url"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ponuch/service_metrics/internal/agent/config"
	models "github.com/ponuch/service_metrics/internal/model"
	"go.uber.org/zap"
)

type MetricSender struct{
	cfg config.AgentConfig
	client  *http.Client
	logger *zap.Logger

}

func NewSender(cfg config.AgentConfig, logger *zap.Logger) *MetricSender {
	return &MetricSender{cfg,
						&http.Client{Timeout: 10 * time.Second},
						 logger}
}

func (s *MetricSender) sendMetricJSON(threshold int, serverURL, metricType, name string, value interface{}) error {
	var metric models.Metrics
	metric.ID = name
	metric.MType = metricType

	switch v := value.(type) {
	case float64:
		metric.Value = &v
	case int64:
		metric.Delta = &v
	default:
		return fmt.Errorf("unsupported metric type: %T", value)
	}

	// Сериализуем в JSON
	jsonData, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	var body io.Reader
	var compressed bool
	
	// Проверяем размер данных и сжимаем если нужно
	if len(jsonData) > threshold {
		compressedData, err := compressData(jsonData)
		if err != nil {
			log.Printf("Failed to compress data: %v, sending uncompressed", err)
			body = bytes.NewBuffer(jsonData)
		} else {
			body = bytes.NewBuffer(compressedData)
			compressed = true
			log.Printf("Compressed data: %d -> %d bytes (%.1f%%)", 
				len(jsonData), len(compressedData), 
				float64(len(compressedData))/float64(len(jsonData))*100)
		}
	} else {
		body = bytes.NewBuffer(jsonData)
	}

	baseURL, err := url.Parse(serverURL)

	if err != nil {
		return fmt.Errorf("failed to parse server url: %w", err)
	}

	url := &url.URL{
		Scheme: baseURL.Scheme,
		Host: baseURL.Host,
		Path: "update",
	}
	
	req, err := http.NewRequest("POST", url.String(), body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// Добавляем заголовок Accept-Encoding для получения сжатых ответов
	req.Header.Set("Accept-Encoding", "gzip")
	
	// Если данные сжаты, добавляем заголовок Content-Encoding
	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}
	
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}
	
	// Проверяем Content-Type ответа
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		log.Printf("Warning: Unexpected Content-Type in response: %s", contentType)
	}
	
	// Обрабатываем сжатый ответ если нужно
	var reader io.Reader = resp.Body
	contentEncoding := resp.Header.Get("Content-Encoding")
	if strings.Contains(contentEncoding, "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gz.Close()
		reader = gz
	}
	
	// Декодируем ответ для проверки
	var responseMetric models.Metrics
	if err := json.NewDecoder(reader).Decode(&responseMetric); err != nil {
		return fmt.Errorf("failed to decode response JSON: %w", err)
	}
	
	// Проверяем, что ответ корректный
	if responseMetric.ID != name || responseMetric.MType != metricType {
		return fmt.Errorf("invalid response from server")
	}
	
	return nil
}

// sendMetricLegacy отправляет одну метрику на сервер в старом формате
func (s *MetricSender) sendMetricLegacy(serverURL, metricType, name string, value interface{}) error {
	var valueStr string
	
	switch v := value.(type) {
	case float64:
		valueStr = strconv.FormatFloat(v, 'f', -1, 64)
	case int64:
		valueStr = strconv.FormatInt(v, 10)
	default:
		return fmt.Errorf("unsupported metric type: %T", value)
	}
	url, err := buildURL(serverURL, metricType, name, valueStr)

	if err != nil {
		return fmt.Errorf("failed to create url: %w", err)
	}
	
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	// Добавляем заголовок Accept-Encoding для получения сжатых ответов
	req.Header.Set("Accept-Encoding", "gzip")
	
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}
	
	return nil
}

func buildURL (serverURL, metricType, name, valueStr string) (string, error) {
	baseURL, err := url.Parse(serverURL)

	if err != nil {
		return "", err
	}

	fixMetricType := url.PathEscape(metricType)
	fixName := url.PathEscape(name)
	fixValueStr := url.PathEscape(valueStr)

	baseURL.Path = path.Join(baseURL.Path, "update", fixMetricType, fixName, fixValueStr)
	
	return baseURL.String(), nil

}

// sendMetric отправляет одну метрику на сервер (использует новый JSON формат)
func (s *MetricSender) sendMetric(threshold int, serverURL, metricType, name string, value interface{}) error {
	return s.sendMetricJSON(threshold, serverURL, metricType, name, value)
}

// sendMetrics отправляет все метрики на сервер
func (s *MetricSender) SendMetrics(gauges map[string]float64, counters map[string]int64, serverURL string, threshold int) {
	allMetrics := len(counters) + len(gauges)
	log.Printf("Sending %d metrics to %s", allMetrics, serverURL)

	sentCount := 0

	for name, value := range gauges {
		if err := s.sendMetric(threshold, serverURL, "gauge", name, value); err != nil {
			log.Printf("Failed to send metric %s: %v", name, err)
		} else {
			sentCount++
		}
	}

	for name, value := range counters {
		if err := s.sendMetric(threshold, serverURL, "counter", name, value); err != nil {
			log.Printf("Failed to send metric %s: %v", name, err)
		} else {
			sentCount++
		}
	}

	log.Printf("Successfully sent %d metrics", sentCount)
}

// compressData сжимает данные с использованием gzip
func compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	
	if err := gz.Close(); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}