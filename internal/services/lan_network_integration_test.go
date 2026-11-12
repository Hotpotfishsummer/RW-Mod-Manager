package services

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rw_mod_manager/internal/models"
)

// setupMockGameDir 构建一个符合铁锈战争规范的临时沙箱游戏目录
func setupMockGameDir(t *testing.T, label string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), label)
	if err := os.MkdirAll(filepath.Join(dir, "mods", "units"), 0755); err != nil {
		t.Fatalf("创建模组目录失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "mods", "maps"), 0755); err != nil {
		t.Fatalf("创建地图目录失败: %v", err)
	}
	// 标记文件，使 ValidateGameDir 通过
	if err := os.WriteFile(filepath.Join(dir, "game-lib.jar"), []byte("mock-jar-content"), 0644); err != nil {
		t.Fatalf("创建 game-lib.jar 失败: %v", err)
	}
	// 空的 preferences.ini
	if err := os.WriteFile(filepath.Join(dir, "preferences.ini"), []byte("modSettings:\n"), 0644); err != nil {
		t.Fatalf("创建 preferences.ini 失败: %v", err)
	}
	return dir
}

// createZipMod 创建一个带有 mod-info.txt 的 .rwmod 文件
func createZipMod(t *testing.T, filePath, title, desc string) {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	content := fmt.Sprintf("[mod]\ntitle: %s\ndescription: %s\n", title, desc)
	f, err := zw.Create("mod-info.txt")
	if err != nil {
		t.Fatalf("创建 mod-info.txt 失败: %v", err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatalf("写入 mod-info.txt 失败: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭 zip 失败: %v", err)
	}
	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("写出 rwmod 文件失败: %v", err)
	}
}

// createDirMod 创建一个文件夹模组 (内含 mod-info.txt 及子文件)
func createDirMod(t *testing.T, dirPath, title, desc string) {
	t.Helper()
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("创建文件夹模组失败: %v", err)
	}
	content := fmt.Sprintf("[mod]\ntitle: %s\ndescription: %s\n", title, desc)
	if err := os.WriteFile(filepath.Join(dirPath, "mod-info.txt"), []byte(content), 0644); err != nil {
		t.Fatalf("写入文件夹 mod-info.txt 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirPath, "unit_data.ini"), []byte("[core]\nname: test_unit\n"), 0644); err != nil {
		t.Fatalf("写入子文件失败: %v", err)
	}
}

