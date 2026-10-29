package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"rw_mod_manager/internal/models"
)

// UpdateService 统一处理局域网内 P2P 更新检测、分发与 GitHub Releases 线上更新
type UpdateService struct {
	gameService *GameService
	lanService  *LanService
	wailsCtx    context.Context

	mu             sync.Mutex
	isUpdating     bool
	lastCheckInfo  *models.UpdateInfo
}

// NewUpdateService 构造函数
func NewUpdateService(gameSvc *GameService, lanSvc *LanService) *UpdateService {
	s := &UpdateService{
		gameService: gameSvc,
		lanService:  lanSvc,
	}
	s.CleanupOldVersions()
	return s
}

// SetWailsContext 注入 Wails 运行时上下文
func (s *UpdateService) SetWailsContext(ctx context.Context) {
	s.wailsCtx = ctx
}

// GetCurrentVersion 获取当前运行的应用版本号
func (s *UpdateService) GetCurrentVersion() string {
	return models.AppVersion
}

// CleanupOldVersions 清理上一次热替换遗留的 .old / .new / .downloading 临时文件
func (s *UpdateService) CleanupOldVersions() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exePath)
	base := filepath.Base(exePath)

	_ = os.Remove(filepath.Join(dir, base+".old"))
	_ = os.Remove(filepath.Join(dir, base+".new"))
	_ = os.Remove(filepath.Join(dir, base+".downloading"))
}

// CheckForUpdates 统一更新检测：同时探测局域网节点与 GitHub Releases
func (s *UpdateService) CheckForUpdates() (*models.UpdateInfo, error) {
	result := &models.UpdateInfo{
		CurrentVersion: models.AppVersion,
		LatestVersion:  models.AppVersion,
		HasUpdate:      false,
	}

	var lanInfo *models.UpdateInfo
	var ghInfo *models.UpdateInfo

	var wg sync.WaitGroup
	wg.Add(2)

	// 1. 局域网源检测
	go func() {
		defer wg.Done()
		info, err := s.CheckLanUpdate()
		if err == nil && info != nil && info.HasUpdate {
			lanInfo = info
		}
	}()

	// 2. GitHub Releases 源检测
	go func() {
		defer wg.Done()
		info, err := s.CheckGithubUpdate()
		if err == nil && info != nil && info.HasUpdate {
			ghInfo = info
		}
	}()

	wg.Wait()

	// 整理检测结果
	if lanInfo != nil {
		result.LanAvailable = true
		result.LanVersion = lanInfo.LatestVersion
	}
	if ghInfo != nil {
		result.GithubAvailable = true
		result.GithubVersion = ghInfo.LatestVersion
	}

	// 智能选择推荐更新源：局域网优先或版本更高者
	if lanInfo != nil && ghInfo != nil {
		cmp := CompareVersions(lanInfo.LatestVersion, ghInfo.LatestVersion)
		if cmp >= 0 {
			// 局域网版本更高或相同，推荐局域网 (速度极快且免流量)
			result.HasUpdate = true
			result.LatestVersion = lanInfo.LatestVersion
			result.Source = models.UpdateSourceLan
			result.ProviderName = lanInfo.ProviderName
			result.DownloadURL = lanInfo.DownloadURL
			result.ReleaseNotes = lanInfo.ReleaseNotes
			result.FileSize = lanInfo.FileSize
			result.PublishedAt = lanInfo.PublishedAt
		} else {
			// GitHub 版本更高
			result.HasUpdate = true
			result.LatestVersion = ghInfo.LatestVersion
			result.Source = models.UpdateSourceGithub
			result.ProviderName = ghInfo.ProviderName
			result.DownloadURL = ghInfo.DownloadURL
			result.ReleaseNotes = ghInfo.ReleaseNotes
			result.FileSize = ghInfo.FileSize
			result.PublishedAt = ghInfo.PublishedAt
		}
	} else if lanInfo != nil {
		result.HasUpdate = true
		result.LatestVersion = lanInfo.LatestVersion
		result.Source = models.UpdateSourceLan
		result.ProviderName = lanInfo.ProviderName
		result.DownloadURL = lanInfo.DownloadURL
		result.ReleaseNotes = lanInfo.ReleaseNotes
		result.FileSize = lanInfo.FileSize
		result.PublishedAt = lanInfo.PublishedAt
	} else if ghInfo != nil {
		result.HasUpdate = true
		result.LatestVersion = ghInfo.LatestVersion
		result.Source = models.UpdateSourceGithub
		result.ProviderName = ghInfo.ProviderName
		result.DownloadURL = ghInfo.DownloadURL
		result.ReleaseNotes = ghInfo.ReleaseNotes
		result.FileSize = ghInfo.FileSize
		result.PublishedAt = ghInfo.PublishedAt
	}

	s.mu.Lock()
	s.lastCheckInfo = result
	s.mu.Unlock()

	return result, nil
}

