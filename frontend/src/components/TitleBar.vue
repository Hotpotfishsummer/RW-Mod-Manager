<script setup lang="ts">
import { computed } from 'vue'
import { Cpu } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    isMaximized?: boolean
    currentVersion?: string
    gameDir?: string
    isValidGame?: boolean
  }>(),
  {
    isMaximized: false,
    currentVersion: '1.2.0',
    gameDir: '',
    isValidGame: false
  }
)

const emit = defineEmits<{
  (e: 'minimize'): void
  (e: 'toggleMaximize'): void
  (e: 'close'): void
}>()
</script>

<template>
  <div
    class="wails-draggable flex items-center justify-between h-[34px] bg-m3-surface-low/95 backdrop-blur-md border-b border-m3-outline-variant/60 shrink-0 select-none overflow-hidden z-50 text-m3-on-surface transition-colors duration-200"
    @dblclick.self="emit('toggleMaximize')"
  >
    <!-- 左侧：应用图标、标题与版本标签 -->
    <div
      class="flex items-center gap-2 pl-3 pr-2 h-full shrink-0 min-w-0"
      @dblclick="emit('toggleMaximize')"
    >
      <!-- 微型图标 (带有主题微渐变与阴影) -->
      <div
        class="w-5 h-5 rounded-md bg-m3-primary/15 text-m3-primary flex items-center justify-center shrink-0 border border-m3-primary/20 shadow-xs"
        title="铁锈战争 模组与地图管理器 (Rusted Warfare Workshop & LAN Sync Manager)"
      >
        <Cpu class="w-3.5 h-3.5 text-m3-primary" />
      </div>

      <!-- 应用中文主标题 -->
      <span
        class="text-xs font-semibold text-m3-on-surface tracking-tight whitespace-nowrap truncate"
      >
        铁锈战争 模组与地图管理器
      </span>

      <!-- 英文副标题 (大屏展示，体现专业度) -->
      <span
        class="hidden lg:inline text-[10px] text-m3-on-surface-variant/70 font-mono tracking-wider whitespace-nowrap truncate"
      >
        RW Mod Manager
      </span>

      <!-- 版本号胶囊 -->
      <span
        v-if="currentVersion"
        class="text-[10px] font-mono px-1.5 py-0.5 rounded-full bg-m3-surface-container border border-m3-outline/60 text-m3-on-surface-variant leading-none shrink-0"
      >
        v{{ currentVersion }}
      </span>
    </div>

    <!-- 中间：宽敞的拖拽区域 (支持双击最大化/还原窗口) -->
    <div
      class="wails-draggable flex-1 h-full min-w-8 flex items-center justify-center cursor-default"
      @dblclick="emit('toggleMaximize')"
    >
      <!-- 可选：居中微型淡色标识或留白，保持现代 Windows 11 Fluent 干净视觉 -->
      <span class="text-[11px] text-m3-on-surface-variant/40 font-mono tracking-widest hidden 2xl:inline select-none pointer-events-none">
        RUSTED WARFARE TOOLSUITE
      </span>
    </div>

    <!-- 右侧：Windows 11 客户端自绘控制按钮 (CSD: 最小化 / 最大化或还原 / 关闭) -->
    <div class="flex items-center h-full shrink-0">
      <!-- 最小化按钮 -->
      <button
        type="button"
        class="csd-btn w-[46px] h-full text-m3-on-surface-variant hover:text-m3-on-surface hover:bg-m3-surface-container/80 dark:hover:bg-white/10 active:bg-m3-surface-high transition-colors"
        title="最小化"
        @click.stop="emit('minimize')"
      >
        <svg class="w-2.5 h-2.5" viewBox="0 0 10 1" fill="currentColor">
          <rect width="10" height="1" rx="0.5" />
        </svg>
      </button>

      <!-- 最大化 / 还原按钮 -->
      <button
        type="button"
        class="csd-btn w-[46px] h-full text-m3-on-surface-variant hover:text-m3-on-surface hover:bg-m3-surface-container/80 dark:hover:bg-white/10 active:bg-m3-surface-high transition-colors"
        :title="isMaximized ? '向下还原' : '最大化'"
        @click.stop="emit('toggleMaximize')"
      >
        <!-- 还原图标 (当已最大化时显示重叠矩形) -->
        <svg
          v-if="isMaximized"
          class="w-3 h-3"
          viewBox="0 0 12 12"
          fill="none"
          stroke="currentColor"
          stroke-width="1.1"
        >
          <path d="M2.5 0.5h8a1 1 0 0 1 1 1v8" stroke-linecap="round" />
          <rect x="0.5" y="2.5" width="8.5" height="8.5" rx="1" />
        </svg>

        <!-- 最大化图标 (普通窗口状态下显示单个矩形) -->
        <svg
          v-else
          class="w-2.5 h-2.5"
          viewBox="0 0 10 10"
          fill="none"
          stroke="currentColor"
          stroke-width="1.2"
        >
          <rect x="0.6" y="0.6" width="8.8" height="8.8" rx="1" />
        </svg>
      </button>

      <!-- 关闭按钮 (原生 Windows 红色悬停特效) -->
      <button
        type="button"
        class="csd-btn w-[46px] h-full text-m3-on-surface-variant hover:text-white hover:bg-[#e81123] active:bg-[#c4101f] transition-colors"
        :class="{ 'rounded-tr-xl': !isMaximized }"
        title="关闭"
        @click.stop="emit('close')"
      >
        <svg class="w-3 h-3" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round">
          <line x1="1" y1="1" x2="9" y2="9" />
          <line x1="9" y1="1" x2="1" y2="9" />
        </svg>
      </button>
    </div>
  </div>
</template>
