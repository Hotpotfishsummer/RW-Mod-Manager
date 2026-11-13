package services

import (
	"archive/zip"
	"compress/flate"
	"crypto/md5"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"rw_mod_manager/internal/applog"
	"rw_mod_manager/internal/models"
)

// MapService 负责 maps 目录与各模组内嵌地图的扫描、属性解析与缩略图生成
type MapService struct {
	prefService *PrefService
}

func NewMapService(pref *PrefService) *MapService {
	return &MapService{
		prefService: pref,
	}
}

// TmxRoot 仅解析 TMX XML 根节点的关键属性
type TmxRoot struct {
	XMLName xml.Name `xml:"map"`
	Width   int      `xml:"width,attr"`
	Height  int      `xml:"height,attr"`
}

var (
	rePlayerCountBracket = regexp.MustCompile(`(?i)\[(?:.*;)?p?([0-9]+)p?\]`)
	rePlayerCountParen   = regexp.MustCompile(`(?i)[\(（](?:p)?([0-9]+)\s*p?[^\)）]*[\)）]`)
	rePlayerCountPeople  = regexp.MustCompile(`([0-9]+)\s*人`)
	rePlayerCountWord    = regexp.MustCompile(`(?i)(?:[_\s\-])([0-9]{1,2})\s*p\b`)
	rePlayerCountMix     = regexp.MustCompile(`(?i)([0-9]{1,2})\s*p(?:混战|图)?`)
)

// ScanMaps 扫描独立地图与所有 rwmod 包中附带的地图，并标识其在游戏中的激活状态 (带有持久化缓存与零内存膨胀流式图片)
func (s *MapService) ScanMaps(gameDir string) ([]models.MapItem, error) {
	allMaps := make([]models.MapItem, 0)
	cm := GetCacheManager()

	// 1. 读取 preferences.ini 中的模组启用状态映射
	statusMap, _ := s.prefService.ReadModSettings(gameDir)

	// 2. 扫描 mods/maps/ 目录下的独立地图与地图包
	mapsDir := filepath.Join(gameDir, "mods", "maps")
	if entries, err := os.ReadDir(mapsDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || entry.IsDir() {
				continue
			}

			fullPath := filepath.Join(mapsDir, name)
			nameLower := strings.ToLower(name)

			info, _ := entry.Info()
			var mtime, size int64
			if info != nil {
				mtime = info.ModTime().UnixNano()
				size = info.Size()
			}

			if strings.HasSuffix(nameLower, ".tmx") {
				// 独立地图：尝试从缓存命中
				if cached, hit := cm.GetStandaloneMap(fullPath, mtime, size); hit {
					cached.LocationType = "standalone"
					cached.Active = true
					// 快速检查预览更新
					baseName := strings.TrimSuffix(name, filepath.Ext(name))
					previewPath := filepath.Join(mapsDir, baseName+"_map.png")
					if pfi, err := os.Stat(previewPath); err == nil && !pfi.IsDir() {
						cached.HasPreview = true
						cached.PreviewData = fmt.Sprintf("/api/thumbnail/map?id=%s&v=%d", url.QueryEscape(cached.ID), pfi.ModTime().UnixNano())
					}
					allMaps = append(allMaps, cached)
				} else {
					m := s.parseTmxFile(fullPath, entry)
					m.LocationType = "standalone"
					m.Active = true
					cm.SetStandaloneMap(fullPath, mtime, size, m)
					allMaps = append(allMaps, m)
				}
			} else if strings.HasSuffix(nameLower, ".rwmod") {
				// 地图包 .rwmod：尝试从包缓存命中
				if packMaps, hit := cm.GetPackMaps(fullPath, mtime, size); hit {
					for i := range packMaps {
						packMaps[i].LocationType = "map_pack"
						packMaps[i].Active = true
						packMaps[i].ParentModFile = ""
					}
					allMaps = append(allMaps, packMaps...)
				} else {
					packMaps := s.parseMapPack(fullPath, entry, "map_pack", true, "")
					cm.SetPackMaps(fullPath, mtime, size, packMaps)
					allMaps = append(allMaps, packMaps...)
				}
			}
		}
	}

	// 3. 扫描 mods/units/ 目录下的所有 rwmod，提取包含的地图并标记其激活状态
	unitsDir := filepath.Join(gameDir, "mods", "units")
	if entries, err := os.ReadDir(unitsDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || entry.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".rwmod") {
				continue
			}

			fullPath := filepath.Join(unitsDir, name)
			modHash := s.prefService.CalculateModHash(name)
			isEnabled := statusMap[modHash] // 只有当所属模组启用时，其内嵌地图才在游戏中激活

			info, _ := entry.Info()
			var mtime, size int64
			if info != nil {
				mtime = info.ModTime().UnixNano()
				size = info.Size()
			}

			if packMaps, hit := cm.GetPackMaps(fullPath, mtime, size); hit {
				for i := range packMaps {
					packMaps[i].LocationType = "unit_mod"
					packMaps[i].Active = isEnabled
					packMaps[i].ParentModFile = name
				}
				allMaps = append(allMaps, packMaps...)
			} else {
				modMaps := s.parseMapPack(fullPath, entry, "unit_mod", isEnabled, name)
				cm.SetPackMaps(fullPath, mtime, size, modMaps)
				allMaps = append(allMaps, modMaps...)
			}
		}
	}

	// 异步持久化保存地图缓存
	cm.SaveMapCache()

	applog.Debug("MapService", "扫描地图完成: dir=%s, count=%d", gameDir, len(allMaps))
	return allMaps, nil
}

