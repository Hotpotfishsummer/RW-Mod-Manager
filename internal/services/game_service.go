package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"

	"rw_mod_manager/internal/models"
)

// GameService 处理游戏路径探测、健全性验证与管理器全局配置持久化
type GameService struct {
	configPath string
	Config     models.AppConfig
}

// NewGameService 构造函数
func NewGameService() *GameService {
	// 优先存放在用户 AppData 目录，回退到当前目录
	configDir, err := os.UserConfigDir()
	var cfgPath string
	if err == nil {
		appDir := filepath.Join(configDir, "RWModManager")
		_ = os.MkdirAll(appDir, 0755)
		cfgPath = filepath.Join(appDir, "config.json")
	} else {
		cfgPath = "config.json"
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "RW-Player"
	}

	s := &GameService{
		configPath: cfgPath,
		Config: models.AppConfig{
			KnownGameDirs: []string{},
			LaunchConfig: models.LaunchConfig{
				ExecutableType: "java", // 默认推荐使用内置 64位 JVM 模式，以支持自定义内存扩展与 G1GC 调优
				MaxMemoryMB:    4096,
				Fullscreen:     false,
				NoMods:         false,
				SafeMode:       false,
				ExtraArgs:      "",
			},
			LanConfig: models.LanConfig{
				InstanceID:             uuid.New().String(),
				DeviceName:             hostname,
				Role:                   models.RoleClient,
				BroadcastPort:          38567,
				HttpPort:               38568,
				AutoAcceptClientPushes: false,
			},
		},
	}

	s.LoadConfig()

	// 若未配置游戏路径或当前路径无效，尝试自动探测
	if s.Config.GameDir == "" || !s.ValidateGameDir(s.Config.GameDir) {
		detected := s.AutoDetectGameDir()
		if detected != "" {
			s.Config.GameDir = detected
			_ = s.SaveConfig(s.Config)
		}
	} else {
		// 确保当前有效路径记录在已知路径列表中
		s.ensureKnownGameDir(s.Config.GameDir)
	}

	return s
}

// LoadConfig 从磁盘读取配置
func (s *GameService) LoadConfig() {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return
	}
	var cfg models.AppConfig
	if err := json.Unmarshal(data, &cfg); err == nil {
		if cfg.GameDir != "" {
			s.Config.GameDir = cfg.GameDir
		}
		if len(cfg.KnownGameDirs) > 0 {
			s.Config.KnownGameDirs = cfg.KnownGameDirs
		}
		if cfg.LaunchConfig.MaxMemoryMB > 0 {
			s.Config.LaunchConfig = cfg.LaunchConfig
			// 若用户配置了大于 1024MB 的 JVM 内存，但依然停留在旧版本的 exe64/exe32 启动模式（由于官方二进制硬编码锁定 1000MB 内存导致设置失效），
			// 自动平滑升级为内置 java 模式，确保内存调优真正生效
			if s.Config.LaunchConfig.MaxMemoryMB > 1024 && (s.Config.LaunchConfig.ExecutableType == "exe64" || s.Config.LaunchConfig.ExecutableType == "exe32") {
				s.Config.LaunchConfig.ExecutableType = "java"
				_ = s.SaveConfig(s.Config)
			}
		}
		if cfg.LanConfig.InstanceID != "" {
			s.Config.LanConfig.InstanceID = cfg.LanConfig.InstanceID
		}
		if cfg.LanConfig.DeviceName != "" {
			s.Config.LanConfig.DeviceName = cfg.LanConfig.DeviceName
		}
		if cfg.LanConfig.Role != "" {
			s.Config.LanConfig.Role = cfg.LanConfig.Role
		}
		if cfg.LanConfig.BroadcastPort > 0 {
			s.Config.LanConfig.BroadcastPort = cfg.LanConfig.BroadcastPort
		}
		if cfg.LanConfig.HttpPort > 0 {
			s.Config.LanConfig.HttpPort = cfg.LanConfig.HttpPort
		}
		s.Config.LanConfig.SelectedSubnet = cfg.LanConfig.SelectedSubnet
		s.Config.LanConfig.AutoAcceptClientPushes = cfg.LanConfig.AutoAcceptClientPushes
	}
}

// SaveConfig 保存配置到磁盘
func (s *GameService) SaveConfig(cfg models.AppConfig) error {
	s.Config = cfg
	data, err := json.MarshalIndent(s.Config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath, data, 0644)
}

// ValidateGameDir 验证指定目录是否为有效的《铁锈战争》根目录
func (s *GameService) ValidateGameDir(dir string) bool {
	if dir == "" {
		return false
	}
	cleanDir := filepath.Clean(dir)
	// 校验核心特征文件：game-lib.jar 和 mods 文件夹
	jarPath := filepath.Join(cleanDir, "game-lib.jar")
	if fi, err := os.Stat(jarPath); err != nil || fi.IsDir() {
		return false
	}
	modsPath := filepath.Join(cleanDir, "mods")
	if fi, err := os.Stat(modsPath); err != nil || !fi.IsDir() {
		return false
	}
	return true
}

