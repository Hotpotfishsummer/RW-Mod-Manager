package services

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"rw_mod_manager/internal/models"
)

// DiscoveryBeacon UDP 局域网广播心跳包
type DiscoveryBeacon struct {
	Type          string         `json:"type"`          // "beacon" | "goodbye"
	InstanceID    string         `json:"instanceId"`    // 实例 ID
	DeviceName    string         `json:"deviceName"`    // 设备名
	Role          models.LanRole `json:"role"`          // "server" | "client"
	HttpPort      int            `json:"httpPort"`      // 节点 HTTP API 端口
	Subnet        string         `json:"subnet"`        // 绑定的子网 (若有)
	PresetVersion string         `json:"presetVersion"` // 动态预设特征哈希
	AppVersion    string         `json:"appVersion"`    // 应用版本号
	Timestamp     int64          `json:"timestamp"`     // 时间戳
}

// LanService 负责局域网服务发现、服务端唯一性校验、启动预设动态感知与模组双向同步
type LanService struct {
	gameService *GameService
	prefService *PrefService
	modService  *ModService
	mapService  *MapService

	mu                  sync.RWMutex
	peers               map[string]*models.LanPeer
	pendingPushRequests map[string]*models.PushRequest
	currentPreset       *models.LaunchPreset
	hasServerConflict   bool
	conflictingServers  []models.LanPeer

	wailsCtx  context.Context
	ctx       context.Context
	cancel    context.CancelFunc
	udpConn   *net.UDPConn
	httpSrv   *http.Server
	actualPort int
}

func NewLanService(game *GameService, pref *PrefService, mod *ModService, mapSvc *MapService) *LanService {
	ctx, cancel := context.WithCancel(context.Background())
	return &LanService{
		gameService:         game,
		prefService:         pref,
		modService:          mod,
		mapService:          mapSvc,
		peers:               make(map[string]*models.LanPeer),
		pendingPushRequests: make(map[string]*models.PushRequest),
		ctx:                 ctx,
		cancel:              cancel,
	}
}

// SetWailsContext 注入 Wails 运行时上下文以便向前端发射推送事件
func (s *LanService) SetWailsContext(ctx context.Context) {
	s.wailsCtx = ctx
}

// Start 启动局域网后台服务 (HTTP API, UDP 心跳广播与动态预设监听)
func (s *LanService) Start() {
	s.RefreshDynamicPreset()

	go s.startHttpServer()
	go s.startUdpListener()
	go s.startHeartbeatBroadcaster()
	go s.startPresetWatcher()
	go s.startPeerHousekeeping()
}

// Stop 停止局域网服务并广播告别信标
func (s *LanService) Stop() {
	s.broadcastGoodbye()
	if s.cancel != nil {
		s.cancel()
	}
	if s.udpConn != nil {
		_ = s.udpConn.Close()
	}
	if s.httpSrv != nil {
		ctx, c := context.WithTimeout(context.Background(), 2*time.Second)
		defer c()
		_ = s.httpSrv.Shutdown(ctx)
	}
}

// GetNetworkInterfaces 枚举系统所有的有效网络适配器与子网
func (s *LanService) GetNetworkInterfaces() []models.NetworkInterfaceInfo {
	list := make([]models.NetworkInterfaceInfo, 0)

	ifaces, err := net.Interfaces()
	if err != nil {
		return list
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip4 := ipNet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() {
				continue
			}

			mask := ipNet.Mask
			if len(mask) != 4 {
				continue
			}

			// 计算广播地址: IP | (~Mask)
			bcast := make(net.IP, 4)
			for i := 0; i < 4; i++ {
				bcast[i] = ip4[i] | ^mask[i]
			}

			maskStr := fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
			isDef := strings.HasPrefix(ip4.String(), "192.168.") || strings.HasPrefix(ip4.String(), "10.")

			list = append(list, models.NetworkInterfaceInfo{
				Name:        iface.Name,
				IP:          ip4.String(),
				Mask:        maskStr,
				Subnet:      ipNet.String(),
				BroadcastIP: bcast.String(),
				IsDefault:   isDef,
			})
		}
	}

	return list
}

// GetLanConfig 获取当前局域网与同步设置
func (s *LanService) GetLanConfig() models.LanConfig {
	return s.gameService.Config.LanConfig
}

