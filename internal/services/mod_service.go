package services

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"rw_mod_manager/internal/models"
)

// ModService 负责模组的并发扫描、元数据解析与封面提取
type ModService struct {
	prefService *PrefService
}

func NewModService(pref *PrefService) *ModService {
	return &ModService{
		prefService: pref,
	}
}

// ScanMods 并发扫描指定游戏目录下的所有单位模组
func (s *ModService) ScanMods(gameDir string) ([]models.ModItem, error) {
	unitsDir := filepath.Join(gameDir, "mods", "units")
	entries, err := os.ReadDir(unitsDir)
	if err != nil {
		return nil, err
	}

	// 先读取当前 preferences.ini 中记录的状态映射
	statusMap, _ := s.prefService.ReadModSettings(gameDir)

	var targets []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.EqualFold(name, "custom_units_here.txt") {
			continue
		}
		if entry.IsDir() || strings.HasSuffix(strings.ToLower(name), ".rwmod") {
			targets = append(targets, entry)
		}
	}

	results := make([]models.ModItem, len(targets))
	var wg sync.WaitGroup
	// 限制最大并发数
	sem := make(chan struct{}, 8)

	for i, entry := range targets {
		wg.Add(1)
		go func(idx int, d os.DirEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			fullPath := filepath.Join(unitsDir, d.Name())
			item := s.parseMod(fullPath, d)
			// 计算 hash 并匹配启用状态
			item.ID = s.prefService.CalculateModHash(item.FileName)
			if enabled, ok := statusMap[item.ID]; ok {
				item.Enabled = enabled
			} else {
				// 未在配置中的新模组默认禁用
				item.Enabled = false
			}
			results[idx] = item
		}(i, entry)
	}

	wg.Wait()

	// 过滤有效项（去掉空项）
	var finalMods []models.ModItem
	for _, m := range results {
		if m.FileName != "" {
			finalMods = append(finalMods, m)
		}
	}

	return finalMods, nil
}

// parseMod 解析单个 .rwmod 包或解压文件夹
func (s *ModService) parseMod(path string, d os.DirEntry) models.ModItem {
	info, err := d.Info()
	modTime := int64(0)
	fileSize := int64(0)
	if err == nil {
		modTime = info.ModTime().Unix()
		fileSize = info.Size()
	}

	baseItem := models.ModItem{
		FileName: d.Name(),
		FilePath: path,
		IsDir:    d.IsDir(),
		Title:    d.Name(),
		ModTime:  modTime,
		FileSize: fileSize,
	}

	if d.IsDir() {
		s.parseFolderMod(path, &baseItem)
	} else {
		s.parseRwmodArchive(path, &baseItem)
	}

	return baseItem
}

// parseRwmodArchive 解析 .rwmod ZIP 归档
func (s *ModService) parseRwmodArchive(path string, item *models.ModItem) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return
	}
	defer r.Close()

	var modInfoFile *zip.File
	var steamDatFile *zip.File
	var thumbFile *zip.File
	thumbNameCandidate := ""

	for _, f := range r.File {
		nameLower := strings.ToLower(f.Name)
		if strings.HasSuffix(nameLower, ".ini") && !strings.HasPrefix(filepath.Base(f.Name), "__") {
			item.UnitCount++
		} else if strings.HasSuffix(nameLower, ".tmx") {
			item.MapCount++
		}

		if strings.HasSuffix(nameLower, "mod-info.txt") && modInfoFile == nil {
			modInfoFile = f
		} else if strings.HasSuffix(nameLower, "steam.dat") && steamDatFile == nil {
			steamDatFile = f
		}
	}

	// 1. 读取 mod-info.txt
	if modInfoFile != nil {
		if rc, err := modInfoFile.Open(); err == nil {
			buf := new(bytes.Buffer)
			_, _ = io.Copy(buf, rc)
			rc.Close()
			thumbNameCandidate = s.parseModInfoText(buf.Bytes(), item)
		}
	}

	// 2. 读取 steam.dat
	if steamDatFile != nil {
		if rc, err := steamDatFile.Open(); err == nil {
			buf := new(bytes.Buffer)
			_, _ = io.Copy(buf, rc)
			rc.Close()
			item.SteamID = s.parseSteamDat(buf.Bytes())
		}
	}

	// 3. 寻找缩略图
	if thumbNameCandidate != "" {
		for _, f := range r.File {
			if strings.EqualFold(filepath.Base(f.Name), thumbNameCandidate) ||
				strings.HasSuffix(strings.ToLower(f.Name), strings.ToLower(thumbNameCandidate)) {
				thumbFile = f
				break
			}
		}
	}

	// 若未指定缩略图，尝试常见候选名字
	if thumbFile == nil {
		for _, f := range r.File {
			base := strings.ToLower(filepath.Base(f.Name))
			if base == "icon.png" || base == "logo.png" || base == "mod-thumbnail.png" || strings.HasSuffix(base, "_map.png") {
				thumbFile = f
				break
			}
		}
	}

	// 提取并编码缩略图为 Base64
	if thumbFile != nil && thumbFile.UncompressedSize64 < 5*1024*1024 {
		if rc, err := thumbFile.Open(); err == nil {
			data, err := io.ReadAll(rc)
			rc.Close()
			if err == nil && len(data) > 0 {
				mime := "image/png"
				if strings.HasSuffix(strings.ToLower(thumbFile.Name), ".jpg") || strings.HasSuffix(strings.ToLower(thumbFile.Name), ".jpeg") {
					mime = "image/jpeg"
				}
				item.Thumbnail = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
			}
		}
	}
}

// parseFolderMod 解析解压目录模组
func (s *ModService) parseFolderMod(dirPath string, item *models.ModItem) {
	_ = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			base := strings.ToLower(info.Name())
			if ext == ".ini" {
				item.UnitCount++
			} else if ext == ".tmx" {
				item.MapCount++
			} else if base == "mod-info.txt" && item.Title == item.FileName {
				if data, err := os.ReadFile(path); err == nil {
					_ = s.parseModInfoText(data, item)
				}
			} else if base == "steam.dat" && item.SteamID == "" {
				if data, err := os.ReadFile(path); err == nil {
					item.SteamID = s.parseSteamDat(data)
				}
			}
		}
		return nil
	})
}

// parseModInfoText 解析 mod-info.txt 文本，返回指定的缩略图文件名
func (s *ModService) parseModInfoText(data []byte, item *models.ModItem) string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	thumbnailName := ""
	curSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			curSection = strings.ToLower(strings.Trim(line, "[] \t"))
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			parts = strings.SplitN(line, "=", 2)
		}
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		if curSection == "mod" || curSection == "" {
			switch key {
			case "title":
				if item.Title == item.FileName {
					item.Title = val
				}
			case "title_zh":
				item.TitleZh = val
			case "description":
				item.Description = strings.ReplaceAll(val, `\n`, "\n")
			case "description_zh":
				// 若有中文描述，优先展示
				if val != "" {
					item.Description = strings.ReplaceAll(val, `\n`, "\n")
				}
			case "minversion":
				item.MinVersion = val
			case "tags":
				tagList := strings.Split(val, ",")
				for _, t := range tagList {
					cleanTag := strings.TrimSpace(t)
					if cleanTag != "" {
						item.Tags = append(item.Tags, cleanTag)
					}
				}
			case "thumbnail":
				thumbnailName = filepath.Base(val)
			}
		}
	}

	return thumbnailName
}

// parseSteamDat 从 steam.dat 提取工坊 ID
func (s *ModService) parseSteamDat(data []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "id:") {
			return strings.TrimSpace(line[3:])
		}
	}
	return ""
}
