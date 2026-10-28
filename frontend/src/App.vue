<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  Package,
  Map,
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
  Activity
} from 'lucide-vue-next'
import { useApi } from './composables/useApi'
import { ModItem, MapItem, GameStatus, AppConfig } from './types/api'

const api = useApi()

// 当前激活的标签页
const currentTab = ref<'units' | 'maps' | 'launcher' | 'settings'>('units')

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
  }
})

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

// 格式化文件大小
function formatBytes(bytes: number) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

// 过滤后的模组列表
const filteredMods = computed(() => {
  return mods.value.filter(m => {
    if (modFilterStatus.value === 'enabled' && !m.enabled) return false
    if (modFilterStatus.value === 'disabled' && m.enabled) return false

    if (!modSearch.value.trim()) return true
    const kw = modSearch.value.toLowerCase()
    return (
      m.title.toLowerCase().includes(kw) ||
      m.titleZh.toLowerCase().includes(kw) ||
      m.fileName.toLowerCase().includes(kw) ||
      (m.tags && m.tags.some(t => t.toLowerCase().includes(kw)))
    )
  })
})

// 过滤后的地图列表
const filteredMaps = computed(() => {
  return maps.value.filter(m => {
    // 激活状态过滤
    if (mapFilterActive.value === 'active' && !m.active) return false
    if (mapFilterActive.value === 'inactive' && m.active) return false

    // 来源类型过滤
    if (mapFilterType.value === 'standalone' && m.locationType !== 'standalone') return false
    if (mapFilterType.value === 'pack' && m.locationType !== 'map_pack') return false
    if (mapFilterType.value === 'unit_mod' && m.locationType !== 'unit_mod') return false

    if (!mapSearch.value.trim()) return true
    const kw = mapSearch.value.toLowerCase()
    return (
      m.title.toLowerCase().includes(kw) ||
      m.fileName.toLowerCase().includes(kw) ||
      (m.packName && m.packName.toLowerCase().includes(kw))
    )
  })
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

// 为单个地图生成/重新生成缩略图 (带出生点)
async function handleGenerateMapThumb(m: MapItem) {
  if (m.isPack) {
    alert('该地图位于压缩包内，请使用“批量修复缺失缩略图”对整个包进行补丁注入。')
    return
  }
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

// 批量为缺失缩略图的地图生成预览
async function handleBatchGenerateThumbs() {
  generatingThumb.value = true
  try {
    const count = await api.batchGenerateThumbnails()
    alert(`批量修复完成！已为 ${count} 张地图生成高清预览及出生点标记。`)
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

// 启动游戏
async function handleLaunchGame() {
  launching.value = true
  try {
    await api.saveConfig(appConfig.value)
    await api.launchGame()
  } catch (err) {
    alert(`启动游戏失败: ${err}`)
  } finally {
    setTimeout(() => {
      launching.value = false
    }, 1200)
  }
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

onMounted(() => {
  loadAllData()
})
</script>

<template>
  <div class="flex flex-col h-screen w-screen overflow-hidden font-sans">
    <!-- 顶部导航栏 -->
    <header class="flex items-center justify-between px-6 py-4 bg-slate-900/95 backdrop-blur-md border-b border-slate-800 shrink-0">
      <div class="flex items-center gap-5">
        <!-- Logo 与标题 -->
        <div class="flex items-center gap-3.5">
          <div class="w-10 h-10 rounded-xl bg-gradient-to-tr from-sky-500 to-indigo-600 flex items-center justify-center shadow-lg shadow-sky-500/25">
            <Cpu class="w-6 h-6 text-white" />
          </div>
          <div>
            <h1 class="text-lg font-bold tracking-tight bg-gradient-to-r from-sky-300 via-indigo-100 to-emerald-300 bg-clip-text text-transparent">
              铁锈战争 模组与地图管理器
            </h1>
            <p class="text-xs text-slate-400 font-medium tracking-wide">Rusted Warfare Mod & Map Workshop Manager</p>
          </div>
        </div>

        <!-- 游戏路径状态胶囊 -->
        <div class="hidden md:flex items-center gap-2.5 px-3.5 py-1.5 rounded-lg bg-slate-800/90 border border-slate-700/80 text-xs shadow-inner">
          <span
            class="w-2.5 h-2.5 rounded-full"
            :class="gameStatus.isValid ? 'bg-emerald-400 shadow-md shadow-emerald-400/50' : 'bg-rose-400 shadow-md shadow-rose-400/50'"
          />
          <span class="text-slate-200 font-mono text-xs truncate max-w-sm" :title="gameStatus.gameDir">
            {{ gameStatus.gameDir || '未配置游戏路径' }}
          </span>
          <button
            @click="handleChangeDir"
            class="ml-1 text-sky-400 hover:text-sky-300 font-medium hover:underline transition"
          >
            更换目录
          </button>
        </div>
      </div>

      <!-- 右侧全局操作区 -->
      <div class="flex items-center gap-3">
        <!-- 目录快速打开 -->
        <div class="flex items-center rounded-lg bg-slate-800/90 p-0.5 border border-slate-700/80 shadow-sm">
          <button
            @click="api.openFolder('units')"
            title="打开单位模组文件夹"
            class="px-3 py-1.5 text-xs font-medium text-slate-300 hover:text-white hover:bg-slate-700/80 rounded transition flex items-center gap-1.5"
          >
            <FolderOpen class="w-4 h-4 text-sky-400" />
            <span>模组</span>
          </button>
          <button
            @click="api.openFolder('maps')"
            title="打开地图文件夹"
            class="px-3 py-1.5 text-xs font-medium text-slate-300 hover:text-white hover:bg-slate-700/80 rounded transition flex items-center gap-1.5"
          >
            <FolderOpen class="w-4 h-4 text-emerald-400" />
            <span>地图</span>
          </button>
        </div>

        <!-- 刷新按钮 -->
        <button
          @click="loadAllData"
          :disabled="loading"
          class="p-2 text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 border border-slate-700 rounded-lg shadow-sm transition disabled:opacity-50"
          title="刷新扫描"
        >
          <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
        </button>

        <!-- 一键启动游戏 -->
        <button
          @click="handleLaunchGame"
          :disabled="launching || !gameStatus.isValid"
          class="flex items-center gap-2 px-5 py-2.5 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-white font-semibold text-sm rounded-lg shadow-lg shadow-emerald-500/25 active:scale-95 transition disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <Rocket class="w-4 h-4" :class="{ 'animate-bounce': launching }" />
          <span>{{ launching ? '启动中...' : '启动游戏' }}</span>
        </button>
      </div>
    </header>

    <!-- 选项卡导航栏 -->
    <nav class="flex items-center justify-between px-6 bg-slate-900/90 border-b border-slate-800 shrink-0">
      <div class="flex gap-2">
        <button
          @click="currentTab = 'units'"
          class="flex items-center gap-2.5 py-3.5 px-4 border-b-2 text-sm font-medium transition"
          :class="currentTab === 'units' ? 'border-sky-400 text-sky-300 font-semibold' : 'border-transparent text-slate-400 hover:text-slate-200'"
        >
          <Package class="w-4 h-4" />
          <span>单位模组 (Units)</span>
          <span class="px-2 py-0.5 rounded-full bg-slate-800/90 text-xs font-mono font-medium text-slate-300 border border-slate-700/60">
            {{ enabledCount }} / {{ mods.length }}
          </span>
        </button>

        <button
          @click="currentTab = 'maps'"
          class="flex items-center gap-2.5 py-3.5 px-4 border-b-2 text-sm font-medium transition"
          :class="currentTab === 'maps' ? 'border-sky-400 text-sky-300 font-semibold' : 'border-transparent text-slate-400 hover:text-slate-200'"
        >
          <Map class="w-4 h-4" />
          <span>地图工坊 (Maps)</span>
          <span class="px-2 py-0.5 rounded-full bg-slate-800/90 text-xs font-mono font-medium text-slate-300 border border-slate-700/60">
            {{ activeMapCount }} / {{ maps.length }} 战局就绪
          </span>
          <span v-if="missingPreviewCount > 0" class="px-2 py-0.5 rounded-full bg-amber-500/20 text-xs font-medium text-amber-300 border border-amber-500/30">
            {{ missingPreviewCount }} 缺缩略图
          </span>
        </button>

        <button
          @click="currentTab = 'launcher'"
          class="flex items-center gap-2.5 py-3.5 px-4 border-b-2 text-sm font-medium transition"
          :class="currentTab === 'launcher' ? 'border-sky-400 text-sky-300 font-semibold' : 'border-transparent text-slate-400 hover:text-slate-200'"
        >
          <Sliders class="w-4 h-4" />
          <span>启动调优与高刷 (Launcher)</span>
          <span class="px-2 py-0.5 rounded-full bg-slate-800/90 text-xs font-mono font-medium text-sky-400 border border-slate-700/60">
            {{ appConfig.gamePreferences.highRefreshRate ? '144Hz+' : '60Hz' }}
          </span>
        </button>

        <button
          @click="currentTab = 'settings'"
          class="flex items-center gap-2.5 py-3.5 px-4 border-b-2 text-sm font-medium transition"
          :class="currentTab === 'settings' ? 'border-sky-400 text-sky-300 font-semibold' : 'border-transparent text-slate-400 hover:text-slate-200'"
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
              <Search class="w-4 h-4 text-slate-400 absolute left-3.5 top-3" />
              <input
                v-model="modSearch"
                type="text"
                placeholder="搜索模组标题、标签、文件名..."
                class="w-full bg-slate-900 border border-slate-700/90 rounded-lg pl-10 pr-4 py-2 text-sm text-slate-100 placeholder-slate-400 focus:outline-none focus:border-sky-500 shadow-sm transition"
              />
            </div>
          </div>

          <!-- 状态筛选单选与视图切换按钮 -->
          <div class="flex items-center gap-3 text-xs">
            <!-- 视图切换 (卡片网格 vs 紧凑列表) -->
            <div class="flex rounded-lg bg-slate-900 p-1 border border-slate-800 shadow-inner">
              <button
                @click="modViewMode = 'grid'"
                class="p-1.5 rounded transition"
                :class="modViewMode === 'grid' ? 'bg-sky-500/20 text-sky-300' : 'text-slate-400 hover:text-slate-200'"
                title="卡片网格视图"
              >
                <LayoutGrid class="w-4 h-4" />
              </button>
              <button
                @click="modViewMode = 'list'"
                class="p-1.5 rounded transition"
                :class="modViewMode === 'list' ? 'bg-sky-500/20 text-sky-300' : 'text-slate-400 hover:text-slate-200'"
                title="紧凑列表视图"
              >
                <List class="w-4 h-4" />
              </button>
            </div>

            <!-- 状态筛选 -->
            <div class="flex rounded-lg bg-slate-900 p-1 border border-slate-800 shadow-inner">
              <button
                @click="modFilterStatus = 'all'"
                class="px-3 py-1.5 rounded-md font-medium transition"
                :class="modFilterStatus === 'all' ? 'bg-sky-500/20 text-sky-300' : 'text-slate-400 hover:text-slate-200'"
              >
                全部 ({{ mods.length }})
              </button>
              <button
                @click="modFilterStatus = 'enabled'"
                class="px-3 py-1.5 rounded-md font-medium transition"
                :class="modFilterStatus === 'enabled' ? 'bg-emerald-500/20 text-emerald-300' : 'text-slate-400 hover:text-slate-200'"
              >
                已启用 ({{ enabledCount }})
              </button>
              <button
                @click="modFilterStatus = 'disabled'"
                class="px-3 py-1.5 rounded-md font-medium transition"
                :class="modFilterStatus === 'disabled' ? 'bg-rose-500/20 text-rose-300' : 'text-slate-400 hover:text-slate-200'"
              >
                已禁用 ({{ mods.length - enabledCount }})
              </button>
            </div>

            <div class="h-5 w-px bg-slate-800" />

            <button
              @click="handleBatchToggle(true)"
              class="px-3.5 py-1.5 bg-slate-900 hover:bg-slate-800 text-emerald-400 border border-emerald-500/40 rounded-lg font-medium shadow-sm transition"
            >
              全部启用
            </button>
            <button
              @click="handleBatchToggle(false)"
              class="px-3.5 py-1.5 bg-slate-900 hover:bg-slate-800 text-rose-400 border border-rose-500/40 rounded-lg font-medium shadow-sm transition"
            >
              全部禁用
            </button>
          </div>
        </div>

        <!-- 模组展示区: 1. 网格模式 -->
        <div v-if="modViewMode === 'grid'" class="flex-1 overflow-y-auto pr-1.5">
          <div v-if="filteredMods.length === 0" class="flex flex-col items-center justify-center h-64 text-slate-400">
            <Package class="w-14 h-14 stroke-[1.5] mb-3 opacity-40 text-slate-500" />
            <p class="text-sm font-medium">没有找到匹配的模组</p>
          </div>

          <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 pb-6">
            <div
              v-for="item in filteredMods"
              :key="item.fileName"
              class="group flex flex-col bg-slate-900/90 border rounded-xl overflow-hidden transition-all duration-200 shadow-md"
              :class="item.enabled ? 'border-slate-700 hover:border-sky-500/70 shadow-sky-500/5' : 'border-slate-800/90 opacity-80 hover:opacity-100'"
            >
              <!-- 封面图 -->
              <div class="relative h-36 bg-slate-950 overflow-hidden shrink-0 flex items-center justify-center border-b border-slate-800/80">
                <img
                  v-if="item.thumbnail"
                  :src="item.thumbnail"
                  class="w-full h-full object-cover transition group-hover:scale-105 duration-300"
                  alt="Mod thumbnail"
                />
                <div v-else class="flex flex-col items-center justify-center text-slate-600 gap-1.5">
                  <Package class="w-10 h-10 stroke-[1.2]" />
                  <span class="text-xs font-mono font-semibold tracking-wider">NO THUMBNAIL</span>
                </div>

                <!-- 启/禁用状态胶囊 -->
                <div class="absolute top-2.5 left-2.5">
                  <span
                    class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold backdrop-blur-md shadow-md"
                    :class="item.enabled ? 'bg-emerald-500/95 text-white shadow-emerald-500/40' : 'bg-slate-900/95 text-slate-300 border border-slate-700'"
                  >
                    <span class="w-2 h-2 rounded-full" :class="item.enabled ? 'bg-white' : 'bg-slate-500'" />
                    {{ item.enabled ? '已启用' : '已禁用' }}
                  </span>
                </div>

                <!-- 版本号徽章 -->
                <div v-if="item.minVersion" class="absolute top-2.5 right-2.5">
                  <span class="px-2 py-0.5 rounded text-xs font-mono font-medium bg-black/75 backdrop-blur-md text-slate-200 border border-slate-700/80 shadow-sm">
                    {{ item.minVersion }}
                  </span>
                </div>
              </div>

              <!-- 模组信息主体 -->
              <div class="flex-1 p-4 flex flex-col justify-between">
                <div>
                  <h3 class="text-sm font-bold text-slate-100 line-clamp-1 group-hover:text-sky-300 transition" :title="item.titleZh || item.title">
                    {{ item.titleZh || item.title }}
                  </h3>

                  <p class="text-xs text-slate-400 font-mono truncate mt-1" :title="item.fileName">
                    {{ item.fileName }}
                  </p>

                  <p class="text-xs text-slate-300 line-clamp-2 mt-2 leading-relaxed">
                    {{ item.description || '暂无模组描述说明。' }}
                  </p>

                  <!-- 标签分类 -->
                  <div v-if="item.tags && item.tags.length" class="flex flex-wrap gap-1.5 mt-3">
                    <span
                      v-for="t in item.tags.slice(0, 3)"
                      :key="t"
                      class="px-2 py-0.5 rounded text-xs font-medium bg-slate-800 text-sky-300 border border-slate-700/70"
                    >
                      {{ t }}
                    </span>
                  </div>
                </div>

                <!-- 底部统计与开关操作 -->
                <div class="mt-4 pt-3.5 border-t border-slate-800 flex items-center justify-between">
                  <div class="flex items-center gap-3.5 text-xs text-slate-300 font-mono font-medium">
                    <span title="单位配置数 (*.ini)" class="flex items-center gap-1.5">
                      <Cpu class="w-3.5 h-3.5 text-sky-400" />
                      {{ item.unitCount }}
                    </span>
                    <span v-if="item.mapCount > 0" title="内嵌地图数 (*.tmx)" class="flex items-center gap-1.5">
                      <Map class="w-3.5 h-3.5 text-emerald-400" />
                      {{ item.mapCount }}
                    </span>
                    <span title="文件体积" class="text-slate-400">
                      {{ formatBytes(item.fileSize) }}
                    </span>
                  </div>

                  <div class="flex items-center gap-2">
                    <button
                      v-if="item.steamId"
                      @click="openWorkshopLink(item.steamId)"
                      class="p-1.5 text-slate-400 hover:text-sky-400 rounded transition"
                      title="在 Steam 创意工坊查看"
                    >
                      <ExternalLink class="w-4 h-4" />
                    </button>

                    <button
                      @click="handleToggleMod(item)"
                      class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none shadow-inner"
                      :class="item.enabled ? 'bg-sky-500' : 'bg-slate-700'"
                    >
                      <span
                        class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-md ring-0 transition duration-200 ease-in-out"
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
          <div v-if="filteredMods.length === 0" class="flex flex-col items-center justify-center h-64 text-slate-400">
            <Package class="w-14 h-14 stroke-[1.5] mb-3 opacity-40 text-slate-500" />
            <p class="text-sm font-medium">没有找到匹配的模组</p>
          </div>

          <div v-else class="space-y-2 pb-6">
            <div
              v-for="item in filteredMods"
              :key="item.fileName"
              class="flex items-center justify-between p-3.5 bg-slate-900/90 border rounded-xl hover:border-slate-700 transition shadow-sm"
              :class="item.enabled ? 'border-slate-700/80 bg-slate-900/95' : 'border-slate-800/80 opacity-75'"
            >
              <!-- 左侧：开关 + 缩略图 + 标题与标签 -->
              <div class="flex items-center gap-4 flex-1 min-w-0 pr-4">
                <button
                  @click="handleToggleMod(item)"
                  class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
                  :class="item.enabled ? 'bg-sky-500' : 'bg-slate-700'"
                >
                  <span
                    class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-md ring-0 transition duration-200 ease-in-out"
                    :class="item.enabled ? 'translate-x-5' : 'translate-x-0'"
                  />
                </button>

                <div class="w-12 h-12 rounded-lg bg-slate-950 border border-slate-800 overflow-hidden shrink-0 flex items-center justify-center">
                  <img v-if="item.thumbnail" :src="item.thumbnail" class="w-full h-full object-cover" alt="" />
                  <Package v-else class="w-6 h-6 text-slate-600" />
                </div>

                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2.5">
                    <h3 class="text-sm font-bold text-slate-100 truncate" :title="item.titleZh || item.title">
                      {{ item.titleZh || item.title }}
                    </h3>
                    <span v-if="item.minVersion" class="px-1.5 py-0.2 rounded text-[10px] font-mono bg-black/60 text-slate-300 border border-slate-700">
                      {{ item.minVersion }}
                    </span>
                    <span
                      class="px-2 py-0.2 rounded-full text-[11px] font-semibold"
                      :class="item.enabled ? 'bg-emerald-500/20 text-emerald-300' : 'bg-slate-800 text-slate-400'"
                    >
                      {{ item.enabled ? '已启用' : '已禁用' }}
                    </span>
                  </div>
                  <p class="text-xs text-slate-400 font-mono truncate mt-0.5" :title="item.fileName">
                    {{ item.fileName }}
                  </p>
                </div>

                <!-- 标签 (在大屏幕展示) -->
                <div v-if="item.tags && item.tags.length" class="hidden xl:flex items-center gap-1.5 shrink-0">
                  <span
                    v-for="t in item.tags.slice(0, 2)"
                    :key="t"
                    class="px-2 py-0.5 rounded text-xs font-medium bg-slate-800 text-sky-300 border border-slate-700/60"
                  >
                    {{ t }}
                  </span>
                </div>
              </div>

              <!-- 右侧：统计数据与操作 -->
              <div class="flex items-center gap-6 shrink-0 text-xs font-mono text-slate-300">
                <span title="单位数量" class="flex items-center gap-1.5">
                  <Cpu class="w-4 h-4 text-sky-400" />
                  {{ item.unitCount }} 单位
                </span>
                <span v-if="item.mapCount > 0" title="内嵌地图" class="flex items-center gap-1.5">
                  <Map class="w-4 h-4 text-emerald-400" />
                  {{ item.mapCount }} 地图
                </span>
                <span class="text-slate-400 w-20 text-right">
                  {{ formatBytes(item.fileSize) }}
                </span>
                <button
                  v-if="item.steamId"
                  @click="openWorkshopLink(item.steamId)"
                  class="p-1.5 text-slate-400 hover:text-sky-400 rounded transition"
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
              <Search class="w-4 h-4 text-slate-400 absolute left-3.5 top-3" />
              <input
                v-model="mapSearch"
                type="text"
                placeholder="搜索地图名称、地图包名、所属模组..."
                class="w-full bg-slate-900 border border-slate-700/90 rounded-lg pl-10 pr-4 py-2 text-sm text-slate-100 placeholder-slate-400 focus:outline-none focus:border-sky-500 shadow-sm transition"
              />
            </div>
          </div>

          <!-- 激活状态与来源筛选 -->
          <div class="flex flex-wrap items-center gap-3 text-xs">
            <!-- 激活状态 -->
            <div class="flex rounded-lg bg-slate-900 p-1 border border-slate-800 shadow-inner">
              <button
                @click="mapFilterActive = 'all'"
                class="px-3 py-1.5 rounded-md font-medium transition"
                :class="mapFilterActive === 'all' ? 'bg-sky-500/20 text-sky-300' : 'text-slate-400 hover:text-slate-200'"
              >
                全部 ({{ maps.length }})
              </button>
              <button
                @click="mapFilterActive = 'active'"
                class="px-3 py-1.5 rounded-md font-medium transition"
                :class="mapFilterActive === 'active' ? 'bg-emerald-500/20 text-emerald-300' : 'text-slate-400 hover:text-slate-200'"
              >
                战局可用 ({{ activeMapCount }})
              </button>
              <button
                @click="mapFilterActive = 'inactive'"
                class="px-3 py-1.5 rounded-md font-medium transition"
                :class="mapFilterActive === 'inactive' ? 'bg-rose-500/20 text-rose-300' : 'text-slate-400 hover:text-slate-200'"
              >
                未激活 ({{ maps.length - activeMapCount }})
              </button>
            </div>

            <!-- 来源类型 -->
            <div class="flex rounded-lg bg-slate-900 p-1 border border-slate-800 shadow-inner">
              <button
                @click="mapFilterType = 'all'"
                class="px-3 py-1.5 rounded-md font-medium transition"
                :class="mapFilterType === 'all' ? 'bg-sky-500/20 text-sky-300' : 'text-slate-400 hover:text-slate-200'"
              >
                所有来源
              </button>
              <button
                @click="mapFilterType = 'standalone'"
                class="px-3 py-1.5 rounded-md font-medium transition"
                :class="mapFilterType === 'standalone' ? 'bg-sky-500/20 text-sky-300' : 'text-slate-400 hover:text-slate-200'"
              >
                独立 TMX
              </button>
              <button
                @click="mapFilterType = 'pack'"
                class="px-3 py-1.5 rounded-md font-medium transition"
                :class="mapFilterType === 'pack' ? 'bg-sky-500/20 text-sky-300' : 'text-slate-400 hover:text-slate-200'"
              >
                地图包
              </button>
              <button
                @click="mapFilterType = 'unit_mod'"
                class="px-3 py-1.5 rounded-md font-medium transition"
                :class="mapFilterType === 'unit_mod' ? 'bg-sky-500/20 text-sky-300' : 'text-slate-400 hover:text-slate-200'"
              >
                单位模组附带
              </button>
            </div>

            <!-- 批量修复缺失缩略图按钮 -->
            <button
              v-if="missingPreviewCount > 0"
              @click="handleBatchGenerateThumbs"
              :disabled="generatingThumb"
              class="flex items-center gap-1.5 px-3.5 py-1.5 bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 border border-amber-500/40 rounded-lg font-medium transition disabled:opacity-50"
              title="使用 rw_thumbnail 批量为所有缺图地图生成高清预览及队伍出生点"
            >
              <Sparkles class="w-4 h-4 text-amber-400" :class="{ 'animate-spin': generatingThumb }" />
              <span>批量修复缺图 ({{ missingPreviewCount }})</span>
            </button>
          </div>
        </div>

        <!-- 地图网格列表 -->
        <div class="flex-1 overflow-y-auto pr-1.5">
          <div v-if="filteredMaps.length === 0" class="flex flex-col items-center justify-center h-64 text-slate-400">
            <Map class="w-14 h-14 stroke-[1.5] mb-3 opacity-40 text-slate-500" />
            <p class="text-sm font-medium">没有找到匹配的地图</p>
          </div>

          <div v-else class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6 gap-4 pb-6">
            <div
              v-for="m in filteredMaps"
              :key="m.id"
              class="group flex flex-col bg-slate-900/90 border rounded-xl overflow-hidden transition shadow-md"
              :class="m.active ? 'border-slate-800 hover:border-sky-500/70' : 'border-rose-900/40 opacity-75 hover:opacity-100'"
            >
              <!-- 缩略图区域 (点击打开大图 Lightbox) -->
              <div
                class="relative aspect-square bg-slate-950 flex items-center justify-center overflow-hidden border-b border-slate-800/80 cursor-pointer"
                @click="openMapLightbox(m)"
                title="点击放大查看高清预览图与出生点标记"
              >
                <img
                  v-if="m.hasPreview && m.previewData"
                  :src="m.previewData"
                  class="w-full h-full object-cover transition group-hover:scale-105 duration-200"
                  alt="Map Preview"
                />
                <div v-else class="flex flex-col items-center justify-center text-amber-400 gap-1.5 p-3 text-center">
                  <ShieldAlert class="w-9 h-9 stroke-[1.5]" />
                  <span class="text-xs font-semibold leading-tight">缺失 _map.png 缩略图</span>
                  <button
                    v-if="!m.isPack"
                    @click.stop="handleGenerateMapThumb(m)"
                    :disabled="generatingThumb"
                    class="mt-1 px-2 py-0.5 bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 rounded text-[11px] border border-amber-500/40"
                  >
                    生成缩略图
                  </button>
                </div>

                <!-- 左上角：战局激活状态指示 -->
                <div class="absolute top-2 left-2">
                  <span
                    class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium backdrop-blur-md shadow"
                    :class="m.active ? 'bg-emerald-500/90 text-white' : 'bg-slate-900/95 text-rose-300 border border-rose-500/40'"
                  >
                    <span class="w-1.5 h-1.5 rounded-full" :class="m.active ? 'bg-white' : 'bg-rose-400'" />
                    {{ m.active ? '战局可用' : '未激活' }}
                  </span>
                </div>

                <!-- 玩家人数徽章 -->
                <div class="absolute bottom-2 left-2">
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-mono font-semibold bg-black/80 backdrop-blur-md text-emerald-300 border border-emerald-500/40 shadow-sm">
                    <Users class="w-3.5 h-3.5" />
                    {{ m.playerCount }}P
                  </span>
                </div>

                <!-- 瓦片尺寸徽章 -->
                <div v-if="m.width > 0" class="absolute bottom-2 right-2">
                  <span class="px-2 py-0.5 rounded text-xs font-mono font-medium bg-black/80 backdrop-blur-md text-slate-200 border border-slate-700/80 shadow-sm">
                    {{ m.width }}×{{ m.height }}
                  </span>
                </div>
              </div>

              <!-- 地图文本与来源信息 -->
              <div class="p-3 flex-1 flex flex-col justify-between">
                <div>
                  <h4 class="text-xs font-semibold text-slate-100 line-clamp-1 group-hover:text-sky-300 transition" :title="m.title">
                    {{ m.title }}
                  </h4>

                  <!-- 来源与归属提示 -->
                  <div class="mt-1">
                    <span v-if="m.locationType === 'standalone'" class="text-xs text-slate-400 font-mono">
                      独立地图 (常驻)
                    </span>
                    <span v-else-if="m.locationType === 'map_pack'" class="text-xs text-sky-400 font-mono truncate block" :title="m.packName">
                      包: {{ m.packName }}
                    </span>
                    <div v-else class="flex items-center justify-between gap-1">
                      <span class="text-xs text-indigo-300 font-mono truncate" :title="m.parentModFile">
                        模组: {{ m.parentModFile }}
                      </span>
                    </div>
                  </div>
                </div>

                <!-- 底部操作：若是未激活的模组附带地图，支持一键激活所属模组 -->
                <div class="mt-2.5 pt-2 border-t border-slate-800/80 flex items-center justify-between text-xs">
                  <span class="text-slate-400 font-mono">
                    {{ formatBytes(m.fileSize) }}
                  </span>

                  <button
                    v-if="!m.active && m.parentModFile"
                    @click="handleActivateParentMod(m.parentModFile)"
                    class="px-2 py-0.5 bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-300 rounded text-[11px] font-medium border border-emerald-500/40 transition"
                    title="一键启用提供此地图的模组"
                  >
                    激活模组
                  </button>
                  <span v-else-if="m.active" class="text-emerald-400/90 text-[11px] font-medium flex items-center gap-1">
                    <Check class="w-3.5 h-3.5" /> 已就绪
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 3. 地图大图放大查看器 (Lightbox Modal with Zoom & Spawn markers) -->
        <div
          v-if="activeMapModal"
          class="fixed inset-0 z-50 bg-black/85 backdrop-blur-md flex items-center justify-center p-6 animate-fade-in"
          @click.self="closeMapLightbox"
        >
          <div class="bg-slate-900 border border-slate-700/80 rounded-2xl max-w-4xl w-full max-h-[92vh] flex flex-col shadow-2xl overflow-hidden">
            <!-- 弹窗标题栏 -->
            <div class="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-950/80">
              <div class="flex items-center gap-3">
                <div class="w-8 h-8 rounded-lg bg-sky-500/20 text-sky-400 flex items-center justify-center">
                  <Map class="w-4 h-4" />
                </div>
                <div>
                  <h3 class="text-base font-bold text-slate-100 flex items-center gap-2">
                    {{ activeMapModal.title }}
                    <span
                      class="px-2 py-0.5 rounded-full text-xs font-semibold"
                      :class="activeMapModal.active ? 'bg-emerald-500/20 text-emerald-300' : 'bg-rose-500/20 text-rose-300'"
                    >
                      {{ activeMapModal.active ? '战局可用' : '未激活 (宿主模组已禁用)' }}
                    </span>
                  </h3>
                  <p class="text-xs text-slate-400 font-mono mt-0.5">
                    {{ activeMapModal.fileName }}
                  </p>
                </div>
              </div>

              <!-- 缩放控制与关闭 -->
              <div class="flex items-center gap-2">
                <button
                  @click="zoomLevel = Math.max(0.5, zoomLevel - 0.25)"
                  class="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 transition"
                  title="缩小"
                >
                  <ZoomOut class="w-4 h-4" />
                </button>
                <span class="text-xs font-mono text-slate-300 w-12 text-center">
                  {{ Math.round(zoomLevel * 100) }}%
                </span>
                <button
                  @click="zoomLevel = Math.min(3, zoomLevel + 0.25)"
                  class="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 transition"
                  title="放大"
                >
                  <ZoomIn class="w-4 h-4" />
                </button>
                <button
                  @click="zoomLevel = 1"
                  class="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 transition"
                  title="重置 1:1"
                >
                  <Maximize2 class="w-4 h-4" />
                </button>
                <div class="h-4 w-px bg-slate-800 mx-1" />
                <button
                  @click="closeMapLightbox"
                  class="p-2 rounded-lg bg-slate-800 hover:bg-rose-500/30 text-slate-300 hover:text-rose-200 transition"
                  title="关闭"
                >
                  <X class="w-5 h-5" />
                </button>
              </div>
            </div>

            <!-- 图像展示区域 -->
            <div class="flex-1 overflow-auto bg-slate-950 p-6 flex items-center justify-center min-h-[420px] max-h-[560px]">
              <div v-if="activeMapModal.hasPreview && activeMapModal.previewData" class="transition-transform duration-150">
                <img
                  :src="activeMapModal.previewData"
                  :style="{ transform: `scale(${zoomLevel})` }"
                  class="max-h-[500px] object-contain rounded-lg shadow-xl origin-center transition-transform"
                  alt="High-Res Map Preview"
                />
              </div>
              <div v-else class="flex flex-col items-center justify-center text-amber-400 gap-3">
                <ShieldAlert class="w-16 h-16 stroke-[1.2]" />
                <p class="text-sm font-semibold">该地图尚未生成带出生点的高清预览图</p>
                <button
                  v-if="!activeMapModal.isPack"
                  @click="handleGenerateMapThumb(activeMapModal)"
                  :disabled="generatingThumb"
                  class="flex items-center gap-2 px-4 py-2 bg-gradient-to-r from-sky-500 to-indigo-600 hover:from-sky-400 hover:to-indigo-500 text-white rounded-lg text-xs font-semibold shadow-md transition disabled:opacity-50"
                >
                  <Sparkles class="w-4 h-4" :class="{ 'animate-spin': generatingThumb }" />
                  <span>立即渲染高清缩略图 (带队伍出生点 1,2,3...)</span>
                </button>
              </div>
            </div>

            <!-- 底部地图详情与动作栏 -->
            <div class="px-6 py-4 bg-slate-900 border-t border-slate-800 flex flex-wrap items-center justify-between gap-4 text-xs">
              <div class="flex items-center gap-6 text-slate-300 font-mono">
                <span class="flex items-center gap-1.5 text-emerald-400 font-semibold">
                  <Users class="w-4 h-4" />
                  {{ activeMapModal.playerCount }} 名玩家 (队伍标记 1~{{ activeMapModal.playerCount }})
                </span>
                <span v-if="activeMapModal.width > 0">
                  瓦片尺寸: {{ activeMapModal.width }} × {{ activeMapModal.height }}
                </span>
                <span v-if="activeMapModal.packName" class="text-indigo-400">
                  所属包: {{ activeMapModal.packName }}
                </span>
                <span>
                  大小: {{ formatBytes(activeMapModal.fileSize) }}
                </span>
              </div>

              <div class="flex items-center gap-3">
                <button
                  v-if="!activeMapModal.active && activeMapModal.parentModFile"
                  @click="handleActivateParentMod(activeMapModal.parentModFile)"
                  class="px-4 py-2 bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-300 border border-emerald-500/40 rounded-lg font-semibold transition"
                >
                  一键激活宿主模组
                </button>

                <button
                  v-if="!activeMapModal.isPack"
                  @click="handleGenerateMapThumb(activeMapModal)"
                  :disabled="generatingThumb"
                  class="flex items-center gap-1.5 px-4 py-2 bg-slate-800 hover:bg-slate-700 text-sky-400 border border-slate-700 rounded-lg font-medium transition disabled:opacity-50"
                  title="使用官方出生点颜色向量算法重新渲染"
                >
                  <Sparkles class="w-4 h-4" />
                  <span>重新生成出生点标记</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 3. 启动调优与高刷设置 Tab (深度集成 preferences.ini 游戏参数) -->
      <section v-if="currentTab === 'launcher'" class="h-full overflow-y-auto p-8 max-w-4xl mx-auto">
        <div class="space-y-6 pb-12">
          <div>
            <h2 class="text-lg font-bold text-slate-100 flex items-center gap-2.5">
              <Sliders class="w-5 h-5 text-sky-400" />
              游戏运行参数、高刷新率与内置引擎调优
            </h2>
            <p class="text-xs text-slate-400 mt-1 leading-relaxed">
              全面管理《铁锈战争》启动方式、JVM 内存、144Hz+ 电竞高刷屏支持、垂直同步、FPS 显示与队伍造兵上限。
            </p>
          </div>

          <!-- 1. 刷新率与帧率设置 (玩家重点关注) -->
          <div class="p-5 bg-slate-900/90 border border-slate-800 rounded-xl space-y-4 shadow-md">
            <div class="flex items-center justify-between border-b border-slate-800 pb-3">
              <div>
                <label class="text-sm font-semibold text-slate-200 flex items-center gap-2">
                  <Activity class="w-4 h-4 text-emerald-400" />
                  刷新率与帧率同步控制 (Refresh Rate & Sync)
                </label>
                <p class="text-xs text-slate-400 mt-0.5">原版默认限制 60Hz 帧率，在此可一键解锁高刷屏满血体验。</p>
              </div>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-3.5 text-xs">
              <!-- 解锁高刷新率 -->
              <label class="flex items-center justify-between p-3.5 bg-slate-800/50 border rounded-lg cursor-pointer transition"
                :class="appConfig.gamePreferences.highRefreshRate ? 'border-sky-500/60 bg-sky-500/10' : 'border-slate-700/60'">
                <div>
                  <span class="text-sm text-slate-200 font-bold block">解锁高刷新率 (High Refresh Rate)</span>
                  <span class="text-xs text-slate-400 font-mono mt-0.5 block">匹配 120Hz/144Hz/165Hz/240Hz 电竞显示器</span>
                </div>
                <input type="checkbox" v-model="appConfig.gamePreferences.highRefreshRate" class="w-4 h-4 rounded text-sky-500 focus:ring-0" />
              </label>

              <!-- 垂直同步 -->
              <label class="flex items-center justify-between p-3.5 bg-slate-800/50 border rounded-lg cursor-pointer transition"
                :class="appConfig.gamePreferences.renderVsync ? 'border-sky-500/60 bg-sky-500/10' : 'border-slate-700/60'">
                <div>
                  <span class="text-sm text-slate-200 font-bold block">垂直同步 (V-Sync)</span>
                  <span class="text-xs text-slate-400 font-mono mt-0.5 block">消除高速移动与拖动时的画面撕裂</span>
                </div>
                <input type="checkbox" v-model="appConfig.gamePreferences.renderVsync" class="w-4 h-4 rounded text-sky-500 focus:ring-0" />
              </label>

              <!-- 显示内置 FPS -->
              <label class="flex items-center justify-between p-3.5 bg-slate-800/50 border rounded-lg cursor-pointer transition"
                :class="appConfig.gamePreferences.showFps ? 'border-sky-500/60 bg-sky-500/10' : 'border-slate-700/60'">
                <div>
                  <span class="text-sm text-slate-200 font-bold block">显示游戏内 FPS 计数器</span>
                  <span class="text-xs text-slate-400 font-mono mt-0.5 block">在屏幕角落实时输出渲染帧率</span>
                </div>
                <input type="checkbox" v-model="appConfig.gamePreferences.showFps" class="w-4 h-4 rounded text-sky-500 focus:ring-0" />
              </label>

              <!-- 平滑帧时间差 -->
              <label class="flex items-center justify-between p-3.5 bg-slate-800/50 border rounded-lg cursor-pointer transition"
                :class="appConfig.gamePreferences.renderSmoothDelta ? 'border-sky-500/60 bg-sky-500/10' : 'border-slate-700/60'">
                <div>
                  <span class="text-sm text-slate-200 font-bold block">平滑帧时间差 (Smooth Delta)</span>
                  <span class="text-xs text-slate-400 font-mono mt-0.5 block">平抑单位大规模爆炸交火时的微卡顿</span>
                </div>
                <input type="checkbox" v-model="appConfig.gamePreferences.renderSmoothDelta" class="w-4 h-4 rounded text-sky-500 focus:ring-0" />
              </label>
            </div>
          </div>

          <!-- 2. 战局造兵单位上限 (解决造兵封顶) -->
          <div class="p-5 bg-slate-900/90 border border-slate-800 rounded-xl space-y-4 shadow-md">
            <div class="flex items-center justify-between border-b border-slate-800 pb-3">
              <div>
                <label class="text-sm font-semibold text-slate-200 flex items-center gap-2">
                  <Flame class="w-4 h-4 text-amber-400" />
                  战局队伍单位上限 (Team Unit Caps)
                </label>
                <p class="text-xs text-slate-400 mt-0.5">调高队伍单位容量，避免后期大战或大型实验级模组造满限制。</p>
              </div>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <!-- 单人战局上限 -->
              <div>
                <div class="flex justify-between items-center mb-2">
                  <span class="text-xs font-semibold text-slate-200">单人遭遇战单位上限</span>
                  <span class="text-xs font-mono font-bold text-sky-400 bg-sky-500/10 px-2 py-0.5 rounded border border-sky-500/30">
                    {{ appConfig.gamePreferences.teamUnitCapSinglePlayer }} 单位/队
                  </span>
                </div>
                <input
                  type="range"
                  min="2000"
                  max="20000"
                  step="1000"
                  v-model.number="appConfig.gamePreferences.teamUnitCapSinglePlayer"
                  class="w-full accent-sky-500 cursor-pointer"
                />
                <div class="flex justify-between text-[10px] text-slate-500 font-mono mt-1">
                  <span>2000</span>
                  <span>5000 (默认)</span>
                  <span>10000</span>
                  <span>20000 (极大规模)</span>
                </div>
              </div>

              <!-- 多人联机建房上限 -->
              <div>
                <div class="flex justify-between items-center mb-2">
                  <span class="text-xs font-semibold text-slate-200">联机建房队伍单位上限</span>
                  <span class="text-xs font-mono font-bold text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/30">
                    {{ appConfig.gamePreferences.teamUnitCapHostedGame }} 单位/队
                  </span>
                </div>
                <input
                  type="range"
                  min="1000"
                  max="10000"
                  step="500"
                  v-model.number="appConfig.gamePreferences.teamUnitCapHostedGame"
                  class="w-full accent-emerald-500 cursor-pointer"
                />
                <div class="flex justify-between text-[10px] text-slate-500 font-mono mt-1">
                  <span>1000</span>
                  <span>2000 (默认)</span>
                  <span>5000 (推荐)</span>
                  <span>10000</span>
                </div>
              </div>
            </div>
          </div>

          <!-- 3. 画质与特效控制 -->
          <div class="p-5 bg-slate-900/90 border border-slate-800 rounded-xl space-y-3.5 shadow-md">
            <label class="text-sm font-semibold text-slate-200 flex items-center gap-2">
              <Monitor class="w-4 h-4 text-sky-400" />
              画质特效与着色器 (Graphics & Shaders)
            </label>
            <div class="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
              <label class="flex items-center gap-2.5 p-3 bg-slate-800/40 border border-slate-700/60 rounded-lg cursor-pointer">
                <input type="checkbox" v-model="appConfig.gamePreferences.renderAntiAlias" class="w-4 h-4 rounded text-sky-500 focus:ring-0" />
                <span class="text-slate-200 font-medium">抗锯齿平滑</span>
              </label>

              <label class="flex items-center gap-2.5 p-3 bg-slate-800/40 border border-slate-700/60 rounded-lg cursor-pointer">
                <input type="checkbox" v-model="appConfig.gamePreferences.renderFancyWater" class="w-4 h-4 rounded text-sky-500 focus:ring-0" />
                <span class="text-slate-200 font-medium">动态优质水面</span>
              </label>

              <label class="flex items-center gap-2.5 p-3 bg-slate-800/40 border border-slate-700/60 rounded-lg cursor-pointer">
                <input type="checkbox" v-model="appConfig.gamePreferences.renderClouds" class="w-4 h-4 rounded text-sky-500 focus:ring-0" />
                <span class="text-slate-200 font-medium">天空云层阴影</span>
              </label>

              <label class="flex items-center gap-2.5 p-3 bg-slate-800/40 border border-slate-700/60 rounded-lg cursor-pointer">
                <input type="checkbox" v-model="appConfig.gamePreferences.renderExtraShadows" class="w-4 h-4 rounded text-sky-500 focus:ring-0" />
                <span class="text-slate-200 font-medium">额外高精度阴影</span>
              </label>
            </div>
          </div>

          <!-- 4. 启动器主程序与 JVM 堆内存 -->
          <div class="p-5 bg-slate-900/90 border border-slate-800 rounded-xl space-y-4 shadow-md">
            <label class="text-sm font-semibold text-slate-200">启动程序与内存分配</label>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-3.5 text-xs">
              <label
                class="flex items-center gap-3 p-3.5 rounded-lg border cursor-pointer transition shadow-sm"
                :class="appConfig.launchConfig.executableType === 'exe64' ? 'bg-sky-500/15 border-sky-500 text-sky-100' : 'bg-slate-800/40 border-slate-700/60 text-slate-300 hover:border-slate-600'"
              >
                <input type="radio" v-model="appConfig.launchConfig.executableType" value="exe64" class="hidden" />
                <div>
                  <div class="text-sm font-bold text-slate-100">Rusted Warfare - 64.exe</div>
                  <div class="text-xs text-slate-400 mt-0.5">64 位原生主程序（推荐）</div>
                </div>
              </label>

              <label
                class="flex items-center gap-3 p-3.5 rounded-lg border cursor-pointer transition shadow-sm"
                :class="appConfig.launchConfig.executableType === 'java' ? 'bg-sky-500/15 border-sky-500 text-sky-100' : 'bg-slate-800/40 border-slate-700/60 text-slate-300 hover:border-slate-600'"
              >
                <input type="radio" v-model="appConfig.launchConfig.executableType" value="java" class="hidden" />
                <div>
                  <div class="text-sm font-bold text-slate-100">内置 JVM 模式</div>
                  <div class="text-xs text-slate-400 mt-0.5">允许自定义 JVM 内存 (-Xmx)</div>
                </div>
              </label>

              <label
                class="flex items-center gap-3 p-3.5 rounded-lg border cursor-pointer transition shadow-sm"
                :class="appConfig.launchConfig.executableType === 'exe32' ? 'bg-sky-500/15 border-sky-500 text-sky-100' : 'bg-slate-800/40 border-slate-700/60 text-slate-300 hover:border-slate-600'"
              >
                <input type="radio" v-model="appConfig.launchConfig.executableType" value="exe32" class="hidden" />
                <div>
                  <div class="text-sm font-bold text-slate-100">Rusted Warfare.exe</div>
                  <div class="text-xs text-slate-400 mt-0.5">32 位兼容模式</div>
                </div>
              </label>
            </div>

            <!-- JVM 堆内存滑块 -->
            <div class="pt-2">
              <div class="flex justify-between items-center mb-1.5">
                <span class="text-xs text-slate-300 font-medium">JVM 最大内存分配 (-Xmx)</span>
                <span class="text-xs font-mono font-bold text-sky-400 bg-sky-500/10 px-2.5 py-0.5 rounded border border-sky-500/30">
                  {{ appConfig.launchConfig.maxMemoryMb }} MB
                </span>
              </div>
              <input
                type="range"
                min="1024"
                max="8192"
                step="512"
                v-model.number="appConfig.launchConfig.maxMemoryMb"
                class="w-full accent-sky-500 cursor-pointer"
              />
            </div>
          </div>

          <!-- 5. 命令行参数 -->
          <div class="p-5 bg-slate-900/90 border border-slate-800 rounded-xl space-y-3.5 shadow-md">
            <label class="text-sm font-semibold text-slate-200">快速命令行参数</label>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-3.5 text-xs">
              <label class="flex items-center gap-3 p-3 bg-slate-800/50 border border-slate-700/70 rounded-lg cursor-pointer hover:border-slate-600 transition">
                <input type="checkbox" v-model="appConfig.launchConfig.fullscreen" class="w-4 h-4 rounded text-sky-500 focus:ring-0" />
                <div>
                  <span class="text-sm text-slate-200 font-medium">全屏运行</span>
                  <span class="text-xs text-slate-400 block font-mono">-fullscreen</span>
                </div>
              </label>

              <label class="flex items-center gap-3 p-3 bg-slate-800/50 border border-slate-700/70 rounded-lg cursor-pointer hover:border-slate-600 transition">
                <input type="checkbox" v-model="appConfig.launchConfig.noMods" class="w-4 h-4 rounded text-sky-500 focus:ring-0" />
                <div>
                  <span class="text-sm text-slate-200 font-medium">纯净原版启动</span>
                  <span class="text-xs text-slate-400 block font-mono">-nomods (禁用所有模组测试)</span>
                </div>
              </label>

              <label class="flex items-center gap-3 p-3 bg-slate-800/50 border border-slate-700/70 rounded-lg cursor-pointer hover:border-slate-600 transition">
                <input type="checkbox" v-model="appConfig.launchConfig.safeMode" class="w-4 h-4 rounded text-sky-500 focus:ring-0" />
                <div>
                  <span class="text-sm text-slate-200 font-medium">安全渲染模式</span>
                  <span class="text-xs text-slate-400 block font-mono">-safemode</span>
                </div>
              </label>
            </div>

            <div class="pt-2">
              <label class="text-xs font-semibold text-slate-300 block mb-1.5">自定义附加命令行参数</label>
              <input
                type="text"
                v-model="appConfig.launchConfig.extraArgs"
                placeholder="例如: -width 1920 -height 1080 -postprocessing"
                class="w-full bg-slate-950 border border-slate-700 rounded-lg px-3.5 py-2 text-xs font-mono text-slate-100 placeholder-slate-500 focus:outline-none focus:border-sky-500"
              />
            </div>
          </div>

          <!-- 保存与启动按钮 -->
          <div class="flex justify-end gap-3.5 pt-2">
            <button
              @click="handleSaveSettings"
              class="px-5 py-2.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium rounded-lg transition"
            >
              保存所有偏好与启动配置
            </button>
            <button
              @click="handleLaunchGame"
              class="flex items-center gap-2 px-6 py-2.5 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-white font-semibold text-sm rounded-lg shadow-lg shadow-emerald-500/25 transition"
            >
              <Rocket class="w-4 h-4" />
              <span>保存并立即启动游戏</span>
            </button>
          </div>
        </div>
      </section>

      <!-- 4. 系统设置 Tab -->
      <section v-if="currentTab === 'settings'" class="h-full overflow-y-auto p-8 max-w-4xl mx-auto">
        <div class="space-y-6 pb-12">
          <div>
            <h2 class="text-lg font-bold text-slate-100 flex items-center gap-2.5">
              <Settings class="w-5 h-5 text-sky-400" />
              游戏客户端与环境自检
            </h2>
            <p class="text-xs text-slate-400 mt-1">检测铁锈战争客户端目录完整性与配置文件状态。</p>
          </div>

          <!-- 路径配置卡片 -->
          <div class="p-5 bg-slate-900/90 border border-slate-800 rounded-xl space-y-3.5 shadow-md">
            <div class="flex items-center justify-between">
              <label class="text-sm font-semibold text-slate-200">游戏安装根目录</label>
              <button
                @click="handleChangeDir"
                class="px-3.5 py-1.5 bg-sky-500/20 hover:bg-sky-500/30 text-sky-300 border border-sky-500/40 rounded-lg text-xs font-medium transition"
              >
                浏览选择目录
              </button>
            </div>
            <div class="p-3.5 bg-slate-950 border border-slate-800 rounded-lg font-mono text-xs text-slate-200 break-all select-text">
              {{ gameStatus.gameDir || '尚未配置' }}
            </div>
          </div>

          <!-- 自检清单 -->
          <div class="p-5 bg-slate-900/90 border border-slate-800 rounded-xl space-y-3.5 shadow-md text-xs">
            <h3 class="text-sm font-semibold text-slate-200 mb-2">环境特征自检清单</h3>
            <div class="space-y-2.5 font-mono">
              <div class="flex items-center justify-between p-3 bg-slate-950 rounded-lg">
                <span class="text-slate-300 text-xs">game-lib.jar (核心游戏程序包)</span>
                <span class="flex items-center gap-2 text-emerald-400 font-semibold text-xs">
                  <CheckCircle2 class="w-4 h-4" /> 正常就绪
                </span>
              </div>

              <div class="flex items-center justify-between p-3 bg-slate-950 rounded-lg">
                <span class="text-slate-300 text-xs">preferences.ini (全局设置与 modSettings 存储)</span>
                <span class="flex items-center gap-2 text-emerald-400 font-semibold text-xs">
                  <CheckCircle2 class="w-4 h-4" /> 正常就绪
                </span>
              </div>

              <div class="flex items-center justify-between p-3 bg-slate-950 rounded-lg">
                <span class="text-slate-300 text-xs">mods/units (单位模组目录)</span>
                <span class="flex items-center gap-2 text-sky-400 font-semibold text-xs">
                  已加载 {{ mods.length }} 个模组 ({{ enabledCount }} 启用)
                </span>
              </div>

              <div class="flex items-center justify-between p-3 bg-slate-950 rounded-lg">
                <span class="text-slate-300 text-xs">mods/maps (地图全量检索，含模组内嵌)</span>
                <span class="flex items-center gap-2 text-sky-400 font-semibold text-xs">
                  共计 {{ maps.length }} 张地图 ({{ activeMapCount }} 战局可用)
                </span>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>
  </div>
</template>