// SaveLanConfig 保存局域网配置
func (s *LanService) SaveLanConfig(cfg models.LanConfig) error {
	s.mu.Lock()
	s.gameService.Config.LanConfig = cfg
	s.mu.Unlock()

	return s.gameService.SaveConfig(s.gameService.Config)
}

// SwitchRole 快速切换客户端/服务端角色，并立即触发局域网心跳
func (s *LanService) SwitchRole(newRole models.LanRole) error {
	s.mu.Lock()
	s.gameService.Config.LanConfig.Role = newRole
	s.mu.Unlock()

	_ = s.gameService.SaveConfig(s.gameService.Config)
	if newRole == models.RoleServer {
		_, _ = s.RefreshDynamicPreset()
	}
	s.broadcastBeacon()
	s.checkServerConflict()

	if s.wailsCtx != nil {
		wailsRuntime.EventsEmit(s.wailsCtx, "lan:status_updated", s.GetLanStatus())
	}
	return nil
}

// GetLanStatus 获取当前局域网运行状态
func (s *LanService) GetLanStatus() models.LanStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg := s.gameService.Config.LanConfig
	localIP := s.resolveSelectedIP()

	var activeSvr *models.LanPeer
	conflicts := make([]models.LanPeer, 0)

	for _, p := range s.peers {
		if p.Role == models.RoleServer {
			if cfg.Role == models.RoleServer && p.InstanceID != cfg.InstanceID {
				conflicts = append(conflicts, *p)
			} else if cfg.Role == models.RoleClient {
				if activeSvr == nil {
					pCopy := *p
					activeSvr = &pCopy
				} else {
					conflicts = append(conflicts, *p)
				}
			}
		}
	}

	hasConflict := s.hasServerConflict
	if cfg.Role == models.RoleClient && len(conflicts) > 0 {
		hasConflict = true
	}

	presetHash := ""
	if s.currentPreset != nil {
		presetHash = s.currentPreset.VersionHash
	}

	return models.LanStatus{
		Role:               cfg.Role,
		InstanceID:         cfg.InstanceID,
		DeviceName:         cfg.DeviceName,
		SelectedSubnet:     cfg.SelectedSubnet,
		LocalIP:            localIP,
		HttpPort:           s.actualPort,
		HasServerConflict:  hasConflict,
		ConflictingServers: conflicts,
		ActiveServer:       activeSvr,
		PeerCount:          len(s.peers),
		PresetVersionHash:  presetHash,
		AppVersion:         models.AppVersion,
		AutoAcceptPushes:   cfg.AutoAcceptClientPushes,
	}
}

// GetDiscoveredPeers 获取当前已探测到的所有局域网在线节点
func (s *LanService) GetDiscoveredPeers() []models.LanPeer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]models.LanPeer, 0)
	for _, p := range s.peers {
		list = append(list, *p)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].DeviceName < list[j].DeviceName
	})
	return list
}

// GetDynamicPreset 获取当前动态维护的启动预设
func (s *LanService) GetDynamicPreset() (*models.LaunchPreset, error) {
	s.mu.RLock()
	preset := s.currentPreset
	s.mu.RUnlock()

	if preset == nil {
		p, err := s.RefreshDynamicPreset()
		if err != nil {
			return &models.LaunchPreset{
				ID:          "dynamic",
				Name:        "动态启动预设",
				UpdatedAt:   time.Now().Unix(),
				VersionHash: "",
				Mods:        []models.PresetModItem{},
				Maps:        []models.PresetMapItem{},
			}, nil
		}
		return p, nil
	}
	return preset, nil
}