// =========================================================================
// 测试 1: 局域网预设比对与双向全量同步 (ComparePresetWithServer & SyncWithServer)
// 包含单文件 .rwmod、解压文件夹模组、独立地图、preferences.ini 状态自动对齐与多余模组禁用
// =========================================================================
func TestPresetComparisonAndSyncFlow(t *testing.T) {
	serverGameDir := setupMockGameDir(t, "server_game")
	clientGameDir := setupMockGameDir(t, "client_game")

	// 1. 服务端准备数据
	serverModFile := filepath.Join(serverGameDir, "mods", "units", "alpha_tank.rwmod")
	createZipMod(t, serverModFile, "Alpha Tank", "Strong tank mod")

	serverDirMod := filepath.Join(serverGameDir, "mods", "units", "beta_infantry")
	createDirMod(t, serverDirMod, "Beta Infantry", "Dir style mod")

	serverMap1 := filepath.Join(serverGameDir, "mods", "maps", "great_river.tmx")
	_ = os.WriteFile(serverMap1, []byte("<map>great river data</map>"), 0644)

	serverMap2 := filepath.Join(serverGameDir, "mods", "maps", "desert_valley.tmx")
	_ = os.WriteFile(serverMap2, []byte("<map>desert valley data</map>"), 0644)

	prefSvc := NewPrefService()
	// 服务端将两个模组设置为 enabled
	_ = prefSvc.SaveModSettings(serverGameDir, map[string]bool{
		"alpha_tank.rwmod": true,
		"beta_infantry":    true,
	})

	serverGameSvc := NewGameService()
	serverGameSvc.Config.GameDir = serverGameDir
	serverGameSvc.Config.LanConfig.Role = models.RoleServer
	serverGameSvc.Config.LanConfig.InstanceID = "mock-server-inst-1"
	serverGameSvc.Config.LanConfig.DeviceName = "Host-PC"
	serverModSvc := NewModService(prefSvc)
	serverMapSvc := NewMapService(prefSvc)
	serverLanSvc := NewLanService(serverGameSvc, prefSvc, serverModSvc, serverMapSvc)
	serverLanSvc.Start()
	defer serverLanSvc.Stop()

	serverPort := serverLanSvc.GetLanStatus().HttpPort
	if serverPort <= 0 {
		t.Fatalf("服务端 HTTP 端口分配失败: %d", serverPort)
	}

	// 2. 客户端初始数据：缺少两个模组与两张地图，但有一个多余的本地模组 extra.rwmod
	clientExtraMod := filepath.Join(clientGameDir, "mods", "units", "extra_local.rwmod")
	createZipMod(t, clientExtraMod, "Extra Local", "Client only mod")
	_ = prefSvc.SaveModSettings(clientGameDir, map[string]bool{
		"extra_local.rwmod": true,
	})

	clientGameSvc := NewGameService()
	clientGameSvc.Config.GameDir = clientGameDir
	clientGameSvc.Config.LanConfig.Role = models.RoleClient
	clientGameSvc.Config.LanConfig.InstanceID = "mock-client-inst-1"
	clientGameSvc.Config.LanConfig.DeviceName = "Client-PC"
	clientModSvc := NewModService(prefSvc)
	clientMapSvc := NewMapService(prefSvc)
	clientLanSvc := NewLanService(clientGameSvc, prefSvc, clientModSvc, clientMapSvc)
	clientLanSvc.Start()
	defer clientLanSvc.Stop()

	// 3. 执行 ComparePresetWithServer
	diff, err := clientLanSvc.ComparePresetWithServer("127.0.0.1", serverPort)
	if err != nil {
		t.Fatalf("ComparePresetWithServer 失败: %v", err)
	}

	if diff.IsInSync {
		t.Fatalf("预期不同步，但 IsInSync 为 true")
	}
	if len(diff.MissingMods) != 2 {
		t.Fatalf("预期缺失 2 个模组，实际: %d", len(diff.MissingMods))
	}
	if len(diff.MissingMaps) != 2 {
		t.Fatalf("预期缺失 2 张地图，实际: %d", len(diff.MissingMaps))
	}
	t.Logf("预设比对成功: %s (缺失模组: %d, 缺失地图: %d)", diff.Summary, len(diff.MissingMods), len(diff.MissingMaps))

	// 4. 执行 SyncWithServer 进行全量同步
	if err := clientLanSvc.SyncWithServer("127.0.0.1", serverPort); err != nil {
		t.Fatalf("SyncWithServer 失败: %v", err)
	}

	// 5. 校验客户端本地文件落地与解压情况
	downloadedModPath := filepath.Join(clientGameDir, "mods", "units", "alpha_tank.rwmod")
	if _, err := os.Stat(downloadedModPath); os.IsNotExist(err) {
		t.Fatalf("同步失败：缺失 alpha_tank.rwmod 文件")
	}

	downloadedDirModPath := filepath.Join(clientGameDir, "mods", "units", "beta_infantry", "mod-info.txt")
	if _, err := os.Stat(downloadedDirModPath); os.IsNotExist(err) {
		t.Fatalf("同步失败：文件夹模组 beta_infantry 未能成功解压落地")
	}

	downloadedMap1 := filepath.Join(clientGameDir, "mods", "maps", "great_river.tmx")
	downloadedMap2 := filepath.Join(clientGameDir, "mods", "maps", "desert_valley.tmx")
	if _, err := os.Stat(downloadedMap1); os.IsNotExist(err) {
		t.Fatalf("同步失败：缺失地图 great_river.tmx")
	}
	if _, err := os.Stat(downloadedMap2); os.IsNotExist(err) {
		t.Fatalf("同步失败：缺失地图 desert_valley.tmx")
	}

	// 6. 校验客户端 preferences.ini 模组状态是否对齐
	clientStatuses, err := prefSvc.ReadModSettings(clientGameDir)
	if err != nil {
		t.Fatalf("读取客户端 preferences.ini 失败: %v", err)
	}

	alphaHash := prefSvc.CalculateModHash("alpha_tank.rwmod")
	betaHash := prefSvc.CalculateModHash("beta_infantry")
	extraHash := prefSvc.CalculateModHash("extra_local.rwmod")

	if !clientStatuses[alphaHash] {
		t.Fatalf("预期 alpha_tank.rwmod 为 enabled，实际未启用")
	}
	if !clientStatuses[betaHash] {
		t.Fatalf("预期 beta_infantry 为 enabled，实际未启用")
	}
	if clientStatuses[extraHash] {
		t.Fatalf("预期本地多余模组 extra_local.rwmod 被联机同步禁用，实际仍处于启用")
	}

	// 7. 再次比对，验证预设哈希与同步状态已达到 100% 一致
	diffAfter, err := clientLanSvc.ComparePresetWithServer("127.0.0.1", serverPort)
	if err != nil {
		t.Fatalf("二次 ComparePresetWithServer 失败: %v", err)
	}
	if !diffAfter.IsInSync {
		t.Fatalf("同步完成后预期 IsInSync 为 true，实际为 false: %+v", diffAfter)
	}
	t.Logf("同步链路完全验证通过！预设特征哈希: %s, 摘要: %s", diffAfter.ServerPresetHash, diffAfter.Summary)
}