// parseTmxFile 解析单张独立 TMX 地图文件
func (s *MapService) parseTmxFile(path string, d os.DirEntry) models.MapItem {
	info, _ := d.Info()
	fileSize := int64(0)
	if info != nil {
		fileSize = info.Size()
	}

	baseName := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
	item := models.MapItem{
		ID:          d.Name(),
		FileName:    d.Name(),
		FilePath:    path,
		Title:       baseName,
		PlayerCount: s.extractPlayerCount(d.Name()),
		FileSize:    fileSize,
	}

	// 1. 读取 TMX 尺寸属性
	if data, err := os.ReadFile(path); err == nil {
		var root TmxRoot
		if err := xml.Unmarshal(data, &root); err == nil {
			item.Width = root.Width
			item.Height = root.Height
		}
	}

	// 2. 检测是否存在同名缩略图 (baseName + "_map.png")，使用流式 URL 代替 Base64，大幅降低 WebView2 与 Go 内存占用
	previewPath := filepath.Join(filepath.Dir(path), baseName+"_map.png")
	if fi, err := os.Stat(previewPath); err == nil && !fi.IsDir() {
		item.HasPreview = true
		item.PreviewData = fmt.Sprintf("/api/thumbnail/map?id=%s&v=%d", url.QueryEscape(item.ID), fi.ModTime().UnixNano())
	}

	return item
}

// parseMapPack 解析地图包归档中的地图与缩略图
func (s *MapService) parseMapPack(path string, d os.DirEntry, locType string, active bool, parentModFile string) []models.MapItem {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil
	}
	defer r.Close()

	var packMaps []models.MapItem
	previewMap := make(map[string]*zip.File)

	// 先建立缩略图索引
	for _, f := range r.File {
		nameLower := strings.TrimSuffix(strings.ToLower(filepath.ToSlash(f.Name)), "/")
		if strings.HasSuffix(nameLower, "_map.png") && !f.FileInfo().IsDir() && f.UncompressedSize64 > 0 {
			base := strings.TrimSuffix(filepath.Base(nameLower), "_map.png")
			previewMap[base] = f
		}
	}

	// 再解析每个 tmx
	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".tmx") {
			fileName := filepath.Base(f.Name)
			baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

			item := models.MapItem{
				ID:            d.Name() + "/" + f.Name,
				FileName:      fileName,
				FilePath:      path + ":" + f.Name,
				IsPack:        true,
				PackName:      d.Name(),
				ParentModFile: parentModFile,
				LocationType:  locType,
				Active:        active,
				Title:         baseName,
				PlayerCount:   s.extractPlayerCount(fileName),
				FileSize:      int64(f.UncompressedSize64),
			}

			// 读取 TMX 尺寸
			if rc, err := f.Open(); err == nil {
				data, err := io.ReadAll(rc)
				rc.Close()
				if err == nil {
					var root TmxRoot
					if err := xml.Unmarshal(data, &root); err == nil {
						item.Width = root.Width
						item.Height = root.Height
					}
				}
			}

			// 检查匹配的缩略图 (采用流式 API 路径，完全避免数百张图片内存常驻)
			if pf, ok := previewMap[strings.ToLower(baseName)]; ok {
				item.HasPreview = true
				item.PreviewData = fmt.Sprintf("/api/thumbnail/map?id=%s&v=%d", url.QueryEscape(item.ID), int64(pf.UncompressedSize64))
			}

			packMaps = append(packMaps, item)
		}
	}

	return packMaps
}