// RefreshDynamicPreset 重新扫描游戏目录与 preferences.ini，构建最新的动态启动预设
func (s *LanService) RefreshDynamicPreset() (*models.LaunchPreset, error) {
	gameDir := s.gameService.Config.GameDir
	if !s.gameService.ValidateGameDir(gameDir) {
		return nil, fmt.Errorf("游戏目录未配置或无效")
	}

	mods, err := s.modService.ScanMods(gameDir)
	if err != nil {
		return nil, err
	}

	maps, err := s.mapService.ScanMaps(gameDir)
	if err != nil {
		return nil, err
	}

	presetMods := make([]models.PresetModItem, 0)
	for _, m := range mods {
		presetMods = append(presetMods, models.PresetModItem{
			FileName: m.FileName,
			Title:    m.Title,
			Hash:     m.ID,
			FileSize: m.FileSize,
			Enabled:  m.Enabled,
			IsDir:    m.IsDir,
		})
	}
	sort.Slice(presetMods, func(i, j int) bool {
		return presetMods[i].FileName < presetMods[j].FileName
	})

	presetMaps := make([]models.PresetMapItem, 0)
	for _, mp := range maps {
		presetMaps = append(presetMaps, models.PresetMapItem{
			FileName:      mp.FileName,
			Title:         mp.Title,
			FilePath:      mp.FilePath,
			LocationType:  mp.LocationType,
			Active:        mp.Active,
			FileSize:      mp.FileSize,
			ParentModFile: mp.ParentModFile,
		})
	}
	sort.Slice(presetMaps, func(i, j int) bool {
		return presetMaps[i].FileName < presetMaps[j].FileName
	})

	// 计算确定性特征哈希 (VersionHash)
	hasher := sha256.New()
	for _, m := range presetMods {
		hasher.Write([]byte(fmt.Sprintf("%s|%d|%t;", m.FileName, m.FileSize, m.Enabled)))
	}
	for _, mp := range presetMaps {
		hasher.Write([]byte(fmt.Sprintf("%s|%d|%t;", mp.FileName, mp.FileSize, mp.Active)))
	}
	versionHash := fmt.Sprintf("%064X", hasher.Sum(nil))[:16]

	preset := &models.LaunchPreset{
		ID:          "dynamic",
		Name:        "动态启动预设",
		UpdatedAt:   time.Now().Unix(),
		VersionHash: versionHash,
		Mods:        presetMods,
		Maps:        presetMaps,
	}

	s.mu.Lock()
	changed := s.currentPreset == nil || s.currentPreset.VersionHash != versionHash
	s.currentPreset = preset
	s.mu.Unlock()

	if changed && s.wailsCtx != nil {
		wailsRuntime.EventsEmit(s.wailsCtx, "lan:preset_changed", preset)
	}

	return preset, nil
}

// ComparePresetWithServer 与指定服务端的启动预设对比差异
func (s *LanService) ComparePresetWithServer(serverIP string, serverPort int) (*models.SyncDiff, error) {
	url := fmt.Sprintf("http://%s:%d/api/lan/preset", serverIP, serverPort)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("无法连接到服务端 (%s:%d): %w", serverIP, serverPort, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务端返回异常状态码: %d", resp.StatusCode)
	}

	var serverPreset models.LaunchPreset
	if err := json.NewDecoder(resp.Body).Decode(&serverPreset); err != nil {
		return nil, fmt.Errorf("解析服务端预设失败: %w", err)
	}

	localPreset, err := s.GetDynamicPreset()
	if err != nil {
		return nil, err
	}

	diff := &models.SyncDiff{
		ServerPresetHash: serverPreset.VersionHash,
		ClientPresetHash: localPreset.VersionHash,
		MissingMods:      make([]models.PresetModItem, 0),
		MissingMaps:      make([]models.PresetMapItem, 0),
		StateMismatches:  make([]models.PresetModItem, 0),
	}

	localModMap := make(map[string]models.PresetModItem)
	for _, m := range localPreset.Mods {
		localModMap[m.FileName] = m
	}

	// 检查模组缺失与状态不一致
	for _, sm := range serverPreset.Mods {
		if lm, exists := localModMap[sm.FileName]; !exists {
			diff.MissingMods = append(diff.MissingMods, sm)
		} else if lm.Enabled != sm.Enabled {
			diff.StateMismatches = append(diff.StateMismatches, sm)
		}
	}

	// 检查地图缺失
	localMapMap := make(map[string]models.PresetMapItem)
	for _, mp := range localPreset.Maps {
		localMapMap[mp.FileName] = mp
	}

	for _, smp := range serverPreset.Maps {
		if smp.LocationType == "standalone" || smp.LocationType == "map_pack" {
			if _, exists := localMapMap[smp.FileName]; !exists {
				diff.MissingMaps = append(diff.MissingMaps, smp)
			}
		}
	}

	diff.IsInSync = len(diff.MissingMods) == 0 && len(diff.MissingMaps) == 0 && len(diff.StateMismatches) == 0

	var summaryParts []string
	if len(diff.MissingMods) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("缺失 %d 个模组", len(diff.MissingMods)))
	}
	if len(diff.MissingMaps) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("缺失 %d 张地图", len(diff.MissingMaps)))
	}
	if len(diff.StateMismatches) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d 个模组启禁用状态不一致", len(diff.StateMismatches)))
	}
	if diff.IsInSync {
		diff.Summary = "启动预设与房主服务端完全一致，已就绪"
	} else {
		diff.Summary = strings.Join(summaryParts, "，")
	}

	return diff, nil
}