// =========================================================================
// 测试 2: 客户端向房主推送文件与审批全流程 (RequestPushToHost & HandlePushDecision)
// 包括自动接受模式、手动审批通过、手动审批拒绝、以及文件夹模组推送打包
// =========================================================================
func TestPushToHostWorkflow(t *testing.T) {
	serverGameDir := setupMockGameDir(t, "server_push")
	clientGameDir := setupMockGameDir(t, "client_push")

	prefSvc := NewPrefService()
	serverGameSvc := NewGameService()
	serverGameSvc.Config.GameDir = serverGameDir
	serverGameSvc.Config.LanConfig.Role = models.RoleServer
	serverGameSvc.Config.LanConfig.InstanceID = "push-server"
	serverGameSvc.Config.LanConfig.DeviceName = "Host-Server"
	serverModSvc := NewModService(prefSvc)
	serverMapSvc := NewMapService(prefSvc)
	serverLanSvc := NewLanService(serverGameSvc, prefSvc, serverModSvc, serverMapSvc)
	serverLanSvc.Start()
	defer serverLanSvc.Stop()

	serverPort := serverLanSvc.GetLanStatus().HttpPort

	clientGameSvc := NewGameService()
	clientGameSvc.Config.GameDir = clientGameDir
	clientGameSvc.Config.LanConfig.Role = models.RoleClient
	clientGameSvc.Config.LanConfig.InstanceID = "push-client"
	clientGameSvc.Config.LanConfig.DeviceName = "Teammate-Client"
	clientModSvc := NewModService(prefSvc)
	clientMapSvc := NewMapService(prefSvc)
	clientLanSvc := NewLanService(clientGameSvc, prefSvc, clientModSvc, clientMapSvc)
	clientLanSvc.Start()
	defer clientLanSvc.Stop()

	// 场景 A: 房主开启自动接收 (AutoAcceptClientPushes = true)
	serverLanSvc.gameService.Config.LanConfig.AutoAcceptClientPushes = true

	pushModFile := filepath.Join(clientGameDir, "mods", "units", "pushed_laser_bot.rwmod")
	createZipMod(t, pushModFile, "Laser Bot", "Client mod")

	resA, err := clientLanSvc.RequestPushToHost("127.0.0.1", serverPort, "mod", "pushed_laser_bot.rwmod")
	if err != nil {
		t.Fatalf("自动接收推送失败: %v", err)
	}
	if resA == nil || !resA.Success || resA.SkippedUpload || resA.Overwritten {
		t.Fatalf("场景 A PushResult 异常: %+v", resA)
	}

	targetServerMod := filepath.Join(serverGameDir, "mods", "units", "pushed_laser_bot.rwmod")
	if _, err := os.Stat(targetServerMod); os.IsNotExist(err) {
		t.Fatalf("服务端未收到推送并保存模组文件")
	}
	t.Logf("场景 A: 自动接受推送测试成功")

	// 场景 B: 房主关闭自动接收 (需要手动审批)，客户端推送地图
	serverLanSvc.gameService.Config.LanConfig.AutoAcceptClientPushes = false

	pushMapFile := filepath.Join(clientGameDir, "mods", "maps", "secret_pass.tmx")
	_ = os.WriteFile(pushMapFile, []byte("<map>secret pass</map>"), 0644)

	// 启动 goroutine 异步发起推送请求 (客户端会轮询等待)
	pushDone := make(chan error, 1)
	go func() {
		_, pushErr := clientLanSvc.RequestPushToHost("127.0.0.1", serverPort, "map", "secret_pass.tmx")
		pushDone <- pushErr
	}()

	// 服务端等待并检测 Pending 列表
	var pendingID string
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		pendingList := serverLanSvc.GetPendingPushRequests()
		if len(pendingList) > 0 {
			pendingID = pendingList[0].ID
			break
		}
	}
	if pendingID == "" {
		t.Fatalf("服务端未能捕获到待审批的推送请求")
	}

	// 房主批准该推送请求
	if err := serverLanSvc.HandlePushDecision(pendingID, true); err != nil {
		t.Fatalf("房主审批出错: %v", err)
	}

	select {
	case err := <-pushDone:
		if err != nil {
			t.Fatalf("审批通过后客户端上传失败: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("等待推送完成超时")
	}

	targetServerMap := filepath.Join(serverGameDir, "mods", "maps", "secret_pass.tmx")
	if _, err := os.Stat(targetServerMap); os.IsNotExist(err) {
		t.Fatalf("审批通过后服务端未落地地图文件")
	}
	t.Logf("场景 B: 手动审批通过推送测试成功")

	// 场景 C: 房主拒绝推送请求
	rejectMapFile := filepath.Join(clientGameDir, "mods", "maps", "bad_map.tmx")
	_ = os.WriteFile(rejectMapFile, []byte("<map>bad</map>"), 0644)

	rejectDone := make(chan error, 1)
	go func() {
		_, rejErr := clientLanSvc.RequestPushToHost("127.0.0.1", serverPort, "map", "bad_map.tmx")
		rejectDone <- rejErr
	}()

	var rejectPendingID string
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		pendingList := serverLanSvc.GetPendingPushRequests()
		for _, r := range pendingList {
			if r.FileName == "bad_map.tmx" {
				rejectPendingID = r.ID
				break
			}
		}
		if rejectPendingID != "" {
			break
		}
	}
	if rejectPendingID == "" {
		t.Fatalf("服务端未能捕获到待拒绝的推送请求")
	}

	// 房主拒绝该推送
	_ = serverLanSvc.HandlePushDecision(rejectPendingID, false)

	select {
	case err := <-rejectDone:
		if err == nil || !strings.Contains(err.Error(), "拒绝") {
			t.Fatalf("预期返回被拒绝错误，实际结果: %v", err)
		}
		t.Logf("场景 C: 房主拒绝推送按预期报错: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatalf("等待拒绝反馈超时")
	}
}

// =========================================================================
// 测试 3: 局域网应用更新包分发、检测与下载流 (UpdateService & Lan Updates)
// 测试同版本不误报、高版本精准探测、流式下载进度汇报与文件落地
// =========================================================================
func TestLanUpdateFlow(t *testing.T) {
	mockGameDir := setupMockGameDir(t, "update_game")
	prefSvc := NewPrefService()
	gameSvc := NewGameService()
	gameSvc.Config.GameDir = mockGameDir
	gameSvc.Config.LanConfig.Role = models.RoleClient
	gameSvc.Config.LanConfig.InstanceID = "client-updater-node"
	modSvc := NewModService(prefSvc)
	mapSvc := NewMapService(prefSvc)
	lanSvc := NewLanService(gameSvc, prefSvc, modSvc, mapSvc)
	updateSvc := NewUpdateService(gameSvc, lanSvc)

	// 1. 测试同版本：局域网内存在一个同版本 (1.1.0) 的房主
	sameVerServer := DiscoveryBeacon{
		Type:          "beacon",
		InstanceID:    "same-ver-server",
		DeviceName:    "Host-SameVer-PC",
		Role:          models.RoleServer,
		HttpPort:      39999,
		AppVersion:    models.AppVersion, // 1.1.0
		PresetVersion: "HASH110",
	}
	lanSvc.handleIncomingBeacon(sameVerServer, "127.0.0.1")

	info, err := updateSvc.CheckLanUpdate()
	if err != nil {
		t.Fatalf("CheckLanUpdate error: %v", err)
	}
	if info != nil {
		t.Fatalf("同版本房主不应触发更新提示，但收到了: %+v", info)
	}
	t.Logf("同版本检测验证通过：未误报更新")

	// 2. 测试更高版本：启动一个模拟高版本 (v2.0.0) 的局域网节点 HTTP 服务
	mockUpdatePayload := []byte("MOCK_EXE_BINARY_V2.0.0_PAYLOAD_TEST_DATA_PADDING_CONTENT")
	mockMux := http.NewServeMux()
	mockMux.HandleFunc("/api/lan/update/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.LanUpdateMeta{
			Version:      "2.0.0",
			FileSize:     int64(len(mockUpdatePayload)),
			ProviderName: "Leader-PC",
			ReleaseNotes: "重大性能优化与新联机模组支持",
		})
	})
	mockMux.HandleFunc("/api/lan/update/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(mockUpdatePayload)))
		_, _ = w.Write(mockUpdatePayload)
	})

	mockListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建 Mock 更新监听器失败: %v", err)
	}
	mockPort := mockListener.Addr().(*net.TCPAddr).Port
	mockSrv := &http.Server{Handler: mockMux}
	go func() {
		_ = mockSrv.Serve(mockListener)
	}()
	defer mockSrv.Close()

	// 注册高版本房主信标
	highVerServer := DiscoveryBeacon{
		Type:          "beacon",
		InstanceID:    "high-ver-server",
		DeviceName:    "Leader-PC",
		Role:          models.RoleServer,
		HttpPort:      mockPort,
		AppVersion:    "2.0.0",
		PresetVersion: "HASH200",
	}
	lanSvc.handleIncomingBeacon(highVerServer, "127.0.0.1")

	// 3. 检查局域网更新检测
	highInfo, err := updateSvc.CheckLanUpdate()
	if err != nil {
		t.Fatalf("高版本 CheckLanUpdate 出错: %v", err)
	}
	if highInfo == nil || !highInfo.HasUpdate {
		t.Fatalf("未能成功检测到高版本 2.0.0 更新")
	}
	if highInfo.LatestVersion != "2.0.0" {
		t.Fatalf("预期最新版本为 2.0.0，实际为: %s", highInfo.LatestVersion)
	}
	if highInfo.Source != models.UpdateSourceLan {
		t.Fatalf("预期更新来源为 lan，实际为: %s", highInfo.Source)
	}
	t.Logf("高版本检测通过: %+v", highInfo)

	// 4. 测试流式下载更新包
	destDownloadPath := filepath.Join(t.TempDir(), "test_update.exe.downloading")
	err = updateSvc.DownloadUpdateFile(highInfo.DownloadURL, highInfo.FileSize, destDownloadPath)
	if err != nil {
		t.Fatalf("DownloadUpdateFile 失败: %v", err)
	}

	downloadedBytes, err := os.ReadFile(destDownloadPath)
	if err != nil {
		t.Fatalf("读取下载更新包失败: %v", err)
	}
	if !bytes.Equal(downloadedBytes, mockUpdatePayload) {
		t.Fatalf("下载更新包内容与源端不一致")
	}
	t.Logf("更新包流式传输与完整性校验通过！大小: %d 字节", len(downloadedBytes))
}

