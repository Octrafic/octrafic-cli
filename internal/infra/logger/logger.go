package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
)

// Init initializes the logger
func Init(debugEnabled bool, logFilePath string) error {
	var err error

	// Parse log level
	level := zapcore.InfoLevel
	if debugEnabled {
		level = zapcore.DebugLevel
	}

	// Configure encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// Build config
	var config zap.Config
	if logFilePath != "" {
		// File logging only (no console output)
		config = zap.Config{
			Level:            zap.NewAtomicLevelAt(level),
			Encoding:         "json",
			EncoderConfig:    encoderConfig,
			OutputPaths:      []string{logFilePath},
			ErrorOutputPaths: []string{logFilePath + ".err"},
		}
	} else {
		// No logging - use nop (discard all output)
		config = zap.Config{
			Level:            zap.NewAtomicLevelAt(level),
			Encoding:         "json",
			EncoderConfig:    encoderConfig,
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		}
	}

	logger, err = config.Build()
	if err != nil {
		return err
	}

	return nil
}

// Close flushes any buffered log entries
func Close() {
	if logger != nil {
		_ = logger.Sync()
	}
}

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Debug(msg, fields...)
	}
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Info(msg, fields...)
	}
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Warn(msg, fields...)
	}
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Error(msg, fields...)
	}
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Fatal(msg, fields...)
	}
}

// Err creates an error field
func Err(err error) zap.Field {
	return zap.Error(err)
}

// String creates a string field (safe for user input)
func String(key string, value string) zap.Field {
	return zap.String(key, value)
}

// Bool creates a bool field
func Bool(key string, value bool) zap.Field {
	return zap.Bool(key, value)
}
