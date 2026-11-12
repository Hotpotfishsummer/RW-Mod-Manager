package main

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	stdRuntime "runtime"
	"runtime/debug"
	"time"

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
	lanService    *services.LanService
	updateService *services.UpdateService
}

// NewApp 创建 App 实例并注入各项服务
func NewApp() *App {
	gameSvc := services.NewGameService()
	prefSvc := services.NewPrefService()
	modSvc := services.NewModService(prefSvc)
	mapSvc := services.NewMapService(prefSvc)
	launchSvc := services.NewLaunchService()
	lanSvc := services.NewLanService(gameSvc, prefSvc, modSvc, mapSvc)
	updateSvc := services.NewUpdateService(gameSvc, lanSvc)

	return &App{
		gameService:   gameSvc,
		prefService:   prefSvc,
		modService:    modSvc,
		mapService:    mapSvc,
		launchService: launchSvc,
		lanService:    lanSvc,
		updateService: updateSvc,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.lanService.SetWailsContext(ctx)
	a.updateService.SetWailsContext(ctx)
	a.lanService.Start()

	// 启动后异步调度内存整理与操作系统内存返还，进一步压缩常驻工作集
	go func() {
		time.Sleep(3 * time.Second)
		stdRuntime.GC()
		debug.FreeOSMemory()
	}()
}

func (a *App) shutdown(ctx context.Context) {
	if a.lanService != nil {
		a.lanService.Stop()
	}
	// 退出前刷盘持久化全部缓存
	services.GetCacheManager().FlushAll()
}

// AssetHandler 提供 Wails AssetServer 自定义 HTTP 资源流式传输 (用于模组与地图封面，避免高占用 Base64)
func (a *App) AssetHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/thumbnail/mod", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		dir := a.gameService.Config.GameDir
		if dir == "" || name == "" {
			http.NotFound(w, r)
			return
		}
		a.modService.ServeModThumbnail(w, r, dir, name)
	})

	mux.HandleFunc("/api/thumbnail/map", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		dir := a.gameService.Config.GameDir
		if dir == "" || id == "" {
			http.NotFound(w, r)
			return
		}
		a.mapService.ServeMapThumbnail(w, r, dir, id)
	})

	return mux
}

// FreeMemory 显式触发 GC 与物理内存返还
func (a *App) FreeMemory() {
	stdRuntime.GC()
	debug.FreeOSMemory()
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
	// 避免前端传递的旧 LanConfig 冲掉已切换的服务端/客户端角色
	currentLan := a.lanService.GetLanConfig()
	cfg.LanConfig = currentLan
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

// DetectGameDirs 自动探测系统中全部可用的铁锈战争安装路径 (注册表/Steam库/进程/磁盘扫描)
func (a *App) DetectGameDirs() []models.GamePathItem {
	return a.gameService.DetectAllGameDirs()
}

// GetKnownGameDirs 获取当前已记录的游戏路径列表
func (a *App) GetKnownGameDirs() []models.GamePathItem {
	return a.gameService.GetKnownGameDirs()
}

// SwitchGameDir 切换当前游戏根路径并刷新健康状态
func (a *App) SwitchGameDir(dir string) (models.GameStatus, error) {
	if err := a.gameService.SwitchGameDir(dir); err != nil {
		return a.GetGameStatus(), err
	}
	return a.GetGameStatus(), nil
}

// AddCustomGameDir 添加并切换至自定义游戏根目录
func (a *App) AddCustomGameDir(dir string) (models.GamePathItem, error) {
	item, err := a.gameService.AddCustomGameDir(dir)
	return item, err
}

// RemoveKnownGameDir 从记录中移除某个游戏路径
func (a *App) RemoveKnownGameDir(dir string) ([]models.GamePathItem, error) {
	if err := a.gameService.RemoveKnownGameDir(dir); err != nil {
		return nil, err
	}
	return a.gameService.GetKnownGameDirs(), nil
}

// SetGameDir 设置并验证新的游戏目录 (兼容既有前端调用)
func (a *App) SetGameDir(dir string) (models.GameStatus, error) {
	return a.SwitchGameDir(dir)
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

// ArchiveMod 归档或删除指定的单位模组
func (a *App) ArchiveMod(fileName string, permanently bool) error {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return fmt.Errorf("游戏路径未配置或无效")
	}
	return a.modService.ArchiveMod(dir, fileName, permanently)
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

// BatchGenerateThumbnails 批量扫描为地图生成或重绘高清出生点预览 (force=true 时全量强制重绘，覆盖已有未标记图；force=false 时仅补齐缺失)
func (a *App) BatchGenerateThumbnails(force bool) (int, error) {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return 0, fmt.Errorf("游戏路径未配置或无效")
	}
	return a.mapService.BatchGenerateThumbnails(dir, force)
}

// ArchiveMap 归档或删除指定的独立地图或地图包
func (a *App) ArchiveMap(mapTarget string, permanently bool) error {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return fmt.Errorf("游戏路径未配置或无效")
	}
	return a.mapService.ArchiveMap(dir, mapTarget, permanently)
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
	case "game", "root":
		target = dir
	}

	return a.OpenPath(target)
}

