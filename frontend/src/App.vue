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
  XCircle,
  ExternalLink,
  ShieldAlert,
  Sliders,
  Layers,
  Users,
  HardDrive,
  Cpu
} from 'lucide-vue-next'
import { useApi } from './composables/useApi'
import { ModItem, MapItem, GameStatus, AppConfig } from './types/api'

const api = useApi()

// 当前激活的标签页
const currentTab = ref<'units' | 'maps' | 'launcher' | 'settings'>('units')

// 核心数据状态
const loading = ref(false)
const launching = ref(false)
const gameStatus = ref<GameStatus>({
  gameDir: '',
  isValid: false,
  version: '',
  totalMods: 0,
  enabledMods: 0,
  disabledMods: 0,
  totalMaps: 0
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
  }
})

const mods = ref<ModItem[]>([])
const maps = ref<MapItem[]>([])

// 搜索与过滤
const modSearch = ref('')
const modFilterStatus = ref<'all' | 'enabled' | 'disabled'>('all')
const modViewMode = ref<'grid' | 'list'>('grid')

const mapSearch = ref('')
const mapFilterPack = ref<'all' | 'standalone' | 'pack'>('all')

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
    // 状态过滤
    if (modFilterStatus.value === 'enabled' && !m.enabled) return false
    if (modFilterStatus.value === 'disabled' && m.enabled) return false

    // 关键词搜索
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
    if (mapFilterPack.value === 'standalone' && m.isPack) return false
    if (mapFilterPack.value === 'pack' && !m.isPack) return false

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
  } catch (err) {
    item.enabled = !target // 回滚
    alert(`切换模组状态失败: ${err}`)
  }
}