// =========================================================================
// 测试 4: 多服务端冲突与下线处理 (Conflict & Offline / Goodbye)
// =========================================================================
func TestServerConflictAndGoodbye(t *testing.T) {
	mockGameDir := setupMockGameDir(t, "conflict_game")
	prefSvc := NewPrefService()
	gameSvc := NewGameService()
	gameSvc.Config.GameDir = mockGameDir
	gameSvc.Config.LanConfig.Role = models.RoleServer
	gameSvc.Config.LanConfig.InstanceID = "my-server-id"
	gameSvc.Config.LanConfig.DeviceName = "My-Server-PC"
	modSvc := NewModService(prefSvc)
	mapSvc := NewMapService(prefSvc)
	lanSvc := NewLanService(gameSvc, prefSvc, modSvc, mapSvc)

	// 1. 本机是 Server，收到另一个 Server 的 Beacon，应触发冲突
	otherServer := DiscoveryBeacon{
		Type:       "beacon",
		InstanceID: "other-server-id",
		DeviceName: "Other-Server-PC",
		Role:       models.RoleServer,
		HttpPort:   38568,
		AppVersion: models.AppVersion,
	}
	lanSvc.handleIncomingBeacon(otherServer, "192.168.1.150")

	st := lanSvc.GetLanStatus()
	if !st.HasServerConflict {
		t.Fatalf("预期服务端冲突，实际未检测到冲突")
	}
	if len(st.ConflictingServers) != 1 || st.ConflictingServers[0].InstanceID != "other-server-id" {
		t.Fatalf("冲突服务端列表不符合预期: %+v", st.ConflictingServers)
	}

	// 2. 收到 otherServer 的 goodbye 信标，冲突应自动解除
	goodbyeBeacon := DiscoveryBeacon{
		Type:       "goodbye",
		InstanceID: "other-server-id",
		Role:       models.RoleServer,
	}
	lanSvc.handleIncomingBeacon(goodbyeBeacon, "192.168.1.150")

	stAfter := lanSvc.GetLanStatus()
	if stAfter.HasServerConflict {
		t.Fatalf("收到 goodbye 后冲突未能自动解除")
	}
	if len(stAfter.ConflictingServers) != 0 {
		t.Fatalf("预期冲突列表已清空，实际剩余: %d", len(stAfter.ConflictingServers))
	}
	t.Logf("服务端冲突触发与下线解除测试通过")
}

