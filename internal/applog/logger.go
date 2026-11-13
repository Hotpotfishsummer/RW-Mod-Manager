package applog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wailsLogger "github.com/wailsapp/wails/v2/pkg/logger"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Level 表示应用日志等级
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

const (
	defaultMaxSizeBytes int64 = 5 * 1024 * 1024 // 单个日志文件上限 5MB
	defaultMaxBackups         = 3               // 最多保留 3 个历史滚动备份
)

// NormalizeLevel 规范化日志等级字符串，非法值回退为 "info"
func NormalizeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return "debug"
	case "info":
		return "info"
	case "warn", "warning":
		return "warn"
	case "error":
		return "error"
	default:
		return "info"
	}
}

// ParseLevel 将等级字符串转换为 Level 枚举
func ParseLevel(level string) Level {
	switch NormalizeLevel(level) {
	case "debug":
		return LevelDebug
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// ToWailsLogLevel 将应用日志等级映射为 Wails 框架日志等级
func ToWailsLogLevel(level string) wailsLogger.LogLevel {
	switch NormalizeLevel(level) {
	case "debug":
		return wailsLogger.DEBUG
	case "warn":
		return wailsLogger.WARNING
	case "error":
		return wailsLogger.ERROR
	default:
		return wailsLogger.INFO
	}
}

// Logger 提供线程安全、支持按大小滚动的本地文件与控制台双路日志记录器
type Logger struct {
	mu         sync.Mutex
	logDir     string
	logPath    string
	file       *os.File
	curSize    int64
	maxSize    int64
	maxBackups int
	level      Level
	wailsCtx   context.Context
}

var (
	globalMu     sync.RWMutex
	globalLogger *Logger
)

// DefaultLogDir 返回默认日志存储目录 (<UserConfigDir>/RWModManager/logs)
func DefaultLogDir() string {
	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		return filepath.Join(configDir, "RWModManager", "logs")
	}
	return filepath.Join(".", "logs")
}

// NewLogger 创建并打开日志文件实例
func NewLogger(logDir string, level string) (*Logger, error) {
	return newLoggerWithLimits(logDir, level, defaultMaxSizeBytes, defaultMaxBackups)
}

func newLoggerWithLimits(logDir string, level string, maxSize int64, maxBackups int) (*Logger, error) {
	if logDir == "" {
		logDir = DefaultLogDir()
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}
	if maxSize <= 0 {
		maxSize = defaultMaxSizeBytes
	}
	if maxBackups < 0 {
		maxBackups = 0
	}

	logPath := filepath.Join(logDir, "app.log")
	l := &Logger{
		logDir:     logDir,
		logPath:    logPath,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		level:      ParseLevel(level),
	}
	if err := l.openFile(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Logger) openFile() error {
	f, err := os.OpenFile(l.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}
	var size int64
	if info, err := f.Stat(); err == nil {
		size = info.Size()
	}
	l.file = f
	l.curSize = size
	return nil
}

// Init 初始化全局日志记录器
func Init(level string) error {
	l, err := NewLogger(DefaultLogDir(), level)
	if err != nil {
		return err
	}
	globalMu.Lock()
	old := globalLogger
	globalLogger = l
	globalMu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}

func getGlobal() *Logger {
	globalMu.RLock()
	l := globalLogger
	globalMu.RUnlock()
	return l
}

// SetWailsContext 绑定 Wails 运行时上下文，使日志等级变更实时同步至 Wails
func SetWailsContext(ctx context.Context) {
	if l := getGlobal(); l != nil {
		l.SetWailsContext(ctx)
	}
}

// SetWailsContext 绑定 Wails 运行时上下文
func (l *Logger) SetWailsContext(ctx context.Context) {
	l.mu.Lock()
	l.wailsCtx = ctx
	lvl := l.level
	l.mu.Unlock()
	if ctx != nil {
		wailsRuntime.LogSetLogLevel(ctx, ToWailsLogLevel(strings.ToLower(lvl.String())))
	}
}

// SetLevel 动态更新全局日志等级，返回规范化后的等级字符串
func SetLevel(level string) string {
	norm := NormalizeLevel(level)
	if l := getGlobal(); l != nil {
		return l.SetLevel(norm)
	}
	return norm
}

// SetLevel 动态更新当前记录器日志等级
func (l *Logger) SetLevel(level string) string {
	norm := NormalizeLevel(level)
	newLevel := ParseLevel(norm)

	l.mu.Lock()
	prev := l.level
	l.level = newLevel
	ctx := l.wailsCtx
	l.mu.Unlock()

	if ctx != nil {
		wailsRuntime.LogSetLogLevel(ctx, ToWailsLogLevel(norm))
	}
	if prev != newLevel {
		// 等级变化本身始终记录，便于排查为何某段日志缺失或突然变详细
		l.writeLine(LevelInfo, "Logger", fmt.Sprintf("日志等级已调整: %s -> %s", prev.String(), newLevel.String()))
	}
	return norm
}

// GetLevel 获取当前全局日志等级（小写）
func GetLevel() string {
	if l := getGlobal(); l != nil {
		return l.GetLevel()
	}
	return "info"
}

// GetLevel 获取当前日志等级（小写）
func (l *Logger) GetLevel() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.ToLower(l.level.String())
}

// GetLogFilePath 获取全局当前活跃日志文件绝对路径
func GetLogFilePath() string {
	if l := getGlobal(); l != nil {
		return l.GetLogFilePath()
	}
	return filepath.Join(DefaultLogDir(), "app.log")
}

// GetLogFilePath 获取当前活跃日志文件绝对路径
func (l *Logger) GetLogFilePath() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.logPath
}