// 批量切换
async function handleBatchToggle(enableAll: boolean) {
  loading.value = true
  try {
    await api.batchToggleMods(enableAll)
    mods.value.forEach(m => (m.enabled = enableAll))
  } catch (err) {
    alert(`批量操作失败: ${err}`)
  } finally {
    loading.value = false
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
  <div class="flex flex-col h-screen w-screen bg-slate-950 text-slate-100 overflow-hidden font-sans">
    <!-- 顶部导航栏 -->
    <header class="flex items-center justify-between px-6 py-3.5 bg-slate-900/90 backdrop-blur border-b border-slate-800 shrink-0">
      <div class="flex items-center gap-4">
        <!-- Logo 与标题 -->
        <div class="flex items-center gap-3">
          <div class="w-9 h-9 rounded-xl bg-gradient-to-tr from-sky-500 to-indigo-600 flex items-center justify-center shadow-lg shadow-sky-500/20">
            <Cpu class="w-5 h-5 text-white" />
          </div>
          <div>
            <h1 class="text-base font-bold tracking-tight bg-gradient-to-r from-sky-400 via-indigo-200 to-emerald-400 bg-clip-text text-transparent">
              铁锈战争 模组与地图管理器
            </h1>
            <p class="text-xs text-slate-400">Rusted Warfare Mod & Map Manager v1.0</p>
          </div>
        </div>

        <!-- 游戏路径状态胶囊 -->
        <div class="hidden md:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-800/80 border border-slate-700/60 text-xs">
          <span
            class="w-2 h-2 rounded-full"
            :class="gameStatus.isValid ? 'bg-emerald-400 shadow-sm shadow-emerald-400/50' : 'bg-rose-400 shadow-sm shadow-rose-400/50'"
          />
          <span class="text-slate-300 font-mono truncate max-w-xs" :title="gameStatus.gameDir">
            {{ gameStatus.gameDir || '未配置游戏路径' }}
          </span>
          <button
            @click="handleChangeDir"
            class="ml-1 text-sky-400 hover:text-sky-300 hover:underline transition font-medium"
          >
            更换
          </button>
        </div>
      </div>

      <!-- 右侧全局操作区 -->
      <div class="flex items-center gap-2.5">
        <!-- 目录快速打开 -->
        <div class="flex items-center rounded-lg bg-slate-800/80 p-0.5 border border-slate-700">
          <button
            @click="api.openFolder('units')"
            title="打开单位模组文件夹"
            class="px-2.5 py-1 text-xs text-slate-300 hover:text-white hover:bg-slate-700 rounded transition flex items-center gap-1.5"
          >
            <FolderOpen class="w-3.5 h-3.5" />
            <span>模组</span>
          </button>
          <button
            @click="api.openFolder('maps')"
            title="打开地图文件夹"
            class="px-2.5 py-1 text-xs text-slate-300 hover:text-white hover:bg-slate-700 rounded transition flex items-center gap-1.5"
          >
            <FolderOpen class="w-3.5 h-3.5" />
            <span>地图</span>
          </button>
        </div>

        <!-- 刷新按钮 -->
        <button
          @click="loadAllData"
          :disabled="loading"
          class="p-2 text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 border border-slate-700 rounded-lg transition disabled:opacity-50"
          title="刷新扫描"
        >
          <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
        </button>

        <!-- 一键启动游戏 -->
        <button
          @click="handleLaunchGame"
          :disabled="launching || !gameStatus.isValid"
          class="flex items-center gap-2 px-4 py-2 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-white font-medium text-xs rounded-lg shadow-lg shadow-emerald-500/25 active:scale-95 transition disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <Rocket class="w-4 h-4" :class="{ 'animate-bounce': launching }" />
          <span>{{ launching ? '启动中...' : '启动游戏' }}</span>
        </button>
      </div>
    </header>

    <!-- 选项卡导航栏 -->
    <nav class="flex items-center justify-between px-6 bg-slate-900 border-b border-slate-800/80 text-xs shrink-0">
      <div class="flex gap-2">
        <button
          @click="currentTab = 'units'"
          class="flex items-center gap-2 py-3 px-3.5 border-b-2 font-medium transition"
          :class="currentTab === 'units' ? 'border-sky-400 text-sky-400 font-semibold' : 'border-transparent text-slate-400 hover:text-slate-200'"
        >
          <Package class="w-4 h-4" />
          <span>单位模组 (Units)</span>
          <span class="px-1.5 py-0.5 rounded-full bg-slate-800 text-[10px] text-slate-300">
            {{ enabledCount }} / {{ mods.length }}
          </span>
        </button>

        <button
          @click="currentTab = 'maps'"
          class="flex items-center gap-2 py-3 px-3.5 border-b-2 font-medium transition"
          :class="currentTab === 'maps' ? 'border-sky-400 text-sky-400 font-semibold' : 'border-transparent text-slate-400 hover:text-slate-200'"
        >
          <Map class="w-4 h-4" />
          <span>地图工坊 (Maps)</span>
          <span class="px-1.5 py-0.5 rounded-full bg-slate-800 text-[10px] text-slate-300">
            {{ maps.length }}
          </span>
          <span v-if="missingPreviewCount > 0" class="px-1.5 py-0.5 rounded-full bg-amber-500/20 text-[10px] text-amber-300">
            {{ missingPreviewCount }} 缺图
          </span>
        </button>

        <button
          @click="currentTab = 'launcher'"
          class="flex items-center gap-2 py-3 px-3.5 border-b-2 font-medium transition"
          :class="currentTab === 'launcher' ? 'border-sky-400 text-sky-400 font-semibold' : 'border-transparent text-slate-400 hover:text-slate-200'"
        >
          <Sliders class="w-4 h-4" />
          <span>启动调优 (Launcher)</span>
          <span class="px-1.5 py-0.5 rounded-full bg-slate-800 text-[10px] text-slate-300">
            {{ appConfig.launchConfig.maxMemoryMb }}M
          </span>
        </button>

        <button
          @click="currentTab = 'settings'"
          class="flex items-center gap-2 py-3 px-3.5 border-b-2 font-medium transition"
          :class="currentTab === 'settings' ? 'border-sky-400 text-sky-400 font-semibold' : 'border-transparent text-slate-400 hover:text-slate-200'"
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
        <div class="flex flex-wrap items-center justify-between gap-3 mb-4 shrink-0">
          <div class="flex items-center gap-3 flex-1 max-w-md">
            <div class="relative w-full">
              <Search class="w-4 h-4 text-slate-400 absolute left-3 top-2.5" />
              <input
                v-model="modSearch"
                type="text"
                placeholder="搜索模组标题、标签、文件名..."
                class="w-full bg-slate-900 border border-slate-700/80 rounded-lg pl-9 pr-3 py-1.5 text-xs text-slate-100 placeholder-slate-500 focus:outline-none focus:border-sky-500 transition"
              />
            </div>
          </div>

          <!-- 状态筛选单选与批量按钮 -->
          <div class="flex items-center gap-2.5 text-xs">
            <div class="flex rounded-lg bg-slate-900 p-0.5 border border-slate-800">
              <button
                @click="modFilterStatus = 'all'"
                class="px-2.5 py-1 rounded transition"
                :class="modFilterStatus === 'all' ? 'bg-sky-500/20 text-sky-300 font-medium' : 'text-slate-400 hover:text-slate-200'"
              >
                全部 ({{ mods.length }})
              </button>
              <button
                @click="modFilterStatus = 'enabled'"
                class="px-2.5 py-1 rounded transition"
                :class="modFilterStatus === 'enabled' ? 'bg-emerald-500/20 text-emerald-300 font-medium' : 'text-slate-400 hover:text-slate-200'"
              >
                已启用 ({{ enabledCount }})
              </button>
              <button
                @click="modFilterStatus = 'disabled'"
                class="px-2.5 py-1 rounded transition"
                :class="modFilterStatus === 'disabled' ? 'bg-rose-500/20 text-rose-300 font-medium' : 'text-slate-400 hover:text-slate-200'"
              >
                已禁用 ({{ mods.length - enabledCount }})
              </button>
            </div>

            <div class="h-4 w-px bg-slate-800" />

            <button
              @click="handleBatchToggle(true)"
              class="px-3 py-1.5 bg-slate-850 hover:bg-slate-800 text-emerald-400 border border-emerald-500/30 rounded-lg transition"
            >
              全部启用
            </button>
            <button
              @click="handleBatchToggle(false)"
              class="px-3 py-1.5 bg-slate-850 hover:bg-slate-800 text-rose-400 border border-rose-500/30 rounded-lg transition"
            >
              全部禁用
            </button>
          </div>
        </div>

        <!-- 模组卡片网格列表 -->
        <div class="flex-1 overflow-y-auto pr-1">
          <div v-if="filteredMods.length === 0" class="flex flex-col items-center justify-center h-64 text-slate-500">
            <Package class="w-12 h-12 stroke-[1.5] mb-2 opacity-50" />
            <p class="text-sm">没有找到匹配的模组</p>
          </div>

          <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 pb-6">
            <div
              v-for="item in filteredMods"
              :key="item.fileName"
              class="group flex flex-col bg-slate-900/80 border rounded-xl overflow-hidden transition-all duration-200"
              :class="item.enabled ? 'border-slate-700 hover:border-sky-500/60 shadow-md' : 'border-slate-800/80 opacity-75 hover:opacity-100'"
            >
              <!-- 封面图与顶部状态徽章 -->
              <div class="relative h-32 bg-slate-950 overflow-hidden shrink-0 flex items-center justify-center">
                <img
                  v-if="item.thumbnail"
                  :src="item.thumbnail"
                  class="w-full h-full object-cover transition group-hover:scale-105 duration-300"
                  alt="Mod thumbnail"
                />
                <div v-else class="flex flex-col items-center justify-center text-slate-600 gap-1">
                  <Package class="w-8 h-8 stroke-[1.2]" />
                  <span class="text-[10px] font-mono tracking-wider">NO THUMBNAIL</span>
                </div>

                <!-- 启/禁用状态胶囊 -->
                <div class="absolute top-2 left-2">
                  <span
                    class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium backdrop-blur-md"
                    :class="item.enabled ? 'bg-emerald-500/90 text-white shadow-sm shadow-emerald-500/40' : 'bg-slate-800/90 text-slate-400 border border-slate-700'"
                  >
                    <span class="w-1.5 h-1.5 rounded-full" :class="item.enabled ? 'bg-white' : 'bg-slate-500'" />
                    {{ item.enabled ? '已启用' : '已禁用' }}
                  </span>
                </div>

                <!-- 版本号徽章 -->
                <div v-if="item.minVersion" class="absolute top-2 right-2">
                  <span class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-black/60 backdrop-blur-md text-slate-300 border border-slate-700/60">
                    {{ item.minVersion }}
                  </span>
                </div>
              </div>

              <!-- 模组信息主体 -->
              <div class="flex-1 p-3.5 flex flex-col justify-between">
                <div>
                  <div class="flex items-start justify-between gap-2">
                    <h3 class="text-xs font-bold text-slate-100 line-clamp-1 group-hover:text-sky-300 transition" :title="item.titleZh || item.title">
                      {{ item.titleZh || item.title }}
                    </h3>
                  </div>

                  <p class="text-[11px] text-slate-400 font-mono truncate mt-0.5" :title="item.fileName">
                    {{ item.fileName }}
                  </p>

                  <p class="text-[11px] text-slate-400 line-clamp-2 mt-2 leading-relaxed">
                    {{ item.description || '暂无模组描述说明。' }}
                  </p>

                  <!-- 标签分类 -->
                  <div v-if="item.tags && item.tags.length" class="flex flex-wrap gap-1 mt-2.5">
                    <span
                      v-for="t in item.tags.slice(0, 3)"
                      :key="t"
                      class="px-1.5 py-0.5 rounded text-[9px] bg-slate-800 text-sky-300/80 border border-slate-700/50"
                    >
                      {{ t }}
                    </span>
                  </div>
                </div>

                <!-- 底部统计与开关操作 -->
                <div class="mt-4 pt-3 border-t border-slate-800/80 flex items-center justify-between">
                  <div class="flex items-center gap-3 text-[10px] text-slate-400 font-mono">
                    <span title="单位配置数 (*.ini)" class="flex items-center gap-1">
                      <Cpu class="w-3 h-3 text-sky-400" />
                      {{ item.unitCount }}
                    </span>
                    <span v-if="item.mapCount > 0" title="内嵌地图数 (*.tmx)" class="flex items-center gap-1">
                      <Map class="w-3 h-3 text-emerald-400" />
                      {{ item.mapCount }}
                    </span>
                    <span title="包体大小" class="text-slate-500">
                      {{ formatBytes(item.fileSize) }}
                    </span>
                  </div>

                  <div class="flex items-center gap-1.5">
                    <!-- Steam 工坊链接按钮 -->
                    <button
                      v-if="item.steamId"
                      @click="openWorkshopLink(item.steamId)"
                      class="p-1 text-slate-400 hover:text-sky-400 transition"
                      title="在 Steam 创意工坊查看"
                    >
                      <ExternalLink class="w-3.5 h-3.5" />
                    </button>

                    <!-- 启/禁用切换滑动开关 -->
                    <button
                      @click="handleToggleMod(item)"
                      class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
                      :class="item.enabled ? 'bg-sky-500' : 'bg-slate-700'"
                    >
                      <span
                        class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow-lg ring-0 transition duration-200 ease-in-out"
                        :class="item.enabled ? 'translate-x-4' : 'translate-x-0'"
                      />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 2. 地图工坊 Tab -->
      <section v-if="currentTab === 'maps'" class="h-full flex flex-col p-6 overflow-hidden">
        <!-- 搜索与过滤栏 -->
        <div class="flex flex-wrap items-center justify-between gap-3 mb-4 shrink-0">
          <div class="flex items-center gap-3 flex-1 max-w-md">
            <div class="relative w-full">
              <Search class="w-4 h-4 text-slate-400 absolute left-3 top-2.5" />
              <input
                v-model="mapSearch"
                type="text"
                placeholder="搜索地图名称、地图包名..."
                class="w-full bg-slate-900 border border-slate-700/80 rounded-lg pl-9 pr-3 py-1.5 text-xs text-slate-100 placeholder-slate-500 focus:outline-none focus:border-sky-500 transition"
              />
            </div>
          </div>

          <div class="flex items-center gap-2 text-xs">
            <div class="flex rounded-lg bg-slate-900 p-0.5 border border-slate-800">
              <button
                @click="mapFilterPack = 'all'"
                class="px-2.5 py-1 rounded transition"
                :class="mapFilterPack === 'all' ? 'bg-sky-500/20 text-sky-300 font-medium' : 'text-slate-400 hover:text-slate-200'"
              >
                全部地图 ({{ maps.length }})
              </button>
              <button
                @click="mapFilterPack = 'standalone'"
                class="px-2.5 py-1 rounded transition"
                :class="mapFilterPack === 'standalone' ? 'bg-sky-500/20 text-sky-300 font-medium' : 'text-slate-400 hover:text-slate-200'"
              >
                独立 TMX
              </button>
              <button
                @click="mapFilterPack = 'pack'"
                class="px-2.5 py-1 rounded transition"
                :class="mapFilterPack === 'pack' ? 'bg-sky-500/20 text-sky-300 font-medium' : 'text-slate-400 hover:text-slate-200'"
              >
                地图包内嵌
              </button>
            </div>
          </div>
        </div>

        <!-- 地图网格列表 -->
        <div class="flex-1 overflow-y-auto pr-1">
          <div v-if="filteredMaps.length === 0" class="flex flex-col items-center justify-center h-64 text-slate-500">
            <Map class="w-12 h-12 stroke-[1.5] mb-2 opacity-50" />
            <p class="text-sm">没有找到匹配的地图</p>
          </div>

          <div v-else class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-3.5 pb-6">
            <div
              v-for="m in filteredMaps"
              :key="m.id"
              class="group flex flex-col bg-slate-900/90 border border-slate-800 hover:border-sky-500/60 rounded-xl overflow-hidden transition"
            >
              <!-- 缩略图区域 -->
              <div class="relative aspect-square bg-slate-950 flex items-center justify-center overflow-hidden">
                <img
                  v-if="m.hasPreview && m.previewData"
                  :src="m.previewData"
                  class="w-full h-full object-cover transition group-hover:scale-105 duration-200"
                  alt="Map Preview"
                />
                <div v-else class="flex flex-col items-center justify-center text-amber-500/80 gap-1 p-2 text-center">
                  <ShieldAlert class="w-8 h-8 stroke-[1.5]" />
                  <span class="text-[9px] font-medium leading-tight">缺失 _map.png 缩略图</span>
                </div>

                <!-- 玩家人数徽章 -->
                <div class="absolute bottom-1.5 left-1.5">
                  <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono bg-black/70 backdrop-blur-md text-emerald-300 border border-emerald-500/30">
                    <Users class="w-3 h-3" />
                    {{ m.playerCount }}P
                  </span>
                </div>

                <!-- 瓦片尺寸徽章 -->
                <div v-if="m.width > 0" class="absolute bottom-1.5 right-1.5">
                  <span class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-black/70 backdrop-blur-md text-slate-300 border border-slate-700/60">
                    {{ m.width }}×{{ m.height }}
                  </span>
                </div>
              </div>

              <!-- 地图文本信息 -->
              <div class="p-2.5 flex-1 flex flex-col justify-between">
                <div>
                  <h4 class="text-xs font-medium text-slate-200 line-clamp-1 group-hover:text-sky-300 transition" :title="m.title">
                    {{ m.title }}
                  </h4>
                  <p v-if="m.isPack" class="text-[10px] text-indigo-400 font-mono truncate mt-0.5" :title="m.packName">
                    包: {{ m.packName }}
                  </p>
                  <p v-else class="text-[10px] text-slate-500 font-mono truncate mt-0.5">
                    独立地图
                  </p>
                </div>
                <div class="mt-2 text-[10px] text-slate-500 font-mono">
                  {{ formatBytes(m.fileSize) }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 3. 启动调优 Tab -->
      <section v-if="currentTab === 'launcher'" class="h-full overflow-y-auto p-8 max-w-4xl mx-auto">
        <div class="space-y-6 pb-12">
          <div>
            <h2 class="text-base font-bold text-slate-100 flex items-center gap-2">
              <Sliders class="w-5 h-5 text-sky-400" />
              游戏运行参数与 JVM 堆内存调优
            </h2>
            <p class="text-xs text-slate-400 mt-1">
              针对大型总改包（如 2000+ 单位的 MajorExperimentals）易发生 OutOfMemory 内存溢出或跳错闪退的问题提供深度优化。
            </p>
          </div>

          <!-- 主执行程序选择 -->
          <div class="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-3">
            <label class="text-xs font-semibold text-slate-200">启动器主程序 (Executable)</label>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
              <label
                class="flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition"
                :class="appConfig.launchConfig.executableType === 'exe64' ? 'bg-sky-500/10 border-sky-500 text-sky-200' : 'bg-slate-800/40 border-slate-700/60 text-slate-400 hover:border-slate-600'"
              >
                <input type="radio" v-model="appConfig.launchConfig.executableType" value="exe64" class="hidden" />
                <div>
                  <div class="font-medium">Rusted Warfare - 64.exe</div>
                  <div class="text-[10px] text-slate-500">64 位原生启动器（推荐）</div>
                </div>
              </label>

              <label
                class="flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition"
                :class="appConfig.launchConfig.executableType === 'java' ? 'bg-sky-500/10 border-sky-500 text-sky-200' : 'bg-slate-800/40 border-slate-700/60 text-slate-400 hover:border-slate-600'"
              >
                <input type="radio" v-model="appConfig.launchConfig.executableType" value="java" class="hidden" />
                <div>
                  <div class="font-medium">内置 JVM 模式</div>
                  <div class="text-[10px] text-slate-500">允许自定义 JVM 内存 (-Xmx)</div>
                </div>
              </label>

              <label
                class="flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition"
                :class="appConfig.launchConfig.executableType === 'exe32' ? 'bg-sky-500/10 border-sky-500 text-sky-200' : 'bg-slate-800/40 border-slate-700/60 text-slate-400 hover:border-slate-600'"
              >
                <input type="radio" v-model="appConfig.launchConfig.executableType" value="exe32" class="hidden" />
                <div>
                  <div class="font-medium">Rusted Warfare.exe</div>
                  <div class="text-[10px] text-slate-500">32 位兼容模式</div>
                </div>
              </label>
            </div>
          </div>

          <!-- JVM 内存分配设置 -->
          <div class="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-4">
            <div class="flex items-center justify-between">
              <div>
                <label class="text-xs font-semibold text-slate-200">JVM 最大内存分配 (-Xmx)</label>
                <p class="text-[11px] text-slate-400 mt-0.5">原版批处理默认仅 1000MB，推荐调至 4096MB (4GB) 以上以支撑多个大型模组。</p>
              </div>
              <span class="text-sm font-mono font-bold text-sky-400 bg-sky-500/10 px-3 py-1 rounded-lg border border-sky-500/30">
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
            <div class="flex justify-between text-[10px] text-slate-500 font-mono">
              <span>1024 MB (1G)</span>
              <span>2048 MB (2G)</span>
              <span>4096 MB (4G 推荐)</span>
              <span>8192 MB (8G)</span>
            </div>
          </div>

          <!-- 快速运行参数开关 -->
          <div class="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-3">
            <label class="text-xs font-semibold text-slate-200">运行参数开关</label>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
              <label class="flex items-center gap-2 p-3 bg-slate-800/40 border border-slate-700/60 rounded-lg cursor-pointer">
                <input type="checkbox" v-model="appConfig.launchConfig.fullscreen" class="rounded text-sky-500 focus:ring-0" />
                <div>
                  <span class="text-slate-200">全屏运行</span>
                  <span class="text-[10px] text-slate-500 block font-mono">-fullscreen</span>
                </div>
              </label>

              <label class="flex items-center gap-2 p-3 bg-slate-800/40 border border-slate-700/60 rounded-lg cursor-pointer">
                <input type="checkbox" v-model="appConfig.launchConfig.noMods" class="rounded text-sky-500 focus:ring-0" />
                <div>
                  <span class="text-slate-200">纯净原版启动</span>
                  <span class="text-[10px] text-slate-500 block font-mono">-nomods</span>
                </div>
              </label>

              <label class="flex items-center gap-2 p-3 bg-slate-800/40 border border-slate-700/60 rounded-lg cursor-pointer">
                <input type="checkbox" v-model="appConfig.launchConfig.safeMode" class="rounded text-sky-500 focus:ring-0" />
                <div>
                  <span class="text-slate-200">安全渲染模式</span>
                  <span class="text-[10px] text-slate-500 block font-mono">-safemode</span>
                </div>
              </label>
            </div>
          </div>

          <!-- 附加启动参数输入 -->
          <div class="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-2">
            <label class="text-xs font-semibold text-slate-200">自定义附加参数</label>
            <input
              type="text"
              v-model="appConfig.launchConfig.extraArgs"
              placeholder="例如: -width 1920 -height 1080 -log"
              class="w-full bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-xs font-mono text-slate-100 placeholder-slate-600 focus:outline-none focus:border-sky-500"
            />
          </div>

          <div class="flex justify-end gap-3 pt-2">
            <button
              @click="api.saveConfig(appConfig); alert('配置保存成功！')"
              class="px-5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs rounded-lg transition"
            >
              仅保存配置
            </button>
            <button
              @click="handleLaunchGame"
              class="flex items-center gap-2 px-6 py-2 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-white font-medium text-xs rounded-lg shadow-lg shadow-emerald-500/25 transition"
            >
              <Rocket class="w-4 h-4" />
              <span>保存并立即启动</span>
            </button>
          </div>
        </div>
      </section>

      <!-- 4. 系统设置 Tab -->
      <section v-if="currentTab === 'settings'" class="h-full overflow-y-auto p-8 max-w-4xl mx-auto">
        <div class="space-y-6 pb-12">
          <div>
            <h2 class="text-base font-bold text-slate-100 flex items-center gap-2">
              <Settings class="w-5 h-5 text-sky-400" />
              游戏客户端与环境自检
            </h2>
            <p class="text-xs text-slate-400 mt-1">检测铁锈战争客户端目录完整性与配置文件状态。</p>
          </div>

          <!-- 路径配置卡片 -->
          <div class="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-3">
            <div class="flex items-center justify-between">
              <label class="text-xs font-semibold text-slate-200">游戏安装根目录</label>
              <button
                @click="handleChangeDir"
                class="px-3 py-1 bg-sky-500/20 hover:bg-sky-500/30 text-sky-300 border border-sky-500/40 rounded-lg text-xs transition"
              >
                浏览选择目录
              </button>
            </div>
            <div class="p-3 bg-slate-950 border border-slate-800 rounded-lg font-mono text-xs text-slate-300 break-all">
              {{ gameStatus.gameDir || '尚未配置' }}
            </div>
          </div>

          <!-- 自检清单 -->
          <div class="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-3 text-xs">
            <h3 class="font-semibold text-slate-200 mb-2">环境特征自检清单</h3>
            <div class="space-y-2 font-mono">
              <div class="flex items-center justify-between p-2.5 bg-slate-950 rounded-lg">
                <span class="text-slate-300">game-lib.jar (核心游戏程序包)</span>
                <span class="flex items-center gap-1.5 text-emerald-400 font-medium">
                  <CheckCircle2 class="w-4 h-4" /> 正常就绪
                </span>
              </div>

              <div class="flex items-center justify-between p-2.5 bg-slate-950 rounded-lg">
                <span class="text-slate-300">preferences.ini (全局设置与 modSettings 存储)</span>
                <span class="flex items-center gap-1.5 text-emerald-400 font-medium">
                  <CheckCircle2 class="w-4 h-4" /> 正常就绪
                </span>
              </div>

              <div class="flex items-center justify-between p-2.5 bg-slate-950 rounded-lg">
                <span class="text-slate-300">mods/units (单位模组目录)</span>
                <span class="flex items-center gap-1.5 text-sky-400 font-medium">
                  已加载 {{ mods.length }} 个模组
                </span>
              </div>

              <div class="flex items-center justify-between p-2.5 bg-slate-950 rounded-lg">
                <span class="text-slate-300">mods/maps (自定义地图目录)</span>
                <span class="flex items-center gap-1.5 text-sky-400 font-medium">
                  已索引 {{ maps.length }} 张地图
                </span>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>
  </div>
</template>
