package services

import (
	"archive/zip"
	"encoding/base64"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"rw_mod_manager/internal/models"
)

// MapService 负责 maps 目录与内嵌地图的扫描与属性解析
type MapService struct{}

func NewMapService() *MapService {
	return &MapService{}
}

// TmxRoot 仅解析 TMX XML 根节点的关键属性
type TmxRoot struct {
	XMLName xml.Name `xml:"map"`
	Width   int      `xml:"width,attr"`
	Height  int      `xml:"height,attr"`
}

var (
	rePlayerCountBracket = regexp.MustCompile(`(?i)\[(?:.*;)?p([0-9]+)\]`)
	rePlayerCountParen   = regexp.MustCompile(`(?i)[\(（]([0-9]+)\s*p[^\)）]*[\)）]`)
	rePlayerCountPeople  = regexp.MustCompile(`([0-9]+)\s*人`)
)

// ScanMaps 扫描游戏 mods/maps 目录下的所有地图
func (s *MapService) ScanMaps(gameDir string) ([]models.MapItem, error) {
	mapsDir := filepath.Join(gameDir, "mods", "maps")
	entries, err := os.ReadDir(mapsDir)
	if err != nil {
		return nil, err
	}

	var maps []models.MapItem

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(mapsDir, name)
		nameLower := strings.ToLower(name)

		if strings.HasSuffix(nameLower, ".tmx") {
			// 单张独立 TMX 地图
			m := s.parseTmxFile(fullPath, entry)
			maps = append(maps, m)
		} else if strings.HasSuffix(nameLower, ".rwmod") {
			// 地图包 .rwmod，提取其中包含的所有地图
			packMaps := s.parseMapPack(fullPath, entry)
			maps = append(maps, packMaps...)
		}
	}

	return maps, nil
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
		if fi.Size() < 3*1024*1024 {
			if imgData, err := os.ReadFile(previewPath); err == nil {
				item.PreviewData = "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgData)
			}
		}
	}

	return item
}

// parseMapPack 解析地图包归档中的地图与缩略图
func (s *MapService) parseMapPack(path string, d os.DirEntry) []models.MapItem {
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
				ID:          d.Name() + "/" + f.Name,
				FileName:    fileName,
				FilePath:    path,
				IsPack:      true,
				PackName:    d.Name(),
				Title:       baseName,
				PlayerCount: s.extractPlayerCount(fileName),
				FileSize:    int64(f.UncompressedSize64),
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
				if pf.UncompressedSize64 < 3*1024*1024 {
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
		if c, err := strconv.Atoi(m[1]); err == nil && c > 0 {
			return c
		}
	}
	if m := rePlayerCountParen.FindStringSubmatch(filename); len(m) > 1 {
		if c, err := strconv.Atoi(m[1]); err == nil && c > 0 {
			return c
		}
	}
	if m := rePlayerCountPeople.FindStringSubmatch(filename); len(m) > 1 {
		if c, err := strconv.Atoi(m[1]); err == nil && c > 0 {
			return c
		}
	}
	return 2 // 默认双人
}
