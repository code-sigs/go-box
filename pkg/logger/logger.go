package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/code-sigs/go-box/pkg/trace"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	zlogger *zap.Logger
)

type options struct {
	logLevel     string
	maxAgeDays   int
	enableStdout bool // 新增：是否输出到终端
}

type Option func(*options)

func WithLogLevel(level string) Option {
	return func(o *options) { o.logLevel = level }
}

func WithMaxAge(days int) Option {
	return func(o *options) { o.maxAgeDays = days }
}

// 新增：配置是否输出到终端
func WithStdout(enable bool) Option {
	return func(o *options) { o.enableStdout = enable }
}

func init() {
	Init("./logs") // 默认路径
}

func Init(logDir string, opts ...Option) {
	// 设置默认值
	conf := &options{
		logLevel:     "info",
		maxAgeDays:   7,
		enableStdout: true, // 默认不输出到终端
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

	var core zapcore.Core
	if conf.enableStdout {
		consoleCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			level,
		)
		core = zapcore.NewTee(fileCore, consoleCore)
	} else {
		core = fileCore
	}

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

func Debugf(ctx context.Context, format string, args ...interface{}) {
	logWithTrace(ctx).Debugf(format, args...)
}

func Infof(ctx context.Context, format string, args ...interface{}) {
	logWithTrace(ctx).Infof(format, args...)
}

func Warnf(ctx context.Context, format string, args ...interface{}) {
	logWithTrace(ctx).Warnf(format, args...)
}

func Errorf(ctx context.Context, format string, args ...interface{}) {
	logWithTrace(ctx).Errorf(format, args...)
}

func Debugw(ctx context.Context, msg string, kvs ...interface{}) {
	logWithTrace(ctx).Debugw(msg, kvs...)
}

func Infow(ctx context.Context, msg string, kvs ...interface{}) {
	logWithTrace(ctx).Infow(msg, kvs...)
}

func Warnw(ctx context.Context, msg string, kvs ...interface{}) {
	logWithTrace(ctx).Warnw(msg, kvs...)
}

func Errorw(ctx context.Context, msg string, kvs ...interface{}) {
	logWithTrace(ctx).Errorw(msg, kvs...)
}

// 提取 traceID 并注入到日志中
func logWithTrace(ctx context.Context) *zap.SugaredLogger {
	traceID := trace.GetTraceID(ctx)
	if traceID != "" {
		return zlogger.Sugar().With("traceID", traceID)
	}
	return zlogger.Sugar()
}