// OpenPath 在系统文件管理器中打开任意有效路径
func (a *App) OpenPath(targetPath string) error {
	if targetPath == "" {
		return fmt.Errorf("路径为空")
	}
	var cmd *exec.Cmd
	if stdRuntime.GOOS == "windows" {
		cmd = exec.Command("explorer", targetPath)
	} else {
		cmd = exec.Command("open", targetPath)
	}
	return cmd.Start()
}

// GetLanConfig 获取当前局域网与同步设置
func (a *App) GetLanConfig() models.LanConfig {
	return a.lanService.GetLanConfig()
}

// SaveLanConfig 保存局域网配置
func (a *App) SaveLanConfig(cfg models.LanConfig) error {
	return a.lanService.SaveLanConfig(cfg)
}

// GetNetworkInterfaces 获取网卡列表
func (a *App) GetNetworkInterfaces() []models.NetworkInterfaceInfo {
	return a.lanService.GetNetworkInterfaces()
}

// SwitchLanRole 切换客户端/服务端角色
func (a *App) SwitchLanRole(role string) error {
	return a.lanService.SwitchRole(models.LanRole(role))
}

// GetLanStatus 获取局域网总览状态
func (a *App) GetLanStatus() models.LanStatus {
	return a.lanService.GetLanStatus()
}

// GetDiscoveredPeers 获取发现的所有节点
func (a *App) GetDiscoveredPeers() []models.LanPeer {
	return a.lanService.GetDiscoveredPeers()
}

// GetDynamicPreset 获取本机动态启动预设
func (a *App) GetDynamicPreset() (*models.LaunchPreset, error) {
	return a.lanService.GetDynamicPreset()
}

// RefreshDynamicPreset 强制刷新动态预设
func (a *App) RefreshDynamicPreset() (*models.LaunchPreset, error) {
	return a.lanService.RefreshDynamicPreset()
}

// ComparePresetWithServer 与指定服务端预设对比差异
func (a *App) ComparePresetWithServer(serverIP string, serverPort int) (*models.SyncDiff, error) {
	return a.lanService.ComparePresetWithServer(serverIP, serverPort)
}

// SyncFromHost 客户端从服务端同步完整预设 (文件与状态)
func (a *App) SyncFromHost(serverIP string, serverPort int) error {
	return a.lanService.SyncWithServer(serverIP, serverPort)
}

// RequestPushToHost 客户端发起推送文件给服务端
func (a *App) RequestPushToHost(serverIP string, serverPort int, itemType string, fileName string) (*models.PushResult, error) {
	return a.lanService.RequestPushToHost(serverIP, serverPort, itemType, fileName)
}

// HandlePushDecision 服务端审批客户端推送
func (a *App) HandlePushDecision(requestID string, accept bool) error {
	return a.lanService.HandlePushDecision(requestID, accept)
}

// GetPendingPushRequests 服务端获取待审批的推送
func (a *App) GetPendingPushRequests() []models.PushRequest {
	return a.lanService.GetPendingPushRequests()
}

// LaunchGameOnline 联机模式启动：检查冲突并强制从服务端同步预设后拉起游戏
func (a *App) LaunchGameOnline() error {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return fmt.Errorf("游戏路径未配置或无效")
	}

	st := a.lanService.GetLanStatus()
	if st.HasServerConflict {
		return fmt.Errorf("局域网检测到多个服务端冲突，操作已被禁止！请先在局域网面板中切换解决冲突")
	}

	if st.Role == models.RoleClient {
		if st.ActiveServer == nil {
			return fmt.Errorf("当前网络中未找到有效的房主服务端，请确认房主已启动并处于同一网络")
		}
		// 自动执行从服务端到客户端的完整同步 (下载缺失文件 + 同步启用状态)
		if err := a.lanService.SyncWithServer(st.ActiveServer.IP, st.ActiveServer.HttpPort); err != nil {
			return fmt.Errorf("联机前同步房主预设失败: %w", err)
		}
	} else if st.Role == models.RoleServer {
		// 服务端即为房主：生成并更新本地动态预设，供所有连入的客户端同步
		_, _ = a.lanService.RefreshDynamicPreset()
	}

	return a.launchService.LaunchGame(dir, a.gameService.Config.LaunchConfig)
}

// LaunchGameSingle 单机模式启动：以本机当前独立配置拉起游戏，不干涉联机预设
func (a *App) LaunchGameSingle() error {
	dir := a.gameService.Config.GameDir
	if !a.gameService.ValidateGameDir(dir) {
		return fmt.Errorf("游戏路径未配置或无效")
	}
	return a.launchService.LaunchGame(dir, a.gameService.Config.LaunchConfig)
}

// CheckForUpdates 统一检测更新 (同时检索局域网在线节点与 GitHub Releases)
func (a *App) CheckForUpdates() (*models.UpdateInfo, error) {
	return a.updateService.CheckForUpdates()
}

// DownloadAndApplyUpdate 下载更新包并执行平滑热替换与自动重启
func (a *App) DownloadAndApplyUpdate(source string) error {
	return a.updateService.DownloadAndApplyUpdate(models.UpdateSource(source))
}

// GetCurrentVersion 获取当前运行的应用版本号
func (a *App) GetCurrentVersion() string {
	return a.updateService.GetCurrentVersion()
}
