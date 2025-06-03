package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	zlogger *zap.Logger
)

type options struct {
	logLevel   string
	maxAgeDays int
}

type Option func(*options)

func WithLogLevel(level string) Option {
	return func(o *options) { o.logLevel = level }
}

func WithMaxAge(days int) Option {
	return func(o *options) { o.maxAgeDays = days }
}

func init() {
	Init("./logs") // 默认路径
}

func Init(logDir string, opts ...Option) {
	// 设置默认值
	conf := &options{
		logLevel:   "info",
		maxAgeDays: 7,
	}
	for _, opt := range opts {
		opt(conf)
	}
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		panic(fmt.Sprintf("failed to create log directory: %v", err))
	}

	writer, err := rotatelogs.New(
		filepath.Join(logDir, "app-%Y-%m-%d.log"),
		rotatelogs.WithLinkName(filepath.Join(logDir, "latest.log")),
		rotatelogs.WithMaxAge(time.Duration(conf.maxAgeDays)*24*time.Hour),
		rotatelogs.WithRotationTime(24*time.Hour),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create rotatelogs: %v", err))
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:      "ts",
		LevelKey:     "level",
		MessageKey:   "msg",
		CallerKey:    "caller",
		EncodeLevel:  zapcore.CapitalLevelEncoder,
		EncodeTime:   zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"),
		EncodeCaller: shortCallerEncoder,
	}

	level := parseLevel(conf.logLevel)
	fileCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(writer),
		level,
	)
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)

	// 同时输出到文件和终端
	core := zapcore.NewTee(fileCore, consoleCore)

	zlogger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
}

// shortCallerEncoder 显示 caller 的上一级目录 + 文件名 + 行号
func shortCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	parts := strings.Split(caller.File, "/")
	n := len(parts)
	if n >= 2 {
		enc.AppendString(fmt.Sprintf("%s/%s:%d", parts[n-2], parts[n-1], caller.Line))
	} else {
		enc.AppendString(fmt.Sprintf("%s:%d", caller.File, caller.Line))
	}
}

func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// 结构化日志
func Debugw(msg string, kvs ...interface{}) { zlogger.Sugar().Debugw(msg, kvs...) }
func Infow(msg string, kvs ...interface{})  { zlogger.Sugar().Infow(msg, kvs...) }
func Warnw(msg string, kvs ...interface{})  { zlogger.Sugar().Warnw(msg, kvs...) }
func Errorw(msg string, kvs ...interface{}) { zlogger.Sugar().Errorw(msg, kvs...) }

// Printf 风格日志
func Debugf(format string, args ...interface{}) { zlogger.Sugar().Debugf(format, args...) }
func Infof(format string, args ...interface{})  { zlogger.Sugar().Infof(format, args...) }
func Warnf(format string, args ...interface{})  { zlogger.Sugar().Warnf(format, args...) }
func Errorf(format string, args ...interface{}) { zlogger.Sugar().Errorf(format, args...) }
