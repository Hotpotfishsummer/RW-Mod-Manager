package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	stdRuntime "runtime"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"rw_mod_manager/internal/models"
	"rw_mod_manager/internal/services"
)

// App 结构体，所有公开方法将暴露给前端调用
type App struct {
	ctx           context.Context
	gameService   *services.GameService
	prefService   *services.PrefService
	modService    *services.ModService
	mapService    *services.MapService
	launchService *services.LaunchService
}

// NewApp 创建 App 实例并注入各项服务
func NewApp() *App {
	gameSvc := services.NewGameService()
	prefSvc := services.NewPrefService()
	modSvc := services.NewModService(prefSvc)
	mapSvc := services.NewMapService(prefSvc)
	launchSvc := services.NewLaunchService()

	return &App{
		gameService:   gameSvc,
		prefService:   prefSvc,
		modService:    modSvc,
		mapService:    mapSvc,
		launchService: launchSvc,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
}

// GetConfig 获取当前配置
func (a *App) GetConfig() models.AppConfig {
	cfg := a.gameService.Config
	if a.gameService.ValidateGameDir(cfg.GameDir) {
		if prefs, err := a.prefService.ReadGamePreferences(cfg.GameDir); err == nil {
			cfg.GamePreferences = prefs
		}
	}
	return cfg
}

// SaveConfig 保存用户配置 (如启动参数等)
func (a *App) SaveConfig(cfg models.AppConfig) error {
	if a.gameService.ValidateGameDir(cfg.GameDir) {
		_ = a.prefService.SaveGamePreferences(cfg.GameDir, cfg.GamePreferences)
	}
	return a.gameService.SaveConfig(cfg)
}

// SelectGameDir 打开系统原生目录选择对话框
func (a *App) SelectGameDir() string {
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择《铁锈战争》根目录 (包含 game-lib.jar 和 mods)",
	})
	if err != nil || dir == "" {
		return ""
	}
	return dir
}

// SetGameDir 设置并验证新的游戏目录
func (a *App) SetGameDir(dir string) (models.GameStatus, error) {
	if !a.gameService.ValidateGameDir(dir) {
		return a.GetGameStatus(), fmt.Errorf("所选目录不是有效的铁锈战争根目录（需包含 game-lib.jar 和 mods/ 文件夹）")
	}
	a.gameService.Config.GameDir = dir
	_ = a.gameService.SaveConfig(a.gameService.Config)
	return a.GetGameStatus(), nil
}

// GetGameStatus 获取当前游戏客户端与模组整体健康状态
func (a *App) GetGameStatus() models.GameStatus {
	dir := a.gameService.Config.GameDir
	status := models.GameStatus{
		GameDir: dir,
		IsValid: a.gameService.ValidateGameDir(dir),
	}
	if !status.IsValid {
		return status
	}

	mods, err := a.modService.ScanMods(dir)
	if err == nil {
		status.TotalMods = len(mods)
		for _, m := range mods {
			if m.Enabled {
				status.EnabledMods++
			} else {
				status.DisabledMods++
			}
		}
	}

	maps, err := a.mapService.ScanMaps(dir)
	if err == nil {
		status.TotalMaps = len(maps)
		for _, m := range maps {
			if m.Active {
				status.ActiveMaps++
			}
		}
	}

	return status
}

// ScanMods 扫描并返回所有单位模组
func (a *App) ScanMods() ([]models.ModItem, error) {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return nil, fmt.Errorf("游戏路径未配置或无效")
	}
	return a.modService.ScanMods(dir)
}

// ToggleMod 切换单个模组的启用/禁用状态
func (a *App) ToggleMod(fileName string, enabled bool) error {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return fmt.Errorf("游戏路径未配置或无效")
	}

	currentMods, err := a.modService.ScanMods(dir)
	if err != nil {
		return err
	}

	statusMap := make(map[string]bool)
	for _, m := range currentMods {
		if m.FileName == fileName {
			statusMap[m.FileName] = enabled
		} else {
			statusMap[m.FileName] = m.Enabled
		}
	}

	return a.prefService.SaveModSettings(dir, statusMap)
}

// BatchToggleMods 全选启用或全部禁用
func (a *App) BatchToggleMods(enableAll bool) error {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return fmt.Errorf("游戏路径未配置或无效")
	}

	currentMods, err := a.modService.ScanMods(dir)
	if err != nil {
		return err
	}

	statusMap := make(map[string]bool)
	for _, m := range currentMods {
		statusMap[m.FileName] = enableAll
	}

	return a.prefService.SaveModSettings(dir, statusMap)
}

// ScanMaps 扫描并返回所有地图 (包含独立地图与所有模组中附带的地图)
func (a *App) ScanMaps() ([]models.MapItem, error) {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return nil, fmt.Errorf("游戏路径未配置或无效")
	}
	return a.mapService.ScanMaps(dir)
}

// ActivateMapParentMod 一键激活该地图所属的宿主模组
func (a *App) ActivateMapParentMod(parentModFile string) error {
	if parentModFile == "" {
		return nil
	}
	return a.ToggleMod(parentModFile, true)
}

// GenerateMapThumbnail 为指定地图生成高清缩略图 (带 1,2,3... 出生点)
func (a *App) GenerateMapThumbnail(tmxPath string) (string, error) {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return "", fmt.Errorf("游戏路径未配置或无效")
	}
	return a.mapService.GenerateThumbnail(dir, tmxPath)
}

// BatchGenerateThumbnails 批量扫描为缺失缩略图的地图生成预览
func (a *App) BatchGenerateThumbnails() (int, error) {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return 0, fmt.Errorf("游戏路径未配置或无效")
	}
	return a.mapService.BatchGenerateMissingThumbnails(dir)
}

// GetGamePreferences 获取 preferences.ini 中的内置画质、高刷与帧率参数
func (a *App) GetGamePreferences() (models.GamePreferences, error) {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return models.GamePreferences{}, fmt.Errorf("游戏路径未配置或无效")
	}
	return a.prefService.ReadGamePreferences(dir)
}

// SaveGamePreferences 保存画质、高刷、垂直同步与单位上限等设置
func (a *App) SaveGamePreferences(prefs models.GamePreferences) error {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return fmt.Errorf("游戏路径未配置或无效")
	}
	a.gameService.Config.GamePreferences = prefs
	_ = a.gameService.SaveConfig(a.gameService.Config)
	return a.prefService.SaveGamePreferences(dir, prefs)
}

// LaunchGame 拉起启动铁锈战争游戏
func (a *App) LaunchGame() error {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return fmt.Errorf("游戏路径未配置或无效")
	}
	return a.launchService.LaunchGame(dir, a.gameService.Config.LaunchConfig)
}

// OpenFolder 在资源管理器中打开指定目录
func (a *App) OpenFolder(folderType string) error {
	dir := a.gameService.Config.GameDir
	if dir == "" {
		return fmt.Errorf("游戏根目录未设置")
	}

	target := dir
	switch folderType {
	case "units":
		target = filepath.Join(dir, "mods", "units")
	case "maps":
		target = filepath.Join(dir, "mods", "maps")
	case "tools":
		target = filepath.Join(dir, "tools")
	}

	var cmd *exec.Cmd
	if stdRuntime.GOOS == "windows" {
		cmd = exec.Command("explorer", target)
	} else {
		cmd = exec.Command("open", target)
	}
	return cmd.Start()
}
