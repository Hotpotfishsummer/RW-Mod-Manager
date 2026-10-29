<script setup lang="ts">
import { ref, computed, onMounted, onErrorCaptured } from 'vue'
import {
  Package,
  Map as MapIcon,
  Rocket,
  Settings,
  RefreshCw,
  FolderOpen,
  Search,
  CheckCircle2,
  ExternalLink,
  ShieldAlert,
  Sliders,
  Users,
  Cpu,
  LayoutGrid,
  List,
  ZoomIn,
  ZoomOut,
  Maximize2,
  X,
  Sparkles,
  Eye,
  Check,
  Monitor,
  Flame,
  Activity,
  Radio,
  Wifi,
  WifiOff,
  Share2,
  DownloadCloud,
  UploadCloud,
  AlertTriangle,
  Server,
  UserCheck,
  Play,
  ArrowRightLeft,
  CheckCheck,
  ShieldCheck,
  Layers,
  Folder,
  Send,
  ChevronDown,
  ChevronRight,
  Github,
  ArrowUpCircle
} from 'lucide-vue-next'
import { useApi } from './composables/useApi'
import {
  ModItem,
  MapItem,
  GameStatus,
  AppConfig,
  LanConfig,
  LanStatus,
  LanPeer,
  NetworkInterfaceInfo,
  LaunchPreset,
  SyncDiff,
  PushRequest,
  UpdateInfo,
  UpdateProgress
} from './types/api'

const api = useApi()

// 当前激活的标签页
const currentTab = ref<'units' | 'maps' | 'lan' | 'launcher' | 'settings'>('units')

// 核心数据状态
const loading = ref(false)
const launching = ref(false)
const generatingThumb = ref(false)
const gameStatus = ref<GameStatus>({
  gameDir: '',
  isValid: false,
  version: '',
  totalMods: 0,
  enabledMods: 0,
  disabledMods: 0,
  totalMaps: 0,
  activeMaps: 0
})
const appConfig = ref<AppConfig>({
  gameDir: '',
  launchConfig: {
    executableType: 'exe64',
    maxMemoryMb: 4096,
    fullscreen: false,
    noMods: false,
    safeMode: false,
    extraArgs: ''
  },
  gamePreferences: {
    highRefreshRate: true,
    renderVsync: true,
    showFps: true,
    renderSmoothDelta: false,
    renderAntiAlias: true,
    renderFancyWater: false,
    renderClouds: true,
    renderExtraShadows: true,
    teamShaders: true,
    newRender: false,
    teamUnitCapSinglePlayer: 5000,
    teamUnitCapHostedGame: 2000,
    uiRenderScale: 1.0
  },
  lanConfig: {
    instanceId: '',
    deviceName: '',
    role: 'client',
    selectedSubnet: '',
    broadcastPort: 38567,
    httpPort: 38568,
    autoAcceptClientPushes: false
  }
})

// 局域网与模组预设同步状态
const lanStatus = ref<LanStatus>({
  role: 'client',
  instanceId: '',
  deviceName: '',
  selectedSubnet: '',
  localIp: '',
  httpPort: 38568,
  hasServerConflict: false,
  conflictingServers: [],
  activeServer: null,
  peerCount: 0,
  presetVersionHash: '',
  autoAcceptPushes: false
})
const lanConfig = ref<LanConfig>({
  instanceId: '',
  deviceName: '',
  role: 'client',
  selectedSubnet: '',
  broadcastPort: 38567,
  httpPort: 38568,
  autoAcceptClientPushes: false
})
const interfaces = ref<NetworkInterfaceInfo[]>([])
const peers = ref<LanPeer[]>([])
const dynamicPreset = ref<LaunchPreset>({
  id: 'dynamic',
  name: '动态启动预设',
  updatedAt: 0,
  versionHash: '',
  mods: [],
  maps: []
})
const syncDiff = ref<SyncDiff | null>(null)
const syncing = ref(false)
const pushing = ref(false)
const pendingPushRequests = ref<PushRequest[]>([])
const lanNoticeMsg = ref<{ type: 'success' | 'error' | 'info'; text: string } | null>(null)
const pushModalTarget = ref<{ type: 'mod' | 'map'; name: string; size?: number } | null>(null)
const promptHostModal = ref(false)
const pushHubType = ref<'mod' | 'map'>('mod')
const pushHubTargetFile = ref<string>('')

// 更新器状态
const currentVersion = ref('1.0.0')
const updateInfo = ref<UpdateInfo | null>(null)
const updateProgress = ref<UpdateProgress | null>(null)
const checkingUpdate = ref(false)
const updating = ref(false)
const showUpdateModal = ref(false)
const selectedUpdateSource = ref<'lan' | 'github'>('lan')

const presetTotalMods = computed(() => dynamicPreset.value?.mods?.length || 0)
const presetEnabledMods = computed(() => dynamicPreset.value?.mods?.filter(m => m.enabled)?.length || 0)
const presetDisabledMods = computed(() => dynamicPreset.value?.mods?.filter(m => !m.enabled)?.length || 0)
const presetActiveMaps = computed(() => dynamicPreset.value?.maps?.filter(m => m.active)?.length || 0)

function showNotice(text: string, type: 'success' | 'error' | 'info' = 'success') {
  lanNoticeMsg.value = { type, text }
  setTimeout(() => {
    if (lanNoticeMsg.value?.text === text) {
      lanNoticeMsg.value = null
    }
  }, 4500)
}

const mods = ref<ModItem[]>([])
const maps = ref<MapItem[]>([])

// 模组视图模式与筛选
const modViewMode = ref<'grid' | 'list'>('grid')
const modSearch = ref('')
const modFilterStatus = ref<'all' | 'enabled' | 'disabled'>('all')

// 地图视图与筛选
const mapSearch = ref('')
const mapFilterActive = ref<'all' | 'active' | 'inactive'>('all')
const mapFilterType = ref<'all' | 'standalone' | 'pack' | 'unit_mod'>('all')

// 地图预览模态框 (Lightbox)
const activeMapModal = ref<MapItem | null>(null)
const zoomLevel = ref(1)