// GetLogDir 获取全局日志目录绝对路径
func GetLogDir() string {
	if l := getGlobal(); l != nil {
		return l.GetLogDir()
	}
	return DefaultLogDir()
}

// GetLogDir 获取日志目录绝对路径
func (l *Logger) GetLogDir() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.logDir
}

// GetLogFileSize 获取当前日志文件总大小（含滚动备份，单位字节）
func GetLogFileSize() int64 {
	if l := getGlobal(); l != nil {
		return l.GetLogFileSize()
	}
	return 0
}

// GetLogFileSize 获取当前日志文件与备份的总大小（字节）
func (l *Logger) GetLogFileSize() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	var total int64
	if info, err := os.Stat(l.logPath); err == nil {
		total += info.Size()
	}
	for i := 1; i <= l.maxBackups; i++ {
		if info, err := os.Stat(l.backupPath(i)); err == nil {
			total += info.Size()
		}
	}
	return total
}

func (l *Logger) backupPath(index int) string {
	return fmt.Sprintf("%s.%d", l.logPath, index)
}

// Clear 清空当前日志文件及历史滚动备份
func Clear() error {
	if l := getGlobal(); l != nil {
		return l.Clear()
	}
	return nil
}

// Clear 清空当前日志文件与历史备份并重新打开
func (l *Logger) Clear() error {
	l.mu.Lock()
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
	_ = os.Remove(l.logPath)
	for i := 1; i <= l.maxBackups; i++ {
		_ = os.Remove(l.backupPath(i))
	}
	err := l.openFile()
	l.mu.Unlock()
	if err != nil {
		return err
	}
	l.writeLine(LevelInfo, "Logger", "日志文件已被用户手动清空")
	return nil
}

// Close 关闭日志文件句柄
func Close() {
	if l := getGlobal(); l != nil {
		l.Close()
	}
}

// Close 刷盘并关闭底层文件
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Sync()
		_ = l.file.Close()
		l.file = nil
	}
}

// Log 按等级记录日志
func (l *Logger) Log(lvl Level, tag string, format string, args ...any) {
	l.mu.Lock()
	enabled := lvl >= l.level
	l.mu.Unlock()
	if !enabled {
		return
	}
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	l.writeLine(lvl, tag, msg)
}