// SyncWithServer 从服务端完整同步模组与启用状态
func (s *LanService) SyncWithServer(serverIP string, serverPort int) error {
	gameDir := s.gameService.Config.GameDir
	if !s.gameService.ValidateGameDir(gameDir) {
		return fmt.Errorf("游戏目录未配置或无效")
	}

	diff, err := s.ComparePresetWithServer(serverIP, serverPort)
	if err != nil {
		return err
	}

	unitsDir := filepath.Join(gameDir, "mods", "units")
	mapsDir := filepath.Join(gameDir, "mods", "maps")
	_ = os.MkdirAll(unitsDir, 0755)
	_ = os.MkdirAll(mapsDir, 0755)

	// 1. 下载缺失的模组
	for _, m := range diff.MissingMods {
		fileURL := fmt.Sprintf("http://%s:%d/api/lan/file?type=mod&name=%s", serverIP, serverPort, m.FileName)
		targetPath := filepath.Join(unitsDir, m.FileName)
		if err := s.downloadFile(fileURL, targetPath); err != nil {
			return fmt.Errorf("下载模组 %s 失败: %w", m.FileName, err)
		}
	}

	// 2. 下载缺失的地图
	for _, mp := range diff.MissingMaps {
		fileURL := fmt.Sprintf("http://%s:%d/api/lan/file?type=map&name=%s", serverIP, serverPort, mp.FileName)
		targetPath := filepath.Join(mapsDir, mp.FileName)
		if err := s.downloadFile(fileURL, targetPath); err != nil {
			return fmt.Errorf("下载地图 %s 失败: %w", mp.FileName, err)
		}
	}

	// 3. 对齐 preferences.ini 中的启用状态
	// 获取服务端的完整预设
	resp, err := http.Get(fmt.Sprintf("http://%s:%d/api/lan/preset", serverIP, serverPort))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var serverPreset models.LaunchPreset
	if err := json.NewDecoder(resp.Body).Decode(&serverPreset); err != nil {
		return err
	}

	// 重新扫描本地模组，并将服务端预设的启用状态应用上去
	localMods, _ := s.modService.ScanMods(gameDir)
	serverStatusMap := make(map[string]bool)
	for _, sm := range serverPreset.Mods {
		serverStatusMap[sm.FileName] = sm.Enabled
	}

	finalStatusMap := make(map[string]bool)
	for _, lm := range localMods {
		if en, ok := serverStatusMap[lm.FileName]; ok {
			finalStatusMap[lm.FileName] = en
		} else {
			// 服务端未启用的多余本地模组，在联机对齐时保持禁用
			finalStatusMap[lm.FileName] = false
		}
	}

	if err := s.prefService.SaveModSettings(gameDir, finalStatusMap); err != nil {
		return fmt.Errorf("写入 preferences.ini 模组状态失败: %w", err)
	}

	// 重新计算动态预设
	_, _ = s.RefreshDynamicPreset()
	return nil
}