// CheckLanUpdate 探测局域网内是否存在更高版本的在线节点
func (s *UpdateService) CheckLanUpdate() (*models.UpdateInfo, error) {
	if s.lanService == nil {
		return nil, fmt.Errorf("局域网服务未初始化")
	}

	peers := s.lanService.GetDiscoveredPeers()
	if len(peers) == 0 {
		return nil, nil
	}

	// 寻找局域网内版本最高的节点 (优先房主)
	var bestPeer *models.LanPeer
	highestVer := models.AppVersion

	for _, p := range peers {
		if p.AppVersion == "" {
			continue
		}
		if CompareVersions(p.AppVersion, highestVer) > 0 {
			highestVer = p.AppVersion
			pCopy := p
			bestPeer = &pCopy
		} else if CompareVersions(p.AppVersion, highestVer) == 0 && p.Role == models.RoleServer {
			pCopy := p
			bestPeer = &pCopy
		}
	}

	if bestPeer == nil {
		return nil, nil
	}

	// 向该节点查询完整的更新元数据
	client := &http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("http://%s:%d/api/lan/update/info", bestPeer.IP, bestPeer.HttpPort)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("节点返回状态码异常: %d", resp.StatusCode)
	}

	var meta models.LanUpdateMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, err
	}

	downloadURL := fmt.Sprintf("http://%s:%d/api/lan/update/download", bestPeer.IP, bestPeer.HttpPort)
	providerName := bestPeer.DeviceName
	if bestPeer.Role == models.RoleServer {
		providerName = fmt.Sprintf("房主 (%s)", bestPeer.DeviceName)
	} else {
		providerName = fmt.Sprintf("队友 (%s)", bestPeer.DeviceName)
	}

	return &models.UpdateInfo{
		HasUpdate:      true,
		CurrentVersion: models.AppVersion,
		LatestVersion:  meta.Version,
		Source:         models.UpdateSourceLan,
		ProviderName:   providerName,
		DownloadURL:    downloadURL,
		ReleaseNotes:   meta.ReleaseNotes,
		FileSize:       meta.FileSize,
		Sha256:         meta.Sha256,
		PublishedAt:    "局域网实时共享",
		LanAvailable:   true,
		LanVersion:     meta.Version,
	}, nil
}

