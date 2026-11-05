package services

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"rw_mod_manager/internal/models"
)

// ModService 负责模组的并发扫描、元数据解析与封面按需流式提取
type ModService struct {
	prefService *PrefService
}

func NewModService(pref *PrefService) *ModService {
	return &ModService{
		prefService: pref,
	}
}

// ScanMods 并发扫描指定游戏目录下的所有单位模组 (带有磁盘持久化元数据缓存与零内存膨胀流式封面)
func (s *ModService) ScanMods(gameDir string) ([]models.ModItem, error) {
	unitsDir := filepath.Join(gameDir, "mods", "units")
	entries, err := os.ReadDir(unitsDir)
	if err != nil {
		return nil, err
	}

	// 先读取当前 preferences.ini 中记录的状态映射
	statusMap, _ := s.prefService.ReadModSettings(gameDir)

	var targets []os.DirEntry
	existingNames := make(map[string]bool)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.EqualFold(name, "custom_units_here.txt") {
			continue
		}
		if entry.IsDir() || strings.HasSuffix(strings.ToLower(name), ".rwmod") {
			targets = append(targets, entry)
			existingNames[name] = true
		}
	}

	cm := GetCacheManager()
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

			info, err := d.Info()
			modTime := int64(0)
			fileSize := int64(0)
			if err == nil {
				modTime = info.ModTime().UnixNano()
				fileSize = info.Size()
			}

			name := d.Name()
			fullPath := filepath.Join(unitsDir, name)

			// 1. 尝试从持久化元数据缓存命中
			if cached, hit := cm.GetMod(name, modTime, fileSize); hit {
				cached.FilePath = fullPath
				cached.ID = s.prefService.CalculateModHash(name)
				if enabled, ok := statusMap[cached.ID]; ok {
					cached.Enabled = enabled
				} else {
					cached.Enabled = false
				}
				results[idx] = cached
				return
			}

			// 2. 缓存未命中时解析单个模组
			item := s.parseMod(fullPath, d)
			item.ID = s.prefService.CalculateModHash(item.FileName)
			if enabled, ok := statusMap[item.ID]; ok {
				item.Enabled = enabled
			} else {
				item.Enabled = false
			}

			cm.SetMod(name, modTime, fileSize, item)
			results[idx] = item
		}(i, entry)
	}

	wg.Wait()

	// 清理多余缓存并写盘
	cm.PruneMods(existingNames)
	cm.SaveModCache()

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

// parseRwmodArchive 解析 .rwmod ZIP 归档，不再将多兆字节的大图直接 Base64 塞入 JSON，改为按需 HTTP 路由
func (s *ModService) parseRwmodArchive(path string, item *models.ModItem) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return
	}
	defer r.Close()

	var modInfoFile *zip.File
	var steamDatFile *zip.File
	hasThumbnailCandidate := false
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

	// 3. 寻找是否存在缩略图文件
	if thumbNameCandidate != "" {
		for _, f := range r.File {
			if strings.EqualFold(filepath.Base(f.Name), thumbNameCandidate) ||
				strings.HasSuffix(strings.ToLower(f.Name), strings.ToLower(thumbNameCandidate)) {
				hasThumbnailCandidate = true
				break
			}
		}
	}

	if !hasThumbnailCandidate {
		for _, f := range r.File {
			base := strings.ToLower(filepath.Base(f.Name))
			if base == "icon.png" || base == "logo.png" || base == "mod-thumbnail.png" || strings.HasSuffix(base, "_map.png") {
				hasThumbnailCandidate = true
				break
			}
		}
	}

	// 若存在封面，生成高效轻量的按需 API URL，彻底避免向前端发送数十兆 Base64 导致内存翻倍与启动卡顿
	if hasThumbnailCandidate {
		item.Thumbnail = fmt.Sprintf("/api/thumbnail/mod?name=%s&t=%d", url.QueryEscape(item.FileName), item.ModTime)
	}
}

