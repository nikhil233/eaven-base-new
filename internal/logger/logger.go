package logger

import (
	"context"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger represents a structured logger
type Logger struct {
	*zap.SugaredLogger
	serviceName string
}

// NewLogger creates a new logger instance for a specific service
func NewLogger(serviceName string) *Logger {
	// Get environment - default to development if not specified
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	// Configure encoder
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Configure output format based on environment
	var encoder zapcore.Encoder
	if env == "production" {
		// JSON format for production (better for log aggregation)
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		// Console format for development (more human-readable)
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Configure log level
	logLevel := zap.InfoLevel
	if env == "development" {
		logLevel = zap.DebugLevel
	}

	// Create core
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		zap.NewAtomicLevelAt(logLevel),
	)

	// Create logger with caller information
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	// Return sugar-coated logger
	return &Logger{
		SugaredLogger: zapLogger.Sugar().With("service", serviceName),
		serviceName:   serviceName,
	}
}

// WithContext returns a logger with request context fields added
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract request ID if present
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return &Logger{
			SugaredLogger: l.With("request_id", requestID),
			serviceName:   l.serviceName,
		}
	}
	return l
}

// WithUser returns a logger with user ID added
func (l *Logger) WithUser(userID int64) *Logger {
	return &Logger{
		SugaredLogger: l.With("user_id", userID),
		serviceName:   l.serviceName,
	}
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &Logger{
		SugaredLogger: l.With(args...),
		serviceName:   l.serviceName,
	}
}

// Audit logs a high-importance audit event
func (l *Logger) Audit(msg string, keysAndValues ...interface{}) {
	l.With("audit", true, "timestamp", time.Now().UTC()).Infow(msg, keysAndValues...)
}

// Fatal logs a fatal-level message and then calls os.Exit(1)
func (l *Logger) Fatal(msg string, keysAndValues ...interface{}) {
	l.Fatalw(msg, keysAndValues...)
}

// Error logs an error-level message
func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	l.Errorw(msg, keysAndValues...)
}

// Warn logs a warn-level message
func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	l.Warnw(msg, keysAndValues...)
}

// Info logs an info-level message
func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.Infow(msg, keysAndValues...)
}

// Debug logs a debug-level message
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.Debugw(msg, keysAndValues...)
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.SugaredLogger.Sync()
}
