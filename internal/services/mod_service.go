package services

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/flate"
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
	thumbNameCandidate := ""

	for _, f := range r.File {
		clean := strings.Trim(strings.ReplaceAll(f.Name, "\\", "/"), "/")
		nameLower := strings.ToLower(clean)
		if strings.HasSuffix(nameLower, ".ini") && !strings.HasPrefix(filepath.Base(clean), "__") {
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
		if data, err := s.readZipEntryBytes(modInfoFile); err == nil {
			thumbNameCandidate = s.parseModInfoText(data, item)
		}
	}

	// 2. 读取 steam.dat
	if steamDatFile != nil {
		if data, err := s.readZipEntryBytes(steamDatFile); err == nil {
			item.SteamID = s.parseSteamDat(data)
		}
	}

	// 3. 严格校验是否存在有效可读的封面文件 (非目录项且大小大于 0)
	if tf := s.findModThumbnailInZip(&r.Reader, thumbNameCandidate); tf != nil {
		item.Thumbnail = fmt.Sprintf("/api/thumbnail/mod?name=%s&t=%d", url.QueryEscape(item.FileName), item.ModTime)
	} else {
		item.Thumbnail = ""
	}
}

// parseFolderMod 解析解压目录模组
func (s *ModService) parseFolderMod(dirPath string, item *models.ModItem) {
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
			}
		}
		return nil
	})

	if thumbPath := s.findModThumbnailInFolder(dirPath, thumbCandidateName); thumbPath != "" {
		item.Thumbnail = fmt.Sprintf("/api/thumbnail/mod?name=%s&t=%d", url.QueryEscape(item.FileName), item.ModTime)
	} else {
		item.Thumbnail = ""
	}
}

