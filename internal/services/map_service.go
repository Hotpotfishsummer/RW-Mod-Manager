package services

import (
	"archive/zip"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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

// ScanMaps 扫描独立地图与所有 rwmod 包中附带的地图，并标识其在游戏中的激活状态
func (s *MapService) ScanMaps(gameDir string) ([]models.MapItem, error) {
	var allMaps []models.MapItem

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

			if strings.HasSuffix(nameLower, ".tmx") {
				// 独立地图：在游戏中常驻可用
				m := s.parseTmxFile(fullPath, entry)
				m.LocationType = "standalone"
				m.Active = true
				allMaps = append(allMaps, m)
			} else if strings.HasSuffix(nameLower, ".rwmod") {
				// 地图包 .rwmod
				packMaps := s.parseMapPack(fullPath, entry, "map_pack", true, "")
				allMaps = append(allMaps, packMaps...)
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

			modMaps := s.parseMapPack(fullPath, entry, "unit_mod", isEnabled, name)
			allMaps = append(allMaps, modMaps...)
		}
	}

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

	// 2. 检测是否存在同名缩略图 (baseName + "_map.png")
	previewPath := filepath.Join(filepath.Dir(path), baseName+"_map.png")
	if fi, err := os.Stat(previewPath); err == nil && !fi.IsDir() {
		item.HasPreview = true
		if fi.Size() < 5*1024*1024 {
			if imgData, err := os.ReadFile(previewPath); err == nil {
				item.PreviewData = "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgData)
			}
		}
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
		nameLower := strings.ToLower(f.Name)
		if strings.HasSuffix(nameLower, "_map.png") {
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

			// 检查匹配的缩略图
			if pf, ok := previewMap[strings.ToLower(baseName)]; ok {
				item.HasPreview = true
				if pf.UncompressedSize64 < 5*1024*1024 {
					if prc, err := pf.Open(); err == nil {
						imgData, err := io.ReadAll(prc)
						prc.Close()
						if err == nil {
							item.PreviewData = "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgData)
						}
					}
				}
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

	return totalCount, nil
}

// BatchGenerateMissingThumbnails 保持向后兼容
func (s *MapService) BatchGenerateMissingThumbnails(gameDir string) (int, error) {
	return s.BatchGenerateThumbnails(gameDir, false)
}
