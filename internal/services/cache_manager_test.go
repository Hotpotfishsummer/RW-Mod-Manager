package services

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rw_mod_manager/internal/models"
)

func TestCacheManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rw_cache_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm := &CacheManager{
		cacheDir:  tempDir,
		hashCache: make(map[string]HashCacheEntry),
		modCache:  make(map[string]ModCacheEntry),
		mapCache: MapCacheData{
			Standalone: make(map[string]StandaloneMapCacheEntry),
			Packs:      make(map[string]PackMapCacheEntry),
		},
	}

	testFile := filepath.Join(tempDir, "sample.rwmod")
	_ = os.WriteFile(testFile, []byte("fake rwmod content"), 0644)
	fi, _ := os.Stat(testFile)
	mtime := fi.ModTime().UnixNano()
	size := fi.Size()

	// 1. 测试内容哈希缓存
	cm.SetHash(testFile, mtime, size, "hash_abc_123")
	gotHash, hit := cm.GetHash(testFile, mtime, size)
	if !hit || gotHash != "hash_abc_123" {
		t.Fatalf("Expected hash_abc_123, got %s, hit: %v", gotHash, hit)
	}

	// 测试大小/时间戳不一致时缓存失效
	_, hit = cm.GetHash(testFile, mtime+1, size)
	if hit {
		t.Fatalf("Expected cache miss on mtime mismatch")
	}

	// 2. 测试模组元数据缓存
	sampleMod := models.ModItem{
		FileName: "sample.rwmod",
		Title:    "示例模组",
		UnitCount: 15,
	}
	cm.SetMod("sample.rwmod", mtime, size, sampleMod)
	gotMod, hit := cm.GetMod("sample.rwmod", mtime, size)
	if !hit || gotMod.Title != "示例模组" || gotMod.UnitCount != 15 {
		t.Fatalf("Expected mod hit with Title '示例模组', got %+v", gotMod)
	}

	// 3. 测试地图缓存
	sampleMap := models.MapItem{
		ID:       "sample.tmx",
		FileName: "sample.tmx",
		Title:    "示例地图",
		Width:    100,
		Height:   100,
	}
	cm.SetStandaloneMap("sample.tmx", mtime, size, sampleMap)
	gotMap, hit := cm.GetStandaloneMap("sample.tmx", mtime, size)
	if !hit || gotMap.Title != "示例地图" || gotMap.Width != 100 {
		t.Fatalf("Expected map hit with Title '示例地图', got %+v", gotMap)
	}

	// 4. 测试落盘持久化与加载恢复
	cm.FlushAll()

	// 重新构造实例读取已保存的 JSON
	cm2 := &CacheManager{
		cacheDir:  tempDir,
		hashCache: make(map[string]HashCacheEntry),
		modCache:  make(map[string]ModCacheEntry),
		mapCache: MapCacheData{
			Standalone: make(map[string]StandaloneMapCacheEntry),
			Packs:      make(map[string]PackMapCacheEntry),
		},
	}
	cm2.loadAll()

	restoredHash, hit := cm2.GetHash(testFile, mtime, size)
	if !hit || restoredHash != "hash_abc_123" {
		t.Fatalf("Restored hash mismatch: got %s, hit: %v", restoredHash, hit)
	}

	restoredMod, hit := cm2.GetMod("sample.rwmod", mtime, size)
	if !hit || restoredMod.Title != "示例模组" {
		t.Fatalf("Restored mod mismatch: got %+v, hit: %v", restoredMod, hit)
	}

	restoredMap, hit := cm2.GetStandaloneMap("sample.tmx", mtime, size)
	if !hit || restoredMap.Title != "示例地图" {
		t.Fatalf("Restored map mismatch: got %+v, hit: %v", restoredMap, hit)
	}
}

