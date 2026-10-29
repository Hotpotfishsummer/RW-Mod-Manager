package services

import (
	"path/filepath"
	"testing"

	"rw_mod_manager/internal/models"
)

func TestGameServices(t *testing.T) {
	gameDir := `E:\Rusted Warfare\Rusted Warfare`

	gameSvc := NewGameService()
	if !gameSvc.ValidateGameDir(gameDir) {
		t.Fatalf("ValidateGameDir failed for %s", gameDir)
	}

	prefSvc := NewPrefService()
	statusMap, err := prefSvc.ReadModSettings(gameDir)
	if err != nil {
		t.Fatalf("ReadModSettings error: %v", err)
	}
	t.Logf("Read %d mod settings from preferences.ini", len(statusMap))

	modSvc := NewModService(prefSvc)
	mods, err := modSvc.ScanMods(gameDir)
	if err != nil {
		t.Fatalf("ScanMods error: %v", err)
	}
	t.Logf("Successfully scanned %d mods in %s", len(mods), filepath.Join(gameDir, "mods", "units"))

	enabledCount := 0
	hasThumbCount := 0
	for _, m := range mods {
		if m.Enabled {
			enabledCount++
		}
		if m.Thumbnail != "" {
			hasThumbCount++
		}
	}
	t.Logf("Mods stats: %d enabled, %d with thumbnails extracted", enabledCount, hasThumbCount)

	mapSvc := NewMapService(prefSvc)
	maps, err := mapSvc.ScanMaps(gameDir)
	if err != nil {
		t.Fatalf("ScanMaps error: %v", err)
	}
	t.Logf("Successfully scanned %d maps in %s", len(maps), filepath.Join(gameDir, "mods", "maps"))
}

func TestLanInterfacesAndPreset(t *testing.T) {
	gameDir := `E:\Rusted Warfare\Rusted Warfare`
	gameSvc := NewGameService()
	gameSvc.Config.GameDir = gameDir
	gameSvc.Config.LanConfig.Role = models.RoleClient
	prefSvc := NewPrefService()
	modSvc := NewModService(prefSvc)
	mapSvc := NewMapService(prefSvc)

	lanSvc := NewLanService(gameSvc, prefSvc, modSvc, mapSvc)

	// 1. 测试网络适配器枚举
	ifaces := lanSvc.GetNetworkInterfaces()
	t.Logf("Discovered %d network interfaces", len(ifaces))
	for _, iface := range ifaces {
		t.Logf("Interface: %s, IP: %s, Subnet: %s, Broadcast: %s, Default: %t",
			iface.Name, iface.IP, iface.Subnet, iface.BroadcastIP, iface.IsDefault)
	}

	// 2. 测试动态预设生成与哈希计算
	preset, err := lanSvc.RefreshDynamicPreset()
	if err != nil {
		t.Fatalf("RefreshDynamicPreset failed: %v", err)
	}
	if preset.VersionHash == "" {
		t.Fatalf("Preset VersionHash should not be empty")
	}
	t.Logf("Generated Preset: %s, VersionHash: %s, Mods: %d, Maps: %d",
		preset.Name, preset.VersionHash, len(preset.Mods), len(preset.Maps))

	// 3. 测试默认角色与切换角色
	status := lanSvc.GetLanStatus()
	if status.Role != "client" {
		t.Fatalf("Default LAN role should be 'client', got: %s", status.Role)
	}
	if err := lanSvc.SwitchRole("server"); err != nil {
		t.Fatalf("SwitchRole to server failed: %v", err)
	}
	status = lanSvc.GetLanStatus()
	if status.Role != "server" {
		t.Fatalf("Role should be 'server', got: %s", status.Role)
	}

	// 4. 测试服务端唯一性与冲突检测
	// 模拟接收到一个来自其他服务端的信标
	otherServerBeacon := DiscoveryBeacon{
		Type:          "beacon",
		InstanceID:    "test-other-server-id-1234",
		DeviceName:    "Player-B-PC",
		Role:          "server",
		HttpPort:      38568,
		PresetVersion: "PRESET1234",
	}
	lanSvc.handleIncomingBeacon(otherServerBeacon, "192.168.1.188")

	conflictStatus := lanSvc.GetLanStatus()
	if !conflictStatus.HasServerConflict {
		t.Fatalf("Expected server conflict when another server is heard, but got false")
	}
	if len(conflictStatus.ConflictingServers) == 0 {
		t.Fatalf("Expected at least 1 conflicting server in list")
	}
	t.Logf("Server conflict detected successfully: %s (%s)",
		conflictStatus.ConflictingServers[0].DeviceName, conflictStatus.ConflictingServers[0].IP)

	// 模拟切换回客户端角色
	_ = lanSvc.SwitchRole("client")
	// 此时网络中有 1 个服务端 (Player-B)，客户端不发生冲突
	clientStatus := lanSvc.GetLanStatus()
	if clientStatus.HasServerConflict {
		t.Fatalf("Expected no conflict when client sees only 1 server, got true")
	}
	if clientStatus.ActiveServer == nil || clientStatus.ActiveServer.InstanceID != "test-other-server-id-1234" {
		t.Fatalf("Expected ActiveServer to be Player-B")
	}
	t.Logf("ActiveServer recognized by client: %s (%s)",
		clientStatus.ActiveServer.DeviceName, clientStatus.ActiveServer.IP)
}

func TestVersionComparisonAndUpdater(t *testing.T) {
	// 1. 测试语义化版本号比较
	cases := []struct {
		v1, v2 string
		expect int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.2.0", "1.1.9", 1},
		{"v2.0.0", "1.9.9", 1},
		{"1.0.0-beta", "1.0.0", 0},
		{"1.10.0", "1.9.0", 1},
	}

	for _, c := range cases {
		res := CompareVersions(c.v1, c.v2)
		if res != c.expect {
			t.Errorf("CompareVersions(%q, %q) = %d; expected %d", c.v1, c.v2, res, c.expect)
		}
	}

	// 2. 测试局域网在线节点版本探测
	gameSvc := NewGameService()
	gameSvc.Config.GameDir = `E:\Rusted Warfare\Rusted Warfare`
	prefSvc := NewPrefService()
	modSvc := NewModService(prefSvc)
	mapSvc := NewMapService(prefSvc)
	lanSvc := NewLanService(gameSvc, prefSvc, modSvc, mapSvc)
	updateSvc := NewUpdateService(gameSvc, lanSvc)

	// 初始状态下无节点，不应有局域网更新
	info, err := updateSvc.CheckLanUpdate()
	if err != nil {
		t.Fatalf("CheckLanUpdate unexpected error: %v", err)
	}
	if info != nil {
		t.Fatalf("Expected nil when no peers online, got %+v", info)
	}

	// 模拟接收到一个带有更高版本的节点广播
	higherPeerBeacon := DiscoveryBeacon{
		Type:          "beacon",
		InstanceID:    "test-higher-peer-id",
		DeviceName:    "Host-NewVersion-PC",
		Role:          "server",
		HttpPort:      38568,
		PresetVersion: "PRESET_V2",
		AppVersion:    "1.0.5",
	}
	lanSvc.handleIncomingBeacon(higherPeerBeacon, "192.168.1.188")

	// 校验 peers 中已正确记录了 AppVersion
	peers := lanSvc.GetDiscoveredPeers()
	if len(peers) == 0 || peers[0].AppVersion != "1.0.5" {
		t.Fatalf("Expected peer with AppVersion 1.0.5, got peers: %+v", peers)
	}

	t.Logf("Peer version detection verified: %+v", peers[0])
}
