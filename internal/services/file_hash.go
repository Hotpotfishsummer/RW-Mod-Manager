package services

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type hashCacheItem struct {
	modTime  int64
	fileSize int64
	hash     string
}

var (
	contentHashCache sync.Map // map[string]hashCacheItem (key: absolutePath)
)

// CalculateContentHash 计算单文件或解包目录的真实内容 MD5 哈希
// 带有 (modTime + fileSize) 内存缓存，避免频繁全量读取文件带来 I/O 负担
func CalculateContentHash(path string, isDir bool) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}

	mtime := fi.ModTime().UnixNano()
	size := fi.Size()

	if val, ok := contentHashCache.Load(path); ok {
		cached := val.(hashCacheItem)
		if cached.modTime == mtime && cached.fileSize == size && cached.hash != "" {
			return cached.hash
		}
	}

	var hashStr string
	if isDir {
		hashStr = calculateDirContentHash(path)
	} else {
		hashStr = calculateSingleFileHash(path)
	}

	if hashStr != "" {
		contentHashCache.Store(path, hashCacheItem{
			modTime:  mtime,
			fileSize: size,
			hash:     hashStr,
		})
	}

	return hashStr
}

func calculateSingleFileHash(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func calculateDirContentHash(dirPath string) string {
	var fileList []string
	_ = filepath.Walk(dirPath, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dirPath, p)
		fileHash := calculateSingleFileHash(p)
		fileList = append(fileList, fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), info.Size(), fileHash))
		return nil
	})

	if len(fileList) == 0 {
		return ""
	}

	sort.Strings(fileList)
	h := md5.New()
	for _, item := range fileList {
		h.Write([]byte(item + ";"))
	}
	return hex.EncodeToString(h.Sum(nil))
}