// extractPlayerCount 从文件名中提取推荐玩家数
func (s *MapService) extractPlayerCount(filename string) int {
	if m := rePlayerCountBracket.FindStringSubmatch(filename); len(m) > 1 {
		if c, err := strconv.Atoi(m[1]); err == nil && c >= 1 && c <= 24 {
			return c
		}
	}
	if m := rePlayerCountParen.FindStringSubmatch(filename); len(m) > 1 {
		if c, err := strconv.Atoi(m[1]); err == nil && c >= 1 && c <= 24 {
			return c
		}
	}
	if m := rePlayerCountPeople.FindStringSubmatch(filename); len(m) > 1 {
		if c, err := strconv.Atoi(m[1]); err == nil && c >= 1 && c <= 24 {
			return c
		}
	}
	if m := rePlayerCountWord.FindStringSubmatch(filename); len(m) > 1 {
		if c, err := strconv.Atoi(m[1]); err == nil && c >= 1 && c <= 24 {
			return c
		}
	}
	if m := rePlayerCountMix.FindStringSubmatch(filename); len(m) > 1 {
		if c, err := strconv.Atoi(m[1]); err == nil && c >= 1 && c <= 24 {
			return c
		}
	}
	return 2
}

// GenerateThumbnail 为指定 TMX 文件生成（或重新生成）带出生点标记的高清缩略图 (100% 纯 Go 原生实现，无外部 Python 依赖)
// 既支持独立地图文件（如 mods/maps/foo.tmx），也支持 rwmod 包内条目（如 mods/units/pack.rwmod:sub/foo.tmx）
func (s *MapService) GenerateThumbnail(gameDir string, tmxPath string) (string, error) {
	renderer := NewMapRenderer(gameDir)

	var pngBytes []byte
	var err error

	if strings.Contains(tmxPath, ".rwmod:") {
		parts := strings.SplitN(tmxPath, ".rwmod:", 2)
		packPath := parts[0] + ".rwmod"
		entryName := parts[1]
		pngBytes, err = renderer.RenderPackEntry(packPath, entryName, 500)
	} else {
		pngBytes, err = renderer.RenderStandalone(tmxPath, 500)
	}

	if err != nil {
		return "", fmt.Errorf("生成缩略图失败: %w", err)
	}

	// 使得对应地图缓存失效，下次重新读取最新图
	GetCacheManager().InvalidateMapCache(tmxPath)

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes), nil
}

// BatchGenerateThumbnails 批量扫描为地图生成或全量重绘出生点预览 (100% 纯 Go 原生实现)
// force 为 true 时强制重新生成覆盖已有旧图；为 false 时仅补齐缺失
func (s *MapService) BatchGenerateThumbnails(gameDir string, force bool) (int, error) {
	renderer := NewMapRenderer(gameDir)
	totalCount := 0

	// 1. 处理独立地图目录 mods/maps
	mapsDir := filepath.Join(gameDir, "mods", "maps")
	if fi, err := os.Stat(mapsDir); err == nil && fi.IsDir() {
		if c, err := renderer.BatchRenderStandalone(mapsDir, force); err == nil {
			totalCount += c
		}
	}

	// 2. 处理所有 .rwmod 地图包 (mods/units 与 mods/maps 下的 .rwmod)
	var packs []string
	for _, sub := range []string{"units", "maps"} {
		dir := filepath.Join(gameDir, "mods", sub)
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".rwmod") {
					packs = append(packs, filepath.Join(dir, e.Name()))
				}
			}
		}
	}

	if len(packs) > 0 {
		if c, err := renderer.BatchPatchPacks(packs, force); err == nil {
			totalCount += c
		}
	}

	// 批量更新后清空地图缓存
	GetCacheManager().FlushAll()

	return totalCount, nil
}

// BatchGenerateMissingThumbnails 保持向后兼容
func (s *MapService) BatchGenerateMissingThumbnails(gameDir string) (int, error) {
	return s.BatchGenerateThumbnails(gameDir, false)
}

