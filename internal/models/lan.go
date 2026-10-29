package models

// LanRole 局域网角色：默认 client，可切换为 server
type LanRole string

const (
	RoleClient LanRole = "client"
	RoleServer LanRole = "server"
)

// NetworkInterfaceInfo 本地网络适配器与子网信息
type NetworkInterfaceInfo struct {
	Name        string `json:"name"`        // 网卡名称，例如 "以太网" / "Wi-Fi"
	IP          string `json:"ip"`          // 本机 IPv4，例如 "192.168.1.100"
	Mask        string `json:"mask"`        // 子网掩码，例如 "255.255.255.0"
	Subnet      string `json:"subnet"`      // 子网 CIDR，例如 "192.168.1.0/24"
	BroadcastIP string `json:"broadcastIp"` // 局域网广播地址，例如 "192.168.1.255"
	IsDefault   bool   `json:"isDefault"`   // 是否为推荐/默认主网卡
}

// LanPeer 局域网内探测发现的同伴节点
type LanPeer struct {
	InstanceID    string  `json:"instanceId"`    // 唯一实例 ID
	DeviceName    string  `json:"deviceName"`    // 设备主机名 / 显示名
	IP            string  `json:"ip"`            // 节点 IP 地址
	HttpPort      int     `json:"httpPort"`      // 节点 HTTP API 与传输端口
	Role          LanRole `json:"role"`          // "server" | "client"
	PresetVersion string  `json:"presetVersion"` // 当前动态预设的特征哈希
	AppVersion    string  `json:"appVersion"`    // 节点运行的应用版本号
	LastSeen      int64   `json:"lastSeen"`      // 最近心跳时间戳 (Unix 秒)
	IsConflict    bool    `json:"isConflict"`    // 是否为冲突的多余服务端
}

// LanConfig 局域网与同步相关配置
type LanConfig struct {
	InstanceID             string  `json:"instanceId"`             // 唯一实例 ID (自动生成并持久化)
	DeviceName             string  `json:"deviceName"`             // 设备别名
	Role                   LanRole `json:"role"`                   // 默认 "client"
	SelectedSubnet         string  `json:"selectedSubnet"`         // 选定的子网 CIDR 或 IP
	BroadcastPort          int     `json:"broadcastPort"`          // 默认 38567
	HttpPort               int     `json:"httpPort"`               // 默认 38568
	AutoAcceptClientPushes bool    `json:"autoAcceptClientPushes"` // 服务端是否自动接受客户端推送的模组/地图
}

// PresetModItem 启动预设中的模组条目
type PresetModItem struct {
	FileName string `json:"fileName"` // 文件名 (例如 "od_origin.rwmod")
	Title    string `json:"title"`    // 模组标题
	Hash     string `json:"hash"`     // 识别码 (SHA256)
	FileSize int64  `json:"fileSize"` // 文件大小 (字节)
	Enabled  bool   `json:"enabled"`  // preferences.ini 中的启用状态
	IsDir    bool   `json:"isDir"`    // 是否为解压文件夹
}

// PresetMapItem 启动预设中的地图条目
type PresetMapItem struct {
	FileName      string `json:"fileName"`      // 文件名 (例如 "city_battle.tmx")
	Title         string `json:"title"`         // 地图显示标题
	FilePath      string `json:"filePath"`      // 相对路径 (mods/maps/xxx 或 mods/units/xxx)
	LocationType  string `json:"locationType"`  // "standalone" | "map_pack" | "unit_mod"
	Active        bool   `json:"active"`        // 是否战局可用
	FileSize      int64  `json:"fileSize"`      // 大小
	ParentModFile string `json:"parentModFile"` // 宿主模组文件名 (若是 unit_mod)
}

// LaunchPreset 启动预设 (名单与状态快照)
type LaunchPreset struct {
	ID          string          `json:"id"`          // "dynamic" 或自定义预设 ID
	Name        string          `json:"name"`        // 预设名称 (例如 "动态启动预设")
	UpdatedAt   int64           `json:"updatedAt"`   // 产生或更新的时间戳 (Unix 秒)
	VersionHash string          `json:"versionHash"` // 快速特征版本哈希
	Mods        []PresetModItem `json:"mods"`        // 模组列表与启禁用状态
	Maps        []PresetMapItem `json:"maps"`        // 地图列表与激活状态
}

// SyncDiff 客户端与服务端的预设差异对比结果
type SyncDiff struct {
	ServerPresetHash string          `json:"serverPresetHash"`
	ClientPresetHash string          `json:"clientPresetHash"`
	IsInSync         bool            `json:"isInSync"`        // 两端预设是否完全一致
	MissingMods      []PresetModItem `json:"missingMods"`     // 客户端缺失、需要从服务端下载的模组
	MissingMaps      []PresetMapItem `json:"missingMaps"`     // 客户端缺失、需要从服务端下载的地图
	StateMismatches  []PresetModItem `json:"stateMismatches"` // 模组已存在但启用/禁用状态不同的项 (需对齐)
	ExtraModsCount   int             `json:"extraModsCount"`  // 客户端多出来的本地模组
	Summary          string          `json:"summary"`         // 差异摘要文本
}

// PushRequest 客户端向服务端发起推送模组/地图的请求
type PushRequest struct {
	ID        string  `json:"id"`        // 请求唯一 ID
	PeerID    string  `json:"peerId"`    // 发起端实例 ID
	PeerName  string  `json:"peerName"`  // 发起端主机名
	PeerIP    string  `json:"peerIp"`    // 发起端 IP
	ItemType  string  `json:"itemType"`  // "mod" | "map"
	FileName  string  `json:"fileName"`  // 文件名
	FileSize  int64   `json:"fileSize"`  // 文件大小
	Status    string  `json:"status"`    // "pending" | "accepted" | "rejected" | "completed"
	CreatedAt int64   `json:"createdAt"` // 创建时间
}

// LanStatus 当前节点的局域网与同步状态总览
type LanStatus struct {
	Role               LanRole                `json:"role"`               // 当前角色 "client" | "server"
	InstanceID         string                 `json:"instanceId"`         // 本机实例 ID
	DeviceName         string                 `json:"deviceName"`         // 本机设备名
	SelectedSubnet     string                 `json:"selectedSubnet"`     // 当前绑定的子网
	LocalIP            string                 `json:"localIp"`            // 当前通讯使用的 IP
	HttpPort           int                    `json:"httpPort"`           // HTTP 服务端口
	HasServerConflict  bool                   `json:"hasServerConflict"`  // 是否检测到服务端冲突 (多于一个服务端)
	ConflictingServers []LanPeer              `json:"conflictingServers"` // 冲突的服务端列表
	ActiveServer       *LanPeer               `json:"activeServer"`       // 当前网络中合法的唯一服务端 (若为 client)
	PeerCount          int                    `json:"peerCount"`          // 发现的其他在线节点数
	PresetVersionHash  string                 `json:"presetVersionHash"`  // 本机当前动态预设版本哈希
	AppVersion         string                 `json:"appVersion"`         // 本机应用版本号
	AutoAcceptPushes   bool                   `json:"autoAcceptPushes"`   // 服务端是否自动接收推送
}
