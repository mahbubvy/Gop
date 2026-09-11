<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Button from "./components/ui/button/Button.vue"
import { Progress } from "./components/ui/progress"
import { ScrollArea } from "./components/ui/scroll-area"
import ThreadProgress from "./components/ThreadProgress.vue"
import { uploadManager } from './utils/UploadManager'
import { X, Clock, Zap, FolderPlus, AlertTriangle } from '@lucide/vue'
import { folderCounts, mediaLabel } from './utils/uploadPresentation'

defineProps<{ account: string }>()

const { state } = uploadManager
const phaseHeading = computed(() => state.cancelRequested ? 'Cancelling…' : state.isCreatingAlbum ? 'Adding files to albums…' : state.totalFiles === 0 ? 'Scanning your selection…' : state.uploadedFiles >= state.totalFiles ? 'Finishing upload…' : `Uploading ${state.totalFiles} ${mediaLabel(state.workPaths)}`)
const destination = computed(() => state.albumAutoMode ? 'Auto albums · One new album per folder' : state.albumName ? `Album · ${state.albumDisplayName}` : 'Google Photos library · No album')

// Elapsed time ticker
const elapsedSeconds = ref(0)
let elapsedInterval: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  elapsedInterval = setInterval(() => {
    if (state.startTime > 0) {
      elapsedSeconds.value = Math.floor((Date.now() - state.startTime) / 1000)
    }
  }, 1000)
})

onUnmounted(() => {
  if (elapsedInterval) {
    clearInterval(elapsedInterval)
  }
})

const threadsList = computed(() => {
  return Array.from(state.threads.values())
    .filter(thread => !['idle', 'completed', 'skipped', 'error'].includes(thread.Status))
    .sort((a, b) => a.WorkerID - b.WorkerID)
})

const progressPercent = computed(() => {
  if (state.totalFiles === 0) return 0
  return Math.round((state.uploadedFiles / state.totalFiles) * 100)
})

// Format bytes to human readable
function formatBytes(bytes: number, decimals = 1): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + ' ' + sizes[i]
}

// Format speed
const speedDisplay = computed(() => {
  if (state.uploadSpeed <= 0) return '--'
  return formatBytes(state.uploadSpeed) + '/s'
})

// Format elapsed time
const elapsedDisplay = computed(() => {
  const seconds = elapsedSeconds.value
  if (seconds < 60) return `${seconds}s`
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  if (mins < 60) return `${mins}m ${secs}s`
  const hours = Math.floor(mins / 60)
  const remainingMins = mins % 60
  return `${hours}h ${remainingMins}m`
})

// Bytes progress display
const bytesDisplay = computed(() => {
  if (state.totalBytes === 0) return ''
  return `${formatBytes(state.uploadedBytes)} / ${formatBytes(state.totalBytes)}`
})

// Album progress display
const albumProgressPercent = computed(() => {
  if (!state.albumStatus || state.albumStatus.TotalItems === 0) return 0
  return Math.round((state.albumStatus.ItemsAdded / state.albumStatus.TotalItems) * 100)
})

const plannedAutoAlbumFolders = computed(() => {
  const folders = state.autoAlbumFolders
  const nameCounts = new Map<string, number>()
  for (const folder of folders) {
    const name = folder.split(/[\\/]/).filter(Boolean).pop() || folder
    nameCounts.set(name, (nameCounts.get(name) || 0) + 1)
  }
  return folders.map(folder => {
    const name = folder.split(/[\\/]/).filter(Boolean).pop() || folder
    return { path: folder, name: nameCounts.get(name) === 1 ? name : folder, ...folderCounts(folder, state.workPaths, state.processedPaths) }
  })
})

function warningFiles(paths: string[]): string {
  return paths.map(path => path.split(/[\\/]/).pop()).filter(Boolean).join(', ')
}
</script>

