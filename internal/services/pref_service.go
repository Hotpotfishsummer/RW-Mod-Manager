package services

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"rw_mod_manager/internal/models"
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
func (s *PrefService) SaveModSettings(gameDir string, modStatuses map[string]bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefPath := filepath.Join(gameDir, "preferences.ini")
	data, err := os.ReadFile(prefPath)
	if err != nil {
		return err
	}

	var settingsList []string
	for filename, enabled := range modStatuses {
		hash := s.CalculateModHash(filename)
		status := "disabled"
		if enabled {
			status = "enabled"
		}
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

// ReadGamePreferences 读取画质、高刷与帧率控制参数
func (s *PrefService) ReadGamePreferences(gameDir string) (models.GamePreferences, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefPath := filepath.Join(gameDir, "preferences.ini")
	prefs := models.GamePreferences{
		HighRefreshRate:         true,
		RenderVsync:             true,
		ShowFps:                 true,
		RenderAntiAlias:         true,
		RenderFancyWater:        false,
		RenderClouds:            true,
		RenderExtraShadows:      true,
		TeamShaders:             true,
		NewRender:               false,
		TeamUnitCapSinglePlayer: 5000,
		TeamUnitCapHostedGame:   2000,
		UiRenderScale:           1.0,
	}

	data, err := os.ReadFile(prefPath)
	if err != nil {
		return prefs, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "highRefreshRate":
			prefs.HighRefreshRate = (val == "true")
		case "renderVsync":
			prefs.RenderVsync = (val == "true")
		case "showFps":
			prefs.ShowFps = (val == "true")
		case "renderSmoothDelta":
			prefs.RenderSmoothDelta = (val == "true")
		case "renderAntiAlias":
			prefs.RenderAntiAlias = (val == "true")
		case "renderFancyWater":
			prefs.RenderFancyWater = (val == "true")
		case "renderClouds":
			prefs.RenderClouds = (val == "true")
		case "renderExtraShadows":
			prefs.RenderExtraShadows = (val == "true")
		case "teamShaders":
			prefs.TeamShaders = (val == "true")
		case "newRender":
			prefs.NewRender = (val == "true")
		case "teamUnitCapSinglePlayer":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				prefs.TeamUnitCapSinglePlayer = n
			}
		case "teamUnitCapHostedGame":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				prefs.TeamUnitCapHostedGame = n
			}
		case "uiRenderScale":
			if f, err := strconv.ParseFloat(val, 64); err == nil && f > 0 {
				prefs.UiRenderScale = f
			}
		}
	}

	return prefs, nil
}

// SaveGamePreferences 将画质、高刷与单位上限配置写回 preferences.ini
func (s *PrefService) SaveGamePreferences(gameDir string, prefs models.GamePreferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefPath := filepath.Join(gameDir, "preferences.ini")
	data, err := os.ReadFile(prefPath)
	if err != nil {
		return err
	}

	targetMap := map[string]string{
		"highRefreshRate":         fmt.Sprintf("%t", prefs.HighRefreshRate),
		"renderVsync":             fmt.Sprintf("%t", prefs.RenderVsync),
		"showFps":                 fmt.Sprintf("%t", prefs.ShowFps),
		"renderSmoothDelta":       fmt.Sprintf("%t", prefs.RenderSmoothDelta),
		"renderAntiAlias":         fmt.Sprintf("%t", prefs.RenderAntiAlias),
		"renderFancyWater":        fmt.Sprintf("%t", prefs.RenderFancyWater),
		"renderClouds":            fmt.Sprintf("%t", prefs.RenderClouds),
		"renderExtraShadows":      fmt.Sprintf("%t", prefs.RenderExtraShadows),
		"teamShaders":             fmt.Sprintf("%t", prefs.TeamShaders),
		"newRender":               fmt.Sprintf("%t", prefs.NewRender),
		"teamUnitCapSinglePlayer": fmt.Sprintf("%d", prefs.TeamUnitCapSinglePlayer),
		"teamUnitCapHostedGame":   fmt.Sprintf("%d", prefs.TeamUnitCapHostedGame),
		"uiRenderScale":           fmt.Sprintf("%.1f", prefs.UiRenderScale),
	}

	updatedKeys := make(map[string]bool)
	var outputLines []string

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			if v, ok := targetMap[k]; ok {
				outputLines = append(outputLines, fmt.Sprintf("%s:%s", k, v))
				updatedKeys[k] = true
				continue
			}
		}
		outputLines = append(outputLines, line)
	}

	// 追加未更新的项
	for k, v := range targetMap {
		if !updatedKeys[k] {
			outputLines = append(outputLines, fmt.Sprintf("%s:%s", k, v))
		}
	}

	finalContent := strings.Join(outputLines, "\r\n") + "\r\n"
	return os.WriteFile(prefPath, []byte(finalContent), 0644)
}
