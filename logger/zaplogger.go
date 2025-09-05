package logger

import (
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	zapLogger   *zap.Logger
	zapOnce     sync.Once
	atomicLevel zap.AtomicLevel
)

// Added missing environment variable constants used by zap logger.
const (
	EnvAppEnv    = "APP_ENV"
	EnvLogLevel  = "LOG_LEVEL"
	EnvLogFormat = "LOG_FORMAT" // json | console
	EnvLogFile   = "LOG_FILE"
)

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// initZap builds the global zap logger lazily.
func initZap() {
	zapOnce.Do(func() {
		env := firstNonEmpty(os.Getenv(EnvAppEnv), os.Getenv("GO_ENV"), os.Getenv("ENV"))
		if env == "" {
			env = "development"
		}

		// Level (use atomic for runtime changes)
		atomicLevel = zap.NewAtomicLevel()
		lvl := parseZapLevel(strings.ToLower(os.Getenv(EnvLogLevel)), env)
		atomicLevel.SetLevel(lvl)

		// Format
		format := strings.ToLower(os.Getenv(EnvLogFormat))
		if format == "" {
			if env == "production" || env == "prod" {
				format = "json"
			} else {
				format = "console"
			}
		}

		encoderCfg := zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stack",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     iso8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		var enc zapcore.Encoder
		if format == "json" {
			enc = zapcore.NewJSONEncoder(encoderCfg)
		} else {
			enc = zapcore.NewConsoleEncoder(encoderCfg)
		}

		stdoutSyncer := zapcore.Lock(os.Stdout)
		var core zapcore.Core

		// Optional file sink
		if filePath := os.Getenv(EnvLogFile); filePath != "" {
			if f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
				fileSyncer := zapcore.AddSync(f)
				core = zapcore.NewCore(enc, zapcore.NewMultiWriteSyncer(stdoutSyncer, fileSyncer), atomicLevel)
			} else {
				core = zapcore.NewCore(enc, stdoutSyncer, atomicLevel)
			}
		} else {
			core = zapcore.NewCore(enc, stdoutSyncer, atomicLevel)
		}

		// Optional sampling (enabled by default in production unless LOG_SAMPLING=0)
		if env == "production" || env == "prod" {
			if os.Getenv("LOG_SAMPLING") != "0" { // enable sampling
				core = zapcore.NewSamplerWithOptions(core, time.Second, 100, 10)
			}
		}

		opts := []zap.Option{}
		if env != "production" && env != "prod" {
			opts = append(opts, zap.AddCaller(), zap.Development())
		} else {
			// In production capture stack only for errors & above
			opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
		}

		zapLogger = zap.New(core, opts...)
	})
}

// parseZapLevel maps string to zapcore.Level (defaulting by environment when empty).
func parseZapLevel(lvl string, env string) zapcore.Level {
	if lvl == "" {
		if env == "production" || env == "prod" {
			return zapcore.InfoLevel
		}
		return zapcore.DebugLevel
	}
	switch lvl {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "dpanic":
		return zapcore.DPanicLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

func iso8601TimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format("2006-01-02T15:04:05Z07:00"))
}

// Zap returns the base *zap.Logger
func Zap() *zap.Logger {
	initZap()
	return zapLogger
}

// ZapSugar returns a *zap.SugaredLogger
func ZapSugar() *zap.SugaredLogger { return Zap().Sugar() }

// ZapForService returns a sugared logger with service + env fields.
func ZapForService(service string) *zap.SugaredLogger {
	initZap()
	env := firstNonEmpty(os.Getenv(EnvAppEnv), os.Getenv("GO_ENV"), os.Getenv("ENV"))
	if env == "" {
		env = "development"
	}
	return zapLogger.With(zap.String("service", service), zap.String("env", env)).Sugar()
}

// ZapWith adds arbitrary zap fields to base logger.
func ZapWith(fields ...zap.Field) *zap.Logger { return Zap().With(fields...) }

// ZapAddContext creates a derived sugared logger with dynamic context fields.
func ZapAddContext(l *zap.SugaredLogger, kv map[string]any) *zap.SugaredLogger {
	if len(kv) == 0 {
		return l
	}
	fields := make([]any, 0, len(kv)*2)
	for k, v := range kv {
		fields = append(fields, k, v)
	}
	return l.With(fields...)
}

// SetLevel changes the log level at runtime (e.g., SetLevel("debug")).
func SetLevel(level string) error {
	initZap()
	switch strings.ToLower(level) {
	case "debug":
		atomicLevel.SetLevel(zapcore.DebugLevel)
	case "info":
		atomicLevel.SetLevel(zapcore.InfoLevel)
	case "warn", "warning":
		atomicLevel.SetLevel(zapcore.WarnLevel)
	case "error":
		atomicLevel.SetLevel(zapcore.ErrorLevel)
	case "dpanic":
		atomicLevel.SetLevel(zapcore.DPanicLevel)
	case "panic":
		atomicLevel.SetLevel(zapcore.PanicLevel)
	case "fatal":
		atomicLevel.SetLevel(zapcore.FatalLevel)
	default:
		return errors.New("unknown log level")
	}
	return nil
}

// Level returns the current level string.
func Level() string { initZap(); return atomicLevel.Level().String() }

// Sync flushes any buffered logs (call on shutdown).
func Sync() {
	if zapLogger != nil {
		_ = zapLogger.Sync()
	}
}