// readZipEntryBytes 安全读取 zip 条目字节数据，具备对末尾带斜杠或特定打包格式条目的 OpenRaw 容错回退
func (s *ModService) readZipEntryBytes(f *zip.File) ([]byte, error) {
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

	// 2. 若标准 Open 报错（如部分模组作者打包导致文件名末尾带 / 被识别为 dirReader 产生 ErrFormat）
	// 通过 OpenRaw 读取底层压缩数据流并手动解压
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

// findModThumbnailInZip 在 zip 归档中定位真实有效的封面文件，支持 mod-info.txt 自定义名称、常见标准命名及地图预览图兜底
func (s *ModService) findModThumbnailInZip(r *zip.Reader, thumbCandidate string) *zip.File {
	isCandidateFile := func(f *zip.File) bool {
		if f.UncompressedSize64 == 0 || f.UncompressedSize64 > 15*1024*1024 {
			return false
		}
		data, err := s.readZipEntryBytes(f)
		if err != nil || len(data) < 4 {
			return false
		}
		// 校验图片头部 (PNG, JPEG, GIF, BMP, WebP)
		return (len(data) >= 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G') ||
			(len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF) ||
			(len(data) >= 6 && (string(data[:6]) == "GIF89a" || string(data[:6]) == "GIF87a")) ||
			(len(data) >= 2 && string(data[:2]) == "BM") ||
			(len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP")
	}

	// 1. 若 mod-info.txt 指定了封面图，具有最高优先级
	if thumbCandidate != "" {
		candBase := strings.ToLower(filepath.Base(strings.Trim(strings.ReplaceAll(thumbCandidate, "\\", "/"), "/")))
		for _, f := range r.File {
			clean := strings.Trim(strings.ReplaceAll(f.Name, "\\", "/"), "/")
			base := strings.ToLower(filepath.Base(clean))
			if base == candBase || strings.HasSuffix(strings.ToLower(clean), strings.ToLower(thumbCandidate)) {
				if isCandidateFile(f) {
					return f
				}
			}
		}
	}

	// 2. 依次匹配模组标准封面图候选名字
	candidates := []string{
		"thumbnail.png", "thumbnail.jpg", "thumbnail.jpeg",
		"mod-thumbnail.png", "mod-thumbnail.jpg", "mod-thumbnail2.png",
		"icon.png", "icon.jpg", "icon.jpeg",
		"logo.png", "logo.jpg", "logo.jpeg",
		"cover.png", "cover.jpg",
	}

	for _, cand := range candidates {
		for _, f := range r.File {
			clean := strings.Trim(strings.ReplaceAll(f.Name, "\\", "/"), "/")
			base := strings.ToLower(filepath.Base(clean))
			if base == cand {
				if isCandidateFile(f) {
					return f
				}
			}
		}
	}

	// 3. 回退：模组未设置专属封面时，允许匹配模组内嵌自定义地图的预览图（_map.png）
	for _, f := range r.File {
		clean := strings.Trim(strings.ReplaceAll(f.Name, "\\", "/"), "/")
		base := strings.ToLower(filepath.Base(clean))
		if strings.HasSuffix(base, "_map.png") {
			if isCandidateFile(f) {
				return f
			}
		}
	}

	return nil
}

// findModThumbnailInFolder 在解压文件夹模组中寻找真实存在的封面文件
func (s *ModService) findModThumbnailInFolder(dirPath, thumbCandidate string) string {
	if thumbCandidate != "" {
		candPath := filepath.Join(dirPath, thumbCandidate)
		if fi, err := os.Stat(candPath); err == nil && !fi.IsDir() && fi.Size() > 0 {
			return candPath
		}
	}

	candidates := []string{
		"thumbnail.png", "thumbnail.jpg", "thumbnail.jpeg",
		"mod-thumbnail.png", "mod-thumbnail.jpg",
		"icon.png", "icon.jpg",
		"logo.png", "logo.jpg",
		"cover.png", "cover.jpg",
	}

	for _, cand := range candidates {
		candPath := filepath.Join(dirPath, cand)
		if fi, err := os.Stat(candPath); err == nil && !fi.IsDir() && fi.Size() > 0 {
			return candPath
		}
	}

	// 模组无专属封面时，检查是否有自定义地图预览图作为回退
	var mapFallback string
	_ = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), "_map.png") {
			mapFallback = path
			return filepath.SkipAll
		}
		return nil
	})

	return mapFallback
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
		// 寻找文件夹模组中的封面
		thumbCandidateName := ""
		if modInfoData, err := os.ReadFile(filepath.Join(modPath, "mod-info.txt")); err == nil {
			var dummy models.ModItem
			thumbCandidateName = s.parseModInfoText(modInfoData, &dummy)
		}
		foundPath := s.findModThumbnailInFolder(modPath, thumbCandidateName)
		if foundPath != "" {
			if data, err := os.ReadFile(foundPath); err == nil && len(data) > 0 {
				imgBytes = data
				lower := strings.ToLower(foundPath)
				if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
					isJpeg = true
				}
			}
		}
	} else {
		// .rwmod ZIP 归档
		if zr, err := zip.OpenReader(modPath); err == nil {
			defer zr.Close()

			// 1. 读取 mod-info.txt 获取候选封面
			thumbCandidateName := ""
			for _, f := range zr.File {
				clean := strings.Trim(strings.ReplaceAll(f.Name, "\\", "/"), "/")
				if strings.HasSuffix(strings.ToLower(clean), "mod-info.txt") {
					if data, err := s.readZipEntryBytes(f); err == nil {
						var dummy models.ModItem
						thumbCandidateName = s.parseModInfoText(data, &dummy)
					}
					break
				}
			}

			// 2. 准确定位封面文件条目并提取
			if tf := s.findModThumbnailInZip(&zr.Reader, thumbCandidateName); tf != nil {
				if data, err := s.readZipEntryBytes(tf); err == nil && len(data) > 0 {
					imgBytes = data
					detectedMime := http.DetectContentType(data)
					if strings.HasPrefix(detectedMime, "image/jpeg") {
						isJpeg = true
					}
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
