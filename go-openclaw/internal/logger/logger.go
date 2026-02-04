package logger

import (
	"os"
	"sync"

	"github.com/openclaw/go-openclaw/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	instance *zap.Logger
	sugar    *zap.SugaredLogger
	once     sync.Once
)

// Init initializes the global logger
func Init(cfg *config.Config) error {
	var err error
	once.Do(func() {
		instance, err = newLogger(cfg)
		if instance != nil {
			sugar = instance.Sugar()
		}
	})
	return err
}

// newLogger creates a new zap logger instance
func newLogger(cfg *config.Config) (*zap.Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Create encoder config
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
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Choose encoder based on format
	var encoder zapcore.Encoder
	if cfg.Logging.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create writer
	var writer zapcore.WriteSyncer
	if cfg.Logging.Output == "stdout" || cfg.Logging.Output == "" {
		writer = zapcore.AddSync(os.Stdout)
	} else {
		// Use lumberjack for log rotation
		writer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.Logging.Output,
			MaxSize:    cfg.Logging.MaxSize,
			MaxBackups: cfg.Logging.MaxBackups,
			MaxAge:     cfg.Logging.MaxAge,
			Compress:   true,
		})
	}

	// Create core
	core := zapcore.NewCore(encoder, writer, level)

	// Create logger
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}

// Get returns the global logger instance
func Get() *zap.Logger {
	if instance == nil {
		// Initialize with default config if not initialized yet
		cfg, err := config.Load("")
		if err != nil {
			panic(err)
		}
		if err := Init(cfg); err != nil {
			panic(err)
		}
	}
	return instance
}

// Sugar returns the sugared logger instance
func Sugar() *zap.SugaredLogger {
	if sugar == nil {
		sugar = Get().Sugar()
	}
	return sugar
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}

// With creates a child logger with additional fields
func With(fields ...zap.Field) *zap.Logger {
	return Get().With(fields...)
}

// Sync flushes any buffered log entries
func Sync() error {
	if instance != nil {
		return instance.Sync()
	}
	return nil
}
