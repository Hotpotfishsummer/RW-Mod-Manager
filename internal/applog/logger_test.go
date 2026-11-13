package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	wailsLogger "github.com/wailsapp/wails/v2/pkg/logger"
)

func TestNormalizeAndParseLevel(t *testing.T) {
	cases := []struct {
		in       string
		wantNorm string
		wantLvl  Level
		wantWls  wailsLogger.LogLevel
	}{
		{"", "info", LevelInfo, wailsLogger.INFO},
		{"invalid", "info", LevelInfo, wailsLogger.INFO},
		{"DEBUG", "debug", LevelDebug, wailsLogger.DEBUG},
		{" info ", "info", LevelInfo, wailsLogger.INFO},
		{"WARN", "warn", LevelWarn, wailsLogger.WARNING},
		{"warning", "warn", LevelWarn, wailsLogger.WARNING},
		{"Error", "error", LevelError, wailsLogger.ERROR},
	}

	for _, tc := range cases {
		if got := NormalizeLevel(tc.in); got != tc.wantNorm {
			t.Errorf("NormalizeLevel(%q) = %q, want %q", tc.in, got, tc.wantNorm)
		}
		if got := ParseLevel(tc.in); got != tc.wantLvl {
			t.Errorf("ParseLevel(%q) = %v, want %v", tc.in, got, tc.wantLvl)
		}
		if got := ToWailsLogLevel(tc.in); got != tc.wantWls {
			t.Errorf("ToWailsLogLevel(%q) = %v, want %v", tc.in, got, tc.wantWls)
		}
	}
}

func TestLoggerFilteringAndDynamicSetLevel(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLogger(dir, "warn")
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer l.Close()

	l.Log(LevelDebug, "Test", "debug hidden")
	l.Log(LevelInfo, "Test", "info hidden")
	l.Log(LevelWarn, "Test", "warn visible")
	l.Log(LevelError, "Test", "error visible")

	l.SetLevel("debug")
	l.Log(LevelDebug, "Test", "debug now visible")

	data, err := os.ReadFile(l.GetLogFilePath())
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "debug hidden") || strings.Contains(content, "info hidden") {
		t.Fatalf("unexpected lower-level logs written:\n%s", content)
	}
	if !strings.Contains(content, "warn visible") || !strings.Contains(content, "error visible") {
		t.Fatalf("expected warn/error logs missing:\n%s", content)
	}
	if !strings.Contains(content, "日志等级已调整: WARN -> DEBUG") {
		t.Fatalf("expected level transition message missing:\n%s", content)
	}
	if !strings.Contains(content, "debug now visible") {
		t.Fatalf("expected debug log after SetLevel missing:\n%s", content)
	}
}

func TestLoggerRotationAndClear(t *testing.T) {
	dir := t.TempDir()
	// 小上限触发多次滚动，最多保留 2 个备份
	l, err := newLoggerWithLimits(dir, "debug", 250, 2)
	if err != nil {
		t.Fatalf("newLoggerWithLimits failed: %v", err)
	}
	defer l.Close()

	for i := 0; i < 20; i++ {
		l.Log(LevelInfo, "RotateTest", "payload-line-%02d-abcdefghijklmnopqrstuvwxyz", i)
	}

	if _, err := os.Stat(filepath.Join(dir, "app.log")); err != nil {
		t.Fatalf("active app.log missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app.log.1")); err != nil {
		t.Fatalf("backup app.log.1 missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app.log.2")); err != nil {
		t.Fatalf("backup app.log.2 missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app.log.3")); !os.IsNotExist(err) {
		t.Fatalf("backup app.log.3 should not exist when maxBackups=2")
	}

	if size := l.GetLogFileSize(); size <= 0 {
		t.Fatalf("expected positive total log file size, got %d", size)
	}

	if err := l.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app.log.1")); !os.IsNotExist(err) {
		t.Fatalf("backup app.log.1 should be removed after Clear")
	}
	data, err := os.ReadFile(l.GetLogFilePath())
	if err != nil {
		t.Fatalf("ReadFile after Clear failed: %v", err)
	}
	if !strings.Contains(string(data), "日志文件已被用户手动清空") {
		t.Fatalf("expected clear marker in fresh app.log, got:\n%s", string(data))
	}
}

func TestLoggerConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	l, err := newLoggerWithLimits(dir, "info", 1024, 2)
	if err != nil {
		t.Fatalf("newLoggerWithLimits failed: %v", err)
	}
	defer l.Close()

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				l.Log(LevelInfo, fmt.Sprintf("G%d", id), "concurrent log entry %d", i)
			}
		}(g)
	}
	wg.Wait()
}
