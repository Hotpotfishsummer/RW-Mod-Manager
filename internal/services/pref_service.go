package services

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PrefService 负责 preferences.ini 的安全读写与 modSettings 状态同步
type PrefService struct {
	mu sync.RWMutex
}

func NewPrefService() *PrefService {
	return &PrefService{}
}

// CalculateModHash 计算与铁锈战争原生完全一致的 SHA-256 识别码
func (s *PrefService) CalculateModHash(filename string) string {
	h := sha256.Sum256([]byte(filename))
	return fmt.Sprintf("%064X", h)
}

// ModSettingEntry 单条 modSettings 记录
type ModSettingEntry struct {
	Title  string
	Hash   string
	Status string // "enabled" | "disabled"
}

// ReadModSettings 从 preferences.ini 中读取所有模组的启禁用状态映射
// 返回值：map[Hash]bool (true 为 enabled, false 为 disabled)
func (s *PrefService) ReadModSettings(gameDir string) (map[string]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefPath := filepath.Join(gameDir, "preferences.ini")
	result := make(map[string]bool)

	data, err := os.ReadFile(prefPath)
	if err != nil {
		return result, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "modSettings:") {
			raw := strings.TrimPrefix(line, "modSettings:")
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			items := strings.Split(raw, ",")
			for _, item := range items {
				parts := strings.Split(item, "|")
				if len(parts) >= 3 {
					hash := strings.TrimSpace(parts[1])
					status := strings.TrimSpace(parts[2])
					result[hash] = (status == "enabled")
				}
			}
			break
		}
	}

	return result, nil
}

// SaveModSettings 将前端更新后的模组状态批量写回 preferences.ini
// entries: map[filename]isEnabled
func (s *PrefService) SaveModSettings(gameDir string, modStatuses map[string]bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefPath := filepath.Join(gameDir, "preferences.ini")
	data, err := os.ReadFile(prefPath)
	if err != nil {
		return err
	}

	// 构造新的 modSettings 字符串
	var settingsList []string
	for filename, enabled := range modStatuses {
		hash := s.CalculateModHash(filename)
		status := "disabled"
		if enabled {
			status = "enabled"
		}
		// 展示标题处理：官方截断逻辑如果大于 15 则截取
		displayTitle := filename
		displayTitle = strings.ReplaceAll(displayTitle, ",", "")
		displayTitle = strings.ReplaceAll(displayTitle, "|", "")
		if len([]rune(displayTitle)) > 15 {
			runes := []rune(displayTitle)
			if len(runes) > 12 {
				displayTitle = string(runes[12:]) + "..."
			}
		}

		settingsList = append(settingsList, fmt.Sprintf("%s|%s|%s", displayTitle, hash, status))
	}

	newModSettingsLine := "modSettings:" + strings.Join(settingsList, ",")
	newLastModCountLine := fmt.Sprintf("lastModCount:%d", len(modStatuses))

	var outputLines []string
	foundModSettings := false
	foundLastModCount := false
	foundVersion := false

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "modSettings:") {
			outputLines = append(outputLines, newModSettingsLine)
			foundModSettings = true
		} else if strings.HasPrefix(trimmed, "lastModCount:") {
			outputLines = append(outputLines, newLastModCountLine)
			foundLastModCount = true
		} else if strings.HasPrefix(trimmed, "modSettingsVersion:") {
			outputLines = append(outputLines, "modSettingsVersion:1")
			foundVersion = true
		} else {
			outputLines = append(outputLines, line)
		}
	}

	// 如果 preferences.ini 里原本没有这些配置项，则追加
	if !foundModSettings {
		outputLines = append(outputLines, newModSettingsLine)
	}
	if !foundLastModCount {
		outputLines = append(outputLines, newLastModCountLine)
	}
	if !foundVersion {
		outputLines = append(outputLines, "modSettingsVersion:1")
	}

	finalContent := strings.Join(outputLines, "\r\n") + "\r\n"
	return os.WriteFile(prefPath, []byte(finalContent), 0644)
}
