<script setup lang="ts">
import { ref, watch } from 'vue'
import { AlertTriangle, Trash2, Archive, X, Package, Map as MapIcon, HardDrive } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    show: boolean
    targetType: 'mod' | 'map' | 'pack'
    title: string
    fileName: string
    fileSize?: number
    loading?: boolean
  }>(),
  {
    fileSize: 0,
    loading: false
  }
)

const emit = defineEmits<{
  (e: 'confirm', permanently: boolean): void
  (e: 'close'): void
}>()

const permanently = ref(false)

// 每次打开弹窗重置复选框
watch(
  () => props.show,
  val => {
    if (val) {
      permanently.value = false
    }
  }
)

function formatBytes(bytes?: number | null) {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  if (i < 0 || i >= sizes.length) return '0 B'
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function handleConfirm() {
  emit('confirm', permanently.value)
}
</script>

<template>
  <div
    v-if="show"
    class="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex items-center justify-center p-4 animate-in fade-in duration-200"
    @click.self="!loading && emit('close')"
  >
    <div
      class="bg-m3-surface-container border border-m3-outline rounded-3xl max-w-md w-full shadow-2xl overflow-hidden p-6 space-y-5 animate-in zoom-in-95 duration-200"
    >
      <!-- 头部 -->
      <div class="flex items-start justify-between gap-3">
        <div class="flex items-center gap-3">
          <div
            class="w-10 h-10 rounded-2xl flex items-center justify-center shrink-0 border"
            :class="permanently ? 'bg-m3-error-container text-m3-on-error-container border-m3-error/30' : 'bg-m3-warning-container text-m3-on-warning-container border-m3-warning/30'"
          >
            <Trash2 v-if="permanently" class="w-5 h-5 text-m3-error" />
            <Archive v-else class="w-5 h-5 text-m3-warning" />
          </div>
          <div>
            <h3 class="text-base font-bold text-m3-on-surface">
              {{ permanently ? '彻底删除' : '确认归档' }}{{ targetType === 'mod' ? '模组' : targetType === 'pack' ? '地图包' : '地图' }}
            </h3>
            <p class="text-xs text-m3-on-surface-dim mt-0.5">
              {{ permanently ? '此操作将直接从磁盘永久移除文件' : '将目标移入已归档目录，移出游戏活跃库' }}
            </p>
          </div>
        </div>

        <button
          @click="!loading && emit('close')"
          :disabled="loading"
          class="p-1.5 rounded-xl text-m3-on-surface-dim hover:text-m3-on-surface hover:bg-m3-surface-highest transition disabled:opacity-40"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- 目标卡片预览 -->
      <div class="p-3.5 rounded-2xl bg-m3-surface-dim border border-m3-outline-variant space-y-1.5">
        <div class="flex items-center gap-2 text-xs font-semibold text-m3-on-surface truncate">
          <Package v-if="targetType === 'mod'" class="w-4 h-4 text-m3-primary shrink-0" />
          <MapIcon v-else class="w-4 h-4 text-m3-success shrink-0" />
          <span class="truncate">{{ title || fileName }}</span>
        </div>
        <div class="flex items-center justify-between text-[11px] text-m3-on-surface-dim font-mono pt-1 border-t border-m3-outline-variant/50">
          <span class="truncate max-w-[240px]">{{ fileName }}</span>
          <span v-if="fileSize > 0" class="shrink-0 flex items-center gap-1">
            <HardDrive class="w-3 h-3 text-m3-tertiary" />
            {{ formatBytes(fileSize) }}
          </span>
        </div>
      </div>

      <!-- 说明与选项 -->
      <div class="space-y-3 text-xs text-m3-on-surface-variant leading-relaxed">
        <p v-if="!permanently" class="text-m3-on-surface-variant/90">
          📦 默认操作为<strong>安全归档</strong>：文件将自动转移至游戏目录下对应的 <code class="px-1.5 py-0.5 rounded bg-black/40 text-m3-primary font-mono text-[11px]">mods/_archived/</code> 文件夹中。铁锈战争游戏与管理器将不再加载它，后续需要时您可随时手动还原。
        </p>
        <p v-else class="text-m3-error bg-m3-error-container/20 border border-m3-error/30 p-2.5 rounded-xl text-xs font-medium">
          ⚠️ 注意：勾选彻底删除后，该文件将直接从磁盘永久清除，<strong>无法恢复</strong>！请谨慎操作。
        </p>

        <!-- 彻底删除复选框选项 -->
        <label class="flex items-center gap-2.5 pt-2 cursor-pointer select-none text-m3-on-surface">
          <input
            type="checkbox"
            v-model="permanently"
            :disabled="loading"
            class="w-4 h-4 rounded border-m3-outline text-m3-error focus:ring-m3-error accent-red-500 cursor-pointer"
          />
          <span class="text-xs font-medium" :class="permanently ? 'text-m3-error font-semibold' : 'text-m3-on-surface-variant'">
            同时彻底从磁盘删除文件 (不保留在 _archived 备份中)
          </span>
        </label>
      </div>

      <!-- 底部操作按钮 -->
      <div class="flex items-center justify-end gap-2.5 pt-2 border-t border-m3-outline-variant">
        <button
          @click="emit('close')"
          :disabled="loading"
          class="px-4 py-2 rounded-xl text-xs font-medium text-m3-on-surface bg-m3-surface-high hover:bg-m3-surface-highest border border-m3-outline transition disabled:opacity-40"
        >
          取消
        </button>

        <button
          @click="handleConfirm"
          :disabled="loading"
          class="px-4 py-2 rounded-xl text-xs font-semibold transition flex items-center gap-1.5 shadow-sm disabled:opacity-40"
          :class="permanently ? 'bg-m3-error hover:bg-red-600 text-white' : 'bg-m3-primary hover:bg-m3-primary-hover text-white'"
        >
          <Trash2 v-if="permanently" class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
          <Archive v-else class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
          <span>{{ loading ? '处理中...' : permanently ? '确认彻底删除' : '确认归档' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