// RequestPushToHost 客户端向服务端发起推送模组/地图文件的请求
func (s *LanService) RequestPushToHost(serverIP string, serverPort int, itemType string, fileName string) error {
	gameDir := s.gameService.Config.GameDir
	var localFilePath string
	if itemType == "mod" {
		localFilePath = filepath.Join(gameDir, "mods", "units", fileName)
	} else {
		localFilePath = filepath.Join(gameDir, "mods", "maps", fileName)
	}

	fi, err := os.Stat(localFilePath)
	if err != nil {
		return fmt.Errorf("本地文件不存在: %w", err)
	}

	cfg := s.gameService.Config.LanConfig
	reqPayload := map[string]interface{}{
		"peerId":   cfg.InstanceID,
		"peerName": cfg.DeviceName,
		"itemType": itemType,
		"fileName": fileName,
		"fileSize": fi.Size(),
	}
	payloadBytes, _ := json.Marshal(reqPayload)

	reqURL := fmt.Sprintf("http://%s:%d/api/lan/push-request", serverIP, serverPort)
	resp, err := http.Post(reqURL, "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("发起推送请求失败: %w", err)
	}
	defer resp.Body.Close()

	var pushResp struct {
		Accepted  bool   `json:"accepted"`
		Pending   bool   `json:"pending"`
		RequestID string `json:"requestId"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pushResp); err != nil {
		return fmt.Errorf("服务端响应解析失败: %w", err)
	}

	if pushResp.Accepted {
		// 服务端配置了自动接受，立即上传
		return s.uploadFileToServer(serverIP, serverPort, pushResp.RequestID, itemType, fileName, localFilePath)
	}

	if pushResp.Pending {
		// 轮询等待服务端房主手动确认 (最多等待 60 秒)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		timeout := time.After(60 * time.Second)

		for {
			select {
			case <-timeout:
				return fmt.Errorf("等待房主确认超时 (60s)")
			case <-ticker.C:
				statusURL := fmt.Sprintf("http://%s:%d/api/lan/push-status?id=%s", serverIP, serverPort, pushResp.RequestID)
				sResp, err := http.Get(statusURL)
				if err != nil {
					continue
				}
				var st models.PushRequest
				_ = json.NewDecoder(sResp.Body).Decode(&st)
				sResp.Body.Close()

				if st.Status == "accepted" {
					return s.uploadFileToServer(serverIP, serverPort, pushResp.RequestID, itemType, fileName, localFilePath)
				} else if st.Status == "rejected" {
					return fmt.Errorf("房主已拒绝此模组/地图的推送请求")
				}
			}
		}
	}

	return fmt.Errorf("推送失败: %s", pushResp.Message)
}

// HandlePushDecision 服务端处理客户端发起的推送请求 (接受或拒绝)
func (s *LanService) HandlePushDecision(requestID string, accept bool) error {
	s.mu.Lock()
	req, exists := s.pendingPushRequests[requestID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("未找到对应的推送请求: %s", requestID)
	}

	if accept {
		req.Status = "accepted"
	} else {
		req.Status = "rejected"
	}
	s.mu.Unlock()

	if s.wailsCtx != nil {
		wailsRuntime.EventsEmit(s.wailsCtx, "lan:push_requests_updated", s.GetPendingPushRequests())
	}
	return nil
}

// GetPendingPushRequests 获取当前所有处于 pending 状态的待处理推送请求
func (s *LanService) GetPendingPushRequests() []models.PushRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]models.PushRequest, 0)
	for _, r := range s.pendingPushRequests {
		if r.Status == "pending" {
			list = append(list, *r)
		}
	}
	return list
}

// ======================== 私有辅助方法与后台循环 ========================

func (s *LanService) resolveSelectedIP() string {
	cfg := s.gameService.Config.LanConfig
	ifaces := s.GetNetworkInterfaces()

	if cfg.SelectedSubnet != "" {
		for _, iface := range ifaces {
			if iface.Subnet == cfg.SelectedSubnet || iface.IP == cfg.SelectedSubnet {
				return iface.IP
			}
		}
	}

	for _, iface := range ifaces {
		if iface.IsDefault {
			return iface.IP
		}
	}
	if len(ifaces) > 0 {
		return ifaces[0].IP
	}
	return "127.0.0.1"
}

func (s *LanService) resolveBroadcastTargets() []string {
	targets := make([]string, 0)
	cfg := s.gameService.Config.LanConfig
	ifaces := s.GetNetworkInterfaces()

	if cfg.SelectedSubnet != "" {
		for _, iface := range ifaces {
			if (iface.Subnet == cfg.SelectedSubnet || iface.IP == cfg.SelectedSubnet) && iface.BroadcastIP != "" {
				targets = append(targets, fmt.Sprintf("%s:%d", iface.BroadcastIP, cfg.BroadcastPort))
			}
		}
	}

	if len(targets) == 0 {
		for _, iface := range ifaces {
			if iface.BroadcastIP != "" {
				targets = append(targets, fmt.Sprintf("%s:%d", iface.BroadcastIP, cfg.BroadcastPort))
			}
		}
	}

	// 始终附带通用广播与回环测试地址
	targets = append(targets, fmt.Sprintf("255.255.255.255:%d", cfg.BroadcastPort))
	targets = append(targets, fmt.Sprintf("127.0.0.1:%d", cfg.BroadcastPort))
	return targets
}

func (s *LanService) startUdpListener() {
	cfg := s.gameService.Config.LanConfig
	listenPort := cfg.BroadcastPort
	if listenPort <= 0 {
		listenPort = 38567
	}

	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
		},
	}

	packetConn, err := lc.ListenPacket(s.ctx, "udp4", fmt.Sprintf("0.0.0.0:%d", listenPort))
	if err != nil {
		fmt.Printf("[LAN] 监听 UDP 广播端口 %d 失败: %v\n", listenPort, err)
		return
	}
	defer packetConn.Close()

	buf := make([]byte, 4096)
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		n, remoteAddr, err := packetConn.ReadFrom(buf)
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}

		var beacon DiscoveryBeacon
		if err := json.Unmarshal(buf[:n], &beacon); err != nil {
			continue
		}

		remoteIP := ""
		if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok {
			remoteIP = udpAddr.IP.String()
		} else {
			remoteIP = remoteAddr.String()
		}

		s.handleIncomingBeacon(beacon, remoteIP)
	}
}

func (s *LanService) handleIncomingBeacon(b DiscoveryBeacon, remoteIP string) {
	cfg := s.gameService.Config.LanConfig
	if b.InstanceID == cfg.InstanceID {
		return // 忽略本机自身广播
	}

	s.mu.Lock()
	if b.Type == "goodbye" {
		delete(s.peers, b.InstanceID)
		s.mu.Unlock()
		s.checkServerConflict()
		return
	}

	s.peers[b.InstanceID] = &models.LanPeer{
		InstanceID:    b.InstanceID,
		DeviceName:    b.DeviceName,
		IP:            remoteIP,
		HttpPort:      b.HttpPort,
		Role:          b.Role,
		PresetVersion: b.PresetVersion,
		AppVersion:    b.AppVersion,
		LastSeen:      time.Now().Unix(),
	}
	s.mu.Unlock()

	s.checkServerConflict()
}

func (s *LanService) startHeartbeatBroadcaster() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.broadcastBeacon()
		}
	}
}

func (s *LanService) broadcastBeacon() {
	cfg := s.gameService.Config.LanConfig
	presetHash := ""
	s.mu.RLock()
	if s.currentPreset != nil {
		presetHash = s.currentPreset.VersionHash
	}
	s.mu.RUnlock()

	beacon := DiscoveryBeacon{
		Type:          "beacon",
		InstanceID:    cfg.InstanceID,
		DeviceName:    cfg.DeviceName,
		Role:          cfg.Role,
		HttpPort:      s.actualPort,
		Subnet:        cfg.SelectedSubnet,
		PresetVersion: presetHash,
		AppVersion:    models.AppVersion,
		Timestamp:     time.Now().Unix(),
	}

	data, err := json.Marshal(beacon)
	if err != nil {
		return
	}

	targets := s.resolveBroadcastTargets()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return
	}
	defer conn.Close()

	for _, target := range targets {
		dstAddr, err := net.ResolveUDPAddr("udp4", target)
		if err == nil {
			_, _ = conn.WriteTo(data, dstAddr)
		}
	}
}

func (s *LanService) broadcastGoodbye() {
	cfg := s.gameService.Config.LanConfig
	beacon := DiscoveryBeacon{
		Type:       "goodbye",
		InstanceID: cfg.InstanceID,
		DeviceName: cfg.DeviceName,
		Role:       cfg.Role,
		Timestamp:  time.Now().Unix(),
	}
	data, _ := json.Marshal(beacon)

	targets := s.resolveBroadcastTargets()
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4bcast, Port: cfg.BroadcastPort})
	if err == nil {
		defer conn.Close()
		for _, target := range targets {
			if dstAddr, err := net.ResolveUDPAddr("udp4", target); err == nil {
				_, _ = conn.WriteTo(data, dstAddr)
			}
		}
	}
}

func (s *LanService) startPeerHousekeeping() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now().Unix()
			hasPruned := false
			for id, p := range s.peers {
				if now-p.LastSeen > 6 {
					delete(s.peers, id)
					hasPruned = true
				}
			}
			s.mu.Unlock()

			if hasPruned {
				s.checkServerConflict()
			}
		}
	}
}

func (s *LanService) checkServerConflict() {
	s.mu.Lock()
	cfg := s.gameService.Config.LanConfig
	prevConflict := s.hasServerConflict

	conflicting := make([]models.LanPeer, 0)
	serverCount := 0
	if cfg.Role == models.RoleServer {
		serverCount = 1
	}

	for _, p := range s.peers {
		if p.Role == models.RoleServer {
			serverCount++
			if cfg.Role == models.RoleServer && p.InstanceID != cfg.InstanceID {
				conflicting = append(conflicting, *p)
			}
		}
	}

	isConflict := false
	if cfg.Role == models.RoleServer && len(conflicting) > 0 {
		isConflict = true
	} else if cfg.Role == models.RoleClient && serverCount > 1 {
		isConflict = true
	}

	s.hasServerConflict = isConflict
	s.conflictingServers = conflicting
	s.mu.Unlock()

	if isConflict != prevConflict && s.wailsCtx != nil {
		if isConflict {
			wailsRuntime.EventsEmit(s.wailsCtx, "lan:server_conflict", s.GetLanStatus())
		}
		wailsRuntime.EventsEmit(s.wailsCtx, "lan:status_updated", s.GetLanStatus())
	}
}

func (s *LanService) startPresetWatcher() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastPrefModTime int64

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			gameDir := s.gameService.Config.GameDir
			if gameDir == "" {
				continue
			}

			prefPath := filepath.Join(gameDir, "preferences.ini")
			if fi, err := os.Stat(prefPath); err == nil {
				mt := fi.ModTime().UnixNano()
				if mt != lastPrefModTime {
					lastPrefModTime = mt
					_, _ = s.RefreshDynamicPreset()
				}
			}
		}
	}
}

// startHttpServer 提供 RESTful HTTP API 支持预设下发与文件传输
func (s *LanService) startHttpServer() {
	cfg := s.gameService.Config.LanConfig
	basePort := cfg.HttpPort
	if basePort <= 0 {
		basePort = 38568
	}

	mux := http.NewServeMux()

	// 1. 获取预设
	mux.HandleFunc("/api/lan/preset", func(w http.ResponseWriter, r *http.Request) {
		preset, err := s.GetDynamicPreset()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(preset)
	})

	// 2. 文件下载 (支持单个 .rwmod / .tmx 以及解压文件夹模组的动态打包)
	mux.HandleFunc("/api/lan/file", func(w http.ResponseWriter, r *http.Request) {
		itemType := r.URL.Query().Get("type")
		fileName := r.URL.Query().Get("name")
		fileName = filepath.Base(fileName) // 安全清理路径，防止越权

		gameDir := s.gameService.Config.GameDir
		var targetPath string
		if itemType == "mod" {
			targetPath = filepath.Join(gameDir, "mods", "units", fileName)
		} else {
			targetPath = filepath.Join(gameDir, "mods", "maps", fileName)
		}

		fi, err := os.Stat(targetPath)
		if err != nil {
			http.Error(w, "文件未找到", http.StatusNotFound)
			return
		}

		if fi.IsDir() {
			// 若为解压文件夹模组，动态压缩打包为 zip 流发送
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.rwmod\"", fileName))
			zw := zip.NewWriter(w)
			defer zw.Close()

			_ = filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(targetPath, path)
				wFile, err := zw.Create(rel)
				if err != nil {
					return err
				}
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				defer f.Close()
				_, _ = io.Copy(wFile, f)
				return nil
			})
			return
		}

		http.ServeFile(w, r, targetPath)
	})

	// 3. 客户端发起推送请求
	mux.HandleFunc("/api/lan/push-request", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			PeerID   string `json:"peerId"`
			PeerName string `json:"peerName"`
			ItemType string `json:"itemType"`
			FileName string `json:"fileName"`
			FileSize int64  `json:"fileSize"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}

		reqID := uuid.New().String()
		pushReq := &models.PushRequest{
			ID:        reqID,
			PeerID:    payload.PeerID,
			PeerName:  payload.PeerName,
			ItemType:  payload.ItemType,
			FileName:  payload.FileName,
			FileSize:  payload.FileSize,
			Status:    "pending",
			CreatedAt: time.Now().Unix(),
		}

		s.mu.Lock()
		cfg := s.gameService.Config.LanConfig
		if cfg.AutoAcceptClientPushes {
			pushReq.Status = "accepted"
		}
		s.pendingPushRequests[reqID] = pushReq
		s.mu.Unlock()

		if !cfg.AutoAcceptClientPushes && s.wailsCtx != nil {
			wailsRuntime.EventsEmit(s.wailsCtx, "lan:push_request_received", pushReq)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"accepted":  pushReq.Status == "accepted",
			"pending":   pushReq.Status == "pending",
			"requestId": reqID,
		})
	})

	// 4. 查询推送请求审核状态
	mux.HandleFunc("/api/lan/push-status", func(w http.ResponseWriter, r *http.Request) {
		reqID := r.URL.Query().Get("id")
		s.mu.RLock()
		req, exists := s.pendingPushRequests[reqID]
		s.mu.RUnlock()

		if !exists {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(req)
	})

	// 5. 客户端上传文件
	mux.HandleFunc("/api/lan/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		reqID := r.FormValue("requestId")
		s.mu.Lock()
		req, exists := s.pendingPushRequests[reqID]
		if !exists || (req.Status != "accepted" && !s.gameService.Config.LanConfig.AutoAcceptClientPushes) {
			s.mu.Unlock()
			http.Error(w, "Unauthorized or rejected push request", http.StatusForbidden)
			return
		}
		s.mu.Unlock()

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileName := filepath.Base(header.Filename)
		gameDir := s.gameService.Config.GameDir
		var destPath string
		if req.ItemType == "mod" {
			destPath = filepath.Join(gameDir, "mods", "units", fileName)
		} else {
			destPath = filepath.Join(gameDir, "mods", "maps", fileName)
		}

		out, err := os.Create(destPath)
		if err != nil {
			http.Error(w, "Failed to write file", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}

		s.mu.Lock()
		req.Status = "completed"
		s.mu.Unlock()

		_, _ = s.RefreshDynamicPreset()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// 6. 局域网应用更新元数据
	mux.HandleFunc("/api/lan/update/info", func(w http.ResponseWriter, r *http.Request) {
		exePath, err := os.Executable()
		var fileSize int64 = 0
		if err == nil {
			if fi, err := os.Stat(exePath); err == nil {
				fileSize = fi.Size()
			}
		}
		deviceName := s.gameService.Config.LanConfig.DeviceName
		meta := models.LanUpdateMeta{
			Version:      models.AppVersion,
			FileSize:     fileSize,
			ProviderName: deviceName,
			ReleaseNotes: fmt.Sprintf("由局域网节点 %s 实时提供的 v%s 版本二进制可执行程序", deviceName, models.AppVersion),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(meta)
	})

	// 7. 局域网应用自更新下载流 (直接流式传输正在运行的自身可执行程序)
	mux.HandleFunc("/api/lan/update/download", func(w http.ResponseWriter, r *http.Request) {
		exePath, err := os.Executable()
		if err != nil {
			http.Error(w, "无法定位可执行文件: "+err.Error(), http.StatusInternalServerError)
			return
		}

		file, err := os.Open(exePath)
		if err != nil {
			http.Error(w, "读取可执行文件失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		fi, _ := file.Stat()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=\"RW-Mod-Manager.exe\"")
		if fi != nil {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))
		}
		_, _ = io.Copy(w, file)
	})

	// 端口监听自动回退
	for p := basePort; p < basePort+20; p++ {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			s.actualPort = p
			s.httpSrv = &http.Server{Handler: mux}
			_ = s.httpSrv.Serve(listener)
			return
		}
	}
}

func (s *LanService) downloadFile(fileURL, targetPath string) error {
	tmpPath := targetPath + ".tmp"
	resp, err := http.Get(fileURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	_ = os.Remove(targetPath)
	return os.Rename(tmpPath, targetPath)
}

func (s *LanService) uploadFileToServer(serverIP string, serverPort int, reqID, itemType, fileName, localFilePath string) error {
	file, err := os.Open(localFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}

	_ = writer.WriteField("requestId", reqID)
	_ = writer.WriteField("itemType", itemType)
	_ = writer.Close()

	url := fmt.Sprintf("http://%s:%d/api/lan/upload", serverIP, serverPort)
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传失败，服务端状态码: %d", resp.StatusCode)
	}

	return nil
}