// InspectGameDir 深度检查并提取指定路径的游戏特征、版本号及模组统计
func (s *GameService) InspectGameDir(dir string, source, sourceName string, isActive bool) models.GamePathItem {
	cleanDir := filepath.Clean(dir)
	isValid := s.ValidateGameDir(cleanDir)
	item := models.GamePathItem{
		Path:       cleanDir,
		Source:     source,
		SourceName: sourceName,
		IsValid:    isValid,
		IsActive:   isActive,
		Version:    "未知版本",
	}

	if !isValid {
		return item
	}

	// 统计 mods/units 数量
	unitsDir := filepath.Join(cleanDir, "mods", "units")
	if entries, err := os.ReadDir(unitsDir); err == nil {
		count := 0
		for _, e := range entries {
			name := strings.ToLower(e.Name())
			if strings.HasPrefix(name, ".") {
				continue
			}
			if e.IsDir() || strings.HasSuffix(name, ".rwmod") {
				count++
			}
		}
		item.ModCount = count
	}

	// 统计 mods/maps 数量
	mapsDir := filepath.Join(cleanDir, "mods", "maps")
	if entries, err := os.ReadDir(mapsDir); err == nil {
		count := 0
		for _, e := range entries {
			name := strings.ToLower(e.Name())
			if strings.HasPrefix(name, ".") {
				continue
			}
			if strings.HasSuffix(name, ".tmx") || strings.HasSuffix(name, ".rwmod") {
				count++
			}
		}
		item.MapCount = count
	}

	// 检测版本信息 (读取 preferences.ini 中的 settingsGameVersion)
	prefPath := filepath.Join(cleanDir, "preferences.ini")
	if data, err := os.ReadFile(prefPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "settingsGameVersion:") {
				verStr := strings.TrimSpace(strings.TrimPrefix(line, "settingsGameVersion:"))
				switch verStr {
				case "176":
					item.Version = "v1.15 (Build 176)"
				case "175":
					item.Version = "v1.15 (Build 175)"
				case "174":
					item.Version = "v1.14 (Build 174)"
				default:
					item.Version = fmt.Sprintf("Build %s", verStr)
				}
				break
			}
		}
	}

	// 辅助检查是否包含 steam_appid.txt
	if _, err := os.Stat(filepath.Join(cleanDir, "steam_appid.txt")); err == nil {
		if item.Version == "未知版本" {
			item.Version = "Steam 官方版"
		}
	}

	return item
}

// DetectAllGameDirs 全量执行系统探测，结合注册表、Steam库、运行进程与磁盘检索，并聚合历史路径
func (s *GameService) DetectAllGameDirs() []models.GamePathItem {
	var results []models.GamePathItem
	seen := make(map[string]int) // normalized path -> index in results

	addOrUpdate := func(item models.GamePathItem) {
		clean := filepath.Clean(item.Path)
		key := strings.ToLower(clean)
		item.Path = clean

		if idx, exists := seen[key]; exists {
			// 若已存在且旧来源是弱扫描来源，而新来源更具体(如 steam 或 process)，提升来源信息
			if results[idx].Source == "drive_scan" && item.Source != "drive_scan" {
				results[idx].Source = item.Source
				results[idx].SourceName = item.SourceName
			}
			return
		}

		seen[key] = len(results)
		results = append(results, item)
	}

	// 1. 优先加入当前已激活的游戏路径
	currentActive := filepath.Clean(s.Config.GameDir)
	if currentActive != "" && currentActive != "." {
		addOrUpdate(s.InspectGameDir(currentActive, "history", "当前生效路径", true))
	}

	// 2. 加入历史保存过的已知路径
	for _, p := range s.Config.KnownGameDirs {
		if p == "" {
			continue
		}
		cleanP := filepath.Clean(p)
		isActive := strings.EqualFold(cleanP, currentActive)
		addOrUpdate(s.InspectGameDir(cleanP, "history", "历史记录", isActive))
	}

	// 3. 执行平台原生深度探测 (注册表、Steam 库、正在运行的进程、各盘符扫描)
	rawCandidates := s.DetectPlatformGameDirs()
	for _, raw := range rawCandidates {
		isActive := strings.EqualFold(filepath.Clean(raw.Path), currentActive)
		item := s.InspectGameDir(raw.Path, raw.Source, raw.SourceName, isActive)
		// 仅将通过有效性验证的探测结果加入
		if item.IsValid {
			addOrUpdate(item)
		}
	}

	// 4. 将探测到的有效路径同步记录在 KnownGameDirs 中
	var newKnown []string
	knownMap := make(map[string]bool)
	for _, item := range results {
		if item.IsValid {
			key := strings.ToLower(item.Path)
			if !knownMap[key] {
				knownMap[key] = true
				newKnown = append(newKnown, item.Path)
			}
		}
	}
	s.Config.KnownGameDirs = newKnown
	_ = s.SaveConfig(s.Config)

	// 5. 排序规则：当前激活项优先，随后有效项优先，并按来源优先级排序
	sourcePriority := map[string]int{
		"process":    1,
		"steam":      2,
		"history":    3,
		"manual":     4,
		"drive_scan": 5,
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].IsActive != results[j].IsActive {
			return results[i].IsActive
		}
		if results[i].IsValid != results[j].IsValid {
			return results[i].IsValid
		}
		pI := sourcePriority[results[i].Source]
		pJ := sourcePriority[results[j].Source]
		if pI != pJ {
			return pI < pJ
		}
		return results[i].Path < results[j].Path
	})

	return results
}

