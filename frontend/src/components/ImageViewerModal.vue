<script setup lang="ts">
import { ref, watch, nextTick, onMounted, onUnmounted } from 'vue'
import Panzoom, { PanzoomObject } from '@panzoom/panzoom'
import {
  ZoomIn,
  ZoomOut,
  Maximize2,
  RotateCw,
  FlipHorizontal,
  RefreshCw,
  Maximize,
  Minimize2,
  X,
  ShieldAlert
} from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    show: boolean
    src?: string
    title?: string
    subtitle?: string
    badge?: string
    badgeActive?: boolean
    hasImage?: boolean
    emptyText?: string
  }>(),
  {
    src: '',
    title: '',
    subtitle: '',
    badge: '',
    badgeActive: true,
    hasImage: true,
    emptyText: '暂无高清预览图'
  }
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'empty-action'): void
}>()

const viewportRef = ref<HTMLDivElement | null>(null)
const imageRef = ref<HTMLImageElement | null>(null)
let panzoomInstance: PanzoomObject | null = null

const zoomPercent = ref(100)
const rotation = ref(0)
const flipH = ref(false)
const isMaximized = ref(false)
const imageLoading = ref(true)
const imageError = ref(false)

function updateScale() {
  if (!panzoomInstance) return
  const s = panzoomInstance.getScale()
  zoomPercent.value = Math.round(s * 100)
}

function initPanzoom() {
  if (!viewportRef.value || !imageRef.value || !props.hasImage || !props.src) return
  destroyPanzoom()

  imageLoading.value = true
  imageError.value = false
  rotation.value = 0
  flipH.value = false

  panzoomInstance = Panzoom(imageRef.value, {
    maxScale: 12,
    minScale: 0.15,
    cursor: 'grab',
    canvas: false,
    duration: 180,
    easing: 'ease-out',
    setTransform: (elem, { x, y, scale }) => {
      const scaleX = flipH.value ? -scale : scale
      elem.style.transform = `translate(${x}px, ${y}px) scale(${scaleX}, ${scale}) rotate(${rotation.value}deg)`
    }
  })

  // 绑定鼠标滚轮缩放事件
  viewportRef.value.addEventListener('wheel', handleWheel, { passive: false })
  // 绑定变动事件实时更新缩放比
  imageRef.value.addEventListener('panzoomchange', updateScale)
  updateScale()
}

function destroyPanzoom() {
  if (viewportRef.value) {
    viewportRef.value.removeEventListener('wheel', handleWheel)
  }
  if (imageRef.value) {
    imageRef.value.removeEventListener('panzoomchange', updateScale)
  }
  if (panzoomInstance) {
    panzoomInstance.destroy()
    panzoomInstance = null
  }
}

function handleWheel(e: WheelEvent) {
  e.preventDefault()
  e.stopPropagation()
  if (!panzoomInstance) return
  panzoomInstance.zoomWithWheel(e)
  updateScale()
}

function handleDoubleClick(e: MouseEvent) {
  if (!panzoomInstance) return
  const current = panzoomInstance.getScale()
  if (current > 1.25) {
    panzoomInstance.reset({ animate: true })
    rotation.value = 0
    flipH.value = false
  } else {
    panzoomInstance.zoomToPoint(2.5, { clientX: e.clientX, clientY: e.clientY }, { animate: true })
  }
  updateScale()
}

function handleZoomIn() {
  if (!panzoomInstance) return
  panzoomInstance.zoomIn({ animate: true })
  updateScale()
}

function handleZoomOut() {
  if (!panzoomInstance) return
  panzoomInstance.zoomOut({ animate: true })
  updateScale()
}

function handleReset() {
  if (!panzoomInstance) return
  rotation.value = 0
  flipH.value = false
  panzoomInstance.reset({ animate: true })
  updateScale()
}

function handleOneToOne() {
  if (!panzoomInstance) return
  panzoomInstance.zoom(1, { animate: true })
  panzoomInstance.pan(0, 0, { animate: true })
  updateScale()
}

function handleRotate() {
  if (!panzoomInstance) return
  rotation.value = (rotation.value + 90) % 360
  // 触发重绘
  const pan = panzoomInstance.getPan()
  const scale = panzoomInstance.getScale()
  panzoomInstance.pan(pan.x, pan.y, { force: true })
}

