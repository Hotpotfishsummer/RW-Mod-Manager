package services

import (
	"encoding/json"
	"os"
	"path/filepath"
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

	s := &GameService{
		configPath: cfgPath,
		Config: models.AppConfig{
			LaunchConfig: models.LaunchConfig{
				ExecutableType: "exe64",
				MaxMemoryMB:    4096,
				Fullscreen:     false,
				NoMods:         false,
				SafeMode:       false,
				ExtraArgs:      "",
			},
		},
	}

	s.LoadConfig()

	// 若未配置游戏路径，尝试自动探测
	if s.Config.GameDir == "" || !s.ValidateGameDir(s.Config.GameDir) {
		detected := s.AutoDetectGameDir()
		if detected != "" {
			s.Config.GameDir = detected
			_ = s.SaveConfig(s.Config)
		}
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
		if cfg.LaunchConfig.MaxMemoryMB > 0 {
			s.Config.LaunchConfig = cfg.LaunchConfig
		}
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
	// 校验核心特征文件：game-lib.jar 和 mods 文件夹
	jarPath := filepath.Join(dir, "game-lib.jar")
	if fi, err := os.Stat(jarPath); err != nil || fi.IsDir() {
		return false
	}
	modsPath := filepath.Join(dir, "mods")
	if fi, err := os.Stat(modsPath); err != nil || !fi.IsDir() {
		return false
	}
	return true
}

// AutoDetectGameDir 自动探测系统中的《铁锈战争》安装路径
func (s *GameService) AutoDetectGameDir() string {
	candidates := []string{
		`E:\Rusted Warfare\Rusted Warfare`,
		`D:\Rusted Warfare\Rusted Warfare`,
		`C:\Rusted Warfare\Rusted Warfare`,
		`E:\SteamLibrary\steamapps\common\Rusted Warfare`,
		`D:\SteamLibrary\steamapps\common\Rusted Warfare`,
		`C:\Program Files (x86)\Steam\steamapps\common\Rusted Warfare`,
	}

	for _, p := range candidates {
		if s.ValidateGameDir(p) {
			return p
		}
	}
	return ""
}
