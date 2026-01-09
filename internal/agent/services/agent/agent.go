package mainagent

import (
	"log"
	"net/url"
	"time"

	"github.com/ponuch/service_metrics/internal/agent/config"
	"github.com/ponuch/service_metrics/internal/agent/model/storage"
	"github.com/ponuch/service_metrics/internal/agent/services/collector"
	"github.com/ponuch/service_metrics/internal/agent/services/logger"
	"github.com/ponuch/service_metrics/internal/agent/services/sender"

	"go.uber.org/zap"
)

type Agent struct {
	config    config.AgentConfig
	collector *collector.MetricCollector
	storage   *storage.MemStorage
	sender    *sender.MetricSender
	logger    *zap.Logger
}

// NewAgent создает новый агент
func NewAgent(cfg config.AgentConfig) *Agent {

	// Валидируем URL
	if err := validateURL(cfg.ServerURL); err != nil {
		log.Printf("Warning: Invalid server URL %s: %v", cfg.ServerURL, err)
	}

	logger := logger.NewLogger()
	return &Agent{
		config:    cfg,
		collector: collector.NewCollector(),
		storage:   storage.NewMemStorage(),
		sender:    sender.NewSender(cfg, logger),
		logger:    logger,
	}
}

func (a *Agent) collectMetrics() {
	a.storage.StoreGaugesMetrics(a.collector.CollectRuntimeMetrics())
	a.storage.StoreCountersMetrics(a.collector.CollectCustomMetrics())
}

func (a *Agent) sendMetrics() {
	a.sender.SendMetrics(a.storage.GetGauges(), a.storage.GetCounters(),
	 					 a.config.ServerURL, a.config.CompressThreshold)
}

// validateURL проверяет валидность URL
func validateURL(rawURL string) error {
	_, err := url.ParseRequestURI(rawURL)
	return err
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
	log.Printf("  Compression threshold: %d bytes", a.config.CompressThreshold)
	log.Printf("  Using JSON API format with gzip compression")
	
	// Собираем метрики сразу при старте
	a.collectMetrics()
	
	for {
		select {
		case <-pollTicker.C:
			a.collectMetrics()
			allMetrics := len(a.storage.GetCounters()) + len(a.storage.GetGauges())
			log.Printf("Collected %d metrics at %v", allMetrics, time.Now().Format("15:04:05"))

		case <-reportTicker.C:
			a.sendMetrics()
		}
	}
}