// GetKnownGameDirs 获取当前已记录的游戏路径列表（包含当前激活路径与有效性状态）
func (s *GameService) GetKnownGameDirs() []models.GamePathItem {
	var list []models.GamePathItem
	seen := make(map[string]bool)

	currentActive := filepath.Clean(s.Config.GameDir)
	if currentActive != "" && currentActive != "." {
		seen[strings.ToLower(currentActive)] = true
		list = append(list, s.InspectGameDir(currentActive, "history", "当前生效路径", true))
	}

	for _, p := range s.Config.KnownGameDirs {
		if p == "" {
			continue
		}
		cleanP := filepath.Clean(p)
		key := strings.ToLower(cleanP)
		if seen[key] {
			continue
		}
		seen[key] = true
		isActive := strings.EqualFold(cleanP, currentActive)
		list = append(list, s.InspectGameDir(cleanP, "history", "历史记录", isActive))
	}
	return list
}

// SwitchGameDir 切换当前生效的游戏目录，并确保持久化至已知路径库
func (s *GameService) SwitchGameDir(dir string) error {
	cleanDir := filepath.Clean(dir)
	if !s.ValidateGameDir(cleanDir) {
		return fmt.Errorf("所选目录不是有效的铁锈战争根目录（需包含 game-lib.jar 和 mods/ 文件夹）")
	}

	s.Config.GameDir = cleanDir
	s.ensureKnownGameDir(cleanDir)
	return s.SaveConfig(s.Config)
}

// AddCustomGameDir 添加并验证用户自定义指定的路径
func (s *GameService) AddCustomGameDir(dir string) (models.GamePathItem, error) {
	cleanDir := filepath.Clean(dir)
	if !s.ValidateGameDir(cleanDir) {
		return models.GamePathItem{}, fmt.Errorf("所选目录不是有效的铁锈战争根目录（需包含 game-lib.jar 和 mods/ 文件夹）")
	}

	s.Config.GameDir = cleanDir
	s.ensureKnownGameDir(cleanDir)
	_ = s.SaveConfig(s.Config)

	return s.InspectGameDir(cleanDir, "manual", "自定义添加", true), nil
}

// RemoveKnownGameDir 从已知路径库中移除指定路径
func (s *GameService) RemoveKnownGameDir(dir string) error {
	cleanDir := filepath.Clean(dir)
	var updated []string
	for _, p := range s.Config.KnownGameDirs {
		if !strings.EqualFold(filepath.Clean(p), cleanDir) {
			updated = append(updated, p)
		}
	}
	s.Config.KnownGameDirs = updated

	// 若移除的是当前激活项
	if strings.EqualFold(filepath.Clean(s.Config.GameDir), cleanDir) {
		if len(updated) > 0 {
			// 自动回退切换到第一个已记录的有效路径
			s.Config.GameDir = updated[0]
		} else {
			s.Config.GameDir = ""
		}
	}

	return s.SaveConfig(s.Config)
}

// AutoDetectGameDir 自动探测系统中的《铁锈战争》安装路径（首选最佳可用路径）
func (s *GameService) AutoDetectGameDir() string {
	dirs := s.DetectAllGameDirs()
	for _, d := range dirs {
		if d.IsValid {
			return d.Path
		}
	}
	return ""
}

// ensureKnownGameDir 确保路径存在于 KnownGameDirs 数组中
func (s *GameService) ensureKnownGameDir(dir string) {
	cleanDir := filepath.Clean(dir)
	for _, p := range s.Config.KnownGameDirs {
		if strings.EqualFold(filepath.Clean(p), cleanDir) {
			return
		}
	}
	s.Config.KnownGameDirs = append(s.Config.KnownGameDirs, cleanDir)
}