// 格式化文件大小 (添加健壮防空指针与越界保护)
function formatBytes(bytes?: number | null) {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  if (i < 0 || i >= sizes.length) return '0 B'
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

// 全局捕获异常，防止组件崩溃白屏/黑屏
onErrorCaptured((err, instance, info) => {
  console.error('[Vue Unhandled Error Guarded]:', err, info)
  return false
})

// 折叠状态管理 (在全部浏览时默认折叠单位模组，防止瞬间渲染300张卡片导致 WebView2 GPU 显存过载黑屏)
const expandedMapGroups = ref<Record<string, boolean>>({})

function toggleGroupExpand(key: string) {
  expandedMapGroups.value[key] = !isGroupExpanded(key)
}

function isGroupExpanded(key: string, defaultOpen: boolean = false): boolean {
  if (expandedMapGroups.value[key] !== undefined) {
    return expandedMapGroups.value[key]
  }
  // 搜索关键字非空或选定特定模组时自动展开
  if (mapSearch.value.trim().length > 0) return true
  if (selectedMapGroup.value.startsWith('mod:') || selectedMapGroup.value.startsWith('pack:')) return true
  return defaultOpen
}

// 过滤后的模组列表
const filteredMods = computed(() => {
  if (!mods.value) return []
  return mods.value.filter(m => {
    if (!m) return false
    if (modFilterStatus.value === 'enabled' && !m.enabled) return false
    if (modFilterStatus.value === 'disabled' && m.enabled) return false

    if (!modSearch.value.trim()) return true
    const kw = modSearch.value.toLowerCase()
    return (
      (m.title || '').toLowerCase().includes(kw) ||
      (m.titleZh || '').toLowerCase().includes(kw) ||
      (m.fileName || '').toLowerCase().includes(kw) ||
      (m.tags && m.tags.some(t => (t || '').toLowerCase().includes(kw)))
    )
  })
})

// 地图归属分组选择 (all: 全部板块展示, standalone: 独立地图, all_map_packs: 全部地图包, pack:<name>: 指定地图包, all_unit_mods: 全部模组地图, mod:<name>: 指定模组)
const selectedMapGroup = ref<string>('all')

// 过滤后的地图列表 (综合搜索关键词与激活状态及来源分组)
const filteredMaps = computed(() => {
  if (!maps.value) return []
  return maps.value.filter(m => {
    if (!m) return false
    // 激活状态过滤
    if (mapFilterActive.value === 'active' && !m.active) return false
    if (mapFilterActive.value === 'inactive' && m.active) return false

    // 来源分组过滤
    if (selectedMapGroup.value === 'standalone' && m.locationType !== 'standalone') return false
    if (selectedMapGroup.value === 'all_map_packs' && m.locationType !== 'map_pack') return false
    if (selectedMapGroup.value.startsWith('pack:') && m.packName !== selectedMapGroup.value.slice(5)) return false
    if (selectedMapGroup.value === 'all_unit_mods' && m.locationType !== 'unit_mod') return false
    if (selectedMapGroup.value.startsWith('mod:') && m.parentModFile !== selectedMapGroup.value.slice(4)) return false

    if (!mapSearch.value.trim()) return true
    const kw = mapSearch.value.toLowerCase()
    return (
      (m.title || '').toLowerCase().includes(kw) ||
      (m.fileName || '').toLowerCase().includes(kw) ||
      (m.packName && m.packName.toLowerCase().includes(kw)) ||
      (m.parentModFile && m.parentModFile.toLowerCase().includes(kw))
    )
  })
})

// 独立地图总数
const standaloneMapCount = computed(() => (maps.value || []).filter(m => m && m.locationType === 'standalone').length)
// 地图包总地图数
const totalPackMapsCount = computed(() => (maps.value || []).filter(m => m && m.locationType === 'map_pack').length)
// 模组自带地图总数
const totalModMapsCount = computed(() => (maps.value || []).filter(m => m && m.locationType === 'unit_mod').length)

// 所有独立的聚合地图包 (从全部 maps 统计)
const availableMapPacks = computed(() => {
  const packs = new Map<string, { name: string; count: number; active: boolean; size: number }>()
  for (const m of maps.value || []) {
    if (!m) continue
    if (m.locationType === 'map_pack' && m.packName) {
      const existing = packs.get(m.packName) || { name: m.packName, count: 0, active: m.active, size: 0 }
      existing.count++
      if ((m.fileSize || 0) > existing.size) existing.size = m.fileSize || 0
      packs.set(m.packName, existing)
    }
  }
  return Array.from(packs.values()).sort((a, b) => (a.name || '').localeCompare(b.name || ''))
})

// 所有包含地图的单位模组 (从全部 maps 统计)
const availableUnitModsWithMaps = computed(() => {
  const modsMap = new Map<string, { name: string; count: number; active: boolean; size: number }>()
  for (const m of maps.value || []) {
    if (!m) continue
    if (m.locationType === 'unit_mod' && m.parentModFile) {
      const existing = modsMap.get(m.parentModFile) || { name: m.parentModFile, count: 0, active: m.active, size: 0 }
      existing.count++
      existing.active = m.active
      if ((m.fileSize || 0) > existing.size) existing.size = m.fileSize || 0
      modsMap.set(m.parentModFile, existing)
    }
  }
  return Array.from(modsMap.values()).sort((a, b) => (a.name || '').localeCompare(b.name || ''))
})

// 独立地图列表 (根据搜索关键词与激活状态过滤)
const filteredStandaloneMaps = computed(() => {
  if (selectedMapGroup.value !== 'all' && selectedMapGroup.value !== 'standalone') return []
  return (filteredMaps.value || []).filter(m => m && m.locationType === 'standalone')
})

// 聚合地图包分组
const groupedMapPacks = computed(() => {
  if (selectedMapGroup.value === 'standalone' || selectedMapGroup.value === 'all_unit_mods' || selectedMapGroup.value.startsWith('mod:')) {
    return []
  }
  const groups: { packName: string; count: number; active: boolean; size: number; maps: MapItem[] }[] = []
  const mapByPack = new Map<string, MapItem[]>()
  for (const m of filteredMaps.value || []) {
    if (!m) continue
    if (m.locationType === 'map_pack' && m.packName) {
      if (selectedMapGroup.value.startsWith('pack:') && selectedMapGroup.value !== `pack:${m.packName}`) {
        continue
      }
      const list = mapByPack.get(m.packName) || []
      list.push(m)
      mapByPack.set(m.packName, list)
    }
  }
  for (const [packName, list] of mapByPack.entries()) {
    let maxSize = 0
    for (const m of list) {
      if ((m.fileSize || 0) > maxSize) maxSize = m.fileSize || 0
    }
    groups.push({
      packName,
      count: list.length,
      active: list.some(x => x && x.active),
      size: maxSize,
      maps: list
    })
  }
  return groups.sort((a, b) => (a.packName || '').localeCompare(b.packName || ''))
})

// 单位模组内嵌地图分组
const groupedUnitModsWithMaps = computed(() => {
  if (selectedMapGroup.value === 'standalone' || selectedMapGroup.value === 'all_map_packs' || selectedMapGroup.value.startsWith('pack:')) {
    return []
  }
  const groups: { parentModFile: string; count: number; active: boolean; size: number; maps: MapItem[] }[] = []
  const mapByMod = new Map<string, MapItem[]>()
  for (const m of filteredMaps.value || []) {
    if (!m) continue
    if (m.locationType === 'unit_mod' && m.parentModFile) {
      if (selectedMapGroup.value.startsWith('mod:') && selectedMapGroup.value !== `mod:${m.parentModFile}`) {
        continue
      }
      const list = mapByMod.get(m.parentModFile) || []
      list.push(m)
      mapByMod.set(m.parentModFile, list)
    }
  }
  for (const [parentModFile, list] of mapByMod.entries()) {
    let maxSize = 0
    for (const m of list) {
      if ((m.fileSize || 0) > maxSize) maxSize = m.fileSize || 0
    }
    groups.push({
      parentModFile,
      count: list.length,
      active: list.some(x => x && x.active),
      size: maxSize,
      maps: list
    })
  }
  return groups.sort((a, b) => (a.parentModFile || '').localeCompare(b.parentModFile || ''))
})

// 统计信息
const enabledCount = computed(() => mods.value.filter(m => m.enabled).length)
const activeMapCount = computed(() => maps.value.filter(m => m.active).length)
const missingPreviewCount = computed(() => maps.value.filter(m => !m.hasPreview).length)

// 加载全量数据
async function loadAllData() {
  loading.value = true
  try {
    appConfig.value = await api.getConfig()
    gameStatus.value = await api.getGameStatus()

    if (gameStatus.value.isValid) {
      const [modList, mapList] = await Promise.all([
        api.scanMods(),
        api.scanMaps()
      ])
      mods.value = modList
      maps.value = mapList
    }
    if (lanStatus.value?.role && appConfig.value?.lanConfig) {
      appConfig.value.lanConfig.role = lanStatus.value.role
    }
  } catch (err) {
    console.error('加载数据失败:', err)
  } finally {
    loading.value = false
  }
}

// 切换模组状态
async function handleToggleMod(item: ModItem) {
  const target = !item.enabled
  item.enabled = target // 乐观更新
  try {
    await api.toggleMod(item.fileName, target)
    // 联动更新地图中的激活状态
    maps.value.forEach(m => {
      if (m.parentModFile === item.fileName) {
        m.active = target
      }
    })
  } catch (err) {
    item.enabled = !target // 回滚
    alert(`切换模组状态失败: ${err}`)
  }
}

// 批量切换模组
async function handleBatchToggle(enableAll: boolean) {
  loading.value = true
  try {
    await api.batchToggleMods(enableAll)
    mods.value.forEach(m => (m.enabled = enableAll))
    maps.value.forEach(m => {
      if (m.locationType === 'unit_mod') {
        m.active = enableAll
      }
    })
  } catch (err) {
    alert(`批量操作失败: ${err}`)
  } finally {
    loading.value = false
  }
}

// 在地图列表直接激活所属宿主模组
async function handleActivateParentMod(parentModFile: string) {
  if (!parentModFile) return
  loading.value = true
  try {
    await api.activateMapParentMod(parentModFile)
    const targetMod = mods.value.find(m => m.fileName === parentModFile)
    if (targetMod) {
      targetMod.enabled = true
    }
    maps.value.forEach(m => {
      if (m.parentModFile === parentModFile) {
        m.active = true
      }
    })
    if (activeMapModal.value && activeMapModal.value.parentModFile === parentModFile) {
      activeMapModal.value.active = true
    }
  } catch (err) {
    alert(`激活宿主模组失败: ${err}`)
  } finally {
    loading.value = false
  }
}

// 打开地图缩略图大图弹窗
function openMapLightbox(m: MapItem) {
  activeMapModal.value = m
  zoomLevel.value = 1
}

// 关闭弹窗
function closeMapLightbox() {
  activeMapModal.value = null
  zoomLevel.value = 1
}

// 为单个地图生成/重新生成缩略图 (带出生点，支持独立地图与 .rwmod 包内地图)
async function handleGenerateMapThumb(m: MapItem) {
  generatingThumb.value = true
  try {
    const newBase64 = await api.generateMapThumbnail(m.filePath)
    if (newBase64) {
      m.hasPreview = true
      m.previewData = newBase64
      if (activeMapModal.value && activeMapModal.value.id === m.id) {
        activeMapModal.value.previewData = newBase64
        activeMapModal.value.hasPreview = true
      }
    }
  } catch (err) {
    alert(`生成缩略图失败: ${err}`)
  } finally {
    generatingThumb.value = false
  }
}

// 批量为地图生成/重绘预览 (force=true 时全量强制重绘出生点标记，force=false 时仅补充缺图)
async function handleBatchGenerateThumbs(force: boolean = false) {
  if (force) {
    const ok = confirm('确定要全量强制重绘所有地图的出生点标记吗？\n这将逐一分析所有独立地图与 .rwmod 模组包中的地图，重新计算各队伍出生点并覆盖旧缩略图。')
    if (!ok) return
  }
  generatingThumb.value = true
  try {
    const count = await api.batchGenerateThumbnails(force)
    alert(`处理完成！已为 ${count} 张地图生成/更新了带出生点标记的高清预览图。`)
    await loadAllData()
  } catch (err) {
    alert(`批量生成缩略图失败: ${err}`)
  } finally {
    generatingThumb.value = false
  }
}

// 保存全部偏好设置与启动配置
async function handleSaveSettings() {
  try {
    await api.saveConfig(appConfig.value)
    alert('所有启动参数与游戏画质偏好已成功保存！')
  } catch (err) {
    alert(`保存配置失败: ${err}`)
  }
}

// 轮询与拉取局域网运行状态
async function pollLanStatus() {
  try {
    const status = await api.getLanStatus()
    if (status) {
      status.conflictingServers = status.conflictingServers || []
      lanStatus.value = status
    }

    const peerList = await api.getDiscoveredPeers()
    peers.value = peerList || []

    // 若当前为客户端且发现了服务端，检查预设差异
    if (lanStatus.value.role === 'client' && lanStatus.value.activeServer) {
      try {
        const diff = await api.comparePresetWithServer(
          lanStatus.value.activeServer.ip,
          lanStatus.value.activeServer.httpPort
        )
        if (diff) {
          diff.missingMods = diff.missingMods || []
          diff.missingMaps = diff.missingMaps || []
          diff.stateMismatches = diff.stateMismatches || []
          syncDiff.value = diff
        }
      } catch (diffErr) {
        // 房主可能尚未就绪
      }
    } else {
      syncDiff.value = null
    }

    // 若当前为服务端，检查待处理推送
    if (lanStatus.value.role === 'server') {
      try {
        const reqs = await api.getPendingPushRequests()
        pendingPushRequests.value = reqs || []
      } catch (reqErr) {
        pendingPushRequests.value = []
      }
    }

    try {
      const p = await api.getDynamicPreset()
      if (p) {
        p.mods = p.mods || []
        p.maps = p.maps || []
        dynamicPreset.value = p
      }
    } catch (presetErr) {
      //
    }
  } catch (err) {
    // 忽略轮询瞬态网络抖动
  }
}

// 加载局域网配置与网络适配器
async function loadLanData() {
  try {
    lanConfig.value = await api.getLanConfig()
  } catch (err) {
    console.warn('加载局域网配置失败:', err)
  }

  try {
    interfaces.value = (await api.getNetworkInterfaces()) || []
  } catch (err) {
    console.warn('获取网络接口失败:', err)
  }

  try {
    const preset = await api.getDynamicPreset()
    if (preset) {
      preset.mods = preset.mods || []
      preset.maps = preset.maps || []
      dynamicPreset.value = preset
    }
  } catch (err) {
    console.warn('获取动态预设失败:', err)
  }

  await pollLanStatus()
}

// 切换服务端 / 客户端角色
async function handleSwitchRole(role: 'client' | 'server') {
  try {
    await api.switchLanRole(role)
    lanConfig.value.role = role
    lanStatus.value.role = role
    if (appConfig.value?.lanConfig) {
      appConfig.value.lanConfig.role = role
    }
    await pollLanStatus()
    showNotice(`已切换为${role === 'server' ? '服务端 (房主)' : '客户端'}`)
  } catch (err) {
    showNotice(`切换角色失败: ${err}`, 'error')
  }
}

// 更改网络子网范围
async function handleSubnetChange(e: Event) {
  const subnet = (e.target as HTMLSelectElement).value
  lanConfig.value.selectedSubnet = subnet
  try {
    await api.saveLanConfig(lanConfig.value)
    await pollLanStatus()
    showNotice(`已绑定网络子网: ${subnet || '自动选择'}`)
  } catch (err) {
    showNotice(`保存子网设置失败: ${err}`, 'error')
  }
}

// 切换服务端自动接收推送
async function handleToggleAutoAccept() {
  try {
    await api.saveLanConfig(lanConfig.value)
    showNotice(lanConfig.value.autoAcceptClientPushes ? '已开启客户端推送自动接收' : '已关闭自动接收 (开启手动审核)')
  } catch (err) {
    showNotice(`保存失败: ${err}`, 'error')
  }
}

// 客户端从服务端同步完整预设
async function handleSyncFromHost() {
  if (!lanStatus.value.activeServer) {
    showNotice('未找到可用的房主服务端', 'error')
    return
  }
  syncing.value = true
  try {
    await api.syncFromHost(
      lanStatus.value.activeServer.ip,
      lanStatus.value.activeServer.httpPort
    )
    await loadAllData()
    await pollLanStatus()
    showNotice('启动预设与模组文件同步成功！已与房主完全对齐')
  } catch (err) {
    showNotice(`同步失败: ${err}`, 'error')
  } finally {
    syncing.value = false
  }
}

// 打开推送弹窗
function openPushModal(type: 'mod' | 'map', name: string, size?: number) {
  if (!lanStatus.value.activeServer) {
    showNotice('未找到房主服务端，无法发起推送！请等待局域网房主启动上线', 'error')
    return
  }
  pushModalTarget.value = { type, name, size }
}

// 确认发起推送
async function confirmPushToHost() {
  if (!pushModalTarget.value || !lanStatus.value.activeServer) return
  pushing.value = true
  const target = pushModalTarget.value
  try {
    await api.requestPushToHost(
      lanStatus.value.activeServer.ip,
      lanStatus.value.activeServer.httpPort,
      target.type,
      target.name
    )
    showNotice(`成功推送 ${target.name} 至房主！`)
    pushModalTarget.value = null
  } catch (err) {
    showNotice(`推送失败: ${err}`, 'error')
  } finally {
    pushing.value = false
  }
}

// 服务端审批推送
async function handlePushDecision(requestId: string, accept: boolean) {
  try {
    await api.handlePushDecision(requestId, accept)
    const reqs = await api.getPendingPushRequests()
    pendingPushRequests.value = reqs || []
    showNotice(accept ? '已接受客户端推送' : '已拒绝客户端推送')
  } catch (err) {
    showNotice(`处理审批失败: ${err}`, 'error')
  }
}

// 联机模式启动游戏 (带预设自动同步与冲突检测)
async function handleLaunchOnline() {
  if (lanStatus.value.hasServerConflict) {
    showNotice('局域网检测到多个服务端冲突，操作已被禁止！', 'error')
    return
  }
  // 若当前为客户端且未发现任何房主服务端，弹出向导让用户一键切换为房主或以单机模式启动
  if (lanStatus.value.role === 'client' && !lanStatus.value.activeServer) {
    promptHostModal.value = true
    return
  }
  launching.value = true
  try {
    if (appConfig.value?.lanConfig) {
      appConfig.value.lanConfig.role = lanStatus.value.role
    }
    await api.saveConfig(appConfig.value)
    await api.launchGameOnline()
    await loadAllData()
    await pollLanStatus()
    showNotice(lanStatus.value.role === 'server' ? '游戏已作为房主服务端拉起！' : '游戏已对齐房主预设并启动！')
  } catch (err) {
    alert(`联机启动失败: ${err}`)
  } finally {
    setTimeout(() => {
      launching.value = false
    }, 1500)
  }
}

// 客户端无房主时，一键切换为房主并启动
async function handleSwitchToHostAndLaunch() {
  promptHostModal.value = false
  await handleSwitchRole('server')
  await handleLaunchOnline()
}

// 联机协同面板：向房主主动推送本地资源
async function handlePushHubSend() {
  if (!lanStatus.value.activeServer) {
    showNotice('当前未检测到房主服务端，请等待房主上线并处于同一网络！', 'error')
    return
  }
  if (!pushHubTargetFile.value) {
    showNotice('请先在下拉列表中选择要推送的文件！', 'error')
    return
  }
  let size = 0
  if (pushHubType.value === 'mod') {
    const item = mods.value.find(m => m.fileName === pushHubTargetFile.value)
    if (item) size = item.fileSize
  } else {
    const item = maps.value.find(m => m.fileName === pushHubTargetFile.value || m.id === pushHubTargetFile.value)
    if (item) size = item.fileSize
  }
  openPushModal(pushHubType.value, pushHubTargetFile.value, size)
}

// 单机模式启动游戏 (独立运行，不检查联机冲突)
async function handleLaunchSingle() {
  launching.value = true
  try {
    await api.saveConfig(appConfig.value)
    await api.launchGameSingle()
    showNotice('游戏已在单机模式下拉起！')
  } catch (err) {
    alert(`单机启动失败: ${err}`)
  } finally {
    setTimeout(() => {
      launching.value = false
    }, 1500)
  }
}

// 启动游戏 (兼容默认调用)
async function handleLaunchGame() {
  await handleLaunchOnline()
}

// 更换游戏目录
async function handleChangeDir() {
  const dir = await api.selectGameDir()
  if (dir) {
    try {
      const status = await api.setGameDir(dir)
      gameStatus.value = status
      appConfig.value.gameDir = dir
      await loadAllData()
      await loadLanData()
    } catch (err) {
      alert(err)
    }
  }
}

// 打开外部工坊链接
function openWorkshopLink(steamId: string) {
  if (steamId) {
    window.open(`https://steamcommunity.com/sharedfiles/filedetails/?id=${steamId}`, '_blank')
  }
}

// 检查软件更新 (支持静默探测与手动弹窗)
async function handleCheckUpdate(silent: boolean = false) {
  checkingUpdate.value = true
  try {
    const info = await api.checkForUpdates()
    if (info) {
      updateInfo.value = info
      currentVersion.value = info.currentVersion || currentVersion.value
      selectedUpdateSource.value = info.source || 'lan'
      if (info.hasUpdate) {
        if (!silent) {
          showUpdateModal.value = true
        } else {
          showNotice(`检测到新版本 v${info.latestVersion} (${info.source === 'lan' ? '来自局域网房主' : '来自 GitHub Releases'})，点击顶部版本号升级`, 'info')
        }
      } else {
        if (!silent) {
          showNotice(`当前已是最新版本 (v${info.currentVersion})`, 'info')
        }
      }
    }
  } catch (err) {
    if (!silent) {
      showNotice(`检查更新失败: ${err}`, 'error')
    }
  } finally {
    checkingUpdate.value = false
  }
}

// 执行自更新
async function handlePerformUpdate() {
  if (!updateInfo.value?.hasUpdate) return
  updating.value = true
  try {
    await api.downloadAndApplyUpdate(selectedUpdateSource.value)
  } catch (err) {
    showNotice(`自更新失败: ${err}`, 'error')
    updating.value = false
  }
}

onMounted(() => {
  loadAllData()
  loadLanData()

  api.getCurrentVersion().then(v => {
    if (v) currentVersion.value = v
  })

  // 定时心跳轮询以维持局域网节点感知
  setInterval(() => {
    pollLanStatus()
  }, 2000)

  // 启动后异步自动检查一次更新
  setTimeout(() => {
    handleCheckUpdate(true)
  }, 2500)

  // 监听 Wails 后端实时事件
  if (typeof window !== 'undefined' && (window as any).runtime?.EventsOn) {
    const runtime = (window as any).runtime
    runtime.EventsOn('lan:status_updated', (data: LanStatus) => {
      if (data) {
        data.conflictingServers = data.conflictingServers || []
        lanStatus.value = data
      }
    })
    runtime.EventsOn('lan:server_conflict', (data: LanStatus) => {
      if (data) {
        data.conflictingServers = data.conflictingServers || []
        lanStatus.value = data
      }
    })
    runtime.EventsOn('lan:preset_changed', (preset: LaunchPreset) => {
      if (preset) {
        preset.mods = preset.mods || []
        preset.maps = preset.maps || []
        dynamicPreset.value = preset
      }
    })
    runtime.EventsOn('lan:push_requests_updated', (reqs: PushRequest[]) => {
      pendingPushRequests.value = reqs || []
    })
    runtime.EventsOn('lan:push_request_received', (req: PushRequest) => {
      if (!pendingPushRequests.value) pendingPushRequests.value = []
      pendingPushRequests.value.push(req)
      showNotice(`收到来自 ${req.peerName} 的模组推送请求`, 'info')
    })
    runtime.EventsOn('update:progress', (p: UpdateProgress) => {
      updateProgress.value = p
      if (p.status === 'error') {
        updating.value = false
        showNotice(`更新失败: ${p.error || p.message}`, 'error')
      }
    })
  }
})
</script>

<template>
  <div class="flex flex-col h-screen w-screen overflow-hidden font-sans">
    <!-- 顶部导航栏 (Material Design 3 Top App Bar) -->
    <header class="flex items-center justify-between px-6 py-3.5 bg-m3-surface-low border-b border-m3-outline-variant shrink-0">
      <div class="flex items-center gap-5">
        <!-- Logo 与标题 -->
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-2xl bg-m3-primary-container text-m3-primary flex items-center justify-center shadow-m3-1">
            <Cpu class="w-5 h-5 text-m3-on-primary-container" />
          </div>
          <div>
            <h1 class="text-base font-bold tracking-tight text-m3-on-surface">
              铁锈战争 模组与地图管理器
            </h1>
            <p class="text-[11px] text-m3-on-surface-variant font-medium tracking-wide">Rusted Warfare Workshop & LAN Sync Manager</p>
          </div>
        </div>

        <!-- 游戏路径状态胶囊 (M3 Assist Chip) -->
        <div class="hidden md:flex items-center gap-2 px-3 py-1.5 rounded-full bg-m3-surface-container border border-m3-outline text-xs">
          <span
            class="w-2 h-2 rounded-full"
            :class="gameStatus.isValid ? 'bg-m3-success' : 'bg-m3-error'"
          />
          <span class="text-m3-on-surface font-mono text-xs truncate max-w-xs" :title="gameStatus.gameDir">
            {{ gameStatus.gameDir || '未配置游戏路径' }}
          </span>
          <button
            @click="handleChangeDir"
            class="ml-1 text-m3-primary hover:text-m3-primary-hover font-medium transition"
          >
            更换
          </button>
        </div>

        <!-- 局域网状态胶囊 (M3 Status Chip) -->
        <div
          @click="currentTab = 'lan'"
          class="cursor-pointer hidden lg:flex items-center gap-2 px-3 py-1.5 rounded-full border text-xs font-medium transition hover:brightness-110"
          :class="
            lanStatus.hasServerConflict
              ? 'bg-m3-error-container/70 border-m3-error/60 text-m3-on-error-container animate-pulse'
              : lanStatus.role === 'server'
              ? 'bg-m3-primary-container/70 border-m3-primary/60 text-m3-on-primary-container'
              : lanStatus.activeServer && syncDiff?.isInSync
              ? 'bg-m3-success-container/70 border-m3-success/60 text-m3-on-success-container'
              : lanStatus.activeServer
              ? 'bg-m3-warning-container/70 border-m3-warning/60 text-m3-on-warning-container'
              : 'bg-m3-surface-container border-m3-outline text-m3-on-surface-variant'
          "
        >
          <AlertTriangle v-if="lanStatus.hasServerConflict" class="w-3.5 h-3.5 text-m3-error" />
          <Server v-else-if="lanStatus.role === 'server'" class="w-3.5 h-3.5 text-m3-primary" />
          <Wifi v-else-if="lanStatus.activeServer && syncDiff?.isInSync" class="w-3.5 h-3.5 text-m3-success" />
          <ArrowRightLeft v-else-if="lanStatus.activeServer" class="w-3.5 h-3.5 text-m3-warning" />
          <WifiOff v-else class="w-3.5 h-3.5 text-m3-on-surface-dim" />

          <span v-if="lanStatus.hasServerConflict" class="font-bold">服务端冲突！</span>
          <span v-else-if="lanStatus.role === 'server'">房主主机 ({{ peers?.length || 0 }} 在线)</span>
          <span v-else-if="lanStatus.activeServer && syncDiff?.isInSync">房主: {{ lanStatus.activeServer.deviceName }} [已同步]</span>
          <span v-else-if="lanStatus.activeServer">房主: {{ lanStatus.activeServer.deviceName }} [待同步]</span>
          <span v-else>客户端模式 (搜寻房主中...)</span>
        </div>

        <!-- 顶部快速角色切换器 (M3 Segmented Button) -->
        <div class="hidden sm:flex items-center rounded-full bg-m3-surface-container border border-m3-outline p-0.5 text-xs font-medium">
          <button
            @click="handleSwitchRole('client')"
            title="设为客户端 (参与对战成员，游戏预设以房主为准自动同步)"
            class="px-3 py-1 rounded-full transition flex items-center gap-1.5"
            :class="lanStatus.role === 'client' ? 'bg-m3-primary text-white font-semibold shadow-sm' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
          >
            <UserCheck class="w-3.5 h-3.5" /> 客户端
          </button>
          <button
            @click="handleSwitchRole('server')"
            title="设为服务端 (局域网房主，将发布模组预设并充当主机)"
            class="px-3 py-1 rounded-full transition flex items-center gap-1.5"
            :class="lanStatus.role === 'server' ? 'bg-m3-primary text-white font-semibold shadow-sm' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
          >
            <Server class="w-3.5 h-3.5" /> 房主服务端
          </button>
        </div>
      </div>

      <!-- 右侧全局操作区 -->
      <div class="flex items-center gap-2.5">
        <!-- 目录快速打开 (M3 Outlined Button Group) -->
        <div class="flex items-center rounded-xl bg-m3-surface-container p-0.5 border border-m3-outline">
          <button
            @click="api.openFolder('units')"
            title="打开单位模组文件夹"
            class="px-2.5 py-1.5 text-xs font-medium text-m3-on-surface-variant hover:text-m3-on-surface hover:bg-m3-surface-high rounded-lg transition flex items-center gap-1.5"
          >
            <FolderOpen class="w-3.5 h-3.5 text-m3-primary" />
            <span>模组</span>
          </button>
          <button
            @click="api.openFolder('maps')"
            title="打开地图文件夹"
            class="px-2.5 py-1.5 text-xs font-medium text-m3-on-surface-variant hover:text-m3-on-surface hover:bg-m3-surface-high rounded-lg transition flex items-center gap-1.5"
          >
            <FolderOpen class="w-3.5 h-3.5 text-m3-success" />
            <span>地图</span>
          </button>
        </div>

        <!-- 刷新按钮 (M3 Icon Button) -->
        <button
          @click="loadAllData"
          :disabled="loading"
          class="p-2 text-m3-on-surface-variant hover:text-m3-on-surface bg-m3-surface-container hover:bg-m3-surface-high border border-m3-outline rounded-xl transition disabled:opacity-40"
          title="刷新扫描"
        >
          <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
        </button>

        <!-- 版本号与更新检查胶囊 (M3 Assist Chip) -->
        <button
          v-if="updateInfo?.hasUpdate"
          @click="showUpdateModal = true"
          class="px-3 py-1.5 rounded-xl bg-amber-500/20 hover:bg-amber-500/30 border border-amber-500/40 text-amber-300 text-xs font-semibold flex items-center gap-1.5 transition active:scale-95 animate-pulse shadow-sm"
          title="检测到新版本，点击查看并一键自更新"
        >
          <Sparkles class="w-3.5 h-3.5 text-amber-400" />
          <span>新版 v{{ updateInfo.latestVersion }}</span>
        </button>
        <button
          v-else
          @click="handleCheckUpdate(false)"
          :disabled="checkingUpdate"
          class="px-2.5 py-1.5 rounded-xl bg-m3-surface-container hover:bg-m3-surface-high border border-m3-outline text-m3-on-surface-variant hover:text-m3-on-surface text-[11px] font-mono flex items-center gap-1.5 transition disabled:opacity-50"
          title="点击检查局域网与 GitHub 更新"
        >
          <ArrowUpCircle class="w-3.5 h-3.5 text-m3-primary" :class="{ 'animate-spin': checkingUpdate }" />
          <span>v{{ currentVersion }}</span>
        </button>

        <!-- 双模启动按钮组 (M3 Filled & Tonal Buttons) -->
        <div class="flex items-center gap-2">
          <!-- 联机模式启动 (M3 Filled Primary Button) -->
          <button
            @click="handleLaunchOnline"
            :disabled="launching || !gameStatus.isValid || lanStatus.hasServerConflict"
            class="flex items-center gap-2 px-4 py-2 bg-m3-primary hover:bg-m3-primary-hover text-white font-semibold text-xs rounded-xl shadow-m3-1 active:scale-95 transition-all disabled:opacity-40 disabled:cursor-not-allowed"
            :title="lanStatus.role === 'server' ? '以房主服务端启动：发布当前模组预设并启动游戏' : '以客户端启动：自动对齐房主预设后启动游戏'"
          >
            <Rocket class="w-4 h-4" :class="{ 'animate-bounce': launching }" />
            <span>
              {{
                launching
                  ? '启动中...'
                  : lanStatus.role === 'server'
                  ? '房主启动 (广播预设)'
                  : '联机启动 (自动同步)'
              }}
            </span>
          </button>

          <!-- 单机模式启动 (M3 Tonal Button) -->
          <button
            @click="handleLaunchSingle"
            :disabled="launching || !gameStatus.isValid"
            class="flex items-center gap-1.5 px-3 py-2 bg-m3-surface-container hover:bg-m3-surface-high text-m3-on-surface border border-m3-outline font-medium text-xs rounded-xl transition active:scale-95 disabled:opacity-40"
            title="以单机模式启动：使用本地当前模组配置，不检查局域网预设冲突"
          >
            <Play class="w-3.5 h-3.5 text-m3-on-surface-variant" />
            <span>单机启动</span>
          </button>
        </div>
      </div>
    </header>

    <!-- 选项卡导航栏 (Material Design 3 Tabs) -->
    <nav class="flex items-center justify-between px-6 bg-m3-surface-low border-b border-m3-outline-variant shrink-0">
      <div class="flex gap-1">
        <button
          @click="currentTab = 'units'"
          class="flex items-center gap-2.5 py-3 px-4 border-b-2 text-xs font-medium transition"
          :class="currentTab === 'units' ? 'border-m3-primary text-m3-primary font-semibold' : 'border-transparent text-m3-on-surface-variant hover:text-m3-on-surface hover:bg-m3-surface-container/40 rounded-t-lg'"
        >
          <Package class="w-4 h-4" />
          <span>单位模组 (Units)</span>
          <span class="px-2 py-0.5 rounded-full bg-m3-surface-container text-[11px] font-mono font-medium text-m3-on-surface-variant border border-m3-outline">
            {{ enabledCount }} / {{ mods.length }}
          </span>
        </button>

        <button
          @click="currentTab = 'maps'"
          class="flex items-center gap-2.5 py-3 px-4 border-b-2 text-xs font-medium transition"
          :class="currentTab === 'maps' ? 'border-m3-primary text-m3-primary font-semibold' : 'border-transparent text-m3-on-surface-variant hover:text-m3-on-surface hover:bg-m3-surface-container/40 rounded-t-lg'"
        >
          <MapIcon class="w-4 h-4" />
          <span>地图工坊 (Maps)</span>
          <span class="px-2 py-0.5 rounded-full bg-m3-surface-container text-[11px] font-mono font-medium text-m3-on-surface-variant border border-m3-outline">
            {{ activeMapCount }} / {{ maps.length }} 就绪
          </span>
          <span v-if="missingPreviewCount > 0" class="px-2 py-0.5 rounded-full bg-m3-warning-container text-[11px] font-medium text-m3-on-warning-container border border-m3-warning/30">
            {{ missingPreviewCount }} 缺图
          </span>
        </button>

        <!-- 局域网同步 Tab 按钮 -->
        <button
          @click="currentTab = 'lan'"
          class="flex items-center gap-2.5 py-3 px-4 border-b-2 text-xs font-medium transition"
          :class="currentTab === 'lan' ? 'border-m3-primary text-m3-primary font-semibold' : 'border-transparent text-m3-on-surface-variant hover:text-m3-on-surface hover:bg-m3-surface-container/40 rounded-t-lg'"
        >
          <Radio class="w-4 h-4" />
          <span>局域网同步 (LAN Sync)</span>
          <span
            v-if="lanStatus.hasServerConflict"
            class="px-2 py-0.5 rounded-full bg-m3-error-container text-[11px] font-mono font-medium text-m3-on-error-container border border-m3-error/40 animate-pulse"
          >
            冲突
          </span>
          <span
            v-else-if="lanStatus.role === 'server'"
            class="px-2 py-0.5 rounded-full bg-m3-primary-container text-[11px] font-mono font-medium text-m3-on-primary-container border border-m3-primary/40"
          >
            房主
          </span>
          <span
            v-else-if="lanStatus.activeServer && syncDiff?.isInSync"
            class="px-2 py-0.5 rounded-full bg-m3-success-container text-[11px] font-mono font-medium text-m3-on-success-container border border-m3-success/40"
          >
            已同步
          </span>
          <span
            v-else-if="lanStatus.activeServer && !syncDiff?.isInSync"
            class="px-2 py-0.5 rounded-full bg-m3-warning-container text-[11px] font-mono font-medium text-m3-on-warning-container border border-m3-warning/40"
          >
            待同步
          </span>
        </button>

        <button
          @click="currentTab = 'launcher'"
          class="flex items-center gap-2.5 py-3 px-4 border-b-2 text-xs font-medium transition"
          :class="currentTab === 'launcher' ? 'border-m3-primary text-m3-primary font-semibold' : 'border-transparent text-m3-on-surface-variant hover:text-m3-on-surface hover:bg-m3-surface-container/40 rounded-t-lg'"
        >
          <Sliders class="w-4 h-4" />
          <span>启动调优与高刷 (Launcher)</span>
          <span class="px-2 py-0.5 rounded-full bg-m3-surface-container text-[11px] font-mono font-medium text-m3-primary border border-m3-outline">
            {{ appConfig.gamePreferences.highRefreshRate ? '144Hz+' : '60Hz' }}
          </span>
        </button>

        <button
          @click="currentTab = 'settings'"
          class="flex items-center gap-2.5 py-3 px-4 border-b-2 text-xs font-medium transition"
          :class="currentTab === 'settings' ? 'border-m3-primary text-m3-primary font-semibold' : 'border-transparent text-m3-on-surface-variant hover:text-m3-on-surface hover:bg-m3-surface-container/40 rounded-t-lg'"
        >
          <Settings class="w-4 h-4" />
          <span>环境自检 (Settings)</span>
        </button>
      </div>
    </nav>

    <!-- 主体内容区域 -->
    <main class="flex-1 overflow-hidden relative">
      <!-- 1. 单位模组管理 Tab -->
      <section v-if="currentTab === 'units'" class="h-full flex flex-col p-6 overflow-hidden">
        <!-- 搜索与筛选操作栏 -->
        <div class="flex flex-wrap items-center justify-between gap-4 mb-4 shrink-0">
          <div class="flex items-center gap-3 flex-1 max-w-md">
            <div class="relative w-full">
              <Search class="w-4 h-4 text-m3-on-surface-variant absolute left-3.5 top-2.5" />
              <input
                v-model="modSearch"
                type="text"
                placeholder="搜索模组标题、标签、文件名..."
                class="w-full bg-m3-surface-high border border-m3-outline rounded-full pl-10 pr-4 py-2 text-xs text-m3-on-surface placeholder-m3-on-surface-dim focus:outline-none focus:border-m3-primary transition"
              />
            </div>
          </div>

          <!-- 状态筛选单选与视图切换按钮 -->
          <div class="flex items-center gap-2.5 text-xs">
            <!-- 视图切换 (卡片网格 vs 紧凑列表) (M3 Segmented) -->
            <div class="flex rounded-xl bg-m3-surface-container p-0.5 border border-m3-outline">
              <button
                @click="modViewMode = 'grid'"
                class="p-1.5 rounded-lg transition"
                :class="modViewMode === 'grid' ? 'bg-m3-surface-highest text-m3-primary shadow-sm' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
                title="卡片网格视图"
              >
                <LayoutGrid class="w-4 h-4" />
              </button>
              <button
                @click="modViewMode = 'list'"
                class="p-1.5 rounded-lg transition"
                :class="modViewMode === 'list' ? 'bg-m3-surface-highest text-m3-primary shadow-sm' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
                title="紧凑列表视图"
              >
                <List class="w-4 h-4" />
              </button>
            </div>

            <!-- 状态筛选 (M3 Segmented Control) -->
            <div class="flex rounded-xl bg-m3-surface-container p-0.5 border border-m3-outline">
              <button
                @click="modFilterStatus = 'all'"
                class="px-3 py-1.5 rounded-lg font-medium transition"
                :class="modFilterStatus === 'all' ? 'bg-m3-surface-highest text-m3-on-surface font-semibold shadow-sm' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
              >
                全部 ({{ mods.length }})
              </button>
              <button
                @click="modFilterStatus = 'enabled'"
                class="px-3 py-1.5 rounded-lg font-medium transition"
                :class="modFilterStatus === 'enabled' ? 'bg-m3-surface-highest text-m3-success font-semibold shadow-sm' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
              >
                已启用 ({{ enabledCount }})
              </button>
              <button
                @click="modFilterStatus = 'disabled'"
                class="px-3 py-1.5 rounded-lg font-medium transition"
                :class="modFilterStatus === 'disabled' ? 'bg-m3-surface-highest text-m3-error font-semibold shadow-sm' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
              >
                已禁用 ({{ mods.length - enabledCount }})
              </button>
            </div>

            <div class="h-4 w-px bg-m3-outline" />

            <button
              @click="handleBatchToggle(true)"
              class="px-3 py-1.5 bg-m3-surface-container hover:bg-m3-surface-high text-m3-success border border-m3-outline rounded-xl font-medium transition"
            >
              全部启用
            </button>
            <button
              @click="handleBatchToggle(false)"
              class="px-3 py-1.5 bg-m3-surface-container hover:bg-m3-surface-high text-m3-error border border-m3-outline rounded-xl font-medium transition"
            >
              全部禁用
            </button>
          </div>
        </div>

        <!-- 模组展示区: 1. 网格模式 -->
        <div v-if="modViewMode === 'grid'" class="flex-1 overflow-y-auto pr-1.5">
          <div v-if="filteredMods.length === 0" class="flex flex-col items-center justify-center h-64 text-m3-on-surface-dim">
            <Package class="w-12 h-12 stroke-[1.5] mb-2.5 opacity-40 text-m3-on-surface-variant" />
            <p class="text-xs font-medium">没有找到匹配的模组</p>
          </div>

          <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 pb-6">
            <div
              v-for="item in filteredMods"
              :key="item.fileName"
              class="group flex flex-col bg-m3-surface-container border rounded-2xl overflow-hidden transition-all duration-200"
              :class="item.enabled ? 'border-m3-outline-variant hover:border-m3-outline hover:bg-m3-surface-high hover:shadow-m3-1' : 'border-m3-outline-variant/60 opacity-75 hover:opacity-100 hover:bg-m3-surface-high'"
            >
              <!-- 封面图 -->
              <div class="relative h-36 bg-m3-surface-dim overflow-hidden shrink-0 flex items-center justify-center border-b border-m3-outline-variant">
                <img
                  v-if="item.thumbnail"
                  :src="item.thumbnail"
                  class="w-full h-full object-cover transition group-hover:scale-105 duration-300"
                  alt="Mod thumbnail"
                />
                <div v-else class="flex flex-col items-center justify-center text-m3-on-surface-dim gap-1.5">
                  <Package class="w-9 h-9 stroke-[1.2]" />
                  <span class="text-[10px] font-mono font-semibold tracking-wider">NO THUMBNAIL</span>
                </div>

                <!-- 启/禁用状态胶囊 (M3 Status Badge) -->
                <div class="absolute top-2.5 left-2.5">
                  <span
                    class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-medium backdrop-blur-md shadow-sm"
                    :class="item.enabled ? 'bg-m3-success-container/95 text-m3-on-success-container border border-m3-success/30' : 'bg-m3-surface-highest/95 text-m3-on-surface-variant border border-m3-outline'"
                  >
                    <span class="w-1.5 h-1.5 rounded-full" :class="item.enabled ? 'bg-m3-success' : 'bg-m3-on-surface-dim'" />
                    {{ item.enabled ? '已启用' : '已禁用' }}
                  </span>
                </div>

                <!-- 版本号徽章 -->
                <div v-if="item.minVersion" class="absolute top-2.5 right-2.5">
                  <span class="px-2 py-0.5 rounded-md text-[10px] font-mono font-medium bg-black/70 backdrop-blur-md text-m3-on-surface-variant border border-white/10 shadow-sm">
                    {{ item.minVersion }}
                  </span>
                </div>
              </div>

              <!-- 模组信息主体 -->
              <div class="flex-1 p-4 flex flex-col justify-between">
                <div>
                  <h3 class="text-sm font-semibold text-m3-on-surface line-clamp-1 group-hover:text-m3-primary transition" :title="item.titleZh || item.title">
                    {{ item.titleZh || item.title }}
                  </h3>

                  <p class="text-[11px] text-m3-on-surface-dim font-mono truncate mt-0.5" :title="item.fileName">
                    {{ item.fileName }}
                  </p>

                  <p class="text-xs text-m3-on-surface-variant line-clamp-2 mt-2 leading-relaxed">
                    {{ item.description || '暂无模组描述说明。' }}
                  </p>

                  <!-- 标签分类 (M3 Tag Chips) -->
                  <div v-if="item.tags && item.tags.length" class="flex flex-wrap gap-1 mt-3">
                    <span
                      v-for="t in item.tags.slice(0, 3)"
                      :key="t"
                      class="px-2 py-0.5 rounded-md text-[11px] font-medium bg-m3-surface-highest text-m3-secondary border border-m3-outline-variant"
                    >
                      {{ t }}
                    </span>
                  </div>
                </div>

                <!-- 底部统计与开关操作 -->
                <div class="mt-4 pt-3 border-t border-m3-outline-variant flex items-center justify-between">
                  <div class="flex items-center gap-3 text-xs text-m3-on-surface-variant font-mono">
                    <span title="单位配置数 (*.ini)" class="flex items-center gap-1.5">
                      <Cpu class="w-3.5 h-3.5 text-m3-primary" />
                      {{ item.unitCount }}
                    </span>
                    <span v-if="item.mapCount > 0" title="内嵌地图数 (*.tmx)" class="flex items-center gap-1.5">
                      <MapIcon class="w-3.5 h-3.5 text-m3-success" />
                      {{ item.mapCount }}
                    </span>
                    <span title="文件体积" class="text-m3-on-surface-dim">
                      {{ formatBytes(item.fileSize) }}
                    </span>
                  </div>

                  <div class="flex items-center gap-1.5">
                    <!-- 客户端向房主推送此模组 -->
                    <button
                      v-if="lanStatus.role === 'client'"
                      @click="openPushModal('mod', item.fileName, item.fileSize)"
                      class="px-2.5 py-1 rounded-lg text-xs font-medium border border-m3-outline bg-m3-surface-highest/60 hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface transition flex items-center gap-1"
                      :title="lanStatus.activeServer ? '将此模组推送给房主服务端' : '向房主推送此模组 (待房主上线)'"
                    >
                      <UploadCloud class="w-3.5 h-3.5" />
                      <span>推送</span>
                    </button>

                    <button
                      v-if="item.steamId"
                      @click="openWorkshopLink(item.steamId)"
                      class="p-1.5 text-m3-on-surface-dim hover:text-m3-primary rounded-lg transition"
                      title="在 Steam 创意工坊查看"
                    >
                      <ExternalLink class="w-4 h-4" />
                    </button>

                    <!-- M3 Switch 开关 -->
                    <button
                      @click="handleToggleMod(item)"
                      class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
                      :class="item.enabled ? 'bg-m3-primary' : 'bg-m3-surface-highest border border-m3-outline'"
                    >
                      <span
                        class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-sm transition duration-200 ease-in-out"
                        :class="item.enabled ? 'translate-x-5' : 'translate-x-0'"
                      />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 模组展示区: 2. 紧凑列表模式 -->
        <div v-else class="flex-1 overflow-y-auto pr-1.5">
          <div v-if="filteredMods.length === 0" class="flex flex-col items-center justify-center h-64 text-m3-on-surface-dim">
            <Package class="w-12 h-12 stroke-[1.5] mb-2.5 opacity-40 text-m3-on-surface-variant" />
            <p class="text-xs font-medium">没有找到匹配的模组</p>
          </div>

          <div v-else class="space-y-2 pb-6">
            <div
              v-for="item in filteredMods"
              :key="item.fileName"
              class="flex items-center justify-between p-3 bg-m3-surface-container border border-m3-outline-variant hover:border-m3-outline rounded-xl transition shadow-sm"
              :class="item.enabled ? 'bg-m3-surface-container' : 'opacity-70 hover:opacity-90'"
            >
              <!-- 左侧：开关 + 缩略图 + 标题与标签 -->
              <div class="flex items-center gap-3.5 flex-1 min-w-0 pr-4">
                <button
                  @click="handleToggleMod(item)"
                  class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
                  :class="item.enabled ? 'bg-m3-primary' : 'bg-m3-surface-highest border border-m3-outline'"
                >
                  <span
                    class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-sm transition duration-200 ease-in-out"
                    :class="item.enabled ? 'translate-x-5' : 'translate-x-0'"
                  />
                </button>

                <div class="w-11 h-11 rounded-lg bg-m3-surface-dim border border-m3-outline-variant overflow-hidden shrink-0 flex items-center justify-center">
                  <img v-if="item.thumbnail" :src="item.thumbnail" class="w-full h-full object-cover" alt="" />
                  <Package v-else class="w-5 h-5 text-m3-on-surface-dim" />
                </div>

                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <h3 class="text-xs font-semibold text-m3-on-surface truncate" :title="item.titleZh || item.title">
                      {{ item.titleZh || item.title }}
                    </h3>
                    <span v-if="item.minVersion" class="px-1.5 py-0.2 rounded text-[10px] font-mono bg-m3-surface-highest text-m3-on-surface-variant border border-m3-outline-variant">
                      {{ item.minVersion }}
                    </span>
                    <span
                      class="px-2 py-0.2 rounded-full text-[10px] font-medium"
                      :class="item.enabled ? 'bg-m3-success-container text-m3-on-success-container' : 'bg-m3-surface-highest text-m3-on-surface-dim'"
                    >
                      {{ item.enabled ? '已启用' : '已禁用' }}
                    </span>
                  </div>
                  <p class="text-[11px] text-m3-on-surface-dim font-mono truncate mt-0.5" :title="item.fileName">
                    {{ item.fileName }}
                  </p>
                </div>

                <!-- 标签 (在大屏幕展示) -->
                <div v-if="item.tags && item.tags.length" class="hidden xl:flex items-center gap-1.5 shrink-0">
                  <span
                    v-for="t in item.tags.slice(0, 2)"
                    :key="t"
                    class="px-2 py-0.5 rounded text-[11px] font-medium bg-m3-surface-highest text-m3-secondary border border-m3-outline-variant"
                  >
                    {{ t }}
                  </span>
                </div>
              </div>

              <!-- 右侧：统计数据与操作 -->
              <div class="flex items-center gap-5 shrink-0 text-xs font-mono text-m3-on-surface-variant">
                <span title="单位数量" class="flex items-center gap-1.5">
                  <Cpu class="w-3.5 h-3.5 text-m3-primary" />
                  {{ item.unitCount }}
                </span>
                <span v-if="item.mapCount > 0" title="内嵌地图" class="flex items-center gap-1.5">
                  <MapIcon class="w-3.5 h-3.5 text-m3-success" />
                  {{ item.mapCount }}
                </span>
                <span class="text-m3-on-surface-dim w-16 text-right">
                  {{ formatBytes(item.fileSize) }}
                </span>
                <!-- 客户端向房主推送此模组 -->
                <button
                  v-if="lanStatus.role === 'client'"
                  @click="openPushModal('mod', item.fileName, item.fileSize)"
                  class="px-2.5 py-1 rounded-lg text-xs font-medium border border-m3-outline bg-m3-surface-highest/60 hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface transition flex items-center gap-1"
                  :title="lanStatus.activeServer ? '将此模组推送给房主服务端' : '向房主推送此模组 (待房主上线)'"
                >
                  <UploadCloud class="w-3.5 h-3.5" />
                  <span>推送</span>
                </button>
                <button
                  v-if="item.steamId"
                  @click="openWorkshopLink(item.steamId)"
                  class="p-1.5 text-m3-on-surface-dim hover:text-m3-primary rounded-lg transition"
                  title="在 Steam 创意工坊查看"
                >
                  <ExternalLink class="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 2. 地图工坊 Tab (全量汇总与激活联动) -->
      <section v-if="currentTab === 'maps'" class="h-full flex flex-col p-6 overflow-hidden">
        <!-- 搜索与多维度筛选栏 -->
        <div class="flex flex-wrap items-center justify-between gap-4 mb-4 shrink-0">
          <div class="flex items-center gap-3 flex-1 max-w-md">
            <div class="relative w-full">
              <Search class="w-4 h-4 text-m3-on-surface-variant absolute left-3.5 top-2.5" />
              <input
                v-model="mapSearch"
                type="text"
                placeholder="搜索地图名称、地图包名、所属模组..."
                class="w-full bg-m3-surface-high border border-m3-outline rounded-full pl-10 pr-4 py-2 text-xs text-m3-on-surface placeholder-m3-on-surface-dim focus:outline-none focus:border-m3-primary transition"
              />
            </div>
          </div>

          <!-- 激活状态与来源筛选 -->
          <div class="flex flex-wrap items-center gap-2.5 text-xs">
            <!-- 激活状态 (M3 Segmented Button) -->
            <div class="flex rounded-xl bg-m3-surface-container p-0.5 border border-m3-outline">
              <button
                @click="mapFilterActive = 'all'"
                class="px-3 py-1.5 rounded-lg font-medium transition"
                :class="mapFilterActive === 'all' ? 'bg-m3-surface-highest text-m3-on-surface font-semibold shadow-sm' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
              >
                全部 ({{ maps.length }})
              </button>
              <button
                @click="mapFilterActive = 'active'"
                class="px-3 py-1.5 rounded-lg font-medium transition"
                :class="mapFilterActive === 'active' ? 'bg-m3-surface-highest text-m3-success font-semibold shadow-sm' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
              >
                可用 ({{ activeMapCount }})
              </button>
              <button
                @click="mapFilterActive = 'inactive'"
                class="px-3 py-1.5 rounded-lg font-medium transition"
                :class="mapFilterActive === 'inactive' ? 'bg-m3-surface-highest text-m3-error font-semibold shadow-sm' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
              >
                未激活 ({{ maps.length - activeMapCount }})
              </button>
            </div>

            <!-- 来源分组下拉表单 (M3 Outlined Select) -->
            <div class="flex items-center gap-2 bg-m3-surface-container px-3 py-1.5 rounded-xl border border-m3-outline">
              <Layers class="w-3.5 h-3.5 text-m3-primary shrink-0" />
              <label class="text-xs text-m3-on-surface-variant font-medium shrink-0">分组：</label>
              <select
                v-model="selectedMapGroup"
                class="bg-transparent text-xs text-m3-on-surface font-medium focus:outline-none cursor-pointer max-w-[220px] truncate"
              >
                <option value="all" class="bg-m3-surface-high text-m3-on-surface">全部地图</option>
                <optgroup label="独立地图 (mods/maps/*.tmx)" class="bg-m3-surface-high text-m3-on-surface-dim">
                  <option value="standalone" class="bg-m3-surface-high text-m3-on-surface">
                    独立单张地图 ({{ standaloneMapCount }} 张)
                  </option>
                </optgroup>
                <optgroup label="聚合地图包 (mods/maps/*.rwmod)" class="bg-m3-surface-high text-m3-on-surface-dim">
                  <option value="all_map_packs" class="bg-m3-surface-high text-m3-on-surface">
                    全部聚合地图包 ({{ totalPackMapsCount }} 张)
                  </option>
                  <option
                    v-for="p in availableMapPacks"
                    :key="p.name"
                    :value="`pack:${p.name}`"
                    class="bg-m3-surface-high text-m3-on-surface"
                  >
                    &nbsp;&nbsp;└ {{ p.name }} ({{ p.count }} 张)
                  </option>
                </optgroup>
                <optgroup label="单位模组自带地图 (mods/units/*.rwmod)" class="bg-m3-surface-high text-m3-on-surface-dim">
                  <option value="all_unit_mods" class="bg-m3-surface-high text-m3-on-surface">
                    全部内嵌地图 ({{ totalModMapsCount }} 张)
                  </option>
                  <option
                    v-for="um in availableUnitModsWithMaps"
                    :key="um.name"
                    :value="`mod:${um.name}`"
                    class="bg-m3-surface-high text-m3-on-surface"
                  >
                    &nbsp;&nbsp;└ {{ um.name }} ({{ um.count }} 张) [{{ um.active ? '已激活' : '未激活' }}]
                  </option>
                </optgroup>
              </select>
            </div>

            <!-- 批量缩略图与出生点生成按钮组 -->
            <div class="flex items-center gap-2">
              <button
                v-if="missingPreviewCount > 0"
                @click="handleBatchGenerateThumbs(false)"
                :disabled="generatingThumb"
                class="flex items-center gap-1.5 px-3 py-1.5 bg-m3-warning-container text-m3-on-warning-container border border-m3-warning/30 rounded-xl text-xs font-medium transition hover:brightness-110 disabled:opacity-40"
                title="批量为所有缺图地图生成高清预览及队伍出生点"
              >
                <Sparkles class="w-3.5 h-3.5 text-m3-warning" :class="{ 'animate-spin': generatingThumb }" />
                <span>补充缺图 ({{ missingPreviewCount }})</span>
              </button>

              <button
                @click="handleBatchGenerateThumbs(true)"
                :disabled="generatingThumb"
                class="flex items-center gap-1.5 px-3 py-1.5 bg-m3-surface-container hover:bg-m3-surface-high text-m3-primary border border-m3-outline rounded-xl text-xs font-medium transition disabled:opacity-40 shadow-sm"
                title="重新分析所有地图并绘制精确队伍出生点标号"
              >
                <Sparkles class="w-3.5 h-3.5 text-m3-primary" :class="{ 'animate-spin': generatingThumb }" />
                <span>全量重绘出生点</span>
              </button>
            </div>
          </div>
        </div>

        <!-- 地图展示区 (分板块呈现) -->
        <div class="flex-1 overflow-y-auto pr-1.5 space-y-6">
          <div v-if="filteredMaps.length === 0" class="flex flex-col items-center justify-center h-64 text-m3-on-surface-dim">
            <MapIcon class="w-12 h-12 stroke-[1.5] mb-2.5 opacity-40 text-m3-on-surface-variant" />
            <p class="text-xs font-medium">没有找到匹配的地图</p>
          </div>

          <div v-else class="space-y-6 pb-8">
            <!-- 1. 独立 TMX 地图板块 -->
            <section
              v-if="filteredStandaloneMaps.length > 0"
              class="bg-m3-surface-container border border-m3-outline-variant rounded-2xl p-4 space-y-3.5"
            >
              <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
                <div class="flex items-center gap-2.5">
                  <div class="w-7 h-7 rounded-lg bg-m3-surface-highest text-m3-primary flex items-center justify-center font-bold">
                    <Folder class="w-4 h-4" />
                  </div>
                  <div>
                    <h3 class="text-xs font-bold text-m3-on-surface flex items-center gap-2">
                      独立地图 (mods/maps/)
                      <span class="px-2 py-0.5 rounded-full text-[10px] bg-m3-surface-highest text-m3-on-surface-variant font-medium border border-m3-outline-variant">
                        {{ filteredStandaloneMaps.length }} 张地图
                      </span>
                    </h3>
                    <p class="text-[11px] text-m3-on-surface-dim">直接放置在 mods/maps/ 目录下的独立 TMX 文件，战局中常驻可用</p>
                  </div>
                </div>
              </div>

              <!-- 独立地图网格 -->
              <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6 gap-3">
                <div
                  v-for="m in filteredStandaloneMaps"
                  :key="m.id"
                  class="group flex flex-col bg-m3-surface-high border border-m3-outline-variant hover:border-m3-outline rounded-xl overflow-hidden transition shadow-sm hover:shadow-m3-1"
                >
                  <!-- 缩略图区域 (添加原生懒加载与异步解码，杜绝瞬时内存过载) -->
                  <div
                    class="relative aspect-square bg-m3-surface-dim flex items-center justify-center overflow-hidden border-b border-m3-outline-variant cursor-pointer"
                    @click="openMapLightbox(m)"
                    title="点击放大查看高清预览图与出生点标记"
                  >
                    <img
                      v-if="m.hasPreview && m.previewData"
                      :src="m.previewData"
                      loading="lazy"
                      decoding="async"
                      class="w-full h-full object-cover transition group-hover:scale-105 duration-200"
                      alt="Map Preview"
                    />
                    <div v-else class="flex flex-col items-center justify-center text-m3-warning gap-1.5 p-3 text-center">
                      <ShieldAlert class="w-7 h-7 stroke-[1.5]" />
                      <span class="text-[10px] font-medium leading-tight">缺失缩略图</span>
                      <button
                        @click.stop="handleGenerateMapThumb(m)"
                        :disabled="generatingThumb"
                        class="mt-1 px-2 py-0.5 bg-m3-warning-container text-m3-on-warning-container rounded-md text-[10px] font-medium border border-m3-warning/30"
                      >
                        生成缩略图
                      </button>
                    </div>

                    <!-- 卡片右上角快捷重绘按钮 -->
                    <button
                      v-if="m.hasPreview"
                      @click.stop="handleGenerateMapThumb(m)"
                      :disabled="generatingThumb"
                      class="absolute top-2 right-2 p-1.5 bg-black/60 hover:bg-m3-primary text-white rounded-lg backdrop-blur-md opacity-0 group-hover:opacity-100 transition shadow-sm"
                      title="重新计算并绘制带队伍出生点标记的高清预览图"
                    >
                      <Sparkles class="w-3.5 h-3.5" :class="{ 'animate-spin': generatingThumb }" />
                    </button>

                    <!-- 左上角状态指示 -->
                    <div class="absolute top-2 left-2">
                      <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-medium backdrop-blur-md shadow-sm bg-m3-success-container/95 text-m3-on-success-container border border-m3-success/30">
                        <span class="w-1.5 h-1.5 rounded-full bg-m3-success" />
                        常驻可用
                      </span>
                    </div>

                    <!-- 玩家人数徽章 -->
                    <div class="absolute bottom-2 left-2">
                      <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono font-medium bg-black/70 backdrop-blur-md text-m3-on-surface border border-white/10 shadow-sm">
                        <Users class="w-3 h-3 text-m3-success" />
                        {{ m.playerCount }}P
                      </span>
                    </div>

                    <!-- 尺寸徽章 -->
                    <div v-if="m.width > 0" class="absolute bottom-2 right-2">
                      <span class="px-1.5 py-0.5 rounded text-[10px] font-mono font-medium bg-black/70 backdrop-blur-md text-m3-on-surface-variant border border-white/10 shadow-sm">
                        {{ m.width }}×{{ m.height }}
                      </span>
                    </div>
                  </div>

                  <!-- 信息主体 -->
                  <div class="p-2.5 flex-1 flex flex-col justify-between">
                    <div>
                      <h4 class="text-xs font-medium text-m3-on-surface line-clamp-1 group-hover:text-m3-primary transition" :title="m.title">
                        {{ m.title }}
                      </h4>
                      <div class="mt-0.5">
                        <span class="text-[10px] text-m3-on-surface-dim font-mono">独立 TMX</span>
                      </div>
                    </div>

                    <div class="mt-2 pt-2 border-t border-m3-outline-variant flex items-center justify-between text-xs">
                      <span class="text-m3-on-surface-dim font-mono text-[10px]">
                        {{ formatBytes(m.fileSize) }}
                      </span>
                      <div class="flex items-center gap-1">
                        <button
                          v-if="lanStatus.role === 'client'"
                          @click="openPushModal('map', m.fileName, m.fileSize)"
                          class="px-2 py-0.5 rounded-md text-[10px] font-medium border border-m3-outline bg-m3-surface-highest/60 hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface transition flex items-center gap-1"
                          :title="lanStatus.activeServer ? '将此地图推送给房主' : '向房主推送此地图 (待房主上线)'"
                        >
                          <UploadCloud class="w-3 h-3" /> 推送
                        </button>
                        <span class="text-m3-success text-[10px] font-medium flex items-center gap-1">
                          <Check class="w-3 h-3" /> 就绪
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <!-- 2. 聚合地图包板块 -->
            <section
              v-if="groupedMapPacks.length > 0"
              class="space-y-4"
            >
              <div class="flex items-center gap-2 px-1">
                <Package class="w-4 h-4 text-m3-success" />
                <h3 class="text-xs font-bold text-m3-on-surface">
                  聚合地图包 (mods/maps/*.rwmod)
                  <span class="text-xs text-m3-on-surface-dim font-normal ml-1">
                    (共 {{ groupedMapPacks.length }} 个聚合包)
                  </span>
                </h3>
              </div>

              <!-- 每个地图包独立容器 -->
              <div
                v-for="pack in groupedMapPacks"
                :key="pack.packName"
                class="bg-m3-surface-container border border-m3-outline-variant rounded-2xl p-4 space-y-3.5"
              >
                <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3 flex-wrap gap-2">
                  <div class="flex items-center gap-2.5 cursor-pointer select-none" @click="toggleGroupExpand(`pack:${pack.packName}`)">
                    <div class="w-7 h-7 rounded-lg bg-m3-surface-highest text-m3-success flex items-center justify-center font-bold">
                      <Package class="w-4 h-4" />
                    </div>
                    <div>
                      <h4 class="text-xs font-bold text-m3-on-surface flex items-center gap-2">
                        {{ pack.packName }}
                        <span class="px-2 py-0.5 rounded-full text-[10px] bg-m3-surface-highest text-m3-on-surface-variant font-medium border border-m3-outline-variant">
                          {{ pack.count }} 张地图
                        </span>
                        <span class="text-[11px] font-mono text-m3-on-surface-dim font-normal">
                          ({{ formatBytes(pack.size) }})
                        </span>
                      </h4>
                      <p class="text-[11px] text-m3-on-surface-dim font-mono">归档路径：mods/maps/{{ pack.packName }} · 战局常驻生效</p>
                    </div>
                  </div>

                  <div class="flex items-center gap-2">
                    <button
                      @click="toggleGroupExpand(`pack:${pack.packName}`)"
                      class="px-2.5 py-1 rounded-lg text-xs font-medium border border-m3-outline bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface transition flex items-center gap-1"
                    >
                      <span>{{ isGroupExpanded(`pack:${pack.packName}`, true) ? '收起' : `展开 (${pack.count})` }}</span>
                      <ChevronDown v-if="isGroupExpanded(`pack:${pack.packName}`, true)" class="w-3.5 h-3.5" />
                      <ChevronRight v-else class="w-3.5 h-3.5" />
                    </button>
                    <button
                      v-if="lanStatus.role === 'client'"
                      @click="openPushModal('map', pack.packName, pack.size)"
                      class="px-2.5 py-1 rounded-lg text-xs font-medium border border-m3-outline bg-m3-surface-highest/60 hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface transition flex items-center gap-1.5"
                      :title="lanStatus.activeServer ? '将整个地图包推送给房主' : '向房主推送此地图包 (待房主上线)'"
                    >
                      <UploadCloud class="w-3.5 h-3.5" />
                      <span>推送此包</span>
                    </button>
                  </div>
                </div>

                <!-- 该包内地图网格 (受折叠保护) -->
                <div
                  v-if="isGroupExpanded(`pack:${pack.packName}`, true)"
                  class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6 gap-3 pt-1"
                >
                  <div
                    v-for="m in pack.maps"
                    :key="m.id"
                    class="group flex flex-col bg-m3-surface-high border border-m3-outline-variant hover:border-m3-outline rounded-xl overflow-hidden transition shadow-sm hover:shadow-m3-1"
                  >
                    <!-- 缩略图区域 -->
                    <div
                      class="relative aspect-square bg-m3-surface-dim flex items-center justify-center overflow-hidden border-b border-m3-outline-variant cursor-pointer"
                      @click="openMapLightbox(m)"
                      title="点击放大查看高清预览图与出生点标记"
                    >
                      <img
                        v-if="m.hasPreview && m.previewData"
                        :src="m.previewData"
                        loading="lazy"
                        decoding="async"
                        class="w-full h-full object-cover transition group-hover:scale-105 duration-200"
                        alt="Map Preview"
                      />
                      <div v-else class="flex flex-col items-center justify-center text-m3-warning gap-1.5 p-3 text-center">
                        <ShieldAlert class="w-7 h-7 stroke-[1.5]" />
                        <span class="text-[10px] font-medium leading-tight">缺失缩略图</span>
                        <button
                          @click.stop="handleGenerateMapThumb(m)"
                          :disabled="generatingThumb"
                          class="mt-1 px-2 py-0.5 bg-m3-warning-container text-m3-on-warning-container rounded-md text-[10px] font-medium border border-m3-warning/30"
                        >
                          生成缩略图
                        </button>
                      </div>

                      <button
                        v-if="m.hasPreview"
                        @click.stop="handleGenerateMapThumb(m)"
                        :disabled="generatingThumb"
                        class="absolute top-2 right-2 p-1.5 bg-black/60 hover:bg-m3-primary text-white rounded-lg backdrop-blur-md opacity-0 group-hover:opacity-100 transition shadow-sm"
                        title="重新计算并绘制带队伍出生点标记的高清预览图"
                      >
                        <Sparkles class="w-3.5 h-3.5" :class="{ 'animate-spin': generatingThumb }" />
                      </button>

                      <div class="absolute top-2 left-2">
                        <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium backdrop-blur-md shadow-sm bg-m3-success-container/95 text-m3-on-success-container border border-m3-success/30">
                          <span class="w-1.5 h-1.5 rounded-full bg-m3-success" />
                          包内就绪
                        </span>
                      </div>

                      <div class="absolute bottom-2 left-2">
                        <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono font-medium bg-black/70 backdrop-blur-md text-m3-on-surface border border-white/10 shadow-sm">
                          <Users class="w-3 h-3 text-m3-success" />
                          {{ m.playerCount }}P
                        </span>
                      </div>

                      <div v-if="m.width > 0" class="absolute bottom-2 right-2">
                        <span class="px-1.5 py-0.5 rounded text-[10px] font-mono font-medium bg-black/70 backdrop-blur-md text-m3-on-surface-variant border border-white/10 shadow-sm">
                          {{ m.width }}×{{ m.height }}
                        </span>
                      </div>
                    </div>

                    <!-- 信息主体 -->
                    <div class="p-2.5 flex-1 flex flex-col justify-between">
                      <div>
                        <h4 class="text-xs font-medium text-m3-on-surface line-clamp-1 group-hover:text-m3-primary transition" :title="m.title">
                          {{ m.title }}
                        </h4>
                        <div class="mt-0.5">
                          <span class="text-[10px] text-m3-on-surface-dim font-mono truncate block" :title="m.packName">
                            包内条目
                          </span>
                        </div>
                      </div>

                      <div class="mt-2 pt-2 border-t border-m3-outline-variant flex items-center justify-between text-xs">
                        <span class="text-m3-on-surface-dim font-mono text-[10px]">
                          {{ formatBytes(m.fileSize) }}
                        </span>
                        <span class="text-m3-success text-[10px] font-medium flex items-center gap-1">
                          <Check class="w-3 h-3" /> 就绪
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <!-- 3. 单位模组内嵌地图板块 -->
            <section
              v-if="groupedUnitModsWithMaps.length > 0"
              class="space-y-4"
            >
              <div class="flex items-center gap-2 px-1">
                <Cpu class="w-4 h-4 text-m3-primary" />
                <h3 class="text-xs font-bold text-m3-on-surface">
                  单位模组内嵌地图 (mods/units/*.rwmod)
                  <span class="text-xs text-m3-on-surface-dim font-normal ml-1">
                    (共 {{ groupedUnitModsWithMaps.length }} 个模组提供地图)
                  </span>
                </h3>
              </div>

              <!-- 每个模组独立容器 -->
              <div
                v-for="modGroup in groupedUnitModsWithMaps"
                :key="modGroup.parentModFile"
                class="bg-m3-surface-container border rounded-2xl p-4 space-y-3.5 transition"
                :class="modGroup.active ? 'border-m3-outline-variant' : 'border-m3-error/20 bg-m3-surface-container/60'"
              >
                <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3 flex-wrap gap-2">
                  <div class="flex items-center gap-2.5 cursor-pointer select-none" @click="toggleGroupExpand(`mod:${modGroup.parentModFile}`)">
                    <div
                      class="w-7 h-7 rounded-lg flex items-center justify-center font-bold"
                      :class="modGroup.active ? 'bg-m3-surface-highest text-m3-primary' : 'bg-m3-surface-highest text-m3-on-surface-dim'"
                    >
                      <Cpu class="w-4 h-4" />
                    </div>
                    <div>
                      <h4 class="text-xs font-bold text-m3-on-surface flex items-center gap-2">
                        {{ modGroup.parentModFile }}
                        <span class="px-2 py-0.5 rounded-full text-[10px] bg-m3-surface-highest text-m3-on-surface-variant font-medium border border-m3-outline-variant">
                          {{ modGroup.count }} 张地图
                        </span>
                        <span
                          class="px-2 py-0.5 rounded-full text-[10px] font-medium border"
                          :class="modGroup.active ? 'bg-m3-success-container text-m3-on-success-container border-m3-success/30' : 'bg-m3-surface-highest text-m3-on-surface-dim border-m3-outline'"
                        >
                          {{ modGroup.active ? '模组已启用' : '模组未启用' }}
                        </span>
                      </h4>
                      <p class="text-[11px] text-m3-on-surface-dim font-mono">宿主模组：mods/units/{{ modGroup.parentModFile }} · ({{ formatBytes(modGroup.size) }})</p>
                    </div>
                  </div>

                  <div class="flex items-center gap-2">
                    <button
                      @click="toggleGroupExpand(`mod:${modGroup.parentModFile}`)"
                      class="px-2.5 py-1.5 rounded-xl text-xs font-medium border border-m3-outline bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface transition flex items-center gap-1 shadow-sm"
                    >
                      <span>{{ isGroupExpanded(`mod:${modGroup.parentModFile}`, false) ? '收起地图' : `查看地图 (${modGroup.count})` }}</span>
                      <ChevronDown v-if="isGroupExpanded(`mod:${modGroup.parentModFile}`, false)" class="w-3.5 h-3.5" />
                      <ChevronRight v-else class="w-3.5 h-3.5" />
                    </button>
                    <button
                      @click="handleActivateParentMod(modGroup.parentModFile)"
                      class="px-3 py-1.5 rounded-xl text-xs font-medium border transition flex items-center gap-1.5"
                      :class="modGroup.active ? 'bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface border-m3-outline' : 'bg-m3-primary hover:bg-m3-primary-hover text-white border-transparent shadow-sm'"
                    >
                      <CheckCircle2 v-if="modGroup.active" class="w-3.5 h-3.5 text-m3-success" />
                      <span>{{ modGroup.active ? '禁用此模组' : '启用此模组' }}</span>
                    </button>
                    <button
                      v-if="lanStatus.role === 'client'"
                      @click="openPushModal('mod', modGroup.parentModFile, modGroup.size)"
                      class="px-2.5 py-1.5 rounded-xl text-xs font-medium border border-m3-outline bg-m3-surface-highest/60 hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface transition flex items-center gap-1.5"
                      :title="lanStatus.activeServer ? '将所属单位模组推送给房主' : '向房主推送此模组 (待房主上线)'"
                    >
                      <UploadCloud class="w-3.5 h-3.5" />
                      <span>推送</span>
                    </button>
                  </div>
                </div>

                <!-- 该模组附带地图网格 (受折叠保护，默认折叠，防止同时加载近300张卡片导致渲染黑屏) -->
                <div
                  v-if="isGroupExpanded(`mod:${modGroup.parentModFile}`, false)"
                  class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6 gap-3 pt-1"
                >
                  <div
                    v-for="m in modGroup.maps"
                    :key="m.id"
                    class="group flex flex-col bg-m3-surface-high border rounded-xl overflow-hidden transition shadow-sm hover:shadow-m3-1"
                    :class="m.active ? 'border-m3-outline-variant hover:border-m3-outline' : 'border-m3-outline-variant/60 opacity-60 hover:opacity-100'"
                  >
                    <!-- 缩略图区域 -->
                    <div
                      class="relative aspect-square bg-m3-surface-dim flex items-center justify-center overflow-hidden border-b border-m3-outline-variant cursor-pointer"
                      @click="openMapLightbox(m)"
                      title="点击放大查看高清预览图与出生点标记"
                    >
                      <img
                        v-if="m.hasPreview && m.previewData"
                        :src="m.previewData"
                        loading="lazy"
                        decoding="async"
                        class="w-full h-full object-cover transition group-hover:scale-105 duration-200"
                        alt="Map Preview"
                      />
                      <div v-else class="flex flex-col items-center justify-center text-m3-warning gap-1.5 p-3 text-center">
                        <ShieldAlert class="w-7 h-7 stroke-[1.5]" />
                        <span class="text-[10px] font-medium leading-tight">缺失缩略图</span>
                        <button
                          @click.stop="handleGenerateMapThumb(m)"
                          :disabled="generatingThumb"
                          class="mt-1 px-2 py-0.5 bg-m3-warning-container text-m3-on-warning-container rounded-md text-[10px] font-medium border border-m3-warning/30"
                        >
                          生成缩略图
                        </button>
                      </div>

                      <button
                        v-if="m.hasPreview"
                        @click.stop="handleGenerateMapThumb(m)"
                        :disabled="generatingThumb"
                        class="absolute top-2 right-2 p-1.5 bg-black/60 hover:bg-m3-primary text-white rounded-lg backdrop-blur-md opacity-0 group-hover:opacity-100 transition shadow-sm"
                        title="重新计算并绘制带队伍出生点标记的高清预览图"
                      >
                        <Sparkles class="w-3.5 h-3.5" :class="{ 'animate-spin': generatingThumb }" />
                      </button>

                      <div class="absolute top-2 left-2">
                        <span
                          class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-medium backdrop-blur-md shadow-sm"
                          :class="m.active ? 'bg-m3-success-container/95 text-m3-on-success-container border border-m3-success/30' : 'bg-m3-surface-highest/95 text-m3-on-surface-dim border border-m3-outline'"
                        >
                          <span class="w-1.5 h-1.5 rounded-full" :class="m.active ? 'bg-m3-success' : 'bg-m3-on-surface-dim'" />
                          {{ m.active ? '战局可用' : '未激活' }}
                        </span>
                      </div>

                      <div class="absolute bottom-2 left-2">
                        <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono font-medium bg-black/70 backdrop-blur-md text-m3-on-surface border border-white/10 shadow-sm">
                          <Users class="w-3 h-3 text-m3-success" />
                          {{ m.playerCount }}P
                        </span>
                      </div>

                      <div v-if="m.width > 0" class="absolute bottom-2 right-2">
                        <span class="px-1.5 py-0.5 rounded text-[10px] font-mono font-medium bg-black/70 backdrop-blur-md text-m3-on-surface-variant border border-white/10 shadow-sm">
                          {{ m.width }}×{{ m.height }}
                        </span>
                      </div>
                    </div>

                    <!-- 信息主体 -->
                    <div class="p-2.5 flex-1 flex flex-col justify-between">
                      <div>
                        <h4 class="text-xs font-medium text-m3-on-surface line-clamp-1 group-hover:text-m3-primary transition" :title="m.title">
                          {{ m.title }}
                        </h4>
                        <div class="mt-0.5">
                          <span class="text-[10px] text-m3-on-surface-dim font-mono truncate block" :title="m.parentModFile">
                            宿主: {{ m.parentModFile }}
                          </span>
                        </div>
                      </div>

                      <div class="mt-2 pt-2 border-t border-m3-outline-variant flex items-center justify-between text-xs">
                        <span class="text-m3-on-surface-dim font-mono text-[10px]">
                          {{ formatBytes(m.fileSize) }}
                        </span>
                        <div class="flex items-center gap-1">
                          <button
                            v-if="!m.active && m.parentModFile"
                            @click="handleActivateParentMod(m.parentModFile)"
                            class="px-2 py-0.5 bg-m3-primary-container text-m3-on-primary-container hover:bg-m3-primary hover:text-white rounded-md text-[10px] font-medium transition"
                            title="一键启用提供此地图的模组"
                          >
                            激活模组
                          </button>
                          <span v-else-if="m.active" class="text-m3-success text-[10px] font-medium flex items-center gap-1">
                            <Check class="w-3 h-3" /> 就绪
                          </span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </section>
          </div>
        </div>

        <!-- 3. 地图大图放大查看器 (Lightbox Modal - M3 Dialog) -->
        <div
          v-if="activeMapModal"
          class="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-6 animate-fade-in"
          @click.self="closeMapLightbox"
        >
          <div class="bg-m3-surface-container border border-m3-outline rounded-3xl max-w-4xl w-full max-h-[92vh] flex flex-col shadow-m3-3 overflow-hidden">
            <!-- 弹窗标题栏 -->
            <div class="flex items-center justify-between px-6 py-4 border-b border-m3-outline-variant bg-m3-surface-high">
              <div class="flex items-center gap-3">
                <div class="w-8 h-8 rounded-xl bg-m3-primary-container text-m3-primary flex items-center justify-center">
                  <MapIcon class="w-4 h-4 text-m3-on-primary-container" />
                </div>
                <div>
                  <h3 class="text-sm font-bold text-m3-on-surface flex items-center gap-2">
                    {{ activeMapModal.title }}
                    <span
                      class="px-2 py-0.5 rounded-full text-[11px] font-medium"
                      :class="activeMapModal.active ? 'bg-m3-success-container text-m3-on-success-container' : 'bg-m3-surface-highest text-m3-on-surface-dim'"
                    >
                      {{ activeMapModal.active ? '战局可用' : '未激活 (宿主模组已禁用)' }}
                    </span>
                  </h3>
                  <p class="text-[11px] text-m3-on-surface-dim font-mono mt-0.5">
                    {{ activeMapModal.fileName }}
                  </p>
                </div>
              </div>

              <!-- 缩放控制与关闭 (M3 Icon Buttons) -->
              <div class="flex items-center gap-1.5">
                <button
                  @click="zoomLevel = Math.max(0.5, zoomLevel - 0.25)"
                  class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-surface-highest text-m3-on-surface-variant transition border border-m3-outline"
                  title="缩小"
                >
                  <ZoomOut class="w-4 h-4" />
                </button>
                <span class="text-xs font-mono text-m3-on-surface-variant w-12 text-center">
                  {{ Math.round(zoomLevel * 100) }}%
                </span>
                <button
                  @click="zoomLevel = Math.min(3, zoomLevel + 0.25)"
                  class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-surface-highest text-m3-on-surface-variant transition border border-m3-outline"
                  title="放大"
                >
                  <ZoomIn class="w-4 h-4" />
                </button>
                <button
                  @click="zoomLevel = 1"
                  class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-surface-highest text-m3-on-surface-variant transition border border-m3-outline"
                  title="重置 1:1"
                >
                  <Maximize2 class="w-4 h-4" />
                </button>
                <div class="h-4 w-px bg-m3-outline mx-1" />
                <button
                  @click="closeMapLightbox"
                  class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-error-container hover:text-m3-on-error-container text-m3-on-surface-variant transition border border-m3-outline"
                  title="关闭"
                >
                  <X class="w-4 h-4" />
                </button>
              </div>
            </div>

            <!-- 图像展示区域 -->
            <div class="flex-1 overflow-auto bg-m3-surface-dim p-6 flex items-center justify-center min-h-[420px] max-h-[560px]">
              <div v-if="activeMapModal.hasPreview && activeMapModal.previewData" class="transition-transform duration-150">
                <img
                  :src="activeMapModal.previewData"
                  :style="{ transform: `scale(${zoomLevel})` }"
                  class="max-h-[500px] object-contain rounded-xl shadow-m3-2 origin-center transition-transform"
                  alt="High-Res Map Preview"
                />
              </div>
              <div v-else class="flex flex-col items-center justify-center text-m3-warning gap-3">
                <ShieldAlert class="w-14 h-14 stroke-[1.2]" />
                <p class="text-xs font-medium">该地图尚未生成带出生点的高清预览图</p>
                <button
                  @click="handleGenerateMapThumb(activeMapModal)"
                  :disabled="generatingThumb"
                  class="flex items-center gap-2 px-4 py-2 bg-m3-primary hover:bg-m3-primary-hover text-white rounded-xl text-xs font-medium shadow-m3-1 transition disabled:opacity-40"
                >
                  <Sparkles class="w-4 h-4" :class="{ 'animate-spin': generatingThumb }" />
                  <span>渲染高清缩略图与队伍出生点标记</span>
                </button>
              </div>
            </div>

            <!-- 底部地图详情与动作栏 -->
            <div class="px-6 py-4 bg-m3-surface-high border-t border-m3-outline-variant flex flex-wrap items-center justify-between gap-4 text-xs">
              <div class="flex items-center gap-5 text-m3-on-surface-variant font-mono">
                <span class="flex items-center gap-1.5 text-m3-success font-medium">
                  <Users class="w-4 h-4" />
                  {{ activeMapModal.playerCount }} 名玩家 (出生点 1~{{ activeMapModal.playerCount }})
                </span>
                <span v-if="activeMapModal.width > 0">
                  瓦片: {{ activeMapModal.width }} × {{ activeMapModal.height }}
                </span>
                <span v-if="activeMapModal.packName" class="text-m3-primary">
                  包: {{ activeMapModal.packName }}
                </span>
                <span>
                  大小: {{ formatBytes(activeMapModal.fileSize) }}
                </span>
              </div>

              <div class="flex items-center gap-2.5">
                <button
                  v-if="!activeMapModal.active && activeMapModal.parentModFile"
                  @click="handleActivateParentMod(activeMapModal.parentModFile)"
                  class="px-4 py-2 bg-m3-primary-container hover:bg-m3-primary text-m3-on-primary-container hover:text-white rounded-xl font-medium transition"
                >
                  激活宿主模组
                </button>

                <button
                  @click="handleGenerateMapThumb(activeMapModal)"
                  :disabled="generatingThumb"
                  class="flex items-center gap-1.5 px-4 py-2 bg-m3-surface-container hover:bg-m3-surface-highest text-m3-primary border border-m3-outline rounded-xl font-medium transition disabled:opacity-40"
                  title="使用精确出生点算法重新渲染并保存"
                >
                  <Sparkles class="w-4 h-4" />
                  <span>重绘出生点</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 局域网联机与模组预设同步 Tab (M3 Cards & Data Layout) -->
      <section v-if="currentTab === 'lan'" class="h-full overflow-y-auto p-8 max-w-5xl mx-auto">
        <div class="space-y-6 pb-12">
          <!-- 标题区域 -->
          <div>
            <h2 class="text-base font-bold text-m3-on-surface flex items-center gap-2.5">
              <Radio class="w-5 h-5 text-m3-primary" />
              局域网集群架构、模组预设同步与服务端仲裁
            </h2>
            <p class="text-xs text-m3-on-surface-variant mt-1 leading-relaxed">
              支持在局域网/特定子网内自动发现并对齐模组名单。同一网络集合内严格只允许存在 1 个服务端作为房主，支持 S 到 C 的模组/地图与启用状态全量同步，以及客户端主动向房主推送模组。
            </p>
          </div>

          <!-- 1. 节点角色切换与子网范围选择卡片 (M3 Card) -->
          <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-4">
            <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
              <div class="flex items-center gap-2">
                <Sliders class="w-4 h-4 text-m3-primary" />
                <label class="text-xs font-semibold text-m3-on-surface">集群节点角色与网络子网适配器</label>
              </div>
              <span class="text-xs text-m3-on-surface-dim font-mono">本机 IP: {{ lanStatus.localIp }}:{{ lanStatus.httpPort }}</span>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <!-- 角色切换选项 (M3 Segmented Card) -->
              <div class="p-3.5 bg-m3-surface-high border border-m3-outline-variant rounded-xl space-y-2.5">
                <span class="text-xs font-semibold text-m3-on-surface-variant block">本机联机角色模式</span>
                <div class="grid grid-cols-2 gap-2">
                  <button
                    @click="handleSwitchRole('client')"
                    class="py-2.5 px-3 rounded-xl border text-xs font-medium transition flex items-center justify-center gap-2"
                    :class="
                      lanStatus.role === 'client'
                        ? 'bg-m3-primary text-white border-transparent shadow-sm font-semibold'
                        : 'bg-m3-surface-container border-m3-outline text-m3-on-surface-variant hover:text-m3-on-surface'
                    "
                  >
                    <UserCheck class="w-4 h-4" />
                    <span>客户端 (玩家)</span>
                  </button>
                  <button
                    @click="handleSwitchRole('server')"
                    class="py-2.5 px-3 rounded-xl border text-xs font-medium transition flex items-center justify-center gap-2"
                    :class="
                      lanStatus.role === 'server'
                        ? 'bg-m3-primary text-white border-transparent shadow-sm font-semibold'
                        : 'bg-m3-surface-container border-m3-outline text-m3-on-surface-variant hover:text-m3-on-surface'
                    "
                  >
                    <Server class="w-4 h-4" />
                    <span>服务端 (房主)</span>
                  </button>
                </div>
                <p class="text-[11px] text-m3-on-surface-dim leading-relaxed">
                  {{
                    lanStatus.role === 'server'
                      ? '作为房主，您的模组启用名单与地图状态将作为全局“启动预设”，所有客户端将以此预设为准自动同步。'
                      : '作为客户端，联机模式启动时管理器将自动从房主下载缺失模组并将启用状态调整为与房主一致。'
                  }}
                </p>
              </div>

              <!-- 子网选择下拉框与设备别名 -->
              <div class="p-3.5 bg-m3-surface-high border border-m3-outline-variant rounded-xl space-y-2.5">
                <span class="text-xs font-semibold text-m3-on-surface-variant block">网络适配器与子网广播范围</span>
                <select
                  :value="lanConfig.selectedSubnet"
                  @change="handleSubnetChange"
                  class="w-full bg-m3-surface-container border border-m3-outline rounded-xl p-2.5 text-xs text-m3-on-surface focus:outline-none focus:border-m3-primary"
                >
                  <option value="">全部网络适配器 / 自动探测广播</option>
                  <option v-for="iface in interfaces" :key="iface.ip" :value="iface.subnet">
                    {{ iface.name }} - {{ iface.ip }} (广播: {{ iface.broadcastIp }})
                  </option>
                </select>

                <!-- 服务端专用：自动接受推送选项 -->
                <div v-if="lanStatus.role === 'server'" class="pt-2 border-t border-m3-outline-variant">
                  <label class="flex items-center justify-between cursor-pointer">
                    <span class="text-xs text-m3-on-surface font-medium">自动接受客户端推送的模组/地图</span>
                    <input
                      type="checkbox"
                      v-model="lanConfig.autoAcceptClientPushes"
                      @change="handleToggleAutoAccept"
                      class="w-4 h-4 rounded text-m3-primary focus:ring-0"
                    />
                  </label>
                  <p class="text-[10px] text-m3-on-surface-dim mt-0.5">开启后队员向房主推送 mod/map 将自动保存，无需弹窗确认。</p>
                </div>
                <div v-else class="text-[11px] text-m3-on-surface-dim">
                  设备别名：<span class="text-m3-on-surface font-mono">{{ lanConfig.deviceName }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- 2. 联机拓扑与同步状态看板 (左右分栏) -->
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-5">
            <!-- 左侧：房主服务端 / 客户端工作卡片 -->
            <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-4 flex flex-col justify-between">
              <div>
                <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
                  <div class="flex items-center gap-2">
                    <Wifi class="w-4 h-4 text-m3-primary" />
                    <span class="text-xs font-semibold text-m3-on-surface">
                      {{ lanStatus.role === 'server' ? '房主主机服务状态' : '局域网房主连接与同步' }}
                    </span>
                  </div>
                  <span
                    class="px-2.5 py-0.5 rounded-full text-[10px] font-medium border"
                    :class="
                      lanStatus.hasServerConflict
                        ? 'bg-m3-error-container text-m3-on-error-container border-m3-error/40'
                        : lanStatus.role === 'server'
                        ? 'bg-m3-primary-container text-m3-on-primary-container border-m3-primary/40'
                        : lanStatus.activeServer && syncDiff?.isInSync
                        ? 'bg-m3-success-container text-m3-on-success-container border-m3-success/40'
                        : lanStatus.activeServer
                        ? 'bg-m3-warning-container text-m3-on-warning-container border-m3-warning/40'
                        : 'bg-m3-surface-high text-m3-on-surface-dim border-m3-outline'
                    "
                  >
                    {{
                      lanStatus.hasServerConflict
                        ? '服务端冲突！'
                        : lanStatus.role === 'server'
                        ? '房主主机运行中'
                        : lanStatus.activeServer && syncDiff?.isInSync
                        ? '预设已完全对齐'
                        : lanStatus.activeServer
                        ? '预设存在差异'
                        : '搜寻房主中'
                    }}
                  </span>
                </div>

                <!-- 客户端视图：显示房主信息与预设差异详情 -->
                <div v-if="lanStatus.role === 'client'" class="mt-4 space-y-3.5 text-xs">
                  <div v-if="lanStatus.activeServer" class="p-3.5 bg-m3-surface-high border border-m3-outline-variant rounded-xl space-y-2">
                    <div class="flex items-center justify-between">
                      <div class="flex items-center gap-2">
                        <Server class="w-4 h-4 text-m3-primary" />
                        <span class="font-bold text-m3-on-surface text-xs">{{ lanStatus.activeServer.deviceName }}</span>
                        <span class="px-1.5 py-0.5 rounded-md text-[10px] bg-m3-primary-container text-m3-on-primary-container font-medium">房主服务端</span>
                      </div>
                      <span class="text-m3-on-surface-dim font-mono text-[11px]">{{ lanStatus.activeServer.ip }}:{{ lanStatus.activeServer.httpPort }}</span>
                    </div>
                    <div class="text-[11px] text-m3-on-surface-variant font-mono">
                      房主预设版本哈希：<span class="text-m3-primary font-bold">{{ lanStatus.activeServer.presetVersion || '未知' }}</span>
                    </div>
                  </div>

                  <div v-else class="p-6 bg-m3-surface-high border border-m3-outline-variant rounded-xl text-center space-y-2">
                    <WifiOff class="w-8 h-8 text-m3-on-surface-dim mx-auto" />
                    <p class="text-m3-on-surface font-medium text-xs">当前网络内未发现有效服务端</p>
                    <p class="text-[11px] text-m3-on-surface-dim leading-relaxed">
                      请确保局域网房主已打开本管理器并切换为服务端角色，或检查上方子网适配器是否与房主位于同一局域网。
                    </p>
                  </div>

                  <!-- 预设差异与同步详情 -->
                  <div v-if="lanStatus.activeServer" class="space-y-2">
                    <div
                      class="p-3.5 rounded-xl border leading-relaxed text-xs"
                      :class="
                        syncDiff?.isInSync
                          ? 'bg-m3-success-container/50 border-m3-success/30 text-m3-on-success-container'
                          : 'bg-m3-warning-container/50 border-m3-warning/30 text-m3-on-warning-container'
                      "
                    >
                      <div class="font-bold flex items-center gap-1.5 mb-1">
                        <CheckCheck v-if="syncDiff?.isInSync" class="w-4 h-4 text-m3-success" />
                        <AlertTriangle v-else class="w-4 h-4 text-m3-warning" />
                        <span>{{ syncDiff?.summary }}</span>
                      </div>

                      <div v-if="!syncDiff?.isInSync && syncDiff" class="mt-2 space-y-1 font-mono text-[11px] text-m3-on-surface-variant">
                        <div v-if="(syncDiff.missingMods?.length || 0) > 0">
                          <span class="text-m3-warning font-semibold">缺失模组：</span>
                          {{ (syncDiff.missingMods || []).map(m => m.fileName).join(', ') }}
                        </div>
                        <div v-if="(syncDiff.missingMaps?.length || 0) > 0">
                          <span class="text-m3-warning font-semibold">缺失地图：</span>
                          {{ (syncDiff.missingMaps || []).map(m => m.fileName).join(', ') }}
                        </div>
                        <div v-if="(syncDiff.stateMismatches?.length || 0) > 0">
                          <span class="text-m3-warning font-semibold">启禁用需调整：</span>
                          {{ (syncDiff.stateMismatches || []).map(m => `${m.fileName}(需${m.enabled ? '启用' : '禁用'})`).join(', ') }}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 服务端视图：显示房主主机状态与待处理推送 -->
                <div v-else class="mt-4 space-y-3 text-xs">
                  <div class="p-3.5 bg-m3-surface-high border border-m3-outline-variant rounded-xl space-y-2">
                    <div class="flex items-center justify-between">
                      <div class="flex items-center gap-2">
                        <Server class="w-4 h-4 text-m3-primary" />
                        <span class="font-bold text-m3-on-surface text-xs">本机作为房主正在监听与广播</span>
                      </div>
                      <span class="px-2 py-0.5 rounded-full text-[10px] bg-m3-success-container text-m3-on-success-container font-medium border border-m3-success/30">主机就绪</span>
                    </div>
                    <div class="text-[11px] text-m3-on-surface-dim font-mono">
                      广播信标端口：<span class="text-m3-on-surface font-semibold">38567 (UDP)</span> · HTTP 传输端口：<span class="text-m3-on-surface font-semibold">{{ lanStatus.httpPort }}</span>
                    </div>
                  </div>

                  <!-- 客户端模组推送待审核列表 -->
                  <div class="p-3.5 bg-m3-surface-high border border-m3-outline-variant rounded-xl space-y-2">
                    <div class="flex items-center justify-between">
                      <span class="font-semibold text-m3-on-surface text-xs">待审批的队员模组推送</span>
                      <span class="text-xs text-m3-primary font-mono">{{ pendingPushRequests?.length || 0 }} 条待处理</span>
                    </div>

                    <div v-if="!pendingPushRequests || pendingPushRequests.length === 0" class="text-m3-on-surface-dim text-xs py-2 text-center">
                      暂无队员发起模组或地图推送请求
                    </div>

                    <div v-else class="space-y-2 max-h-40 overflow-y-auto">
                      <div
                        v-for="req in (pendingPushRequests || [])"
                        :key="req.id"
                        class="p-2.5 bg-m3-surface-container border border-m3-outline-variant rounded-xl flex items-center justify-between text-xs"
                      >
                        <div class="min-w-0 pr-2 font-mono">
                          <div class="text-m3-on-surface font-medium truncate">
                            {{ req.peerName }} 请求推送 {{ req.itemType === 'mod' ? '模组' : '地图' }}
                          </div>
                          <div class="text-m3-on-surface-dim text-[11px] truncate">
                            {{ req.fileName }} ({{ formatBytes(req.fileSize) }})
                          </div>
                        </div>
                        <div class="flex items-center gap-1.5 shrink-0">
                          <button
                            @click="handlePushDecision(req.id, true)"
                            class="px-2.5 py-1 bg-m3-success-container text-m3-on-success-container border border-m3-success/40 rounded-lg text-[11px] font-medium transition hover:brightness-110"
                          >
                            接受
                          </button>
                          <button
                            @click="handlePushDecision(req.id, false)"
                            class="px-2.5 py-1 bg-m3-error-container text-m3-on-error-container border border-m3-error/40 rounded-lg text-[11px] font-medium transition hover:brightness-110"
                          >
                            拒绝
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 客户端操作按钮：一键同步 (M3 Filled Primary Button) -->
              <div v-if="lanStatus.role === 'client' && lanStatus.activeServer" class="pt-3 border-t border-m3-outline-variant">
                <button
                  @click="handleSyncFromHost"
                  :disabled="syncing"
                  class="w-full py-2.5 px-4 bg-m3-primary hover:bg-m3-primary-hover text-white font-semibold text-xs rounded-xl shadow-m3-1 active:scale-95 transition flex items-center justify-center gap-2 disabled:opacity-40"
                >
                  <DownloadCloud class="w-4 h-4" :class="{ 'animate-bounce': syncing }" />
                  <span>{{ syncing ? '正在下载文件与对齐 preferences.ini 状态...' : '一键同步房主预设 (自动补全缺失模组与状态)' }}</span>
                </button>
              </div>

              <!-- 客户端向房主推送模组/地图专属控制台 -->
              <div v-if="lanStatus.role === 'client'" class="pt-3 border-t border-m3-outline-variant space-y-2.5">
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <Send class="w-3.5 h-3.5 text-m3-primary" />
                    <span class="text-xs font-semibold text-m3-on-surface">向房主推送本地资源</span>
                  </div>
                  <div class="flex items-center gap-1 bg-m3-surface-highest p-0.5 rounded-lg border border-m3-outline text-[11px]">
                    <button
                      @click="pushHubType = 'mod'; pushHubTargetFile = ''"
                      :class="pushHubType === 'mod' ? 'bg-m3-primary text-white font-bold' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
                      class="px-2 py-0.5 rounded-md transition"
                    >
                      模组 ({{ mods.length }})
                    </button>
                    <button
                      @click="pushHubType = 'map'; pushHubTargetFile = ''"
                      :class="pushHubType === 'map' ? 'bg-m3-primary text-white font-bold' : 'text-m3-on-surface-variant hover:text-m3-on-surface'"
                      class="px-2 py-0.5 rounded-md transition"
                    >
                      地图 ({{ maps.length }})
                    </button>
                  </div>
                </div>

                <div class="flex items-center gap-2">
                  <select
                    v-model="pushHubTargetFile"
                    class="flex-1 bg-m3-surface-high border border-m3-outline rounded-xl px-3 py-1.5 text-xs text-m3-on-surface focus:outline-none focus:ring-1 focus:ring-m3-primary font-mono truncate"
                  >
                    <option value="" disabled>-- 请选择要推送给房主的{{ pushHubType === 'mod' ? '模组' : '地图' }} --</option>
                    <template v-if="pushHubType === 'mod'">
                      <option v-for="m in mods" :key="m.fileName" :value="m.fileName">
                        {{ m.title || m.fileName }} ({{ formatBytes(m.fileSize) }})
                      </option>
                    </template>
                    <template v-else>
                      <option v-for="mp in maps" :key="mp.id" :value="mp.id">
                        {{ mp.title || mp.fileName }} [{{ mp.locationType === 'standalone' ? '独立' : mp.locationType === 'map_pack' ? '地图包' : '单位模组' }}] ({{ formatBytes(mp.fileSize) }})
                      </option>
                    </template>
                  </select>

                  <button
                    @click="handlePushHubSend"
                    :disabled="!pushHubTargetFile || !lanStatus.activeServer"
                    class="px-3 py-1.5 bg-m3-secondary-container hover:bg-m3-primary hover:text-white text-m3-on-secondary-container text-xs font-semibold rounded-xl border border-m3-outline transition active:scale-95 disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-1.5 shrink-0"
                    :title="!lanStatus.activeServer ? '当前未发现房主，房主上线后可发起推送' : '将选中文件打包推送给房主进行联机共享'"
                  >
                    <Send class="w-3.5 h-3.5" />
                    <span>推送</span>
                  </button>
                </div>
                <div v-if="!lanStatus.activeServer" class="text-[11px] text-m3-on-surface-dim font-mono">
                  提示：当前局域网未连接房主。房主启动并广播后即可将本地新模组/地图直传给房主审核入库。
                </div>
              </div>
            </div>

            <!-- 右侧：发现的在线节点与同伴列表 -->
            <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-4 flex flex-col justify-between">
              <div>
                <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
                  <div class="flex items-center gap-2">
                    <Users class="w-4 h-4 text-m3-primary" />
                    <span class="text-xs font-semibold text-m3-on-surface">当前局域网在线节点</span>
                  </div>
                  <span class="text-xs text-m3-on-surface-dim font-mono">共发现 {{ peers?.length || 0 }} 台设备</span>
                </div>

                <div class="mt-4">
                  <div v-if="!peers || peers.length === 0" class="py-10 text-center text-m3-on-surface-dim text-xs">
                    <Radio class="w-8 h-8 mx-auto mb-2 opacity-40 animate-pulse text-m3-primary" />
                    <p>正在监听局域网心跳广播...</p>
                  </div>

                  <div v-else class="space-y-2 max-h-72 overflow-y-auto pr-1">
                    <div
                      v-for="p in (peers || [])"
                      :key="p.instanceId"
                      class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl flex items-center justify-between transition"
                    >
                      <div class="flex items-center gap-3">
                        <div
                          class="w-8 h-8 rounded-xl flex items-center justify-center text-xs font-bold"
                          :class="p.role === 'server' ? 'bg-m3-primary-container text-m3-on-primary-container' : 'bg-m3-surface-highest text-m3-on-surface-variant'"
                        >
                          <Server v-if="p.role === 'server'" class="w-4 h-4" />
                          <UserCheck v-else class="w-4 h-4" />
                        </div>
                        <div>
                          <div class="text-xs font-semibold text-m3-on-surface flex items-center gap-1.5">
                            <span>{{ p.deviceName }}</span>
                            <span
                              class="px-1.5 py-0.2 rounded-md text-[10px] font-medium"
                              :class="p.role === 'server' ? 'bg-m3-primary-container text-m3-on-primary-container' : 'bg-m3-surface-highest text-m3-on-surface-dim'"
                            >
                              {{ p.role === 'server' ? '房主服务端' : '客户端' }}
                            </span>
                          </div>
                          <div class="text-[11px] text-m3-on-surface-dim font-mono mt-0.5">
                            IP: {{ p.ip }}:{{ p.httpPort }} · 哈希: {{ p.presetVersion ? p.presetVersion.slice(0, 8) : '未就绪' }}
                          </div>
                        </div>
                      </div>

                      <div class="text-right">
                        <span class="text-[11px] text-m3-success flex items-center gap-1 font-mono font-medium">
                          <span class="w-1.5 h-1.5 rounded-full bg-m3-success" /> 在线
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <div class="pt-3 border-t border-m3-outline-variant text-right">
                <button
                  @click="pollLanStatus"
                  class="px-3 py-1.5 bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface text-xs rounded-xl border border-m3-outline transition"
                >
                  刷新节点列表
                </button>
              </div>
            </div>
          </div>

          <!-- 3. 本机动态启动预设感知看板 -->
          <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-4">
            <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
              <div>
                <label class="text-xs font-semibold text-m3-on-surface flex items-center gap-2">
                  <Flame class="w-4 h-4 text-m3-warning" />
                  本机动态启动预设清单 (Dynamic Launch Preset)
                </label>
                <p class="text-[11px] text-m3-on-surface-dim mt-0.5">
                  动态预设实时反应本机当前 preferences.ini 与 mods 目录生效状态；在游戏内重载模组后本工具亦会自动同步更新。
                </p>
              </div>
              <div class="flex items-center gap-2">
                <span class="px-2.5 py-1 rounded-lg bg-m3-surface-high border border-m3-outline text-m3-primary font-mono text-xs font-bold">
                  哈希: {{ dynamicPreset?.versionHash || '未就绪' }}
                </span>
                <button
                  @click="api.refreshDynamicPreset().then(p => { if (p) dynamicPreset = p })"
                  class="p-1.5 bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface-variant rounded-lg border border-m3-outline transition"
                  title="立即重算预设特征"
                >
                  <RefreshCw class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs font-mono">
              <div class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl">
                <span class="text-m3-on-surface-dim block text-[11px]">模组总数</span>
                <span class="text-m3-on-surface text-lg font-bold">{{ presetTotalMods }}</span>
              </div>
              <div class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl">
                <span class="text-m3-on-surface-dim block text-[11px]">已启用模组</span>
                <span class="text-m3-success text-lg font-bold">
                  {{ presetEnabledMods }}
                </span>
              </div>
              <div class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl">
                <span class="text-m3-on-surface-dim block text-[11px]">已禁用模组</span>
                <span class="text-m3-on-surface-variant text-lg font-bold">
                  {{ presetDisabledMods }}
                </span>
              </div>
              <div class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl">
                <span class="text-m3-on-surface-dim block text-[11px]">战局生效地图</span>
                <span class="text-m3-primary text-lg font-bold">
                  {{ presetActiveMaps }}
                </span>
              </div>
            </div>

            <!-- 预设详细条目预览 -->
            <div class="p-3.5 bg-m3-surface-high border border-m3-outline-variant rounded-xl space-y-2">
              <span class="text-xs font-semibold text-m3-on-surface-variant block">当前动态生效模组名单快照</span>
              <div class="max-h-48 overflow-y-auto space-y-1.5 pr-1 font-mono text-xs">
                <div
                  v-for="m in (dynamicPreset?.mods || [])"
                  :key="m.fileName"
                  class="flex items-center justify-between p-2 rounded-lg bg-m3-surface-container border border-m3-outline-variant"
                >
                  <div class="flex items-center gap-2 truncate pr-2">
                    <span
                      class="w-1.5 h-1.5 rounded-full shrink-0"
                      :class="m.enabled ? 'bg-m3-success' : 'bg-m3-on-surface-dim'"
                    />
                    <span class="text-m3-on-surface font-medium truncate">{{ m.title || m.fileName }}</span>
                    <span class="text-m3-on-surface-dim text-[11px] truncate">({{ m.fileName }})</span>
                  </div>
                  <span
                    class="px-2 py-0.5 rounded-md text-[10px] font-medium shrink-0"
                    :class="m.enabled ? 'bg-m3-success-container text-m3-on-success-container' : 'bg-m3-surface-highest text-m3-on-surface-dim'"
                  >
                    {{ m.enabled ? '已启用' : '已禁用' }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 3. 启动调优与高刷设置 Tab (深度集成 preferences.ini 游戏参数) -->
      <section v-if="currentTab === 'launcher'" class="h-full overflow-y-auto p-8 max-w-4xl mx-auto">
        <div class="space-y-6 pb-12">
          <div>
            <h2 class="text-base font-bold text-m3-on-surface flex items-center gap-2.5">
              <Sliders class="w-5 h-5 text-m3-primary" />
              游戏运行参数、高刷新率与内置引擎调优
            </h2>
            <p class="text-xs text-m3-on-surface-variant mt-1 leading-relaxed">
              全面管理《铁锈战争》启动方式、JVM 内存、144Hz+ 电竞高刷屏支持、垂直同步、FPS 显示与队伍造兵上限。
            </p>
          </div>

          <!-- 1. 刷新率与帧率设置 (M3 Switch Cards) -->
          <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-4">
            <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
              <div>
                <label class="text-xs font-semibold text-m3-on-surface flex items-center gap-2">
                  <Activity class="w-4 h-4 text-m3-primary" />
                  刷新率与帧率同步控制 (Refresh Rate & Sync)
                </label>
                <p class="text-[11px] text-m3-on-surface-dim mt-0.5">原版默认限制 60Hz 帧率，在此可一键解锁高刷屏满血体验。</p>
              </div>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
              <!-- 解锁高刷新率 -->
              <label
                class="flex items-center justify-between p-3.5 bg-m3-surface-high border rounded-xl cursor-pointer transition"
                :class="appConfig.gamePreferences.highRefreshRate ? 'border-m3-primary/60 bg-m3-primary-container/20' : 'border-m3-outline-variant hover:border-m3-outline'"
              >
                <div>
                  <span class="text-xs text-m3-on-surface font-semibold block">解锁高刷新率 (High Refresh Rate)</span>
                  <span class="text-[11px] text-m3-on-surface-dim font-mono mt-0.5 block">匹配 120Hz/144Hz/165Hz/240Hz 显示器</span>
                </div>
                <input type="checkbox" v-model="appConfig.gamePreferences.highRefreshRate" class="w-4 h-4 rounded text-m3-primary focus:ring-0" />
              </label>

              <!-- 垂直同步 -->
              <label
                class="flex items-center justify-between p-3.5 bg-m3-surface-high border rounded-xl cursor-pointer transition"
                :class="appConfig.gamePreferences.renderVsync ? 'border-m3-primary/60 bg-m3-primary-container/20' : 'border-m3-outline-variant hover:border-m3-outline'"
              >
                <div>
                  <span class="text-xs text-m3-on-surface font-semibold block">垂直同步 (V-Sync)</span>
                  <span class="text-[11px] text-m3-on-surface-dim font-mono mt-0.5 block">消除高速移动与拖动时的画面撕裂</span>
                </div>
                <input type="checkbox" v-model="appConfig.gamePreferences.renderVsync" class="w-4 h-4 rounded text-m3-primary focus:ring-0" />
              </label>

              <!-- 显示内置 FPS -->
              <label
                class="flex items-center justify-between p-3.5 bg-m3-surface-high border rounded-xl cursor-pointer transition"
                :class="appConfig.gamePreferences.showFps ? 'border-m3-primary/60 bg-m3-primary-container/20' : 'border-m3-outline-variant hover:border-m3-outline'"
              >
                <div>
                  <span class="text-xs text-m3-on-surface font-semibold block">显示游戏内 FPS 计数器</span>
                  <span class="text-[11px] text-m3-on-surface-dim font-mono mt-0.5 block">在屏幕角落实时输出渲染帧率</span>
                </div>
                <input type="checkbox" v-model="appConfig.gamePreferences.showFps" class="w-4 h-4 rounded text-m3-primary focus:ring-0" />
              </label>

              <!-- 平滑帧时间差 -->
              <label
                class="flex items-center justify-between p-3.5 bg-m3-surface-high border rounded-xl cursor-pointer transition"
                :class="appConfig.gamePreferences.renderSmoothDelta ? 'border-m3-primary/60 bg-m3-primary-container/20' : 'border-m3-outline-variant hover:border-m3-outline'"
              >
                <div>
                  <span class="text-xs text-m3-on-surface font-semibold block">平滑帧时间差 (Smooth Delta)</span>
                  <span class="text-[11px] text-m3-on-surface-dim font-mono mt-0.5 block">平抑单位大规模爆炸交火时的微卡顿</span>
                </div>
                <input type="checkbox" v-model="appConfig.gamePreferences.renderSmoothDelta" class="w-4 h-4 rounded text-m3-primary focus:ring-0" />
              </label>
            </div>
          </div>

          <!-- 2. 战局造兵单位上限 (M3 Range Sliders) -->
          <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-4">
            <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
              <div>
                <label class="text-xs font-semibold text-m3-on-surface flex items-center gap-2">
                  <Flame class="w-4 h-4 text-m3-warning" />
                  战局队伍单位上限 (Team Unit Caps)
                </label>
                <p class="text-[11px] text-m3-on-surface-dim mt-0.5">调高队伍单位容量，避免后期大战或大型实验级模组造满限制。</p>
              </div>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <!-- 单人战局上限 -->
              <div class="space-y-2">
                <div class="flex justify-between items-center mb-1">
                  <span class="text-xs font-medium text-m3-on-surface">单人遭遇战单位上限</span>
                  <span class="text-xs font-mono font-bold text-m3-primary bg-m3-surface-high px-2 py-0.5 rounded-lg border border-m3-outline">
                    {{ appConfig.gamePreferences.teamUnitCapSinglePlayer }} 单位/队
                  </span>
                </div>
                <input
                  type="range"
                  min="2000"
                  max="20000"
                  step="1000"
                  v-model.number="appConfig.gamePreferences.teamUnitCapSinglePlayer"
                  class="w-full cursor-pointer"
                />
                <div class="flex justify-between text-[10px] text-m3-on-surface-dim font-mono">
                  <span>2000</span>
                  <span>5000 (默认)</span>
                  <span>10000</span>
                  <span>20000 (极大规模)</span>
                </div>
              </div>

              <!-- 多人联机建房上限 -->
              <div class="space-y-2">
                <div class="flex justify-between items-center mb-1">
                  <span class="text-xs font-medium text-m3-on-surface">联机建房队伍单位上限</span>
                  <span class="text-xs font-mono font-bold text-m3-success bg-m3-surface-high px-2 py-0.5 rounded-lg border border-m3-outline">
                    {{ appConfig.gamePreferences.teamUnitCapHostedGame }} 单位/队
                  </span>
                </div>
                <input
                  type="range"
                  min="1000"
                  max="10000"
                  step="500"
                  v-model.number="appConfig.gamePreferences.teamUnitCapHostedGame"
                  class="w-full cursor-pointer"
                />
                <div class="flex justify-between text-[10px] text-m3-on-surface-dim font-mono">
                  <span>1000</span>
                  <span>2000 (默认)</span>
                  <span>5000 (推荐)</span>
                  <span>10000</span>
                </div>
              </div>
            </div>
          </div>

          <!-- 3. 画质与特效控制 -->
          <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-3.5">
            <label class="text-xs font-semibold text-m3-on-surface flex items-center gap-2">
              <Monitor class="w-4 h-4 text-m3-primary" />
              画质特效与着色器 (Graphics & Shaders)
            </label>
            <div class="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
              <label class="flex items-center gap-2.5 p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl cursor-pointer hover:border-m3-outline transition">
                <input type="checkbox" v-model="appConfig.gamePreferences.renderAntiAlias" class="w-4 h-4 rounded text-m3-primary focus:ring-0" />
                <span class="text-m3-on-surface font-medium">抗锯齿平滑</span>
              </label>

              <label class="flex items-center gap-2.5 p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl cursor-pointer hover:border-m3-outline transition">
                <input type="checkbox" v-model="appConfig.gamePreferences.renderFancyWater" class="w-4 h-4 rounded text-m3-primary focus:ring-0" />
                <span class="text-m3-on-surface font-medium">动态优质水面</span>
              </label>

              <label class="flex items-center gap-2.5 p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl cursor-pointer hover:border-m3-outline transition">
                <input type="checkbox" v-model="appConfig.gamePreferences.renderClouds" class="w-4 h-4 rounded text-m3-primary focus:ring-0" />
                <span class="text-m3-on-surface font-medium">天空云层阴影</span>
              </label>

              <label class="flex items-center gap-2.5 p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl cursor-pointer hover:border-m3-outline transition">
                <input type="checkbox" v-model="appConfig.gamePreferences.renderExtraShadows" class="w-4 h-4 rounded text-m3-primary focus:ring-0" />
                <span class="text-m3-on-surface font-medium">额外高精度阴影</span>
              </label>
            </div>
          </div>

          <!-- 4. 启动器主程序与 JVM 堆内存 (M3 Radio Selection Cards) -->
          <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-4">
            <label class="text-xs font-semibold text-m3-on-surface">启动程序与内存分配</label>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
              <label
                class="flex items-center gap-3 p-3.5 rounded-xl border cursor-pointer transition shadow-sm"
                :class="appConfig.launchConfig.executableType === 'exe64' ? 'bg-m3-primary-container/25 border-m3-primary text-m3-on-surface' : 'bg-m3-surface-high border-m3-outline-variant text-m3-on-surface-variant hover:border-m3-outline'"
              >
                <input type="radio" v-model="appConfig.launchConfig.executableType" value="exe64" class="hidden" />
                <div>
                  <div class="text-xs font-bold text-m3-on-surface">Rusted Warfare - 64.exe</div>
                  <div class="text-[11px] text-m3-on-surface-dim mt-0.5">64 位原生主程序（推荐）</div>
                </div>
              </label>

              <label
                class="flex items-center gap-3 p-3.5 rounded-xl border cursor-pointer transition shadow-sm"
                :class="appConfig.launchConfig.executableType === 'java' ? 'bg-m3-primary-container/25 border-m3-primary text-m3-on-surface' : 'bg-m3-surface-high border-m3-outline-variant text-m3-on-surface-variant hover:border-m3-outline'"
              >
                <input type="radio" v-model="appConfig.launchConfig.executableType" value="java" class="hidden" />
                <div>
                  <div class="text-xs font-bold text-m3-on-surface">内置 JVM 模式</div>
                  <div class="text-[11px] text-m3-on-surface-dim mt-0.5">允许自定义 JVM 内存 (-Xmx)</div>
                </div>
              </label>

              <label
                class="flex items-center gap-3 p-3.5 rounded-xl border cursor-pointer transition shadow-sm"
                :class="appConfig.launchConfig.executableType === 'exe32' ? 'bg-m3-primary-container/25 border-m3-primary text-m3-on-surface' : 'bg-m3-surface-high border-m3-outline-variant text-m3-on-surface-variant hover:border-m3-outline'"
              >
                <input type="radio" v-model="appConfig.launchConfig.executableType" value="exe32" class="hidden" />
                <div>
                  <div class="text-xs font-bold text-m3-on-surface">Rusted Warfare.exe</div>
                  <div class="text-[11px] text-m3-on-surface-dim mt-0.5">32 位兼容模式</div>
                </div>
              </label>
            </div>

            <!-- JVM 堆内存滑块 -->
            <div class="pt-2 space-y-2">
              <div class="flex justify-between items-center mb-1">
                <span class="text-xs text-m3-on-surface font-medium">JVM 最大内存分配 (-Xmx)</span>
                <span class="text-xs font-mono font-bold text-m3-primary bg-m3-surface-high px-2.5 py-0.5 rounded-lg border border-m3-outline">
                  {{ appConfig.launchConfig.maxMemoryMb }} MB
                </span>
              </div>
              <input
                type="range"
                min="1024"
                max="8192"
                step="512"
                v-model.number="appConfig.launchConfig.maxMemoryMb"
                class="w-full cursor-pointer"
              />
            </div>
          </div>

          <!-- 5. 命令行参数 -->
          <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-3.5">
            <label class="text-xs font-semibold text-m3-on-surface">快速命令行参数</label>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
              <label class="flex items-center gap-3 p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl cursor-pointer hover:border-m3-outline transition">
                <input type="checkbox" v-model="appConfig.launchConfig.fullscreen" class="w-4 h-4 rounded text-m3-primary focus:ring-0" />
                <div>
                  <span class="text-xs text-m3-on-surface font-medium">全屏运行</span>
                  <span class="text-[11px] text-m3-on-surface-dim block font-mono">-fullscreen</span>
                </div>
              </label>

              <label class="flex items-center gap-3 p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl cursor-pointer hover:border-m3-outline transition">
                <input type="checkbox" v-model="appConfig.launchConfig.noMods" class="w-4 h-4 rounded text-m3-primary focus:ring-0" />
                <div>
                  <span class="text-xs text-m3-on-surface font-medium">纯净原版启动</span>
                  <span class="text-[11px] text-m3-on-surface-dim block font-mono">-nomods (禁用所有模组测试)</span>
                </div>
              </label>

              <label class="flex items-center gap-3 p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl cursor-pointer hover:border-m3-outline transition">
                <input type="checkbox" v-model="appConfig.launchConfig.safeMode" class="w-4 h-4 rounded text-m3-primary focus:ring-0" />
                <div>
                  <span class="text-xs text-m3-on-surface font-medium">安全渲染模式</span>
                  <span class="text-[11px] text-m3-on-surface-dim block font-mono">-safemode</span>
                </div>
              </label>
            </div>

            <div class="pt-2">
              <label class="text-xs font-semibold text-m3-on-surface-variant block mb-1.5">自定义附加命令行参数</label>
              <input
                type="text"
                v-model="appConfig.launchConfig.extraArgs"
                placeholder="例如: -width 1920 -height 1080 -postprocessing"
                class="w-full bg-m3-surface-high border border-m3-outline rounded-xl px-3.5 py-2 text-xs font-mono text-m3-on-surface placeholder-m3-on-surface-dim focus:outline-none focus:border-m3-primary"
              />
            </div>
          </div>

          <!-- 保存与启动按钮 (M3 Actions) -->
          <div class="flex justify-end gap-3 pt-2">
            <button
              @click="handleSaveSettings"
              class="px-5 py-2.5 bg-m3-surface-container hover:bg-m3-surface-high text-m3-on-surface text-xs font-medium rounded-xl border border-m3-outline transition"
            >
              保存所有偏好与启动配置
            </button>
            <button
              @click="handleLaunchGame"
              class="flex items-center gap-2 px-6 py-2.5 bg-m3-primary hover:bg-m3-primary-hover text-white font-semibold text-xs rounded-xl shadow-m3-1 transition"
            >
              <Rocket class="w-4 h-4" />
              <span>保存并立即启动游戏</span>
            </button>
          </div>
        </div>
      </section>

      <!-- 4. 系统设置 Tab (M3 Layout) -->
      <section v-if="currentTab === 'settings'" class="h-full overflow-y-auto p-8 max-w-4xl mx-auto">
        <div class="space-y-6 pb-12">
          <div>
            <h2 class="text-base font-bold text-m3-on-surface flex items-center gap-2.5">
              <Settings class="w-5 h-5 text-m3-primary" />
              游戏客户端与环境自检
            </h2>
            <p class="text-xs text-m3-on-surface-variant mt-1">检测铁锈战争客户端目录完整性与配置文件状态。</p>
          </div>

          <!-- 路径配置卡片 -->
          <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-3.5">
            <div class="flex items-center justify-between">
              <label class="text-xs font-semibold text-m3-on-surface">游戏安装根目录</label>
              <button
                @click="handleChangeDir"
                class="px-3 py-1.5 bg-m3-surface-high hover:bg-m3-surface-highest text-m3-primary border border-m3-outline rounded-xl text-xs font-medium transition"
              >
                浏览选择目录
              </button>
            </div>
            <div class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl font-mono text-xs text-m3-on-surface break-all select-text">
              {{ gameStatus.gameDir || '尚未配置' }}
            </div>
          </div>

          <!-- 自检清单 -->
          <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-3.5 text-xs">
            <h3 class="text-xs font-semibold text-m3-on-surface mb-2">环境特征自检清单</h3>
            <div class="space-y-2 font-mono">
              <div class="flex items-center justify-between p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl">
                <span class="text-m3-on-surface text-xs font-sans">game-lib.jar (核心游戏程序包)</span>
                <span class="flex items-center gap-2 text-m3-success font-medium text-xs">
                  <CheckCircle2 class="w-4 h-4" /> 正常就绪
                </span>
              </div>

              <div class="flex items-center justify-between p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl">
                <span class="text-m3-on-surface text-xs font-sans">preferences.ini (全局设置与 modSettings 存储)</span>
                <span class="flex items-center gap-2 text-m3-success font-medium text-xs">
                  <CheckCircle2 class="w-4 h-4" /> 正常就绪
                </span>
              </div>

              <div class="flex items-center justify-between p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl">
                <span class="text-m3-on-surface text-xs font-sans">mods/units (单位模组目录)</span>
                <span class="flex items-center gap-2 text-m3-primary font-medium text-xs">
                  已加载 {{ mods.length }} 个模组 ({{ enabledCount }} 启用)
                </span>
              </div>

              <div class="flex items-center justify-between p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl">
                <span class="text-m3-on-surface text-xs font-sans">mods/maps (地图全量检索，含模组内嵌)</span>
                <span class="flex items-center gap-2 text-m3-primary font-medium text-xs">
                  共计 {{ maps.length }} 张地图 ({{ activeMapCount }} 可用)
                </span>
              </div>
            </div>
          </div>

          <!-- 局域网角色切换与网络配置 -->
          <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-4 text-xs">
            <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
              <h3 class="text-xs font-semibold text-m3-on-surface flex items-center gap-2">
                <Radio class="w-4 h-4 text-m3-primary" />
                局域网联机角色与子网设置
              </h3>
              <span class="text-m3-on-surface-dim font-mono">当前角色：{{ lanStatus.role === 'server' ? '服务端 (房主)' : '客户端' }}</span>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div class="space-y-2">
                <span class="text-m3-on-surface font-medium block">切换联机模式</span>
                <div class="flex gap-2">
                  <button
                    @click="handleSwitchRole('client')"
                    class="flex-1 py-2 px-3 rounded-xl border font-medium transition flex items-center justify-center gap-1.5"
                    :class="lanStatus.role === 'client' ? 'bg-m3-primary text-white border-transparent shadow-sm' : 'bg-m3-surface-high border-m3-outline text-m3-on-surface-variant hover:text-m3-on-surface'"
                  >
                    <UserCheck class="w-3.5 h-3.5" /> 客户端 (默认)
                  </button>
                  <button
                    @click="handleSwitchRole('server')"
                    class="flex-1 py-2 px-3 rounded-xl border font-medium transition flex items-center justify-center gap-1.5"
                    :class="lanStatus.role === 'server' ? 'bg-m3-primary text-white border-transparent shadow-sm' : 'bg-m3-surface-high border-m3-outline text-m3-on-surface-variant hover:text-m3-on-surface'"
                  >
                    <Server class="w-3.5 h-3.5" /> 服务端 (房主)
                  </button>
                </div>
                <p class="text-[11px] text-m3-on-surface-dim">应用的默认设置均为客户端产生，同一个子网集合内严格只能有 1 个服务端。</p>
              </div>

              <div class="space-y-2">
                <span class="text-m3-on-surface font-medium block">选择绑定的网络子网</span>
                <select
                  :value="lanConfig.selectedSubnet"
                  @change="handleSubnetChange"
                  class="w-full bg-m3-surface-high border border-m3-outline rounded-xl p-2 text-xs text-m3-on-surface"
                >
                  <option value="">全部网络 / 自动适配</option>
                  <option v-for="iface in (interfaces || [])" :key="iface.ip" :value="iface.subnet">
                    {{ iface.name }} - {{ iface.ip }}
                  </option>
                </select>
                <p class="text-[11px] text-m3-on-surface-dim">跨虚拟局域网（如 Tailscale/Radmin）对战时，可在此指定网卡子网。</p>
              </div>
            </div>
          </div>

          <!-- 软件自更新中心 (双通道：局域网 P2P + GitHub Releases) -->
          <div class="p-5 bg-m3-surface-container border border-m3-outline rounded-2xl space-y-4 text-xs">
            <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
              <div class="flex items-center gap-2">
                <ArrowUpCircle class="w-4 h-4 text-m3-primary" />
                <h3 class="text-xs font-semibold text-m3-on-surface">应用自更新中心 (局域网 P2P / GitHub Releases)</h3>
                <span class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-m3-primary-container text-m3-on-primary-container font-bold">
                  v{{ currentVersion }}
                </span>
              </div>
              <button
                @click="handleCheckUpdate(false)"
                :disabled="checkingUpdate"
                class="px-3 py-1.5 bg-m3-surface-high hover:bg-m3-surface-dim border border-m3-outline rounded-xl font-medium transition flex items-center gap-1.5 disabled:opacity-50"
              >
                <RefreshCw class="w-3.5 h-3.5 text-m3-primary" :class="{ 'animate-spin': checkingUpdate }" />
                <span>{{ checkingUpdate ? '正在探测更新...' : '立即检查更新' }}</span>
              </button>
            </div>

            <!-- 双通道状态卡片 -->
            <div class="grid grid-cols-1 md:grid-cols-2 gap-3 font-mono">
              <div class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl space-y-1">
                <div class="flex items-center justify-between">
                  <span class="text-m3-on-surface-variant text-[11px] font-sans">局域网自更新网络</span>
                  <span class="text-[10px] px-1.5 py-0.2 rounded font-sans" :class="updateInfo?.lanAvailable ? 'bg-m3-success-container text-m3-success font-bold' : 'bg-m3-surface-container text-m3-on-surface-dim'">
                    {{ updateInfo?.lanAvailable ? '发现局域网新版本 v' + updateInfo.lanVersion : '局域网监听中' }}
                  </span>
                </div>
                <div class="text-[11px] text-m3-on-surface truncate">
                  {{ lanStatus.activeServer ? '已连接房主: ' + lanStatus.activeServer.deviceName : (lanStatus.role === 'server' ? '本机正作为服务端广播' : '未连接房主') }}
                </div>
              </div>

              <div class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl space-y-1">
                <div class="flex items-center justify-between">
                  <span class="text-m3-on-surface-variant text-[11px] font-sans flex items-center gap-1">
                    <Github class="w-3 h-3" /> GitHub Releases 官方源
                  </span>
                  <span class="text-[10px] px-1.5 py-0.2 rounded font-sans" :class="updateInfo?.githubAvailable ? 'bg-m3-primary-container text-m3-primary font-bold' : 'bg-m3-surface-container text-m3-on-surface-dim'">
                    {{ updateInfo?.githubAvailable ? '发现 GitHub 新版 v' + updateInfo.githubVersion : '已接入' }}
                  </span>
                </div>
                <div class="text-[11px] text-m3-on-surface truncate">
                  Hotpotfishsummer/RW-Mod-Manager
                </div>
              </div>
            </div>

            <!-- 检测到更新时的详情与一键更新卡片 -->
            <div v-if="updateInfo?.hasUpdate" class="p-4 bg-gradient-to-br from-amber-500/10 via-m3-surface-high to-m3-surface-container border border-amber-500/30 rounded-2xl space-y-3">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <Sparkles class="w-4 h-4 text-amber-400 animate-pulse" />
                  <span class="font-bold text-m3-on-surface text-sm">
                    检测到可升级版本：v{{ updateInfo.latestVersion }}
                  </span>
                </div>
                <span class="text-amber-400 font-mono text-xs font-semibold">
                  {{ formatBytes(updateInfo.fileSize) }}
                </span>
              </div>

              <!-- 切换更新来源 -->
              <div class="space-y-1.5">
                <span class="text-[11px] text-m3-on-surface-variant block font-sans">选择更新下载通道：</span>
                <div class="flex gap-2">
                  <button
                    v-if="updateInfo.lanAvailable"
                    @click="selectedUpdateSource = 'lan'"
                    class="flex-1 py-1.5 px-3 rounded-xl border text-xs font-medium transition flex items-center justify-center gap-1.5"
                    :class="selectedUpdateSource === 'lan' ? 'bg-m3-success text-white border-transparent font-bold shadow-sm' : 'bg-m3-surface-high border-m3-outline text-m3-on-surface-variant hover:text-m3-on-surface'"
                  >
                    <Wifi class="w-3.5 h-3.5" /> 局域网极速直连 (免流量)
                  </button>
                  <button
                    v-if="updateInfo.githubAvailable"
                    @click="selectedUpdateSource = 'github'"
                    class="flex-1 py-1.5 px-3 rounded-xl border text-xs font-medium transition flex items-center justify-center gap-1.5"
                    :class="selectedUpdateSource === 'github' ? 'bg-m3-primary text-white border-transparent font-bold shadow-sm' : 'bg-m3-surface-high border-m3-outline text-m3-on-surface-variant hover:text-m3-on-surface'"
                  >
                    <Github class="w-3.5 h-3.5" /> GitHub Releases 官方云端
                  </button>
                </div>
              </div>

              <!-- 更新日志 -->
              <div v-if="updateInfo.releaseNotes" class="p-3 bg-m3-surface-container border border-m3-outline-variant rounded-xl space-y-1">
                <span class="text-[11px] text-m3-on-surface-variant font-medium">更新内容与修复说明：</span>
                <p class="text-xs text-m3-on-surface font-sans whitespace-pre-wrap leading-relaxed max-h-32 overflow-y-auto">
                  {{ updateInfo.releaseNotes }}
                </p>
              </div>

              <!-- 下载进度条 -->
              <div v-if="updating && updateProgress" class="space-y-1.5 pt-1">
                <div class="flex items-center justify-between text-[11px] font-mono">
                  <span class="text-m3-primary font-bold">{{ updateProgress.message }}</span>
                  <span class="text-m3-on-surface-variant">{{ formatBytes(updateProgress.speedBytes) }}/s</span>
                </div>
                <div class="w-full bg-m3-surface-container rounded-full h-2.5 overflow-hidden border border-m3-outline-variant">
                  <div
                    class="bg-gradient-to-r from-m3-primary to-m3-success h-full transition-all duration-200"
                    :style="{ width: updateProgress.percentage + '%' }"
                  />
                </div>
              </div>

              <!-- 一键执行更新按钮 -->
              <div class="pt-2">
                <button
                  @click="handlePerformUpdate"
                  :disabled="updating"
                  class="w-full py-2.5 px-4 bg-gradient-to-r from-m3-primary to-m3-primary-hover hover:brightness-110 text-white font-semibold text-xs rounded-xl shadow-m3-1 active:scale-95 transition flex items-center justify-center gap-2 disabled:opacity-50"
                >
                  <DownloadCloud class="w-4 h-4" :class="{ 'animate-bounce': updating }" />
                  <span>{{ updating ? '正在下载并准备热替换重启...' : '立即下载并自动热替换升级' }}</span>
                </button>
              </div>
            </div>

            <div v-else class="text-center py-3 text-m3-on-surface-dim text-xs font-sans">
              当前为最新版本，暂无可用更新。
            </div>
          </div>
        </div>
      </section>
    </main>

    <!-- 1. 局域网服务端唯一性冲突拦截弹窗 (M3 Alert Dialog) -->
    <div
      v-if="lanStatus.hasServerConflict"
      class="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-6"
    >
      <div class="bg-m3-surface-container border border-m3-error rounded-3xl max-w-lg w-full p-6 shadow-m3-3 text-center space-y-4 animate-in fade-in zoom-in-95">
        <div class="w-14 h-14 rounded-2xl bg-m3-error-container text-m3-error mx-auto flex items-center justify-center shadow-sm">
          <AlertTriangle class="w-8 h-8" />
        </div>
        <div>
          <h2 class="text-base font-bold text-m3-on-surface tracking-wide">
            局域网服务端唯一性冲突告警
          </h2>
          <p class="text-xs text-m3-error font-medium mt-1">同一网络集合内只能有 1 个服务端作为房主，其余必须为客户端</p>
        </div>

        <div class="p-4 bg-m3-surface-high border border-m3-outline-variant rounded-2xl text-left space-y-2.5 text-xs">
          <div class="text-m3-on-surface-variant leading-relaxed">
            检测到当前局域网集合内存在 <span class="text-m3-error font-semibold">多个服务端</span>。根据铁锈战争联机同步规范，为防止启动预设冲突，<span class="text-m3-error font-semibold">已锁定操作</span>。
          </div>

          <div v-if="(lanStatus.conflictingServers?.length || 0) > 0" class="space-y-2 mt-2">
            <div class="text-m3-on-surface-dim font-medium">探测到的冲突服务端：</div>
            <div
              v-for="srv in (lanStatus.conflictingServers || [])"
              :key="srv.instanceId"
              class="p-2.5 bg-m3-surface-container border border-m3-error/30 rounded-xl flex items-center justify-between font-mono"
            >
              <div class="flex items-center gap-2">
                <Server class="w-4 h-4 text-m3-error" />
                <span class="text-m3-on-surface font-semibold">{{ srv.deviceName }}</span>
              </div>
              <span class="text-m3-on-surface-dim text-[11px]">{{ srv.ip }}:{{ srv.httpPort }}</span>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-center gap-2.5 pt-2">
          <button
            @click="handleSwitchRole('client')"
            class="px-5 py-2.5 bg-m3-primary hover:bg-m3-primary-hover text-white font-semibold text-xs rounded-xl shadow-m3-1 transition active:scale-95"
          >
            一键将本机切换为客户端 (解除冲突)
          </button>
          <button
            @click="pollLanStatus"
            class="px-4 py-2.5 bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface font-medium text-xs rounded-xl border border-m3-outline transition"
          >
            重新探测
          </button>
        </div>
      </div>
    </div>

    <!-- 2. 客户端向房主推送模组/地图确认弹窗 (M3 Confirmation Dialog) -->
    <div
      v-if="pushModalTarget"
      class="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-6"
    >
      <div class="bg-m3-surface-container border border-m3-outline rounded-3xl max-w-md w-full p-6 shadow-m3-3 space-y-4">
        <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
          <div class="flex items-center gap-2.5">
            <UploadCloud class="w-5 h-5 text-m3-primary" />
            <h3 class="text-sm font-bold text-m3-on-surface">
              推送{{ pushModalTarget.type === 'mod' ? '模组' : '地图' }}至房主
            </h3>
          </div>
          <button @click="pushModalTarget = null" class="text-m3-on-surface-dim hover:text-m3-on-surface">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl space-y-1.5 font-mono">
            <div class="text-m3-on-surface-variant">文件名称：<span class="text-m3-primary font-bold">{{ pushModalTarget.name }}</span></div>
            <div v-if="pushModalTarget.size" class="text-m3-on-surface-variant">文件大小：<span class="text-m3-on-surface">{{ formatBytes(pushModalTarget.size) }}</span></div>
            <div class="text-m3-on-surface-variant">目标房主：<span class="text-m3-success font-medium">{{ lanStatus.activeServer?.deviceName || '房主主机' }} ({{ lanStatus.activeServer?.ip }})</span></div>
          </div>
          <p class="text-m3-on-surface-dim leading-relaxed">
            推送发起后，文件将发送至房主服务端；若房主开启了自动接收将直接入库，否则需房主审批。
          </p>
        </div>

        <div class="flex items-center justify-end gap-2.5 pt-2">
          <button
            @click="pushModalTarget = null"
            class="px-4 py-2 bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface rounded-xl text-xs border border-m3-outline transition"
          >
            取消
          </button>
          <button
            @click="confirmPushToHost"
            :disabled="pushing"
            class="px-5 py-2 bg-m3-primary hover:bg-m3-primary-hover text-white font-semibold rounded-xl text-xs flex items-center gap-2 shadow-m3-1 disabled:opacity-40"
          >
            <UploadCloud class="w-4 h-4" :class="{ 'animate-bounce': pushing }" />
            <span>{{ pushing ? '传输中/等待审批...' : '确认推送' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- 3. 客户端无房主启动引导弹窗 (M3 Confirmation Dialog) -->
    <div
      v-if="promptHostModal"
      class="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-6"
    >
      <div class="bg-m3-surface-container border border-m3-outline rounded-3xl max-w-md w-full p-6 shadow-m3-3 space-y-4 animate-in fade-in zoom-in-95">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-2xl bg-m3-primary-container text-m3-primary flex items-center justify-center">
            <Server class="w-5 h-5 text-m3-on-primary-container" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-m3-on-surface">未发现局域网房主</h3>
            <p class="text-[11px] text-m3-on-surface-variant">当前未检测到正在广播的房主主机</p>
          </div>
        </div>

        <p class="text-xs text-m3-on-surface-variant leading-relaxed">
          您当前处于客户端角色，但局域网内暂无房主。您可以一键切换为房主服务端直接启动并作为主机广播，或者以单机模式拉起游戏。
        </p>

        <div class="flex flex-col gap-2 pt-2">
          <button
            @click="handleSwitchToHostAndLaunch"
            class="w-full py-2.5 px-4 bg-m3-primary hover:bg-m3-primary-hover text-white font-semibold rounded-xl text-xs flex items-center justify-center gap-2 shadow-m3-1 transition"
          >
            <Server class="w-4 h-4" />
            <span>切换为房主并立即启动</span>
          </button>

          <div class="flex items-center justify-end gap-2 pt-1">
            <button
              @click="promptHostModal = false"
              class="px-4 py-2 bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface rounded-xl text-xs border border-m3-outline transition"
            >
              取消
            </button>
            <button
              @click="promptHostModal = false; handleLaunchSingle()"
              class="px-4 py-2 bg-m3-surface-container hover:bg-m3-surface-high text-m3-on-surface rounded-xl text-xs border border-m3-outline transition font-medium"
            >
              改为单机启动
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 4. 软件自更新弹窗 (M3 Update Dialog) -->
    <div
      v-if="showUpdateModal"
      class="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-6"
    >
      <div class="bg-m3-surface-container border border-m3-outline rounded-3xl max-w-lg w-full p-6 shadow-m3-3 space-y-4 animate-in fade-in zoom-in-95">
        <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
          <div class="flex items-center gap-2.5">
            <div class="w-10 h-10 rounded-2xl bg-m3-primary-container text-m3-primary flex items-center justify-center">
              <ArrowUpCircle class="w-5 h-5 text-m3-on-primary-container" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-m3-on-surface">应用版本升级</h3>
              <p class="text-[11px] text-m3-on-surface-variant">支持局域网 P2P 极速分发与 GitHub Releases 官方云端</p>
            </div>
          </div>
          <button
            @click="showUpdateModal = false"
            class="p-1.5 rounded-full hover:bg-m3-surface-high text-m3-on-surface-variant transition"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <div v-if="updateInfo?.hasUpdate" class="space-y-3.5 text-xs">
          <!-- 版本流转 -->
          <div class="flex items-center justify-between p-3.5 bg-m3-surface-high border border-m3-outline-variant rounded-2xl">
            <div>
              <div class="text-[11px] text-m3-on-surface-variant font-medium">当前安装版本</div>
              <div class="text-sm font-bold text-m3-on-surface font-mono mt-0.5">v{{ currentVersion }}</div>
            </div>
            <div class="text-m3-primary font-bold text-sm">➔</div>
            <div class="text-right">
              <div class="text-[11px] text-amber-400 font-medium">可升级版本</div>
              <div class="text-sm font-bold text-amber-300 font-mono mt-0.5">v{{ updateInfo.latestVersion }}</div>
            </div>
          </div>

          <!-- 通道选择 -->
          <div class="space-y-1.5">
            <span class="text-[11px] text-m3-on-surface-variant block font-medium">下载更新通道：</span>
            <div class="flex gap-2">
              <button
                v-if="updateInfo.lanAvailable"
                @click="selectedUpdateSource = 'lan'"
                class="flex-1 py-2 px-3 rounded-xl border text-xs font-medium transition flex items-center justify-center gap-1.5"
                :class="selectedUpdateSource === 'lan' ? 'bg-m3-success text-white border-transparent font-bold shadow-sm' : 'bg-m3-surface-high border-m3-outline text-m3-on-surface-variant hover:text-m3-on-surface'"
              >
                <Wifi class="w-3.5 h-3.5" /> 局域网极速 (免流量)
              </button>
              <button
                v-if="updateInfo.githubAvailable"
                @click="selectedUpdateSource = 'github'"
                class="flex-1 py-2 px-3 rounded-xl border text-xs font-medium transition flex items-center justify-center gap-1.5"
                :class="selectedUpdateSource === 'github' ? 'bg-m3-primary text-white border-transparent font-bold shadow-sm' : 'bg-m3-surface-high border-m3-outline text-m3-on-surface-variant hover:text-m3-on-surface'"
              >
                <Github class="w-3.5 h-3.5" /> GitHub 官方源
              </button>
            </div>
            <div class="text-[11px] text-m3-on-surface-dim font-mono">
              来源：{{ selectedUpdateSource === 'lan' ? updateInfo.providerName : 'GitHub Releases (Hotpotfishsummer/RW-Mod-Manager)' }} | 大小：{{ formatBytes(updateInfo.fileSize) }}
            </div>
          </div>

          <!-- 更新说明 -->
          <div v-if="updateInfo.releaseNotes" class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl space-y-1">
            <span class="text-[11px] text-m3-on-surface-variant font-medium">更新内容日志：</span>
            <div class="text-xs text-m3-on-surface whitespace-pre-wrap leading-relaxed max-h-36 overflow-y-auto select-text font-sans">
              {{ updateInfo.releaseNotes }}
            </div>
          </div>

          <!-- 下载进度条 -->
          <div v-if="updating && updateProgress" class="space-y-1.5 pt-1">
            <div class="flex items-center justify-between text-[11px] font-mono">
              <span class="text-m3-primary font-bold">{{ updateProgress.message }}</span>
              <span class="text-m3-on-surface-variant">{{ formatBytes(updateProgress.speedBytes) }}/s</span>
            </div>
            <div class="w-full bg-m3-surface-high rounded-full h-2.5 overflow-hidden border border-m3-outline-variant">
              <div
                class="bg-gradient-to-r from-m3-primary to-m3-success h-full transition-all duration-200"
                :style="{ width: updateProgress.percentage + '%' }"
              />
            </div>
          </div>
        </div>

        <div v-else class="py-6 text-center text-xs text-m3-on-surface-variant space-y-2">
          <CheckCircle2 class="w-8 h-8 text-m3-success mx-auto" />
          <p class="font-medium">当前已是最新版本 (v{{ currentVersion }})</p>
          <p class="text-[11px] text-m3-on-surface-dim">局域网与 GitHub 暂无更高版本</p>
        </div>

        <div class="flex items-center justify-end gap-2.5 pt-2 border-t border-m3-outline-variant">
          <button
            @click="showUpdateModal = false"
            class="px-4 py-2 bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface rounded-xl text-xs border border-m3-outline transition font-medium"
          >
            关闭
          </button>
          <button
            v-if="updateInfo?.hasUpdate"
            @click="handlePerformUpdate"
            :disabled="updating"
            class="px-5 py-2 bg-m3-primary hover:bg-m3-primary-hover text-white rounded-xl text-xs font-semibold shadow-m3-1 flex items-center gap-1.5 transition active:scale-95 disabled:opacity-50"
          >
            <DownloadCloud class="w-3.5 h-3.5" :class="{ 'animate-bounce': updating }" />
            <span>{{ updating ? '正在下载升级中...' : '立即下载并自更新' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- 5. 全局通知 Toast 浮层 (M3 Snackbar) -->
    <div
      v-if="lanNoticeMsg"
      class="fixed bottom-6 right-6 z-40 px-4 py-3 rounded-2xl border shadow-m3-2 flex items-center gap-3 text-xs font-medium animate-in slide-in-from-bottom-5 bg-m3-surface-high"
      :class="
        lanNoticeMsg.type === 'success'
          ? 'border-m3-success/50 text-m3-on-surface'
          : lanNoticeMsg.type === 'error'
          ? 'border-m3-error/50 text-m3-on-surface'
          : 'border-m3-primary/50 text-m3-on-surface'
      "
    >
      <CheckCircle2 v-if="lanNoticeMsg.type === 'success'" class="w-4 h-4 text-m3-success" />
      <AlertTriangle v-else-if="lanNoticeMsg.type === 'error'" class="w-4 h-4 text-m3-error" />
      <Sparkles v-else class="w-4 h-4 text-m3-primary" />
      <span>{{ lanNoticeMsg.text }}</span>
      <button @click="lanNoticeMsg = null" class="ml-2 text-m3-on-surface-dim hover:text-m3-on-surface">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  </div>
</template>
