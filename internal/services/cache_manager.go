package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"rw_mod_manager/internal/models"
)

// HashCacheEntry 存储单文件或目录的内容哈希与元数据特征
type HashCacheEntry struct {
	ModTime  int64  `json:"modTime"`
	FileSize int64  `json:"fileSize"`
	Hash     string `json:"hash"`
}

// ModCacheEntry 存储单个单位模组的解析元数据与文件特征
type ModCacheEntry struct {
	ModTime  int64          `json:"modTime"`
	FileSize int64          `json:"fileSize"`
	Mod      models.ModItem `json:"mod"`
}

// StandaloneMapCacheEntry 存储单张独立 TMX 地图解析特征
type StandaloneMapCacheEntry struct {
	ModTime  int64          `json:"modTime"`
	FileSize int64          `json:"fileSize"`
	Map      models.MapItem `json:"map"`
}

// PackMapCacheEntry 存储一个 .rwmod 包内所有内嵌地图的解析特征
type PackMapCacheEntry struct {
	ModTime  int64            `json:"modTime"`
	FileSize int64            `json:"fileSize"`
	Maps     []models.MapItem `json:"maps"`
}

// MapCacheData 组合独立地图与地图包持久化结构
type MapCacheData struct {
	Standalone map[string]StandaloneMapCacheEntry `json:"standalone"`
	Packs      map[string]PackMapCacheEntry      `json:"packs"`
}

// CacheManager 负责全局性能与元数据缓存的统一生命周期管理
type CacheManager struct {
	cacheDir string

	// 哈希缓存
	hashMu    sync.RWMutex
	hashCache map[string]HashCacheEntry
	hashDirty bool

	// 模组元数据缓存
	modMu    sync.RWMutex
	modCache map[string]ModCacheEntry
	modDirty bool

	// 地图元数据缓存
	mapMu    sync.RWMutex
	mapCache MapCacheData
	mapDirty bool
}

var (
	defaultCacheManager *CacheManager
	cacheOnce           sync.Once
)

// GetCacheManager 获取全局单例缓存管理器
func GetCacheManager() *CacheManager {
	cacheOnce.Do(func() {
		defaultCacheManager = newCacheManager()
	})
	return defaultCacheManager
}

func newCacheManager() *CacheManager {
	var cDir string
	configDir, err := os.UserConfigDir()
	if err == nil {
		cDir = filepath.Join(configDir, "RWModManager", "cache")
	} else {
		cDir = filepath.Join(".", "cache")
	}

	_ = os.MkdirAll(cDir, 0755)
	_ = os.MkdirAll(filepath.Join(cDir, "thumbnails"), 0755)

	cm := &CacheManager{
		cacheDir:  cDir,
		hashCache: make(map[string]HashCacheEntry),
		modCache:  make(map[string]ModCacheEntry),
		mapCache: MapCacheData{
			Standalone: make(map[string]StandaloneMapCacheEntry),
			Packs:      make(map[string]PackMapCacheEntry),
		},
	}

	cm.loadAll()
	return cm
}

// CacheDir 获取缓存总根目录
func (cm *CacheManager) CacheDir() string {
	return cm.cacheDir
}

// ThumbnailDir 获取缩略图物理缓存目录
func (cm *CacheManager) ThumbnailDir() string {
	return filepath.Join(cm.cacheDir, "thumbnails")
}

func (cm *CacheManager) loadAll() {
	cm.loadHashCache()
	cm.loadModCache()
	cm.loadMapCache()
}

// --- 内容哈希缓存 ---

