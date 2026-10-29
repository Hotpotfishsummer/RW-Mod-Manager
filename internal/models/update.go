package models

// AppVersion 当前应用版本号
const AppVersion = "1.0.0"

// GithubOwnerAndRepo 官方 GitHub 仓库标识
const GithubOwnerAndRepo = "Hotpotfishsummer/RW-Mod-Manager"

// UpdateSource 更新来源类型
type UpdateSource string

const (
	UpdateSourceLan    UpdateSource = "lan"
	UpdateSourceGithub UpdateSource = "github"
)

// UpdateInfo 更新检测结果
type UpdateInfo struct {
	HasUpdate       bool         `json:"hasUpdate"`
	CurrentVersion  string       `json:"currentVersion"`
	LatestVersion   string       `json:"latestVersion"`
	Source          UpdateSource `json:"source"`          // "lan" | "github"
	ProviderName    string       `json:"providerName"`    // 来源名称，例如 "房主 (Player-PC)" 或 "GitHub Releases"
	DownloadURL     string       `json:"downloadUrl"`     // 下载地址
	ReleaseNotes    string       `json:"releaseNotes"`    // 更新日志
	FileSize        int64        `json:"fileSize"`        // 文件大小 (字节)
	Sha256          string       `json:"sha256"`          // 校验和 (可选)
	PublishedAt     string       `json:"publishedAt"`     // 发布时间
	LanAvailable    bool         `json:"lanAvailable"`    // 局域网源是否有新版本
	LanVersion      string       `json:"lanVersion"`      // 局域网源版本
	GithubAvailable bool         `json:"githubAvailable"` // GitHub 源是否有新版本
	GithubVersion   string       `json:"githubVersion"`   // GitHub 源版本
}

// UpdateProgress 更新下载与安装进度汇报
type UpdateProgress struct {
	Status     string  `json:"status"`     // "checking" | "downloading" | "installing" | "ready" | "error"
	Percentage float64 `json:"percentage"` // 0.0 - 100.0
	Downloaded int64   `json:"downloaded"` // 已下载字节数
	Total      int64   `json:"total"`      // 总字节数
	SpeedBytes int64   `json:"speedBytes"` // 下载速度 (字节/秒)
	Message    string  `json:"message"`    // 状态提示文字
	Error      string  `json:"error,omitempty"`
}

// LanUpdateMeta 局域网节点通过 HTTP 提供的版本元数据
type LanUpdateMeta struct {
	Version      string `json:"version"`
	FileSize     int64  `json:"fileSize"`
	Sha256       string `json:"sha256"`
	ProviderName string `json:"providerName"`
	ReleaseNotes string `json:"releaseNotes"`
}