// =========================================================================
// 测试 5: 真实 UDP 广播信标与告别信标网络收发测试 (UDP Socket I/O)
// =========================================================================
func TestLanUDPBroadcastAndReceive(t *testing.T) {
	testPort := 39567

	// 在 127.0.0.1:testPort 建立 UDP 监听，模拟局域网同伴监听端口
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("127.0.0.1:%d", testPort))
	if err != nil {
		t.Fatalf("ResolveUDPAddr 失败: %v", err)
	}
	listener, err := net.ListenUDP("udp4", addr)
	if err != nil {
		t.Fatalf("ListenUDP 失败: %v", err)
	}
	defer listener.Close()

	// 初始化一个 LanService，其广播端口配置为 testPort
	mockGameDir := setupMockGameDir(t, "udp_broadcast_test")
	prefSvc := NewPrefService()
	gameSvc := NewGameService()
	gameSvc.Config.GameDir = mockGameDir
	gameSvc.Config.LanConfig.Role = models.RoleServer
	gameSvc.Config.LanConfig.InstanceID = "udp-test-inst"
	gameSvc.Config.LanConfig.DeviceName = "UDP-Tester-PC"
	gameSvc.Config.LanConfig.BroadcastPort = testPort
	modSvc := NewModService(prefSvc)
	mapSvc := NewMapService(prefSvc)
	lanSvc := NewLanService(gameSvc, prefSvc, modSvc, mapSvc)

	// 1. 发送在线信标
	lanSvc.broadcastBeacon()

	_ = listener.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	n, _, err := listener.ReadFrom(buf)
	if err != nil {
		t.Fatalf("读取 UDP 心跳信标失败: %v", err)
	}

	var beacon DiscoveryBeacon
	if err := json.Unmarshal(buf[:n], &beacon); err != nil {
		t.Fatalf("心跳包 JSON 反序列化失败: %v", err)
	}
	if beacon.Type != "beacon" || beacon.InstanceID != "udp-test-inst" || beacon.DeviceName != "UDP-Tester-PC" {
		t.Fatalf("心跳包内容不符合预期: %+v", beacon)
	}
	t.Logf("UDP 心跳广播接收成功: %+v", beacon)

	// 2. 发送告别信标
	lanSvc.broadcastGoodbye()

	_ = listener.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err = listener.ReadFrom(buf)
	if err != nil {
		t.Fatalf("读取 UDP 告别信标失败: %v", err)
	}

	var goodbye DiscoveryBeacon
	if err := json.Unmarshal(buf[:n], &goodbye); err != nil {
		t.Fatalf("告别包 JSON 反序列化失败: %v", err)
	}
	if goodbye.Type != "goodbye" || goodbye.InstanceID != "udp-test-inst" {
		t.Fatalf("告别包内容不符合预期: %+v", goodbye)
	}
	t.Logf("UDP 告别信标接收成功: %+v", goodbye)
}

