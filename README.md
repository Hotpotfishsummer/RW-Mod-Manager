# 铁锈战争 模组与地图管理器 (Rusted Warfare Mod & Map Manager)

<div align="center">

![Version](https://img.shields.io/badge/version-v1.0.0-blue.svg?style=flat-square)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8.svg?style=flat-square&logo=go)
![Wails](https://img.shields.io/badge/Wails-v2-DF1A55.svg?style=flat-square&logo=wails)
![Vue 3](https://img.shields.io/badge/Vue-3.4+-4FC08D.svg?style=flat-square&logo=vue.js)
![TailwindCSS](https://img.shields.io/badge/TailwindCSS-3.4-38B2AC.svg?style=flat-square&logo=tailwind-css)
![License](https://img.shields.io/badge/License-MIT-green.svg?style=flat-square)

**专为《铁锈战争 (Rusted Warfare)》打造的跨代工坊管理、原生出生点战术标绘、局域网联机预设同步与 P2P 极速自更新一体化工具**

[功能特性](#-核心功能特性) • [系统架构](#-系统架构) • [编译与运行](#-编译与本地运行) • [局域网联机与同步体系](#-局域网联机与预设同步体系) • [双通道自更新与版本规范](#-双通道自更新与版本更新规范)

</div>

---

## 🌟 核心功能特性

### 1. 🗺️ 原生出生点标记高清渲染器 (Native Tactical Map Renderer)
- **纯 Go 零依赖实现**：彻底摆脱对 Python、PIL/Pillow、TMX 外部解释器的环境依赖，单文件可执行程序开箱即用。
- **全格式支持**：兼容独立 `.tmx` 地图、解包地图文件夹、以及 `.rwmod` 模组压缩包内嵌地图的全量读取与流式解构。
- **战术出生点标绘**：自动解析 TMX 对象层，智能提取全部队伍出生点坐标，并在地图预览图对应位置上叠加抗锯齿的高对比度战术圆形序号标记（`1, 2, 3...`），出生点位一目了然。
- **大图灯箱预览**：支持滚轮缩放、自适应缩放与全屏高保真浏览。

### 2. ⚡ 局域网启动预设自动同步与房主仲裁 (LAN Preset Sync & Host Arbitration)
- **严格服务端唯一性仲裁**：同一局域网/虚拟子网（如 Tailscale/Radmin/蒲公英）集合内严格仲裁单一房主服务端。一旦检测到多台机器切换为服务端，立即弹出仲裁告警并全面阻断违规操作，守护预设一致性。
- **动态启动预设感知 (Dynamic Launch Preset)**：管理器动态侦听游戏配置与 `preferences.ini`，并在联机启动前自动进行 S $\rightarrow$ C 预设对齐；客户端缺失的模组与地图将自动从房主主机秒级拉取补齐，启用状态精准同步。
- **双向模组互推与审批**：支持客户端将本机独有模组/地图一键向房主发起推送请求，房主可自由选择手动审批或自动接受。
- **双模启动系统**：
  - **联机启动**：自动校验局域网服务端状态并执行预设对齐后拉起游戏。
  - **单机启动**：以本机当前独立的自由配置启动，不介入联机预设仲裁。

### 3. 🔄 双通道自更新体系 (LAN P2P + GitHub Releases)
- **局域网 P2P 高速通道**：在离线局域网或对战房环境下，任何运行高版本的节点（无论是房主还是队员）均可通过 UDP 信标广播版本号，并通过内置 HTTP 服务（`os.Executable()` 流式读取）向队友直传新版可执行文件，实现零流量百兆/千兆秒更。
- **GitHub Releases 官方通道**：连接公网时自动查询官方 Releases 接口，获取最新 Tag、更新说明与 Release 资产，作为全球云端分发通道。
- **Windows 在位热替换自更新 (Zero-Downtime Hot Replacement)**：利用 Windows 同分区原子重命名技巧，实现运行态平滑替换并自动重启拉起新版本，启动后自动清理解压备份，全程一键静默完成。

### 4. 🎛️ preferences.ini 原生直写与电竞高刷调优 (Engine Tuning)
- 深度融合官方引擎底层参数，可视化配置 JVM 内存上限、60Hz / 120Hz / 144Hz / 240Hz+ 电竞高刷新率支持、垂直同步、FPS 显示与队伍造兵上限。

### 5. 🎨 现代 Material Design 3 暗色美学设计
- 采用 M3 设计语言，支持状态胶囊（Chip）、精致暗调表面海拔高度、骨架屏加载反馈与全局无感 Toast 提示。

---

## 🏗️ 系统架构

```
rw-mod-manager/
├── app.go                          # Wails 后端主生命周期与公开 IPC API 绑定入口
├── main.go                         # 应用程序启动入口 (嵌入静态资源与窗口配置)
├── wails.json                      # Wails 项目配置文件 (含 productVersion 版本声明)
├── internal/
│   ├── models/                     # 核心业务数据模型定义
│   │   ├── models.go               # 模组、地图、偏好设置模型
│   │   ├── lan.go                  # 局域网节点、心跳信标、预设同步模型
│   │   └── update.go               # 更新器模型、版本常量与进度汇报结构
│   └── services/                   # 底层业务服务实现
│       ├── game_service.go         # 游戏安装路径侦测、健全性检查与全局持久化
│       ├── pref_service.go         # preferences.ini 解析器与状态双向同步器
│       ├── mod_service.go          # 模组扫描、元数据解析与启用/禁用控制
│       ├── map_service.go          # 地图全量扫描与缩略图管理中心
│       ├── map_renderer.go         # 原生 Go TMX 与出生点序号图层合成渲染引擎
│       ├── launch_service.go       # 游戏子进程拉起与运行环境装配器
│       ├── lan_service.go          # UDP 组播发现、房主仲裁与 HTTP 传输服务器
│       └── update_service.go       # 局域网 P2P + GitHub Releases 双通道热替换更新器
└── frontend/                       # 前端工程 (Vue 3 + TypeScript + Tailwind CSS)
    ├── package.json                # 前端依赖配置与版本号声明
    ├── src/
    │   ├── App.vue                 # 现代化 MD3 单页主组件 (模组/地图/同步/调优/更新)
    │   ├── types/api.ts            # TypeScript 强类型接口定义
    │   └── composables/useApi.ts   # 后端 Wails IPC 桥接与前端安全兜底层
    └── wailsjs/                    # Wails 自动生成的 Go-TS 绑定代码
```

---

## 🛠️ 编译与本地运行

### 环境准备
- **Go**：`>= 1.21` (推荐 Go 1.22+)
- **Node.js**：`>= 18` (推荐 Node.js 20 LTS)
- **Wails CLI**：`v2.x` (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- **操作系统**：Windows 10 / 11 (64位)

### 本地开发调试
```bash
# 进入工程根目录安装前端依赖
cd frontend
npm install
cd ..

# 启动 Wails 开发模式 (支持热重载)
wails dev
```

### 生产环境单文件编译
```bash
# 清理并构建 Windows x64 独立可执行程序
wails build -clean

# 产物生成位置：build/bin/RW-Mod-Manager.exe
```

### 运行自动化单元测试
```bash
go test -v ./...
```

---

## 🌐 局域网联机与预设同步体系

```
                       ┌─────────────────────────┐
                       │     房主服务端 (Host)    │
                       │  HTTP: 38568 / UDP 广播  │
                       └────────────┬────────────┘
                                    │
           ┌────────────────────────┴────────────────────────┐
           ▼                                                 ▼
┌───────────────────────┐                         ┌───────────────────────┐
│     队员客户端 A       │ ◄─── UDP 节点互寻 ───►  │     队员客户端 B       │
│  HTTP: 38568 (P2P备用)│                         │  HTTP: 38568 (P2P备用)│
└───────────────────────┘                         └───────────────────────┘
```

1. **UDP 心跳组播与监听（默认端口：`38567`）**：
   - 每 2 秒广播一次信标 `DiscoveryBeacon`，包含节点实例 ID、设备名、角色（Server/Client）、HTTP 端口、当前预设版本特征哈希及应用版本号 `AppVersion`。
   - 超过 6 秒未收到心跳的节点将被平滑修剪下线。
2. **服务端唯一性与冲突检测**：
   - 当本机角色为服务端且探测到局域网中存在另一个在线服务端时，立即触发冲突拦截，全屏弹出告警阻断联机启动，直至其中一方切换回客户端角色。
3. **HTTP 动态预设与模组交换（默认端口：`38568`，支持端口冲突自动向下探测）**：
   - `GET /api/lan/preset`：下发当前主机的最新模组与地图生效清单及版本哈希。
   - `GET /api/lan/file?type=mod&name=xxx`：支持单个文件流及解压文件夹动态 Zip 打包下发。
   - `POST /api/lan/request-push`：队员向房主申请推送本机模组。
   - `POST /api/lan/upload`：房主审批通过后队员上传文件流。

---

## 🚀 双通道自更新与版本更新规范

本软件配备了完善的自更新系统。**在后续任何代码修改与功能迭代时，请务必遵循以下版本更新规范：**

### 📌 版本号定义位置清单 (Version Bump Checklist)
当有新功能开发、缺陷修复或功能调整时，必须同步将以下 3 处版本号升级（遵循 [语义化版本规范 SemVer](https://semver.org/)，如 `1.0.0` $\rightarrow$ `1.0.1`）：

1. **后端版本常量**：
   - 文件：[`internal/models/update.go`](internal/models/update.go)
   - 字段：`const AppVersion = "X.Y.Z"`
2. **Wails 清单文件**：
   - 文件：[`wails.json`](wails.json)
   - 字段：`"info": { "productVersion": "X.Y.Z" }`
3. **前端工程配置**：
   - 文件：[`frontend/package.json`](frontend/package.json)
   - 字段：`"version": "X.Y.Z"`

---

### 📦 正式版本发布工作流 (Release Workflow)

```bash
# 1. 完成代码修改并运行测试
go test -v ./...
npm --prefix frontend run build

# 2. 确认版本号已在上述 3 个文件中同步递增，提交 Git
git add -A
git commit -m "chore(release): bump version to v1.0.1"
git push origin master

# 3. 重新编译生成最终 Release 产物
wails build -clean

# 4. 使用 GitHub CLI 打 Tag 并发布正式 Release (附带编译产物)
gh release create v1.0.1 build/bin/RW-Mod-Manager.exe \
  --title "v1.0.1 正式版 - 更新说明标题" \
  --notes "### 更新日志`n`n- 修复了XXX问题`n- 优化了YYY功能"
```

发布完成后：
- 联网客户端点击“检查更新”或启动后台静默探测时，将自动获取 GitHub Releases 最新版本并弹窗提示一键更新。
- 局域网离线对战时，只要房主机器更新为新版，连接到该局域网的所有队员客户端均会优先从房主设备通过 P2P 极速拉取并自动热替换完成升级！

---

## 📄 开源协议 (License)

本项目采用 [MIT License](LICENSE) 开源协议，欢迎广大《铁锈战争》玩家与开发者参与贡献、提交 Issue 与 Pull Request。

- **官方开源仓库**：[https://github.com/Hotpotfishsummer/RW-Mod-Manager](https://github.com/Hotpotfishsummer/RW-Mod-Manager)
- **作者邮箱**：1136296339@qq.com
