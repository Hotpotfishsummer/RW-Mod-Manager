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
	mapSvc := services.NewMapService()
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
	return a.gameService.Config
}

// SaveConfig 保存用户配置 (如启动参数等)
func (a *App) SaveConfig(cfg models.AppConfig) error {
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

	// 重新读取现有所有状态，更新该项后写回
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

// ScanMaps 扫描并返回所有地图
func (a *App) ScanMaps() ([]models.MapItem, error) {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return nil, fmt.Errorf("游戏路径未配置或无效")
	}
	return a.mapService.ScanMaps(dir)
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