func (l *Logger) writeLine(lvl Level, tag string, msg string) {
	if tag == "" {
		tag = "App"
	}
	// 将多行消息中的换行替换/缩进处理，确保单条日志行格式清晰，同时保留完整堆栈信息
	cleanMsg := strings.TrimRight(msg, "\r\n")
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf("%s [%s] [%s] %s\n", timestamp, lvl.String(), tag, cleanMsg)

	// 同时输出至标准输出（开发模式可见；Windows GUI 模式无控制台时忽略写入错误）
	_, _ = os.Stdout.WriteString(line)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		if err := l.openFile(); err != nil {
			return
		}
	}

	lineLen := int64(len(line))
	if l.maxSize > 0 && l.curSize > 0 && l.curSize+lineLen > l.maxSize {
		l.rotateLocked()
	}

	if l.file != nil {
		if n, err := l.file.WriteString(line); err == nil {
			l.curSize += int64(n)
		}
	}
}

func (l *Logger) rotateLocked() {
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}

	if l.maxBackups > 0 {
		oldest := l.backupPath(l.maxBackups)
		_ = os.Remove(oldest)
		for i := l.maxBackups - 1; i >= 1; i-- {
			src := l.backupPath(i)
			dst := l.backupPath(i + 1)
			if _, err := os.Stat(src); err == nil {
				_ = os.Rename(src, dst)
			}
		}
		_ = os.Rename(l.logPath, l.backupPath(1))
	} else {
		_ = os.Remove(l.logPath)
	}

	_ = l.openFile()
}

// Debug 记录调试级别日志
func Debug(tag string, format string, args ...any) {
	if l := getGlobal(); l != nil {
		l.Log(LevelDebug, tag, format, args...)
	}
}

// Info 记录信息级别日志
func Info(tag string, format string, args ...any) {
	if l := getGlobal(); l != nil {
		l.Log(LevelInfo, tag, format, args...)
	}
}

// Warn 记录警告级别日志
func Warn(tag string, format string, args ...any) {
	if l := getGlobal(); l != nil {
		l.Log(LevelWarn, tag, format, args...)
	}
}

// Error 记录错误级别日志
func Error(tag string, format string, args ...any) {
	if l := getGlobal(); l != nil {
		l.Log(LevelError, tag, format, args...)
	}
}

// WriteClientLog 记录来自前端界面或 WebView2 的日志消息
func WriteClientLog(level string, source string, message string) {
	const maxClientMsgLen = 4000
	if len(message) > maxClientMsgLen {
		message = message[:maxClientMsgLen] + "...(truncated)"
	}
	tag := "Frontend"
	if s := strings.TrimSpace(source); s != "" {
		tag = "Frontend:" + s
	}
	lvl := ParseLevel(level)
	if l := getGlobal(); l != nil {
		l.Log(lvl, tag, "%s", message)
	}
}

// wailsAdapter 适配 Wails v2 的 logger.Logger 接口
type wailsAdapter struct{}

// NewWailsLogger 返回可注入 wails.Run options.App.Logger 的适配器
func NewWailsLogger() wailsLogger.Logger {
	return &wailsAdapter{}
}

func (w *wailsAdapter) Print(message string) {
	Info("Wails", "%s", message)
}

func (w *wailsAdapter) Trace(message string) {
	Debug("Wails:Trace", "%s", message)
}

func (w *wailsAdapter) Debug(message string) {
	Debug("Wails", "%s", message)
}

func (w *wailsAdapter) Info(message string) {
	Info("Wails", "%s", message)
}

func (w *wailsAdapter) Warning(message string) {
	Warn("Wails", "%s", message)
}

func (w *wailsAdapter) Error(message string) {
	Error("Wails", "%s", message)
}

func (w *wailsAdapter) Fatal(message string) {
	Error("Wails:Fatal", "%s", message)
}
