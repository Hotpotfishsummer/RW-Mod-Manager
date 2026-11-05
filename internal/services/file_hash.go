package services

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// CalculateContentHash 计算单文件或解包目录的真实内容 MD5 哈希
// 带有磁盘持久化与内存 (modTime + fileSize) 双重缓存，彻底消除启动与同步时的全盘重复 I/O
func CalculateContentHash(path string, isDir bool) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}

	mtime := fi.ModTime().UnixNano()
	size := fi.Size()

	cm := GetCacheManager()
	if cachedHash, hit := cm.GetHash(path, mtime, size); hit {
		return cachedHash
	}

	var hashStr string
	if isDir {
		hashStr = calculateDirContentHash(path)
	} else {
		hashStr = calculateSingleFileHash(path)
	}

	if hashStr != "" {
		cm.SetHash(path, mtime, size, hashStr)
		// 异步或按需保存
		cm.SaveHashCache()
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