// ServeMapThumbnail 流式提供单个地图的缩略图图片，带有浏览器强缓存与磁盘抽离缓存
// readZipEntryBytes 安全读取 zip 条目字节数据，具备对畸形条目或末尾带斜杠的 OpenRaw 容错回退
func (s *MapService) readZipEntryBytes(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 == 0 || f.UncompressedSize64 > 15*1024*1024 {
		return nil, fmt.Errorf("invalid entry size: %d", f.UncompressedSize64)
	}

	// 1. 标准 Open 尝试
	rc, err := f.Open()
	if err == nil {
		data, err := io.ReadAll(rc)
		rc.Close()
		if err == nil && len(data) > 0 {
			return data, nil
		}
	}

	// 2. 若标准 Open 失败，通过 OpenRaw 与 flate 手动解压
	rawR, err := f.OpenRaw()
	if err != nil {
		return nil, err
	}
	var reader io.Reader = rawR
	if f.Method == zip.Deflate {
		flateR := flate.NewReader(rawR)
		defer flateR.Close()
		reader = flateR
	}
	return io.ReadAll(reader)
}

func (s *MapService) ServeMapThumbnail(w http.ResponseWriter, r *http.Request, gameDir, id string) {
	if id == "" {
		http.NotFound(w, r)
		return
	}

	cm := GetCacheManager()

	// 1. 若为独立地图：id 为文件名，如 "foo.tmx"
	if !strings.Contains(id, "/") && !strings.Contains(id, ":") {
		baseName := strings.TrimSuffix(id, filepath.Ext(id))
		previewPath := filepath.Join(gameDir, "mods", "maps", baseName+"_map.png")
		if fi, err := os.Stat(previewPath); err == nil && !fi.IsDir() {
			w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
			http.ServeFile(w, r, previewPath)
			return
		}
	}

	// 2. 若为包内地图：id 格式为 "pack.rwmod/path/to/map.tmx" 或 "pack.rwmod:entry"
	var packName, entryName string
	if strings.Contains(id, ":") {
		parts := strings.SplitN(id, ":", 2)
		packName = parts[0]
		entryName = parts[1]
	} else if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		packName = parts[0]
		entryName = parts[1]
	}

	if packName != "" {
		cacheHash := fmt.Sprintf("%x", md5.Sum([]byte(id)))
		cachedThumbPath := filepath.Join(cm.ThumbnailDir(), "map_"+cacheHash+".png")

		// 检查本地磁盘解压缓存
		if cfi, err := os.Stat(cachedThumbPath); err == nil && !cfi.IsDir() {
			w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
			http.ServeFile(w, r, cachedThumbPath)
			return
		}

		// 寻找宿主 .rwmod 路径 (优先 maps，其次 units)
		var packPath string
		cand1 := filepath.Join(gameDir, "mods", "maps", packName)
		cand2 := filepath.Join(gameDir, "mods", "units", packName)
		if fi, err := os.Stat(cand1); err == nil && !fi.IsDir() {
			packPath = cand1
		} else if fi, err := os.Stat(cand2); err == nil && !fi.IsDir() {
			packPath = cand2
		}

		if packPath != "" {
			if zr, err := zip.OpenReader(packPath); err == nil {
				defer zr.Close()

				mapBase := strings.TrimSuffix(filepath.Base(entryName), filepath.Ext(entryName))
				targetSuffix := strings.ToLower(mapBase + "_map.png")

				for _, f := range zr.File {
					cleanName := strings.Trim(strings.ReplaceAll(f.Name, "\\", "/"), "/")
					cleanBase := filepath.Base(cleanName)
					if strings.EqualFold(cleanBase, targetSuffix) || strings.HasSuffix(strings.ToLower(cleanName), targetSuffix) {
						if data, err := s.readZipEntryBytes(f); err == nil && len(data) > 0 {
							_ = os.WriteFile(cachedThumbPath, data, 0644)
							w.Header().Set("Content-Type", "image/png")
							w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(data)
							return
						}
					}
				}
			}
		}
	}

	http.NotFound(w, r)
}