<template>
  <div class="flex h-full w-full flex-col overflow-y-auto px-4 pt-6 pb-16 md:max-w-4xl md:mx-auto md:px-8" style="--wails-draggable: none">
    <!-- Header with file count -->
    <div class="text-center mb-3">
      <p class="mb-2 text-xs text-muted-foreground break-all">{{ account }}</p>
      <h1 class="text-lg font-semibold" aria-live="polite">{{ phaseHeading }}</h1>
      <p class="mb-3 text-sm text-muted-foreground break-words">{{ destination }}</p>
      <p class="text-2xl font-bold tabular-nums">
        {{ state.uploadedFiles }}<span class="text-muted-foreground font-normal">/</span><span class="text-muted-foreground">{{ state.totalFiles }}</span>
      </p>
      <p class="text-xs text-muted-foreground">
        items processed
      </p>
    </div>

    <!-- Stats row -->
    <div class="flex gap-3 justify-center mb-3 text-xs text-muted-foreground">
      <div class="flex items-center gap-1.5">
        <Zap :size="12" />
        <span class="tabular-nums text-foreground">{{ speedDisplay }}</span>
      </div>
      <span class="text-border">|</span>
      <div class="flex items-center gap-1.5">
        <Clock :size="12" />
        <span class="tabular-nums text-foreground">{{ elapsedDisplay }}</span>
      </div>
    </div>

    <!-- Main progress bar -->
    <div class="mb-4">
      <Progress
        :model-value="progressPercent"
        class="h-2.5"
      />
      <div class="flex justify-between mt-1.5 text-xs text-muted-foreground">
        <span v-if="bytesDisplay">{{ bytesDisplay }}</span>
        <span v-else>&nbsp;</span>
        <span class="font-medium">{{ progressPercent }}%</span>
      </div>
    </div>

    <div
      v-if="state.albumAutoMode || state.albumName"
      class="mb-3 rounded-lg border bg-muted/30 p-3"
    >
      <div class="flex items-center gap-2 mb-1.5">
        <FolderPlus :size="14" class="text-primary" />
        <span class="text-sm font-medium">{{ state.albumAutoMode ? 'Folder upload progress' : 'Album destination' }}</span>
      </div>
      <template v-if="state.albumAutoMode">
        <p class="text-xs text-muted-foreground">
          {{ plannedAutoAlbumFolders.length ? 'Files upload first; album assignment follows.' : 'Looking for folders to turn into albums…' }}
        </p>
        <ul v-if="plannedAutoAlbumFolders.length" tabindex="0" aria-label="Folder upload progress" class="mt-2 max-h-40 overflow-y-auto space-y-3 text-xs text-muted-foreground">
          <li v-for="folder in plannedAutoAlbumFolders" :key="folder.path">
            <div class="flex justify-between gap-2"><span class="break-all">{{ folder.name }}</span><span class="shrink-0">{{ folder.processed }} / {{ folder.total }} processed</span></div>
            <Progress :model-value="folder.total ? folder.processed / folder.total * 100 : 0" class="mt-1 h-1.5" />
          </li>
        </ul>
      </template>
      <p v-else class="text-xs text-muted-foreground break-all">{{ state.albumDisplayName }}</p>
    </div>

    <!-- This is shown only for the album operation happening in this batch. -->
    <div
      v-if="state.albumStatus && (state.isCreatingAlbum || state.albumStatus.IsComplete)"
      class="mb-3 p-3 rounded-lg border bg-muted/30"
    >
      <div class="flex items-center gap-2 mb-2">
        <FolderPlus
          :size="14"
          class="text-primary"
        />
        <span class="text-sm font-medium">
          {{ state.isCreatingAlbum ? 'Adding to album...' : 'Album updated' }}
        </span>
      </div>
      <p class="text-xs text-muted-foreground mb-1.5">
        {{ state.albumStatus.AlbumName }}
      </p>
      <Progress
        :model-value="albumProgressPercent"
        class="h-1.5"
      />
      <p class="text-xs text-muted-foreground mt-1">
        {{ state.albumStatus.ItemsAdded }} / {{ state.albumStatus.TotalItems }} items
      </p>
    </div>

    <div
      v-if="state.warnings.length"
      class="mb-3 space-y-1.5"
    >
      <div
        v-for="(warning, index) in state.warnings"
        :key="`${warning.Code}-${index}`"
        class="flex gap-2 rounded-md bg-amber-500/10 px-2.5 py-2 text-amber-700 dark:text-amber-300"
      >
        <AlertTriangle
          :size="14"
          class="mt-0.5 shrink-0"
        />
        <div class="min-w-0">
          <p class="text-xs leading-snug">
            {{ warning.Message }}
          </p>
          <p class="truncate text-[10px] opacity-75">
            {{ warningFiles(warning.Paths) }}
          </p>
        </div>
      </div>
    </div>

    <!-- Thread list - scrollable -->
    <div class="flex-1 min-h-32 mb-3">
      <p class="text-xs text-muted-foreground mb-1.5">
        Active threads ({{ threadsList.length }})
      </p>
      <ScrollArea tabindex="0" aria-label="Current upload files" class="h-[calc(100%-20px)]">
        <div class="space-y-1.5 pr-3 md:grid md:grid-cols-2 md:gap-2 md:space-y-0">
          <ThreadProgress
            v-for="thread in threadsList"
            :key="thread.WorkerID"
            :thread="thread"
          />
        </div>
      </ScrollArea>
    </div>

    <!-- Cancel button - fixed at bottom -->
    <Button
      variant="destructive"
      size="sm"
      class="w-full md:max-w-md md:self-center"
      :disabled="state.cancelRequested"
      @click="() => uploadManager.cancelUpload()"
    >
      <X
        :size="14"
        class="mr-1"
      />
      {{ state.cancelRequested ? 'Cancelling…' : 'Cancel upload' }}
    </Button>
  </div>
</template>