// GitHubReleaseResponse GitHub API 返回结构
type GitHubReleaseResponse struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	HtmlURL     string `json:"html_url"`
	Assets      []struct {
		Name               string `json:"name"`
		Size               int64  `json:"size"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckGithubUpdate 访问 GitHub Releases API 检查官方最新版本
func (s *UpdateService) CheckGithubUpdate() (*models.UpdateInfo, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", models.GithubOwnerAndRepo)
	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "RW-Mod-Manager-Updater/"+models.AppVersion)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("暂无 GitHub Release 发布版本")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 接口返回异常状态码: %d", resp.StatusCode)
	}

	var ghRel GitHubReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&ghRel); err != nil {
		return nil, err
	}

	cleanTag := strings.TrimPrefix(strings.TrimSpace(ghRel.TagName), "v")
	if CompareVersions(cleanTag, models.AppVersion) <= 0 {
		// 没有比当前版本更高
		return nil, nil
	}

	// 匹配 .exe 资产文件
	var downloadURL string
	var fileSize int64
	for _, a := range ghRel.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".exe") {
			downloadURL = a.BrowserDownloadURL
			fileSize = a.Size
			break
		}
	}

	if downloadURL == "" && len(ghRel.Assets) > 0 {
		downloadURL = ghRel.Assets[0].BrowserDownloadURL
		fileSize = ghRel.Assets[0].Size
	}

	return &models.UpdateInfo{
		HasUpdate:       true,
		CurrentVersion:  models.AppVersion,
		LatestVersion:   cleanTag,
		Source:          models.UpdateSourceGithub,
		ProviderName:    "GitHub Releases 官方发布",
		DownloadURL:     downloadURL,
		ReleaseNotes:    ghRel.Body,
		FileSize:        fileSize,
		PublishedAt:     ghRel.PublishedAt,
		GithubAvailable: true,
		GithubVersion:   cleanTag,
	}, nil
}

// DownloadAndApplyUpdate 下载更新并执行 Windows 在位自替换与重启
func (s *UpdateService) DownloadAndApplyUpdate(source models.UpdateSource) error {
	s.mu.Lock()
	if s.isUpdating {
		s.mu.Unlock()
		return fmt.Errorf("更新任务正在进行中，请勿重复操作")
	}
	s.isUpdating = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isUpdating = false
		s.mu.Unlock()
	}()

	s.emitProgress(models.UpdateProgress{
		Status:     "checking",
		Percentage: 0,
		Message:    "准备下载更新包...",
	})

	// 获取下载 URL
	var downloadURL string
	var expectedSize int64

	s.mu.Lock()
	if s.lastCheckInfo != nil {
		if source == models.UpdateSourceLan && s.lastCheckInfo.LanAvailable {
			downloadURL = s.lastCheckInfo.DownloadURL
			expectedSize = s.lastCheckInfo.FileSize
		} else if source == models.UpdateSourceGithub && s.lastCheckInfo.GithubAvailable {
			downloadURL = s.lastCheckInfo.DownloadURL
			expectedSize = s.lastCheckInfo.FileSize
		} else {
			downloadURL = s.lastCheckInfo.DownloadURL
			expectedSize = s.lastCheckInfo.FileSize
		}
	}
	s.mu.Unlock()

	if downloadURL == "" {
		info, err := s.CheckForUpdates()
		if err != nil || info == nil || !info.HasUpdate {
			return fmt.Errorf("未能获取到有效的更新下载地址")
		}
		downloadURL = info.DownloadURL
		expectedSize = info.FileSize
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取应用运行路径失败: %w", err)
	}

	downloadPath := exePath + ".downloading"
	newExePath := exePath + ".new"
	oldExePath := exePath + ".old"

	_ = os.Remove(downloadPath)
	_ = os.Remove(newExePath)

	// 开始下载文件流
	req, err := http.NewRequestWithContext(context.Background(), "GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "RW-Mod-Manager-Updater/"+models.AppVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.emitProgress(models.UpdateProgress{
			Status:  "error",
			Message: "下载失败: " + err.Error(),
			Error:   err.Error(),
		})
		return fmt.Errorf("连接更新服务器失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("服务器返回错误状态: %d", resp.StatusCode)
		s.emitProgress(models.UpdateProgress{
			Status:  "error",
			Message: err.Error(),
			Error:   err.Error(),
		})
		return err
	}

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		totalSize = expectedSize
	}

	out, err := os.OpenFile(downloadPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("创建临时更新文件失败: %w", err)
	}

	buf := make([]byte, 64*1024)
	var downloaded int64
	lastReport := time.Now()
	var lastBytes int64

	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			_, wErr := out.Write(buf[:n])
			if wErr != nil {
				out.Close()
				return fmt.Errorf("写入更新文件失败: %w", wErr)
			}
			downloaded += int64(n)

			if time.Since(lastReport) > 200*time.Millisecond {
				now := time.Now()
				durationSec := now.Sub(lastReport).Seconds()
				var speed int64
				if durationSec > 0 {
					speed = int64(float64(downloaded-lastBytes) / durationSec)
				}
				lastReport = now
				lastBytes = downloaded

				percentage := 0.0
				if totalSize > 0 {
					percentage = float64(downloaded) / float64(totalSize) * 100.0
					if percentage > 100.0 {
						percentage = 100.0
					}
				}

				s.emitProgress(models.UpdateProgress{
					Status:     "downloading",
					Percentage: percentage,
					Downloaded: downloaded,
					Total:      totalSize,
					SpeedBytes: speed,
					Message:    fmt.Sprintf("正在下载: %.1f%% (%s / %s)", percentage, formatBytes(downloaded), formatBytes(totalSize)),
				})
			}
		}

		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			out.Close()
			return fmt.Errorf("下载流读取中断: %w", rErr)
		}
	}
	out.Close()

	// 重命名为 .new 文件
	if err := os.Rename(downloadPath, newExePath); err != nil {
		return fmt.Errorf("准备新版本可执行文件失败: %w", err)
	}

	s.emitProgress(models.UpdateProgress{
		Status:     "installing",
		Percentage: 100.0,
		Message:    "下载完成！正在执行安全热替换并重新拉起应用...",
	})

	// 执行 Windows 在位热替换
	// Windows 允许重命名正在运行的 .exe (不可写不可删，但可以 Rename 到同分区路径)
	_ = os.Remove(oldExePath)
	renameErr := os.Rename(exePath, oldExePath)
	if renameErr == nil {
		if err := os.Rename(newExePath, exePath); err == nil {
			// 热替换直接成功！启动新进程并退出
			time.Sleep(500 * time.Millisecond)
			cmd := exec.Command(exePath)
			_ = cmd.Start()
			os.Exit(0)
			return nil
		}
		// 恢复原文件名
		_ = os.Rename(oldExePath, exePath)
	}

	// 回退方案：通过后台 detached cmd 脚本延迟 1 秒覆盖并启动
	batchScript := fmt.Sprintf(
		`@echo off
ping 127.0.0.1 -n 2 >nul
del "%s" >nul 2>&1
move /y "%s" "%s" >nul 2>&1
start "" "%s"
exit
`,
		oldExePath, newExePath, exePath, exePath,
	)

	batPath := filepath.Join(os.TempDir(), "rw_update_restart.bat")
	if err := os.WriteFile(batPath, []byte(batchScript), 0755); err == nil {
		cmd := exec.Command("cmd.exe", "/c", batPath)
		_ = cmd.Start()
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
		return nil
	}

	return fmt.Errorf("热替换失败: %v", renameErr)
}

func (s *UpdateService) emitProgress(p models.UpdateProgress) {
	if s.wailsCtx != nil {
		wailsRuntime.EventsEmit(s.wailsCtx, "update:progress", p)
	}
}

// CompareVersions 比较两个版本号字符串 (例如 "1.0.1" 与 "1.0.0", "v1.2" 与 "1.1.9")
// 返回值: 1 (v1 > v2), -1 (v1 < v2), 0 (v1 == v2)
func CompareVersions(v1, v2 string) int {
	clean1 := strings.TrimPrefix(strings.TrimSpace(v1), "v")
	clean1 = strings.TrimPrefix(clean1, "V")
	clean2 := strings.TrimPrefix(strings.TrimSpace(v2), "v")
	clean2 = strings.TrimPrefix(clean2, "V")

	// 去除 pre-release 标签对比主数字
	parts1 := strings.Split(strings.Split(clean1, "-")[0], ".")
	parts2 := strings.Split(strings.Split(clean2, "-")[0], ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		num1 := 0
		if i < len(parts1) {
			num1, _ = strconv.Atoi(parts1[i])
		}
		num2 := 0
		if i < len(parts2) {
			num2, _ = strconv.Atoi(parts2[i])
		}

		if num1 > num2 {
			return 1
		}
		if num1 < num2 {
			return -1
		}
	}

	return 0
}

func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB"}
	i := 0
	val := float64(bytes)
	for val >= 1024 && i < len(sizes)-1 {
		val /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", val, sizes[i])
}