func (cm *CacheManager) loadHashCache() {
	path := filepath.Join(cm.cacheDir, "content_hashes.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var loaded map[string]HashCacheEntry
	if err := json.Unmarshal(data, &loaded); err == nil && loaded != nil {
		cm.hashCache = loaded
	}
}

// GetHash 检索已缓存的文件哈希
func (cm *CacheManager) GetHash(path string, modTime, fileSize int64) (string, bool) {
	cm.hashMu.RLock()
	defer cm.hashMu.RUnlock()

	clean := filepath.Clean(path)
	entry, ok := cm.hashCache[clean]
	if ok && entry.ModTime == modTime && entry.FileSize == fileSize && entry.Hash != "" {
		return entry.Hash, true
	}
	return "", false
}

// SetHash 存储并更新文件哈希
func (cm *CacheManager) SetHash(path string, modTime, fileSize int64, hash string) {
	if hash == "" {
		return
	}
	cm.hashMu.Lock()
	defer cm.hashMu.Unlock()

	clean := filepath.Clean(path)
	cm.hashCache[clean] = HashCacheEntry{
		ModTime:  modTime,
		FileSize: fileSize,
		Hash:     hash,
	}
	cm.hashDirty = true
}

// SaveHashCache 保存哈希缓存至磁盘
func (cm *CacheManager) SaveHashCache() {
	cm.hashMu.Lock()
	if !cm.hashDirty {
		cm.hashMu.Unlock()
		return
	}
	cm.hashDirty = false
	data, err := json.Marshal(cm.hashCache)
	cm.hashMu.Unlock()

	if err == nil {
		_ = atomicWriteFile(filepath.Join(cm.cacheDir, "content_hashes.json"), data)
	}
}

// --- 模组元数据缓存 ---

func (cm *CacheManager) loadModCache() {
	path := filepath.Join(cm.cacheDir, "mod_metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var loaded map[string]ModCacheEntry
	if err := json.Unmarshal(data, &loaded); err == nil && loaded != nil {
		cm.modCache = loaded
	}
}

// GetMod 获取模组缓存
func (cm *CacheManager) GetMod(fileName string, modTime, fileSize int64) (models.ModItem, bool) {
	cm.modMu.RLock()
	defer cm.modMu.RUnlock()

	entry, ok := cm.modCache[fileName]
	if ok && entry.ModTime == modTime && entry.FileSize == fileSize {
		return entry.Mod, true
	}
	return models.ModItem{}, false
}

// SetMod 存储模组缓存
func (cm *CacheManager) SetMod(fileName string, modTime, fileSize int64, item models.ModItem) {
	cm.modMu.Lock()
	defer cm.modMu.Unlock()

	cm.modCache[fileName] = ModCacheEntry{
		ModTime:  modTime,
		FileSize: fileSize,
		Mod:      item,
	}
	cm.modDirty = true
}

// PruneMods 清除不再存在于目录中的冗余模组缓存
func (cm *CacheManager) PruneMods(existingFileNames map[string]bool) {
	cm.modMu.Lock()
	defer cm.modMu.Unlock()

	for k := range cm.modCache {
		if !existingFileNames[k] {
			delete(cm.modCache, k)
			cm.modDirty = true
		}
	}
}

// SaveModCache 保存模组缓存至磁盘
func (cm *CacheManager) SaveModCache() {
	cm.modMu.Lock()
	if !cm.modDirty {
		cm.modMu.Unlock()
		return
	}
	cm.modDirty = false
	data, err := json.Marshal(cm.modCache)
	cm.modMu.Unlock()

	if err == nil {
		_ = atomicWriteFile(filepath.Join(cm.cacheDir, "mod_metadata.json"), data)
	}
}

// --- 地图元数据缓存 ---

func (cm *CacheManager) loadMapCache() {
	path := filepath.Join(cm.cacheDir, "map_metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var loaded MapCacheData
	if err := json.Unmarshal(data, &loaded); err == nil {
		if loaded.Standalone != nil {
			cm.mapCache.Standalone = loaded.Standalone
		}
		if loaded.Packs != nil {
			cm.mapCache.Packs = loaded.Packs
		}
	}
}

// GetStandaloneMap 获取独立地图缓存
func (cm *CacheManager) GetStandaloneMap(path string, modTime, fileSize int64) (models.MapItem, bool) {
	cm.mapMu.RLock()
	defer cm.mapMu.RUnlock()

	clean := filepath.Clean(path)
	entry, ok := cm.mapCache.Standalone[clean]
	if ok && entry.ModTime == modTime && entry.FileSize == fileSize {
		return entry.Map, true
	}
	return models.MapItem{}, false
}

// SetStandaloneMap 存储独立地图缓存
func (cm *CacheManager) SetStandaloneMap(path string, modTime, fileSize int64, item models.MapItem) {
	cm.mapMu.Lock()
	defer cm.mapMu.Unlock()

	clean := filepath.Clean(path)
	cm.mapCache.Standalone[clean] = StandaloneMapCacheEntry{
		ModTime:  modTime,
		FileSize: fileSize,
		Map:      item,
	}
	cm.mapDirty = true
}

// GetPackMaps 获取地图包内嵌地图缓存
func (cm *CacheManager) GetPackMaps(packPath string, modTime, fileSize int64) ([]models.MapItem, bool) {
	cm.mapMu.RLock()
	defer cm.mapMu.RUnlock()

	clean := filepath.Clean(packPath)
	entry, ok := cm.mapCache.Packs[clean]
	if ok && entry.ModTime == modTime && entry.FileSize == fileSize && entry.Maps != nil {
		// 返回深拷贝副本以防调用方外部篡改
		cloned := make([]models.MapItem, len(entry.Maps))
		copy(cloned, entry.Maps)
		return cloned, true
	}
	return nil, false
}

// SetPackMaps 存储地图包内嵌地图缓存
func (cm *CacheManager) SetPackMaps(packPath string, modTime, fileSize int64, items []models.MapItem) {
	cm.mapMu.Lock()
	defer cm.mapMu.Unlock()

	clean := filepath.Clean(packPath)
	cloned := make([]models.MapItem, len(items))
	copy(cloned, items)

	cm.mapCache.Packs[clean] = PackMapCacheEntry{
		ModTime:  modTime,
		FileSize: fileSize,
		Maps:     cloned,
	}
	cm.mapDirty = true
}

// InvalidateMapCache 使某个地图或包的缓存失效
func (cm *CacheManager) InvalidateMapCache(path string) {
	cm.mapMu.Lock()
	defer cm.mapMu.Unlock()

	clean := filepath.Clean(path)
	delete(cm.mapCache.Standalone, clean)
	delete(cm.mapCache.Packs, clean)
	cm.mapDirty = true
}

// SaveMapCache 保存地图缓存至磁盘
func (cm *CacheManager) SaveMapCache() {
	cm.mapMu.Lock()
	if !cm.mapDirty {
		cm.mapMu.Unlock()
		return
	}
	cm.mapDirty = false
	data, err := json.Marshal(cm.mapCache)
	cm.mapMu.Unlock()

	if err == nil {
		_ = atomicWriteFile(filepath.Join(cm.cacheDir, "map_metadata.json"), data)
	}
}

// FlushAll 强制刷写所有脏数据至磁盘
func (cm *CacheManager) FlushAll() {
	cm.SaveHashCache()
	cm.SaveModCache()
	cm.SaveMapCache()
}

// atomicWriteFile 原子写入文件，防止程序异常退出损坏缓存
func atomicWriteFile(filename string, data []byte) error {
	tmpFile := filename + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, filename)
}
