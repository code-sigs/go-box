package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoggerBasic(t *testing.T) {
	logDir := "./testlogs"
	defer os.RemoveAll(logDir)

	Init(logDir, WithLogLevel("debug"), WithMaxAge(1))

	Debugw("debug message", "k", "v")
	Infow("info message", "k", "v")
	Warnw("warn message", "k", "v")
	Errorw("error message", "k", "v")
	Debugf("debugf: %s", "debug")
	Infof("infof: %s", "info")
	Warnf("warnf: %s", "warn")
	Errorf("errorf: %s", "error")

	// 等待日志写入
	time.Sleep(200 * time.Millisecond)

	// 检查日志文件是否生成
	files, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("failed to read log dir: %v", err)
	}
	found := false
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".log" {
			found = true
			break
		}
	}
	if !found {
		t.Error("log file not generated")
	}
}
