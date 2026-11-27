<script setup lang="ts">
import { computed } from 'vue'
import type { ThreadStatus } from '../utils/UploadManager'
import { Progress } from '@/components/ui/progress'

const props = defineProps<{
  thread: ThreadStatus
}>()

const statusLabel = computed(() => {
  switch (props.thread.Status) {
    case 'error':
      return 'Error'
    case 'completed':
      return 'Done'
    case 'skipped':
      return 'Skipped'
    case 'uploading':
      return props.thread.Attempt > 1 ? `Retry ${props.thread.Attempt - 1}` : 'Uploading'
    case 'hashing':
      return 'Preparing file'
    case 'checking':
      return 'Checking'
    case 'finalizing':
      return 'Finalizing'
    default:
      return props.thread.Status
  }
})

const uploadPercent = computed(() => {
  const { Status, BytesTotal, BytesUploaded } = props.thread
  if (Status === 'finalizing') return 100
  if (Status !== 'uploading' || BytesTotal <= 0) {
    return null
  }
  return Math.min(100, Math.max(0, Math.floor(BytesUploaded * 100 / BytesTotal)))
})

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

const fileSizeDisplay = computed(() => {
  if (props.thread.Status !== 'uploading' || !props.thread.BytesTotal) {
    return null
  }
  return `${formatBytes(props.thread.BytesUploaded)} / ${formatBytes(props.thread.BytesTotal)}`
})

const statusMessage = computed(() => props.thread.Message.replace(/^Error:\s*/, ''))
</script>

<template>
  <div class="space-y-2 rounded-md border bg-card p-3">
    <div class="flex items-baseline gap-3">
      <span
        class="min-w-0 flex-1 truncate text-sm font-medium"
        :title="thread.FilePath || thread.FileName"
      >
        {{ thread.FileName || thread.Message }}
      </span>
      <span
        v-if="uploadPercent !== null"
        class="shrink-0 text-lg font-semibold leading-none tabular-nums"
        aria-hidden="true"
      >
        {{ uploadPercent }}%
      </span>
    </div>

    <Progress
      v-if="uploadPercent !== null"
      :model-value="uploadPercent"
      :get-value-label="() => thread.FileName || 'File upload progress'"
      :get-value-text="() => thread.Status === 'finalizing' ? '100%, finalizing' : `${uploadPercent}%`"
      class="h-2.5"
    />
    <div
      v-if="fileSizeDisplay"
      class="flex flex-wrap justify-between gap-x-2 text-xs text-muted-foreground tabular-nums"
    >
      <span>{{ fileSizeDisplay }}</span>
      <span v-if="thread.Attempt > 1">{{ statusLabel }}</span>
    </div>
    <p
      v-else-if="thread.Status === 'error' || thread.Status === 'skipped'"
      class="text-xs leading-snug break-words"
      :class="thread.Status === 'error' ? 'text-destructive' : 'text-muted-foreground'"
    >
      {{ statusLabel }} · {{ statusMessage }}
    </p>
    <p
      v-else
      class="text-xs text-muted-foreground"
    >
      {{ statusLabel }}
    </p>
  </div>
</template>