// parseFolderMod 解析解压目录模组
func (s *ModService) parseFolderMod(dirPath string, item *models.ModItem) {
	hasThumb := false
	thumbCandidateName := ""

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
					thumbCandidateName = s.parseModInfoText(data, item)
				}
			} else if base == "steam.dat" && item.SteamID == "" {
				if data, err := os.ReadFile(path); err == nil {
					item.SteamID = s.parseSteamDat(data)
				}
			} else if base == "icon.png" || base == "logo.png" || base == "mod-thumbnail.png" {
				hasThumb = true
			}
		}
		return nil
	})

	if thumbCandidateName != "" {
		if _, err := os.Stat(filepath.Join(dirPath, thumbCandidateName)); err == nil {
			hasThumb = true
		}
	}

	if hasThumb {
		item.Thumbnail = fmt.Sprintf("/api/thumbnail/mod?name=%s&t=%d", url.QueryEscape(item.FileName), item.ModTime)
	}
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

// ServeModThumbnail 流式提供单个模组的封面图片，带有浏览器强缓存与磁盘抽离缓存，避免 Go 内存常驻
func (s *ModService) ServeModThumbnail(w http.ResponseWriter, r *http.Request, gameDir, modName string) {
	cleanName := filepath.Base(modName)
	if cleanName == "" || cleanName == "." {
		http.NotFound(w, r)
		return
	}

	unitsDir := filepath.Join(gameDir, "mods", "units")
	modPath := filepath.Join(unitsDir, cleanName)
	fi, err := os.Stat(modPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	cm := GetCacheManager()
	cacheHash := fmt.Sprintf("%x", md5.Sum([]byte(cleanName+fmt.Sprintf(":%d:%d", fi.ModTime().UnixNano(), fi.Size()))))
	cachedThumbPath := filepath.Join(cm.ThumbnailDir(), "mod_"+cacheHash+".png")

	// 1. 若已有提取磁盘缓存，直接通过 http.ServeFile 零拷贝流式发送并支持 304 缓存
	if cfi, err := os.Stat(cachedThumbPath); err == nil && !cfi.IsDir() {
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		http.ServeFile(w, r, cachedThumbPath)
		return
	}

	// 2. 提取封面并落盘
	var imgBytes []byte
	var isJpeg bool

	if fi.IsDir() {
		// 文件夹模组：寻找 icon.png / logo.png / mod-thumbnail.png
		candidates := []string{"icon.png", "logo.png", "mod-thumbnail.png", "icon.jpg", "logo.jpg"}
		for _, cand := range candidates {
			candPath := filepath.Join(modPath, cand)
			if data, err := os.ReadFile(candPath); err == nil && len(data) > 0 {
				imgBytes = data
				if strings.HasSuffix(cand, ".jpg") {
					isJpeg = true
				}
				break
			}
		}
	} else {
		// .rwmod ZIP 归档
		if zr, err := zip.OpenReader(modPath); err == nil {
			defer zr.Close()

			var targetFile *zip.File
			for _, f := range zr.File {
				b := strings.ToLower(filepath.Base(f.Name))
				if b == "icon.png" || b == "logo.png" || b == "mod-thumbnail.png" || b == "icon.jpg" || b == "logo.jpg" || strings.HasSuffix(b, "_map.png") {
					targetFile = f
					break
				}
			}

			if targetFile != nil && targetFile.UncompressedSize64 < 10*1024*1024 {
				if rc, err := targetFile.Open(); err == nil {
					if data, err := io.ReadAll(rc); err == nil {
						imgBytes = data
						if strings.HasSuffix(strings.ToLower(targetFile.Name), ".jpg") || strings.HasSuffix(strings.ToLower(targetFile.Name), ".jpeg") {
							isJpeg = true
						}
					}
					rc.Close()
				}
			}
		}
	}

	if len(imgBytes) == 0 {
		http.NotFound(w, r)
		return
	}

	// 写入本地磁盘缓存
	_ = os.WriteFile(cachedThumbPath, imgBytes, 0644)

	mime := "image/png"
	if isJpeg {
		mime = "image/jpeg"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(imgBytes)
}
