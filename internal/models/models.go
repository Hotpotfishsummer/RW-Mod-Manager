package models

// ModItem 代表一个铁锈战争单位模组
type ModItem struct {
	ID          string   `json:"id"`          // preferences.ini 中的 SHA256 哈希值
	FileName    string   `json:"fileName"`    // 文件名 (如 "od_origin.rwmod" 或文件夹名)
	FilePath    string   `json:"filePath"`    // 绝对路径
	IsDir       bool     `json:"isDir"`       // 是否为解压文件夹
	Enabled     bool     `json:"enabled"`     // 是否启用
	Title       string   `json:"title"`       // 模组标题 (优先使用 mod-info.txt)
	TitleZh     string   `json:"titleZh"`     // 中文标题
	Description string   `json:"description"` // 模组详细描述
	MinVersion  string   `json:"minVersion"`  // 支持的铁锈最低版本
	Tags        []string `json:"tags"`        // 标签列表
	Thumbnail   string   `json:"thumbnail"`   // Base64 编码的缩略图 (如 data:image/png;base64,...)
	UnitCount   int      `json:"unitCount"`   // 包含的单位配置 (*.ini) 数量
	MapCount    int      `json:"mapCount"`    // 包含的地图 (*.tmx) 数量
	FileSize    int64    `json:"fileSize"`    // 文件大小 (字节)
	SteamID     string   `json:"steamId"`     // 关联的工坊物品 ID (来自 steam.dat)
	ModTime     int64    `json:"modTime"`     // 文件修改时间 (Unix 秒)
}

// MapItem 代表一张铁锈战争地图
type MapItem struct {
	ID               string `json:"id"`               // 唯一标识 (通常为文件名或相对路径)
	FileName         string `json:"fileName"`         // 地图文件名 (如 "1917-流血的城市.tmx")
	FilePath         string `json:"filePath"`         // 绝对路径或包路径
	IsPack           bool   `json:"isPack"`           // 是否来自 .rwmod 包
	PackName         string `json:"packName"`         // 所属包名称 (若有)
	ParentModFile    string `json:"parentModFile"`    // 所属宿主模组文件名 (用于联动启禁用)
	LocationType     string `json:"locationType"`     // "standalone" (独立地图) | "map_pack" (地图包) | "unit_mod" (模组附带战役/遭遇战地图)
	Active           bool   `json:"active"`           // 当前在游戏中是否处于可用/激活状态
	Title            string `json:"title"`            // 显示标题
	Width            int    `json:"width"`            // 地图瓦片宽度
	Height           int    `json:"height"`           // 地图瓦片高度
	PlayerCount      int    `json:"playerCount"`      // 推荐玩家数 (2P~24P)
	HasPreview       bool   `json:"hasPreview"`       // 是否存在对应 _map.png 缩略图
	PreviewData      string `json:"previewData"`      // Base64 缩略图数据 (data:image/png;base64,...)
	FileSize         int64  `json:"fileSize"`         // 文件大小 (字节)
}

// GameStatus 游戏客户端状态摘要
type GameStatus struct {
	GameDir      string `json:"gameDir"`      // 游戏根目录路径
	IsValid      bool   `json:"isValid"`      // 目录是否有效 (检测 game-lib.jar 和 preferences.ini)
	Version      string `json:"version"`      // 检测到的版本号 (如 1.15)
	TotalMods    int    `json:"totalMods"`    // 模组总数
	EnabledMods  int    `json:"enabledMods"`  // 启用的模组数
	DisabledMods int    `json:"disabledMods"` // 禁用的模组数
	TotalMaps    int    `json:"totalMaps"`    // 地图总数
	ActiveMaps   int    `json:"activeMaps"`   // 处于激活状态的地图总数
}

// LaunchConfig 游戏启动配置项
type LaunchConfig struct {
	ExecutableType string `json:"executableType"` // "exe64" | "exe32" | "java"
	MaxMemoryMB    int    `json:"maxMemoryMb"`    // JVM 堆内存大小 (-Xmx, 默认 4096)
	Fullscreen     bool   `json:"fullscreen"`     // 是否全屏 (-fullscreen)
	NoMods         bool   `json:"noMods"`         // 是否无模组纯原版启动 (-nomods)
	SafeMode       bool   `json:"safeMode"`       // 是否安全模式 (-safemode)
	ExtraArgs      string `json:"extraArgs"`      // 附加命令行参数
}

// GamePreferences 游戏内置画质与性能参数 (读取并同步至 preferences.ini)
type GamePreferences struct {
	HighRefreshRate         bool    `json:"highRefreshRate"`         // 解锁高刷新率 (144Hz/165Hz+)
	RenderVsync             bool    `json:"renderVsync"`             // 垂直同步 V-Sync
	ShowFps                 bool    `json:"showFps"`                 // 显示游戏内 FPS 计数器
	RenderSmoothDelta       bool    `json:"renderSmoothDelta"`       // 平滑帧时间差
	RenderAntiAlias         bool    `json:"renderAntiAlias"`         // 抗锯齿
	RenderFancyWater        bool    `json:"renderFancyWater"`        // 高品质动态水面
	RenderClouds            bool    `json:"renderClouds"`            // 云层阴影
	RenderExtraShadows      bool    `json:"renderExtraShadows"`      // 额外高精度阴影
	TeamShaders             bool    `json:"teamShaders"`             // 着色器队伍特效
	NewRender               bool    `json:"newRender"`               // 实验性新渲染管线
	TeamUnitCapSinglePlayer int     `json:"teamUnitCapSinglePlayer"` // 单人遭遇战队伍单位上限 (默认 5000)
	TeamUnitCapHostedGame   int     `json:"teamUnitCapHostedGame"`   // 多人联机主机队伍单位上限 (默认 2000)
	UiRenderScale           float64 `json:"uiRenderScale"`           // UI 界面缩放倍率 (默认 1.0)
}

// AppConfig 管理器自身的全局配置
type AppConfig struct {
	GameDir         string          `json:"gameDir"`
	LaunchConfig    LaunchConfig    `json:"launchConfig"`
	GamePreferences GamePreferences `json:"gamePreferences"`
	LanConfig       LanConfig       `json:"lanConfig"`
}
