package logger

import (
	"log"
	"go.uber.org/zap"
)

func NewLogger() *zap.Logger {
	// Инициализация логгера zap
    logger, err := zap.NewProduction()
    if err != nil {
        log.Fatalf("Failed to create logger: %v", err)
    }
    
    // Заменяем стандартный логгер на zap
    zap.ReplaceGlobals(logger)

	return logger
}