function handleFlip() {
  if (!panzoomInstance) return
  flipH.value = !flipH.value
  const pan = panzoomInstance.getPan()
  panzoomInstance.pan(pan.x, pan.y, { force: true })
}

function toggleMaximize() {
  isMaximized.value = !isMaximized.value
}

function handleImageLoaded() {
  imageLoading.value = false
  imageError.value = false
  nextTick(() => {
    updateScale()
  })
}

function handleImageError() {
  imageLoading.value = false
  imageError.value = true
}

function onKeydown(e: KeyboardEvent) {
  if (!props.show) return
  if (e.key === 'Escape') {
    emit('close')
  } else if (e.key === '+' || e.key === '=') {
    handleZoomIn()
  } else if (e.key === '-' || e.key === '_') {
    handleZoomOut()
  } else if (e.key === '0') {
    handleReset()
  } else if (e.key === 'r' || e.key === 'R') {
    handleRotate()
  } else if (e.key === 'f' || e.key === 'F') {
    toggleMaximize()
  }
}

watch(
  () => props.show,
  val => {
    if (val) {
      nextTick(() => {
        initPanzoom()
      })
    } else {
      destroyPanzoom()
      isMaximized.value = false
    }
  }
)

watch(
  () => props.src,
  () => {
    if (props.show) {
      nextTick(() => {
        initPanzoom()
      })
    }
  }
)

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  destroyPanzoom()
})
</script>