// =========================================================================
// 测试 5: 多次重复推送点击 & 同名不同 MD5 冲突推送处理
// =========================================================================
func TestPushDuplicateAndConflictHandling(t *testing.T) {
	serverGameDir := setupMockGameDir(t, "server_dup")
	clientGameDir := setupMockGameDir(t, "client_dup")

	prefSvc := NewPrefService()
	serverGameSvc := NewGameService()
	serverGameSvc.Config.GameDir = serverGameDir
	serverGameSvc.Config.LanConfig.Role = models.RoleServer
	serverGameSvc.Config.LanConfig.InstanceID = "dup-server"
	serverGameSvc.Config.LanConfig.DeviceName = "Dup-Server"
	serverGameSvc.Config.LanConfig.AutoAcceptClientPushes = false
	serverLanSvc := NewLanService(serverGameSvc, prefSvc, NewModService(prefSvc), NewMapService(prefSvc))
	serverLanSvc.Start()
	defer serverLanSvc.Stop()

	serverPort := serverLanSvc.GetLanStatus().HttpPort

	clientGameSvc := NewGameService()
	clientGameSvc.Config.GameDir = clientGameDir
	clientGameSvc.Config.LanConfig.Role = models.RoleClient
	clientGameSvc.Config.LanConfig.InstanceID = "dup-client"
	clientGameSvc.Config.LanConfig.DeviceName = "Dup-Client"
	clientLanSvc := NewLanService(clientGameSvc, prefSvc, NewModService(prefSvc), NewMapService(prefSvc))
	clientLanSvc.Start()
	defer clientLanSvc.Stop()

	// 1. 测试：等待审批期间多次发送相同 push-request，服务端只保留 1 条待审批请求 (去重)
	modFile := filepath.Join(clientGameDir, "mods", "units", "dup_mod.rwmod")
	createZipMod(t, modFile, "Dup Mod v1", "Initial version")
	hashV1 := CalculateContentHash(modFile, false)

	postPushReq := func(hash string) map[string]interface{} {
		bodyBytes, _ := json.Marshal(map[string]interface{}{
			"peerId":      "dup-client",
			"peerName":    "Dup-Client",
			"itemType":    "mod",
			"fileName":    "dup_mod.rwmod",
			"fileSize":    1234,
			"contentHash": hash,
		})
		resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/api/lan/push-request", serverPort), "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Fatalf("push-request 调用失败: %v", err)
		}
		defer resp.Body.Close()
		var out map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return out
	}

	r1 := postPushReq(hashV1)
	r2 := postPushReq(hashV1)
	if r1["requestId"] != r2["requestId"] {
		t.Fatalf("同一文件重复请求应复用同一 requestId，实际 r1=%v r2=%v", r1["requestId"], r2["requestId"])
	}
	if pending := serverLanSvc.GetPendingPushRequests(); len(pending) != 1 {
		t.Fatalf("重复推送应去重只保留 1 条待审批，实际: %d", len(pending))
	}

	// 2. 测试：客户端并发防重入 (同一文件正在推送时再次调用直接返回提示)
	// 先用另一个文件启动一个会阻塞等待审批的推送
	inFlightMod := filepath.Join(clientGameDir, "mods", "units", "inflight.rwmod")
	createZipMod(t, inFlightMod, "InFlight", "v1")
	firstDone := make(chan error, 1)
	go func() {
		_, err := clientLanSvc.RequestPushToHost("127.0.0.1", serverPort, "mod", "inflight.rwmod")
		firstDone <- err
	}()
	time.Sleep(150 * time.Millisecond)
	_, secondErr := clientLanSvc.RequestPushToHost("127.0.0.1", serverPort, "mod", "inflight.rwmod")
	if secondErr == nil || !strings.Contains(secondErr.Error(), "正在推送") {
		t.Fatalf("并发重复点击同一文件应被客户端防重入拦截，实际: %v", secondErr)
	}
	// 拒绝 inflight 结束第一个 goroutine
	for _, p := range serverLanSvc.GetPendingPushRequests() {
		if p.FileName == "inflight.rwmod" {
			_ = serverLanSvc.HandlePushDecision(p.ID, false)
		}
	}
	<-firstDone

	// 3. 测试：批准 dup_mod.rwmod 完成首次推送，随后再推送完全相同 MD5 -> 应直接跳过上传 (SkippedUpload = true)
	for _, p := range serverLanSvc.GetPendingPushRequests() {
		if p.FileName == "dup_mod.rwmod" {
			_ = serverLanSvc.HandlePushDecision(p.ID, false) // 清理前面直调 API 留下的 pending
		}
	}
	serverLanSvc.gameService.Config.LanConfig.AutoAcceptClientPushes = true
	res1, err := clientLanSvc.RequestPushToHost("127.0.0.1", serverPort, "mod", "dup_mod.rwmod")
	if err != nil || !res1.Success || res1.SkippedUpload {
		t.Fatalf("首次推送 dup_mod 预期成功且非跳过，实际 err=%v res=%+v", err, res1)
	}

	res2, err := clientLanSvc.RequestPushToHost("127.0.0.1", serverPort, "mod", "dup_mod.rwmod")
	if err != nil || !res2.Success || !res2.SkippedUpload {
		t.Fatalf("相同文件重复推送预期 SkippedUpload=true，实际 err=%v res=%+v", err, res2)
	}

	// 4. 测试：同名但 MD5 不同的文件推送 (手动审批模式 -> 标记 HasConflict；审批通过后自动备份并覆盖更新)
	serverLanSvc.gameService.Config.LanConfig.AutoAcceptClientPushes = false
	createZipMod(t, modFile, "Dup Mod v2", "Modified content with different MD5")
	hashV2 := CalculateContentHash(modFile, false)
	if hashV2 == hashV1 {
		t.Fatalf("修改内容后 MD5 应发生变化")
	}

	conflictDone := make(chan *models.PushResult, 1)
	conflictErrCh := make(chan error, 1)
	go func() {
		res, pErr := clientLanSvc.RequestPushToHost("127.0.0.1", serverPort, "mod", "dup_mod.rwmod")
		conflictDone <- res
		conflictErrCh <- pErr
	}()

	var conflictReq *models.PushRequest
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		for _, p := range serverLanSvc.GetPendingPushRequests() {
			if p.FileName == "dup_mod.rwmod" {
				pCopy := p
				conflictReq = &pCopy
				break
			}
		}
		if conflictReq != nil {
			break
		}
	}
	if conflictReq == nil {
		t.Fatalf("未能捕获同名冲突推送请求")
	}
	if !conflictReq.HasConflict || conflictReq.ExistingHash != hashV1 || conflictReq.ContentHash != hashV2 {
		t.Fatalf("服务端未能准确识别同名不同 MD5 冲突: %+v (expected existing=%s incoming=%s)", conflictReq, hashV1, hashV2)
	}

	if err := serverLanSvc.HandlePushDecision(conflictReq.ID, true); err != nil {
		t.Fatalf("审批覆盖推送失败: %v", err)
	}

	if pErr := <-conflictErrCh; pErr != nil {
		t.Fatalf("冲突文件上传失败: %v", pErr)
	}
	conflictRes := <-conflictDone
	if conflictRes == nil || !conflictRes.Success || !conflictRes.Overwritten || !conflictRes.BackupCreated {
		t.Fatalf("同名覆盖推送结果应包含 Overwritten=true 与 BackupCreated=true，实际: %+v", conflictRes)
	}

	// 验证服务端文件确已更新为 v2 MD5，且原文件已备份
	serverModPath := filepath.Join(serverGameDir, "mods", "units", "dup_mod.rwmod")
	if gotHash := CalculateContentHash(serverModPath, false); gotHash != hashV2 {
		t.Fatalf("覆盖后服务端文件 MD5 不一致: got %s, want %s", gotHash, hashV2)
	}
	backups, _ := filepath.Glob(filepath.Join(serverGameDir, "mods", "_archived", "push_backups", "dup_mod.rwmod.*.bak"))
	if len(backups) == 0 {
		t.Fatalf("覆盖旧版本时未生成备份文件")
	}
	t.Logf("多次重复推送去重 + 相同 MD5 秒跳过 + 同名不同 MD5 冲突检测与备份覆盖全链路测试通过！")
}
