<script setup lang="ts">
import { ref } from 'vue'
import {
  UploadCloud,
  CheckCircle2,
  AlertTriangle,
  X,
  Server,
  HardDrive,
  Hash,
  Clock,
  Copy,
  Check,
  ShieldCheck,
  RefreshCw,
  ArrowRight
} from 'lucide-vue-next'
import type { PushResult, LanPeer } from '../types/api'

const props = withDefaults(
  defineProps<{
    target: { type: 'mod' | 'map'; name: string; size?: number } | null
    activeServer: LanPeer | null
    pushing: boolean
    result: PushResult | null
    errorMsg: string
  }>(),
  {
    pushing: false,
    result: null,
    errorMsg: ''
  }
)

const emit = defineEmits<{
  (e: 'confirm'): void
  (e: 'close'): void
  (e: 'go-lan'): void
}>()

const copiedHash = ref(false)

function formatBytes(bytes?: number | null) {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  if (i < 0 || i >= sizes.length) return '0 B'
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatTime(unixSec?: number) {
  if (!unixSec) return '刚刚'
  const d = new Date(unixSec * 1000)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

async function copyHash(hash: string) {
  if (!hash) return
  try {
    await navigator.clipboard.writeText(hash)
    copiedHash.value = true
    setTimeout(() => {
      copiedHash.value = false
    }, 2000)
  } catch {
    // ignore clipboard errors
  }
}
</script>

<template>
  <div
    v-if="target || result || errorMsg"
    class="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-6 animate-in fade-in duration-150"
    @click.self="!pushing && emit('close')"
  >
    <!-- 1. 推送成功反馈页面 -->
    <div
      v-if="result && result.success"
      class="bg-m3-surface-container border border-m3-success/40 rounded-3xl max-w-md w-full p-6 shadow-m3-3 space-y-5 animate-in zoom-in-95 duration-200"
    >
      <!-- 成功状态头部 -->
      <div class="flex items-start justify-between gap-3">
        <div class="flex items-center gap-3.5">
          <div
            class="w-11 h-11 rounded-2xl flex items-center justify-center shrink-0 border"
            :class="
              result.overwritten
                ? 'bg-m3-warning-container text-m3-warning border-m3-warning/30'
                : 'bg-m3-success-container text-m3-success border-m3-success/30'
            "
          >
            <CheckCircle2 class="w-6 h-6" />
          </div>
          <div>
            <h3 class="text-base font-bold text-m3-on-surface">
              {{
                result.skippedUpload
                  ? '服务端已存在一致资源'
                  : result.overwritten
                    ? '推送成功 · 已覆盖更新'
                    : '推送成功 · 已送达房主'
              }}
            </h3>
            <p class="text-[11px] text-m3-on-surface-variant mt-0.5 leading-snug">
              {{ result.message }}
            </p>
          </div>
        </div>
        <button
          @click="emit('close')"
          class="p-1.5 rounded-xl text-m3-on-surface-dim hover:text-m3-on-surface hover:bg-m3-surface-highest transition"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- 处理状态胶囊 -->
      <div class="flex flex-wrap items-center gap-2">
        <span
          v-if="result.skippedUpload"
          class="px-2.5 py-1 rounded-full text-[11px] font-medium bg-m3-primary-container text-m3-on-primary-container border border-m3-primary/30 flex items-center gap-1.5"
        >
          <ShieldCheck class="w-3.5 h-3.5" />
          MD5 一致 · 跳过重复上传
        </span>
        <span
          v-else-if="result.overwritten"
          class="px-2.5 py-1 rounded-full text-[11px] font-medium bg-m3-warning-container text-m3-on-warning-container border border-m3-warning/30 flex items-center gap-1.5"
        >
          <RefreshCw class="w-3.5 h-3.5" />
          同名不同 MD5 · 已备份旧版并覆盖
        </span>
        <span
          v-else
          class="px-2.5 py-1 rounded-full text-[11px] font-medium bg-m3-success-container text-m3-on-success-container border border-m3-success/30 flex items-center gap-1.5"
        >
          <CheckCircle2 class="w-3.5 h-3.5" />
          校验通过 · 已入库同步
        </span>

        <span
          v-if="result.backupCreated"
          class="px-2.5 py-1 rounded-full text-[11px] font-mono bg-m3-surface-highest text-m3-on-surface-variant border border-m3-outline-variant"
        >
          原文件已安全备份
        </span>
      </div>

      <!-- 详细结果回执卡片 -->
      <div class="p-4 bg-m3-surface-high border border-m3-outline-variant rounded-2xl space-y-2.5 text-xs font-mono">
        <div class="flex items-center justify-between gap-2">
          <span class="text-m3-on-surface-dim">资源类型</span>
          <span class="text-m3-on-surface font-semibold">
            {{ result.itemType === 'mod' ? '单位模组 (mods/units)' : '地图资源 (mods/maps)' }}
          </span>
        </div>

        <div class="flex items-center justify-between gap-2">
          <span class="text-m3-on-surface-dim shrink-0">文件名称</span>
          <span class="text-m3-primary font-bold truncate" :title="result.fileName">{{ result.fileName }}</span>
        </div>

        <div class="flex items-center justify-between gap-2">
          <span class="text-m3-on-surface-dim flex items-center gap-1">
            <HardDrive class="w-3.5 h-3.5" /> 文件体积
          </span>
          <span class="text-m3-on-surface">{{ formatBytes(result.fileSize) }}</span>
        </div>

        <div v-if="result.contentHash" class="flex items-center justify-between gap-2">
          <span class="text-m3-on-surface-dim flex items-center gap-1 shrink-0">
            <Hash class="w-3.5 h-3.5" /> 内容 MD5
          </span>
          <button
            @click="copyHash(result.contentHash)"
            class="flex items-center gap-1.5 text-m3-on-surface-variant hover:text-m3-primary transition truncate"
            :title="`点击复制完整 MD5: ${result.contentHash}`"
          >
            <span class="truncate">{{ result.contentHash.slice(0, 16) }}…</span>
            <Check v-if="copiedHash" class="w-3.5 h-3.5 text-m3-success shrink-0" />
            <Copy v-else class="w-3.5 h-3.5 shrink-0" />
          </button>
        </div>

        <div class="flex items-center justify-between gap-2 pt-2 border-t border-m3-outline-variant/60">
          <span class="text-m3-on-surface-dim flex items-center gap-1">
            <Server class="w-3.5 h-3.5" /> 目标房主
          </span>
          <span class="text-m3-success font-medium truncate">
            {{ result.serverName || '房主主机' }} ({{ result.serverIp }})
          </span>
        </div>

        <div class="flex items-center justify-between gap-2">
          <span class="text-m3-on-surface-dim flex items-center gap-1">
            <Clock class="w-3.5 h-3.5" /> 完成时间
          </span>
          <span class="text-m3-on-surface-variant">{{ formatTime(result.completedAt) }}</span>
        </div>
      </div>

      <!-- 底部行动按钮 -->
      <div class="flex items-center justify-end gap-2.5 pt-1">
        <button
          @click="emit('go-lan')"
          class="px-4 py-2 bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface rounded-xl text-xs font-medium border border-m3-outline transition flex items-center gap-1.5"
        >
          <span>查看联机同步状态</span>
          <ArrowRight class="w-3.5 h-3.5" />
        </button>
        <button
          @click="emit('close')"
          class="px-5 py-2 bg-m3-primary hover:bg-m3-primary-hover text-white font-semibold rounded-xl text-xs shadow-m3-1 transition"
        >
          完成
        </button>
      </div>
    </div>

    <!-- 2. 推送失败反馈状态 -->
    <div
      v-else-if="errorMsg && !pushing"
      class="bg-m3-surface-container border border-m3-error/40 rounded-3xl max-w-md w-full p-6 shadow-m3-3 space-y-4 animate-in zoom-in-95 duration-200"
    >
      <div class="flex items-start justify-between gap-3">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-2xl bg-m3-error-container text-m3-error border border-m3-error/30 flex items-center justify-center shrink-0">
            <AlertTriangle class="w-5 h-5" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-m3-on-surface">推送未能完成</h3>
            <p class="text-[11px] text-m3-on-surface-dim mt-0.5">请检查房主审批或网络连接后重试</p>
          </div>
        </div>
        <button @click="emit('close')" class="text-m3-on-surface-dim hover:text-m3-on-surface">
          <X class="w-4 h-4" />
        </button>
      </div>

      <div v-if="target" class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl text-xs font-mono">
        <div class="text-m3-on-surface-variant">
          目标文件：<span class="text-m3-on-surface font-semibold">{{ target.name }}</span>
        </div>
      </div>

      <div class="p-3 bg-m3-error-container/20 border border-m3-error/30 rounded-xl text-xs text-m3-error leading-relaxed">
        {{ errorMsg }}
      </div>

      <div class="flex items-center justify-end gap-2.5 pt-2">
        <button
          @click="emit('close')"
          class="px-4 py-2 bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface rounded-xl text-xs border border-m3-outline transition"
        >
          关闭
        </button>
        <button
          v-if="target && activeServer"
          @click="emit('confirm')"
          class="px-5 py-2 bg-m3-primary hover:bg-m3-primary-hover text-white font-semibold rounded-xl text-xs flex items-center gap-2 shadow-m3-1 transition"
        >
          <RefreshCw class="w-3.5 h-3.5" />
          <span>重试推送</span>
        </button>
      </div>
    </div>

    <!-- 3. 推送确认 / 传输等待状态 -->
    <div
      v-else-if="target"
      class="bg-m3-surface-container border border-m3-outline rounded-3xl max-w-md w-full p-6 shadow-m3-3 space-y-4"
    >
      <div class="flex items-center justify-between border-b border-m3-outline-variant pb-3">
        <div class="flex items-center gap-2.5">
          <UploadCloud class="w-5 h-5 text-m3-primary" :class="{ 'animate-bounce': pushing }" />
          <h3 class="text-sm font-bold text-m3-on-surface">
            {{ pushing ? '正在推送至房主...' : `推送${target.type === 'mod' ? '模组' : '地图'}至房主` }}
          </h3>
        </div>
        <button
          @click="!pushing && emit('close')"
          :disabled="pushing"
          class="text-m3-on-surface-dim hover:text-m3-on-surface disabled:opacity-30 disabled:cursor-not-allowed"
          :title="pushing ? '正在传输或等待房主审批，请稍候' : '关闭'"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="space-y-3 text-xs">
        <div class="p-3 bg-m3-surface-high border border-m3-outline-variant rounded-xl space-y-1.5 font-mono">
          <div class="text-m3-on-surface-variant">
            文件名称：<span class="text-m3-primary font-bold">{{ target.name }}</span>
          </div>
          <div v-if="target.size" class="text-m3-on-surface-variant">
            文件大小：<span class="text-m3-on-surface">{{ formatBytes(target.size) }}</span>
          </div>
          <div class="text-m3-on-surface-variant">
            目标房主：<span class="text-m3-success font-medium">{{ activeServer?.deviceName || '房主主机' }} ({{ activeServer?.ip }})</span>
          </div>
        </div>

        <p v-if="!pushing" class="text-m3-on-surface-dim leading-relaxed">
          推送前将自动校验内容 MD5：若房主已拥有完全一致的文件将秒级跳过重传；若同名但 MD5 不同将提示房主备份旧版后覆盖更新。
        </p>
        <div
          v-else
          class="p-3 rounded-xl bg-m3-primary-container/20 border border-m3-primary/30 text-m3-on-surface-variant leading-relaxed flex items-center gap-2.5"
        >
          <RefreshCw class="w-4 h-4 text-m3-primary animate-spin shrink-0" />
          <span>正在校验哈希、等待房主审批并流式传输文件，请勿关闭窗口或重复操作...</span>
        </div>
      </div>

      <div class="flex items-center justify-end gap-2.5 pt-2">
        <button
          @click="emit('close')"
          :disabled="pushing"
          class="px-4 py-2 bg-m3-surface-high hover:bg-m3-surface-highest text-m3-on-surface-variant hover:text-m3-on-surface rounded-xl text-xs border border-m3-outline transition disabled:opacity-40 disabled:cursor-not-allowed"
        >
          取消
        </button>
        <button
          @click="emit('confirm')"
          :disabled="pushing"
          class="px-5 py-2 bg-m3-primary hover:bg-m3-primary-hover text-white font-semibold rounded-xl text-xs flex items-center gap-2 shadow-m3-1 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <UploadCloud class="w-4 h-4" :class="{ 'animate-bounce': pushing }" />
          <span>{{ pushing ? '传输中 / 等待审批...' : '确认推送' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