<template>
  <div
    v-if="show"
    class="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex items-center justify-center p-3 sm:p-5 animate-in fade-in duration-200"
    @click.self="emit('close')"
  >
    <div
      class="bg-m3-surface-container border border-m3-outline rounded-3xl flex flex-col shadow-2xl overflow-hidden transition-all duration-300"
      :class="isMaximized ? 'w-[98vw] h-[96vh]' : 'max-w-5xl w-full h-[88vh] max-h-[880px]'"
    >
      <!-- 弹窗标题与工具栏 -->
      <div class="flex items-center justify-between px-5 py-3.5 border-b border-m3-outline-variant bg-m3-surface-high shrink-0 select-none">
        <div class="flex items-center gap-3 min-w-0 pr-4">
          <slot name="header-icon">
            <div class="w-8 h-8 rounded-xl bg-m3-primary-container text-m3-primary flex items-center justify-center shrink-0">
              <Maximize2 class="w-4 h-4 text-m3-on-primary-container" />
            </div>
          </slot>
          <div class="min-w-0">
            <h3 class="text-sm font-bold text-m3-on-surface flex items-center gap-2 truncate">
              <span class="truncate">{{ title || '图片大图预览' }}</span>
              <span
                v-if="badge"
                class="px-2 py-0.5 rounded-full text-[11px] font-medium shrink-0"
                :class="badgeActive ? 'bg-m3-success-container text-m3-on-success-container' : 'bg-m3-surface-highest text-m3-on-surface-dim'"
              >
                {{ badge }}
              </span>
            </h3>
            <p v-if="subtitle" class="text-[11px] text-m3-on-surface-dim font-mono truncate mt-0.5">
              {{ subtitle }}
            </p>
          </div>
        </div>

        <!-- 缩放/操作工具栏 -->
        <div class="flex items-center gap-1.5 shrink-0">
          <slot name="header-extra" />

          <!-- 缩小 -->
          <button
            @click="handleZoomOut"
            class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-surface-highest text-m3-on-surface-variant transition border border-m3-outline active:scale-95"
            title="缩小 (-)"
          >
            <ZoomOut class="w-4 h-4" />
          </button>

          <!-- 缩放比指示 (点击重置) -->
          <button
            @click="handleReset"
            class="px-2.5 py-1.5 rounded-xl bg-m3-surface-container hover:bg-m3-surface-highest text-xs font-mono text-m3-on-surface-variant transition border border-m3-outline text-center min-w-[54px]"
            title="点击还原初始比例 (0)"
          >
            {{ zoomPercent }}%
          </button>

          <!-- 放大 -->
          <button
            @click="handleZoomIn"
            class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-surface-highest text-m3-on-surface-variant transition border border-m3-outline active:scale-95"
            title="放大 (+)"
          >
            <ZoomIn class="w-4 h-4" />
          </button>

          <!-- 1:1 像素尺寸 -->
          <button
            @click="handleOneToOne"
            class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-surface-highest text-m3-on-surface-variant transition border border-m3-outline active:scale-95 hidden sm:inline-flex"
            title="1:1 原始尺寸"
          >
            <Maximize2 class="w-4 h-4" />
          </button>

          <!-- 重置适应 -->
          <button
            @click="handleReset"
            class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-surface-highest text-m3-on-surface-variant transition border border-m3-outline active:scale-95"
            title="居中重置 (0)"
          >
            <RefreshCw class="w-4 h-4" />
          </button>

          <!-- 顺时针旋转90° -->
          <button
            @click="handleRotate"
            class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-surface-highest text-m3-on-surface-variant transition border border-m3-outline active:scale-95 hidden sm:inline-flex"
            title="向右旋转 90° (R)"
          >
            <RotateCw class="w-4 h-4" />
          </button>

          <!-- 水平镜像翻转 -->
          <button
            @click="handleFlip"
            class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-surface-highest text-m3-on-surface-variant transition border border-m3-outline active:scale-95 hidden sm:inline-flex"
            title="水平翻转镜像"
          >
            <FlipHorizontal class="w-4 h-4" />
          </button>

          <div class="h-4 w-px bg-m3-outline mx-1 hidden sm:block" />

          <!-- 最大化/恢复窗口 -->
          <button
            @click="toggleMaximize"
            class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-surface-highest text-m3-on-surface-variant transition border border-m3-outline active:scale-95"
            :title="isMaximized ? '退出全屏窗口 (F)' : '全屏窗口浏览 (F)'"
          >
            <Minimize2 v-if="isMaximized" class="w-4 h-4" />
            <Maximize v-else class="w-4 h-4" />
          </button>

          <!-- 关闭按钮 -->
          <button
            @click="emit('close')"
            class="p-2 rounded-xl bg-m3-surface-container hover:bg-m3-error-container hover:text-m3-on-error-container text-m3-on-surface-variant transition border border-m3-outline active:scale-95"
            title="关闭 (Esc)"
          >
            <X class="w-4 h-4" />
          </button>
        </div>
      </div>

      <!-- 图像交互主展示视口 (Panzoom Viewport) -->
      <div
        ref="viewportRef"
        class="flex-1 relative overflow-hidden bg-m3-surface-dim flex items-center justify-center select-none"
        :class="{ 'cursor-grab': hasImage && src && !imageError, 'cursor-default': !hasImage || !src || imageError }"
        @dblclick="handleDoubleClick"
      >
        <!-- 图片存在且加载成功 -->
        <div
          v-if="hasImage && src && !imageError"
          class="w-full h-full flex items-center justify-center p-4 relative"
        >
          <img
            ref="imageRef"
            :src="src"
            @load="handleImageLoaded"
            @error="handleImageError"
            class="max-w-full max-h-full object-contain rounded-xl shadow-m3-2 select-none pointer-events-auto will-change-transform"
            alt="Viewer Preview"
          />
        </div>

        <!-- 缺失预览图状态 -->
        <div v-else class="flex flex-col items-center justify-center text-m3-warning gap-3 p-6 text-center">
          <ShieldAlert class="w-14 h-14 stroke-[1.2]" />
          <p class="text-xs font-medium text-m3-on-surface-variant">{{ emptyText }}</p>
          <slot name="empty-action">
            <button
              @click="emit('empty-action')"
              class="flex items-center gap-2 px-4 py-2 bg-m3-primary hover:bg-m3-primary-hover text-white rounded-xl text-xs font-medium shadow-m3-1 transition"
            >
              <span>生成预览图</span>
            </button>
          </slot>
        </div>

        <!-- 操作提示浮层 (轻量半透明徽章) -->
        <div
          v-if="hasImage && src && !imageError"
          class="absolute bottom-3 left-4 z-10 pointer-events-none hidden sm:flex items-center gap-2 text-[10px] text-white/50 bg-black/40 backdrop-blur-md px-3 py-1 rounded-full border border-white/10"
        >
          <span>🖱️ 滚轮缩放</span>
          <span>•</span>
          <span>按住鼠标拖动平移</span>
          <span>•</span>
          <span>双击自适应/局部放大</span>
        </div>
      </div>

      <!-- 底部插槽 (元数据展示 + 动作操作栏) -->
      <div v-if="$slots.footer" class="px-6 py-3.5 bg-m3-surface-high border-t border-m3-outline-variant flex flex-wrap items-center justify-between gap-3 text-xs shrink-0">
        <slot name="footer" />
      </div>
    </div>
  </div>
</template>