func TestScanPerformanceWithCache(t *testing.T) {
	gameDir := `E:\Rusted Warfare\Rusted Warfare`
	if _, err := os.Stat(filepath.Join(gameDir, "game-lib.jar")); err != nil {
		t.Skip("本地未找到铁锈战争真实目录，跳过真实目录性能回归测试")
	}

	prefSvc := NewPrefService()
	modSvc := NewModService(prefSvc)
	mapSvc := NewMapService(prefSvc)

	// 第一次扫描 (暖机并写盘缓存)
	start1 := time.Now()
	mods1, err := modSvc.ScanMods(gameDir)
	if err != nil {
		t.Fatalf("First ScanMods failed: %v", err)
	}
	maps1, err := mapSvc.ScanMaps(gameDir)
	if err != nil {
		t.Fatalf("First ScanMaps failed: %v", err)
	}
	dur1 := time.Since(start1)
	t.Logf("第一次扫描耗时 (建立缓存): %v (模组: %d, 地图: %d)", dur1, len(mods1), len(maps1))

	// 第二次扫描 (纯缓存命中)
	start2 := time.Now()
	mods2, err := modSvc.ScanMods(gameDir)
	if err != nil {
		t.Fatalf("Second ScanMods failed: %v", err)
	}
	maps2, err := mapSvc.ScanMaps(gameDir)
	if err != nil {
		t.Fatalf("Second ScanMaps failed: %v", err)
	}
	dur2 := time.Since(start2)
	t.Logf("第二次扫描耗时 (缓存命中): %v (模组: %d, 地图: %d)", dur2, len(mods2), len(maps2))

	if len(mods1) != len(mods2) || len(maps1) != len(maps2) {
		t.Fatalf("Count mismatch between scans")
	}

	// 验证流式封面与缩略图 URL 格式 (非 Base64 大图)
	for _, m := range mods2 {
		if m.Thumbnail != "" {
			if strings.HasPrefix(m.Thumbnail, "data:image") {
				t.Fatalf("Mod Thumbnail is still Base64! val=%s", m.Thumbnail[:100])
			}
			if !strings.HasPrefix(m.Thumbnail, "/api/thumbnail/mod") {
				t.Fatalf("Mod Thumbnail should be streaming URL, got %s", m.Thumbnail)
			}
		}
	}
	for _, mp := range maps2 {
		if mp.PreviewData != "" {
			if strings.HasPrefix(mp.PreviewData, "data:image") {
				t.Fatalf("Map PreviewData is still Base64! val=%s", mp.PreviewData[:100])
			}
			if !strings.HasPrefix(mp.PreviewData, "/api/thumbnail/map") {
				t.Fatalf("Map PreviewData should be streaming URL, got %s", mp.PreviewData)
			}
		}
	}
}

func TestModThumbnailServingAndFallback(t *testing.T) {
	gameDir := `E:\Rusted Warfare\Rusted Warfare`
	prefSvc := NewPrefService()
	modSvc := NewModService(prefSvc)

	// 清理旧缓存确保重新全量走严谨的封面提取流程
	cm := GetCacheManager()
	cm.modMu.Lock()
	cm.modCache = make(map[string]ModCacheEntry)
	cm.modDirty = true
	cm.modMu.Unlock()
	cm.SaveModCache()

	mods, err := modSvc.ScanMods(gameDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, m := range mods {
		if m.Thumbnail != "" {
			req, _ := http.NewRequest("GET", m.Thumbnail, nil)
			rec := httptest.NewRecorder()
			modSvc.ServeModThumbnail(rec, req, gameDir, m.FileName)
			if rec.Code != http.StatusOK {
				t.Errorf("FAILED to serve thumbnail for mod %s: status=%d", m.FileName, rec.Code)
			}
			if rec.Body.Len() == 0 {
				t.Errorf("Empty thumbnail body for mod %s", m.FileName)
			}
		} else {
			// 没有合法封面的模组必须为 ""，确保前端渲染优雅占位而非裂图
			t.Logf("Mod %s correctly has no thumbnail (clean fallback)", m.FileName)
		}
	}
}