// ArchiveMap 归档或彻底删除指定的独立地图或地图包
// mapTarget: 可以是独立地图文件名 (如 "1917-流血的城市.tmx")、地图包文件名 (如 "ww1_maps.rwmod") 或地图 ID
// permanently: true 为永久彻底删除；false 为移入 mods/_archived/maps 目录安全归档
func (s *MapService) ArchiveMap(gameDir string, mapTarget string, permanently bool) error {
	if mapTarget == "" {
		return fmt.Errorf("地图目标名称不能为空")
	}

	cleanTarget := filepath.Clean(mapTarget)
	if strings.Contains(cleanTarget, "/") || strings.Contains(cleanTarget, "\\") {
		parts := strings.FieldsFunc(cleanTarget, func(r rune) bool {
			return r == '/' || r == '\\'
		})
		if len(parts) > 0 && strings.HasSuffix(strings.ToLower(parts[0]), ".rwmod") {
			cleanTarget = parts[0]
		} else {
			cleanTarget = filepath.Base(cleanTarget)
		}
	} else {
		cleanTarget = filepath.Base(cleanTarget)
	}

	if cleanTarget == "." || cleanTarget == "/" || cleanTarget == "\\" || strings.HasPrefix(cleanTarget, "..") {
		return fmt.Errorf("非法的文件名路径: %s", mapTarget)
	}

	mapsDir := filepath.Join(gameDir, "mods", "maps")
	filePath := filepath.Join(mapsDir, cleanTarget)

	info, err := os.Stat(filePath)
	if err != nil {
		unitsDir := filepath.Join(gameDir, "mods", "units")
		if _, uErr := os.Stat(filepath.Join(unitsDir, cleanTarget)); uErr == nil {
			return fmt.Errorf("该地图/资源隶属于单位模组 [%s]，请前往模组管理页面归档宿主模组", cleanTarget)
		}
		if os.IsNotExist(err) {
			return fmt.Errorf("地图文件不存在: %s", cleanTarget)
		}
		return fmt.Errorf("读取地图文件失败: %w", err)
	}

	targetLower := strings.ToLower(cleanTarget)
	cm := GetCacheManager()

	// 1. 如果是独立 TMX 地图
	if strings.HasSuffix(targetLower, ".tmx") {
		baseName := strings.TrimSuffix(cleanTarget, filepath.Ext(cleanTarget))
		previewPath := filepath.Join(mapsDir, baseName+"_map.png")

		if permanently {
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("删除地图文件失败: %w", err)
			}
			if _, pErr := os.Stat(previewPath); pErr == nil {
				_ = os.Remove(previewPath)
			}
		} else {
			archiveDir := filepath.Join(gameDir, "mods", "_archived", "maps")
			if err := os.MkdirAll(archiveDir, 0755); err != nil {
				return fmt.Errorf("创建地图归档目录失败: %w", err)
			}

			destPath := filepath.Join(archiveDir, cleanTarget)
			if _, err := os.Stat(destPath); err == nil {
				_ = os.Remove(destPath)
			}
			if err := moveFileOrDir(filePath, destPath, false); err != nil {
				return fmt.Errorf("移入地图归档目录失败: %w", err)
			}

			// 同步归档已渲染的高清出生点预览图 (若存在)
			if _, pErr := os.Stat(previewPath); pErr == nil {
				destPreview := filepath.Join(archiveDir, baseName+"_map.png")
				_ = os.Remove(destPreview)
				_ = moveFileOrDir(previewPath, destPreview, false)
			}
		}

		cm.InvalidateMapCache(filePath)
		cm.SaveMapCache()
		return nil
	}

	// 2. 如果是地图包 .rwmod
	if strings.HasSuffix(targetLower, ".rwmod") {
		if permanently {
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("删除地图包失败: %w", err)
			}
		} else {
			archiveDir := filepath.Join(gameDir, "mods", "_archived", "maps")
			if err := os.MkdirAll(archiveDir, 0755); err != nil {
				return fmt.Errorf("创建地图归档目录失败: %w", err)
			}

			destPath := filepath.Join(archiveDir, cleanTarget)
			if _, err := os.Stat(destPath); err == nil {
				_ = os.Remove(destPath)
			}
			if err := moveFileOrDir(filePath, destPath, info.IsDir()); err != nil {
				return fmt.Errorf("移入地图包归档目录失败: %w", err)
			}
		}

		cm.InvalidateMapCache(filePath)
		cm.SaveMapCache()
		return nil
	}

	return fmt.Errorf("不支持的地图文件格式: %s", cleanTarget)
}
