<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { TooltipProvider } from '@/components/ui/tooltip'
import {
  Sheet,
  SheetContent,
  SheetTrigger,
} from '@/components/ui/sheet'
import { useColorMode } from '@vueuse/core'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { Files, FolderOpen, UserPlus } from '@lucide/vue'
import DebugLogButton from './components/DebugLogButton.vue'
import { ConfigManager } from '../bindings/app/backend'
import { Dialogs, Events } from '@wailsio/runtime'
import Button from "./components/ui/button/Button.vue"
import GoogleAccountSelect from './components/GoogleAccountSelect.vue'
import GoogleAuthSetup from "./components/GoogleAuthSetup.vue"
import './index.css'
import SettingsPanel from "./SettingsPanel.vue"
import Upload from './Upload.vue'
import UploadComplete from './UploadComplete.vue'
import AlbumHistory from './AlbumHistory.vue'
import UploadRecords from './UploadRecords.vue'
import { uploadManager } from './utils/UploadManager'
import { uploadModes, useUploadSelection } from './utils/uploadSelection'
import { errorCategory, safeName } from './utils/debugLog'
import { folderCounts } from './utils/uploadPresentation'
import versionInfo from '../../build/windows/info.json'
import Toaster from './components/ui/sonner/Sonner.vue'

import { toast } from "vue-sonner"

useColorMode().value = "dark"

const { state: uploadState } = uploadManager

interface RecentAlbum {
  albumId: string
  albumName: string
  lastUsedAt: number
}

type RecentAlbumEntryMode = 'normal' | 'drag-drop'

// Drag state for the three drop zones
const isDraggingFiles = ref(false)

const selectedOption = ref('')
const options = ref<string[]>([])
const accountNeedsTokenBinding = ref<Record<string, boolean>>({})
const tokenBindingEmail = ref('')
const isExtractingTokenBinding = ref(false)
const isAccountSetupOpen = ref(false)
const removingAccount = ref('')
const isRecentAlbumScreen = ref(false)
const isLoadingRecentAlbums = ref(false)
const recentAlbums = ref<RecentAlbum[]>([])
const selectedRecentAlbum = ref<RecentAlbum | null>(null)
const recentAlbumEntryMode = ref<RecentAlbumEntryMode>('normal')
const pendingRecentSources = ref<string[]>([])
const isAlbumHistoryOpen = ref(false)
const isUploadRecordsOpen = ref(false)
const recordReviewOpen = ref(false)
const recordReviewLoading = ref(false)
const recordReviewError = ref('')
const recordReview = ref({ total: 0, matched: 0, usesAlbums: false })
let recordReviewResolve: ((value: boolean | null) => void) | null = null
let recordReviewGeneration = 0
let recordReviewId = ''
const isSettingsOpen = ref(false)
const isExportingLog = ref(false)
const currentScreen = computed(() => isAccountSetupOpen.value ? 'account-setup' : isSettingsOpen.value ? 'settings' : uploadState.isUploading ? 'uploading' : uploadState.completionVisible ? 'upload-results' : recordReviewOpen.value ? 'record-review' : isUploadRecordsOpen.value ? 'upload-records' : isAlbumHistoryOpen.value ? 'album-history' : isRecentAlbumScreen.value ? 'recent-albums' : showAlbumInput.value ? 'selection-review' : 'home')

const selection = useUploadSelection({
  reviewAuto: true,
  previewAuto: paths => ConfigManager.PreviewAutoAlbums(paths),
  countCandidates: paths => ConfigManager.CountUploadCandidates(paths),
  getAccount: () => selectedOption.value,
  isUploading: () => uploadState.isUploading,
  openFile: options => Dialogs.OpenFile(options),
  setSelected: account => ConfigManager.SetSelected(account),
  setAlbumName: name => ConfigManager.SetAlbumName(name),
  setAlbumAutoMode: auto => ConfigManager.SetAlbumAutoMode(auto),
  reviewRecords: files => reviewLocalRecords(files),
  startUpload: (files, bypassLocalRecords) => {
    uploadManager.setNextAlbumDisplayName(isRecentAlbumScreen.value ? selectedRecentAlbum.value?.albumName || '' : '')
    return Events.Emit('startUpload', { files, bypassLocalRecords: !!bypassLocalRecords })
  },
  onError: error => toast.error('Could not prepare upload', {
    description: error instanceof Error ? error.message : String(error),
  }),
})
const { mode: uploadMode, busy: selectionBusy, pendingFiles, albumName: albumNameOrKey } = selection
const showAlbumInput = computed(() => pendingFiles.value.length > 0)
const controlsBusy = computed(() => selectionBusy.value || selection.starting.value || !!removingAccount.value || isExtractingTokenBinding.value || isLoadingRecentAlbums.value)
const sourceNames = computed(() => selection.previewFolders.value.map(path => ({ path, name: path.split(/[\\/]/).filter(Boolean).pop() || path })))
// Register after all screen dependencies exist: watch evaluates its source now.
watch(currentScreen, async () => {
  await nextTick()
  if (isSettingsOpen.value || isAccountSetupOpen.value) return
  const heading = document.querySelector<HTMLElement>('main h1')
  if (heading) {
    heading.tabIndex = -1
    heading.focus({ preventScroll: true })
  }
})
watch(() => uploadState.completionVisible, visible => {
  if (visible) {
    isRecentAlbumScreen.value = false
    selectedRecentAlbum.value = null
    pendingRecentSources.value = []
  }
})

watch(selectedOption, async (newValue) => {
  isUploadRecordsOpen.value = false
  if (newValue) {
    closeRecentAlbums()
    try {
      await ConfigManager.SetSelected(newValue)
      await updateTokenBindingPrompt(newValue)
      console.log('Successfully updated selected value:', newValue)
    } catch (error) {
      console.error('Failed to update selected value:', error)
      toast.error('Failed to update selected account.')
    }
  } else {
    tokenBindingEmail.value = ''
  }
})

async function reviewLocalRecords(files: string[]): Promise<boolean | null> {
  const settings = await ConfigManager.GetSettings()
  if (!settings.skipRecordedUploads || settings.forceUpload || settings.deleteFromHost) return false
  recordReviewOpen.value = true
  recordReviewLoading.value = true
  recordReviewError.value = ''
  const generation = ++recordReviewGeneration
  recordReviewId = `${Date.now()}-${generation}`
  const decision = new Promise<boolean | null>(resolve => { recordReviewResolve = resolve })
  void ConfigManager.PreviewUploadRecords(files, recordReviewId).then(result => {
    if (generation !== recordReviewGeneration) return
    recordReview.value = result
    recordReviewLoading.value = false
    if (!result.matched) finishRecordReview(false)
  }).catch(() => {
    if (generation !== recordReviewGeneration) return
    recordReviewLoading.value = false
    recordReviewError.value = 'Could not check local records. Cancel, or continue using the normal uploader without local skipping.'
  })
  return decision
}

function finishRecordReview(bypass: boolean | null) {
  if (recordReviewLoading.value) void ConfigManager.CancelUploadRecordPreview(recordReviewId).catch(() => {})
  recordReviewGeneration++
  recordReviewOpen.value = false
  recordReviewLoading.value = false
  const resolve = recordReviewResolve
  recordReviewResolve = null
  resolve?.(bypass)
}

async function updateTokenBindingPrompt(email: string) {
  tokenBindingEmail.value = accountNeedsTokenBinding.value[email] ? email : ''
}

async function refreshCredentials() {
  try {
    const state = await ConfigManager.GetAccounts()
    const nextNeedsTokenBinding: Record<string, boolean> = {}
    const nextOptions = state.accounts.map(account => {
      nextNeedsTokenBinding[account.email] = account.needsTokenBinding
      return account.email
    })

    accountNeedsTokenBinding.value = nextNeedsTokenBinding
    options.value = nextOptions
    selectedOption.value = state.selected || ''
    if (state.selected) {
      await updateTokenBindingPrompt(state.selected)
    } else {
      tokenBindingEmail.value = ''
    }
  } catch (error) {
    console.error('Failed to refresh Google accounts:', error)
    toast.error('Could not refresh Google accounts', {
      description: error instanceof Error ? error.message : String(error),
    })
  }
}

async function addTokenBindingAliasFromADB() {
  if (!tokenBindingEmail.value) return

  isExtractingTokenBinding.value = true
  try {
    await ConfigManager.AddTokenBindingAliasFromADB(tokenBindingEmail.value)
    await refreshCredentials()
    tokenBindingEmail.value = ''
    toast.success('Token binding key added.')
  } catch (error) {
    console.error('Failed to add token binding key:', error)
    toast.error('Failed to add token binding key', {
      description: error instanceof Error ? error.message : String(error),
    })
  } finally {
    isExtractingTokenBinding.value = false
  }
}

async function removeCredentials(email: string) {
  removingAccount.value = email
  try {
    await ConfigManager.RemoveCredentials(email)

    const removedSelectedAccount = selectedOption.value === email
    await refreshCredentials()
    if (removedSelectedAccount && options.value.length > 0 && !selectedOption.value) {
      selectedOption.value = options.value[0]
    }
    toast.success('Credentials removed.')
    return true
  } catch (error) {
    console.error('Failed to remove credentials:', error)
    toast.error('Failed to remove credentials.')
    return false
  } finally {
    removingAccount.value = ''
  }
}

function openAccountSetup() {
  isAccountSetupOpen.value = true
}

async function openRecentAlbums(entryMode: RecentAlbumEntryMode, sources: string[] = []) {
  if (controlsBusy.value || !selectedOption.value || !sources.length && entryMode === 'drag-drop') return

  isLoadingRecentAlbums.value = true
  selectedRecentAlbum.value = null
  recentAlbumEntryMode.value = entryMode
  pendingRecentSources.value = [...sources]
  try {
    recentAlbums.value = await ConfigManager.GetRecentAlbums()
    isRecentAlbumScreen.value = true
  } catch (error) {
    console.error('Failed to load recent albums:', error)
    toast.error('Could not load recent albums')
    pendingRecentSources.value = []
  } finally {
    isLoadingRecentAlbums.value = false
  }
}

function closeRecentAlbums() {
  if (selectionBusy.value) return
  isRecentAlbumScreen.value = false
  selectedRecentAlbum.value = null
  pendingRecentSources.value = []
}

async function selectRecentAlbum(album: RecentAlbum) {
  selectedRecentAlbum.value = album
  if (recentAlbumEntryMode.value !== 'drag-drop') return

  uploadManager.setNextAlbumDisplayName(album.albumName)
  await selection.startRecentAlbum(pendingRecentSources.value, album.albumId)
  if (uploadState.isUploading) closeRecentAlbums()
}

async function chooseForRecentAlbum(kind: 'files' | 'folder') {
  if (!selectedRecentAlbum.value) return
  uploadManager.setNextAlbumDisplayName(selectedRecentAlbum.value.albumName)
  await selection.chooseRecentAlbum(kind, selectedRecentAlbum.value.albumId)
  if (uploadState.isUploading) closeRecentAlbums()
}

onMounted(async () => {
  await refreshCredentials()

})

// Global drag event handlers to detect file dragging
let dragLeaveTimeout: ReturnType<typeof setTimeout> | null = null

const onDragEnter = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  
  // Clear any pending drag leave timeout
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
  
  isDraggingFiles.value = true
}

const onDragOver = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  e.preventDefault()
  
  // Clear any pending drag leave timeout - we're still dragging
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
}

const onDragLeave = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  
  // Use timeout to detect if we've truly left the window
  // dragover will cancel this if we're still in the window
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
  }
  dragLeaveTimeout = setTimeout(() => {
    isDraggingFiles.value = false
    dragLeaveTimeout = null
  }, 50)
}

const onDrop = () => {
  // Clear any pending timeout
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
  
  // Delay resetting isDraggingFiles to allow Wails to process the drop target
  // before Vue re-renders and hides the drop zones
  setTimeout(() => {
    isDraggingFiles.value = false
  }, 100)
}

// Handle album error event
const albumErrorHandler = (e: Event) => {
  const event = e as CustomEvent<{ AlbumName: string; Error: string }>
  const { AlbumName, Error } = event.detail
  // Check if it's a 404 error (album key not found)
  if (Error.includes('404')) {
    toast.error('Album not found', {
      description: `The album key "${AlbumName}" does not exist or is invalid.`,
    })
  } else {
    toast.error('Failed to create album', {
      description: `Album "${AlbumName}": ${Error}`,
    })
  }
}

const uploadErrorHandler = (e: Event) => {
  const event = e as CustomEvent<{ FileName: string; Message: string }>
  const { FileName, Message } = event.detail
  const errorMessage = Message.replace(/^Error:\s*/, '')
  toast.error(FileName ? `Upload failed: ${FileName}` : 'Upload failed', {
    description: errorMessage,
    duration: 10000,
    important: true,
  })
}

async function exportDebugLog() {
  if (isExportingLog.value) return
  isExportingLog.value = true
  const screen = currentScreen.value
  const report = {
    schemaVersion: 3, generatedAt: new Date().toISOString(), screen, app: 'gotohp', version: versionInfo.fixed.file_version,
    window: { width: window.innerWidth, height: window.innerHeight },
    selection: { mode: isRecentAlbumScreen.value ? 'recent-album' : uploadMode.value, pendingSources: pendingFiles.value.length, accountSelected: !!selectedOption.value },
    upload: { active: uploadState.isUploading, cancelRequested: uploadState.cancelRequested, total: uploadState.totalFiles, processed: uploadState.uploadedFiles, totalBytes: uploadState.totalBytes, transferredBytes: uploadState.uploadedBytes, speedBytesPerSecond: uploadState.uploadSpeed, startedAt: uploadState.startTime, finishedAt: uploadState.finishedAt, albumAssignmentActive: uploadState.isCreatingAlbum, autoAlbums: uploadState.albumAutoMode, destination: uploadState.albumAutoMode ? 'auto-albums' : uploadState.albumName ? 'album' : 'library', threads: Array.from(uploadState.threads.values()).map(thread => ({ worker: thread.WorkerID, status: thread.Status, fileName: safeName(thread.FileName), attempt: thread.Attempt })) },
    results: { uploaded: uploadState.results.success.length, failed: uploadState.results.fail.length, cancelled: uploadState.results.cancelled.length, noFinalResult: Math.max(0, uploadState.totalFiles - uploadState.uploadedFiles), cancelledFiles: uploadState.results.cancelled.map(safeName), skipped: uploadState.results.skipped.length, warnings: uploadState.results.warnings.length, failureDetails: uploadState.failures.map(failure => ({ ...failure })), skippedDetails: uploadState.results.skipped.map(item => ({ code: item.code, files: item.paths.map(safeName) })) },
    folders: uploadState.autoAlbumFolders.map(folder => ({ name: safeName(folder), ...folderCounts(folder, uploadState.workPaths, uploadState.processedPaths) })),
    albums: uploadState.albumsUpdated.map(album => ({ name: safeName(album.AlbumName), itemsAdded: album.ItemsAdded, complete: album.IsComplete })),
    albumErrors: uploadState.albumErrors.map(error => ({ name: safeName(error.AlbumName), category: errorCategory(error.Error) })),
    privacy: 'Account addresses, credentials, full paths and raw server errors are omitted. File and album names are included. Error categories are inferred from error text.',
  }
  try {
    const path = await Dialogs.SaveFile({
      Title: 'Export gotohp debug log',
      Filename: `gotohp-debug-${new Date().toISOString().replace(/[:.]/g, '-')}.json`,
      CanCreateDirectories: true,
      Filters: [{ DisplayName: 'JSON log', Pattern: '*.json' }],
    })
    if (!path) return
    await ConfigManager.ExportDebugLog(path, JSON.stringify(report, null, 2))
    toast.success('Debug log exported')
  } catch (error) {
    toast.error('Could not export debug log', { description: error instanceof Error ? error.message : String(error) })
  } finally {
    isExportingLog.value = false
  }
}

let unsubscribeFilesDropped: (() => void) | undefined

onMounted(() => {
  document.addEventListener('dragenter', onDragEnter)
  document.addEventListener('dragleave', onDragLeave)
  document.addEventListener('dragover', onDragOver)
  document.addEventListener('drop', onDrop)
  window.addEventListener('albumError', albumErrorHandler)
  window.addEventListener('uploadError', uploadErrorHandler)

  // Listen for files-dropped event from backend
  unsubscribeFilesDropped = Events.On('files-dropped', (event: { data: { files: string[]; dropZone: string } }) => {
    const { files, dropZone } = event.data
    if (controlsBusy.value || isAccountSetupOpen.value || uploadState.isUploading || uploadState.completionVisible || isAlbumHistoryOpen.value || isUploadRecordsOpen.value || recordReviewOpen.value) return
    if (isRecentAlbumScreen.value) {
      if (selectedRecentAlbum.value) {
        uploadManager.setNextAlbumDisplayName(selectedRecentAlbum.value.albumName)
        void selection.startRecentAlbum(files, selectedRecentAlbum.value.albumId)
      } else {
        pendingRecentSources.value = files
        recentAlbumEntryMode.value = 'drag-drop'
      }
      return
    }
    if (dropZone === 'recent-album') {
      void openRecentAlbums('drag-drop', files)
      return
    }
    void selection.drop(files, dropZone)
  })
})

onUnmounted(() => {
  unsubscribeFilesDropped?.()
  document.removeEventListener('dragenter', onDragEnter)
  document.removeEventListener('dragleave', onDragLeave)
  document.removeEventListener('dragover', onDragOver)
  document.removeEventListener('drop', onDrop)
  window.removeEventListener('albumError', albumErrorHandler)
  window.removeEventListener('uploadError', uploadErrorHandler)
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
  }
})
</script>

<template>
  <main
    class="w-screen h-screen flex flex-col items-center"
    style="--wails-draggable: drag"
  >
    <!-- Drop zones shown when dragging files -->
    <div
      v-if="!uploadState.isUploading && !uploadState.completionVisible && isDraggingFiles && options.length > 0 && !showAlbumInput && !controlsBusy && !isAccountSetupOpen && !isAlbumHistoryOpen && !isUploadRecordsOpen && !isRecentAlbumScreen"
      class="grid h-screen w-screen grid-cols-2 gap-3 p-6"
      style="--wails-draggable: none"
    >
      <div
        v-for="option in uploadModes"
        :key="option.value"
        data-file-drop-target
        :data-drop-zone="option.value"
        class="flex flex-col items-center justify-center border-2 border-dashed rounded-xl transition-all duration-200 drop-zone p-3"
        :class="uploadMode === option.value ? 'border-primary bg-primary/10' : 'border-muted-foreground/50'"
      >
        <h2 class="text-xl font-semibold select-none text-muted-foreground">
          {{ option.title }}
        </h2>
        <p class="text-sm text-muted-foreground/70 mt-2 select-none text-center px-4">
          {{ option.description }}
        </p>
      </div>
      <div
        data-file-drop-target
        data-drop-zone="recent-album"
        class="flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-primary/60 p-3 drop-zone"
      >
        <h2 class="text-xl font-semibold select-none">Recent Album</h2>
        <p class="mt-2 text-center text-sm text-muted-foreground select-none">Drop here, then choose a saved album.</p>
      </div>
    </div>

    <!-- Normal UI (not dragging) -->
    <div
      v-else-if="!uploadState.isUploading && !uploadState.completionVisible"
      class="w-screen h-screen flex flex-col items-center gap-3 overflow-y-auto px-6 pb-20 md:max-w-3xl md:px-10"
      :class="options.length === 0 ? 'pt-30' : 'pt-6'"
      data-file-drop-target
    >
      <template v-if="options.length === 0">
        <div class="flex max-w-xs flex-col items-center gap-4 text-center">
          <div class="flex size-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
            <UserPlus class="size-5" />
          </div>
          <div class="flex flex-col gap-1">
            <h1 class="text-xl font-semibold select-none">
              Connect Google Photos
            </h1>
            <p class="text-sm text-muted-foreground select-none">
              Add an account before uploading photos and videos.
            </p>
          </div>
          <Button
            class="cursor-pointer select-none"
            @click="openAccountSetup"
          >
            Add Google account
          </Button>
        </div>
      </template>

      <template v-else>
        <!-- Show album input screen when files dropped on album zone -->
        <template v-if="recordReviewOpen">
          <section class="w-full max-w-2xl space-y-4 pt-8" style="--wails-draggable: none">
            <h1 class="text-xl font-semibold">Check local upload records</h1>
            <p v-if="recordReviewLoading" role="status">Checking selected files…</p>
            <p v-else-if="recordReviewError" role="alert" class="text-sm">{{ recordReviewError }}</p>
            <template v-else>
              <p>{{ recordReview.total }} items selected · {{ recordReview.matched }} recorded · {{ recordReview.total - recordReview.matched }} need normal upload checks.</p>
              <p class="text-sm text-muted-foreground">{{ recordReview.usesAlbums ? 'Recorded files will reuse saved media keys for the selected album destination. Only other files need normal upload checks.' : 'Recorded files will be skipped. No album will be updated.' }}</p>
              <p class="text-xs text-muted-foreground">Records only confirm past success on this PC. Files are rechecked when the upload starts; counts may change if you edit files.</p>
            </template>
            <div class="flex flex-wrap gap-2">
              <Button variant="outline" @click="finishRecordReview(null)">Cancel</Button>
              <Button v-if="!recordReviewLoading" variant="outline" @click="finishRecordReview(true)">Ignore local records this time</Button>
              <Button v-if="!recordReviewLoading && !recordReviewError" @click="finishRecordReview(false)">{{ recordReview.usesAlbums ? 'Continue and update albums' : recordReview.total === recordReview.matched ? 'Finish without uploading' : 'Upload unrecorded files' }}</Button>
            </div>
            <p v-if="!recordReviewLoading" class="text-xs text-muted-foreground">Ignoring local records still uses the uploader's normal remote duplicate check.</p>
          </section>
        </template>
        <template v-else-if="isUploadRecordsOpen">
          <UploadRecords :account="selectedOption" @back="isUploadRecordsOpen = false" />
        </template>
        <template v-else-if="showAlbumInput">
          <div
            class="flex w-full max-w-2xl flex-col items-center justify-center gap-6 pt-12"
            style="--wails-draggable: none"
          >
            <h1 class="text-xl font-semibold select-none">
              {{ uploadMode === 'auto-album' ? 'Review auto albums' : 'New album' }}
            </h1>
            <p class="text-muted-foreground select-none">
              {{ pendingFiles.length }} file(s) or folder(s) selected
            </p>
            
            <p class="text-xs text-muted-foreground break-all">{{ selectedOption }}</p>
            <div v-if="uploadMode === 'auto-album'" class="w-full space-y-2">
              <p class="text-sm">Create a new album for each file’s containing folder.</p>
              <ul class="max-h-40 overflow-y-auto rounded-lg border p-3 text-sm space-y-1">
                <li v-for="folder in sourceNames" :key="folder.path" class="break-all" :title="folder.path">{{ folder.name }}</li>
              </ul>
              <p class="text-xs text-muted-foreground">{{ sourceNames.length ? 'Planned album names shown above. Albums are created only for folders with eligible uploaded files. Matching names create new albums.' : 'No eligible folders found with the current settings.' }}</p>
            </div>
            <div v-else class="flex w-full max-w-md flex-col gap-2">
              <Label
                for="album-input"
                class="text-muted-foreground text-sm"
              >Album name</Label>
              <Input
                id="album-input"
                v-model="albumNameOrKey"
                placeholder="e.g. Summer trip"
                :disabled="selectionBusy"
                autofocus
                @keydown.enter.prevent="selection.confirmAlbum"
              />
              <p class="text-xs text-muted-foreground">A name creates a new album, even if that name exists. Use Recent Album to reuse one. Existing album keys are also accepted.</p>
            </div>

            <div class="flex gap-4">
              <Button
                variant="outline"
                class="cursor-pointer select-none"
                :disabled="selectionBusy"
                @click="selection.cancelAlbum"
              >
                Cancel
              </Button>
              <Button
                class="cursor-pointer select-none"
                :disabled="(uploadMode === 'album' && !albumNameOrKey.trim()) || selectionBusy"
                @click="selection.confirmAlbum"
              >
                {{ selectionBusy ? 'Starting…' : 'Upload' }}
              </Button>
            </div>
          </div>
        </template>

        <template v-else-if="isAlbumHistoryOpen">
          <AlbumHistory :account="selectedOption" @back="isAlbumHistoryOpen = false" />
        </template>

        <template v-else-if="isRecentAlbumScreen">
          <div class="flex w-full max-w-2xl flex-col items-center gap-4 pt-8" style="--wails-draggable: none">
            <div class="text-center">
              <h1 class="text-xl font-semibold select-none">Recent albums</h1>
              <p class="mt-1 text-xs text-muted-foreground break-all">{{ selectedOption }}</p>
              <p class="mt-1 text-sm text-muted-foreground">
                {{ recentAlbumEntryMode === 'drag-drop' ? `${pendingRecentSources.length} file(s) or folder(s) ready. Choose an album to start.` : 'Choose an album, then choose files or a folder.' }}
              </p>
            </div>

            <div v-if="!recentAlbums.length" class="w-full max-w-md rounded-lg border p-5 text-center">
              <p class="font-medium">No recent albums yet</p>
              <p class="mt-1 text-sm text-muted-foreground">Albums you successfully upload to will appear here for quick access.</p>
            </div>
            <div v-else class="w-full max-w-md space-y-2">
              <button
                v-for="album in recentAlbums"
                :key="album.albumId"
                type="button"
                class="flex w-full items-center justify-between gap-3 rounded-lg border p-3 text-left transition-colors"
                :class="selectedRecentAlbum?.albumId === album.albumId ? 'border-primary bg-primary/10' : 'hover:bg-muted/50'"
                :disabled="selectionBusy"
                @click="selectRecentAlbum(album)"
              >
                <span class="min-w-0 break-all text-sm font-medium">{{ album.albumName || 'Saved album' }}</span>
                <span class="shrink-0 text-xs text-muted-foreground">Use album</span>
              </button>
            </div>

            <div v-if="selectedRecentAlbum && recentAlbumEntryMode === 'normal'" class="grid w-full max-w-md grid-cols-2 gap-2">
              <Button :disabled="selectionBusy" @click="chooseForRecentAlbum('files')"><Files class="size-4" />Choose files</Button>
              <Button variant="outline" :disabled="selectionBusy" @click="chooseForRecentAlbum('folder')"><FolderOpen class="size-4" />Choose folder</Button>
            </div>
            <Button variant="outline" :disabled="selectionBusy" @click="closeRecentAlbums">Back to Home</Button>
          </div>
        </template>

        <!-- Normal UI when not dragging -->
        <template v-else>
          <h1 class="text-xl font-semibold select-none">
            Upload photos & videos
          </h1>
          <GoogleAccountSelect
            v-model="selectedOption"
            :options="options"
            :removing-account="removingAccount"
            :disabled="controlsBusy"
            @item-removed="removeCredentials"
            @add="openAccountSetup"
          />
          <div
            v-if="tokenBindingEmail"
            class="w-full max-w-xs border rounded-lg p-3 flex flex-col gap-3"
            style="--wails-draggable: none"
          >
            <p class="text-sm text-muted-foreground">
              This credential needs a token binding key from the rooted Android device it was captured from.
            </p>
            <Button
              class="cursor-pointer select-none"
              :disabled="controlsBusy"
              @click="addTokenBindingAliasFromADB"
            >
              {{ isExtractingTokenBinding ? 'Reading ADB...' : 'Read from ADB' }}
            </Button>
          </div>

          <fieldset
            class="w-full max-w-3xl"
            :disabled="controlsBusy"
            style="--wails-draggable: none"
          >
            <legend class="mb-2 text-xs font-medium text-muted-foreground">
              Upload mode
            </legend>
            <div class="grid grid-cols-2 gap-2">
              <label
                v-for="option in uploadModes"
                :key="option.value"
                class="flex items-center gap-3 rounded-lg border p-3 cursor-pointer transition-colors focus-within:ring-2 focus-within:ring-ring"
                :class="uploadMode === option.value ? 'border-primary bg-primary/10' : 'border-border hover:bg-muted/50'"
              >
                <input
                  v-model="uploadMode"
                  type="radio"
                  name="upload-mode"
                  :value="option.value"
                  class="size-4 shrink-0 accent-primary"
                >
                <span class="min-w-0">
                  <span class="block text-sm font-medium">{{ option.title }}</span>
                  <span class="block text-xs text-muted-foreground mt-0.5">{{ option.description }}</span>
                </span>
              </label>
              <button
                type="button"
                class="flex items-center gap-3 rounded-lg border border-primary/50 p-3 text-left transition-colors hover:bg-primary/10"
                :disabled="controlsBusy || !selectedOption"
                @click="openRecentAlbums('normal')"
              >
                <span class="size-4 shrink-0 rounded-full border-2 border-primary" />
                <span class="min-w-0">
                  <span class="block text-sm font-medium">Recent Album</span>
                  <span class="block text-xs text-muted-foreground mt-0.5">Quickly upload to a recently used album.</span>
                </span>
              </button>
            </div>
          </fieldset>

          <div
            class="w-full max-w-3xl flex flex-col gap-2"
            style="--wails-draggable: none"
          >
            <div class="grid grid-cols-2 gap-2 md:mx-auto md:w-full md:max-w-md">
              <Button
                :disabled="controlsBusy || !selectedOption"
                @click="selection.choose('files')"
              >
                <Files class="size-4" />
                Choose files
              </Button>
              <Button
                variant="outline"
                :disabled="controlsBusy || !selectedOption"
                @click="selection.choose('folder')"
              >
                <FolderOpen class="size-4" />
                Choose folder
              </Button>
            </div>
            <p
              class="text-xs text-muted-foreground text-center"
              role="status"
            >
              {{ selection.starting.value ? 'Starting upload…' : selectionBusy ? 'Preparing selection…' : uploadMode === 'album' ? 'Choose files or a folder, then name your album.' : uploadMode === 'auto-album' ? 'Choose files or folders, then review before uploading.' : 'Google Photos library · No album. Choosing files starts uploading.' }}
            </p>
            <p class="text-xs text-muted-foreground text-center">
              Subfolders follow Settings → Recursive Directory Upload.
              Drag & drop shows all four destinations. Drop onto the one you want.
            </p>
          </div>

          <Sheet v-model:open="isSettingsOpen">
            <SheetTrigger as-child>
              <Button
                variant="outline"
                class="cursor-pointer select-none"
                :disabled="controlsBusy"
              >
                Settings
              </Button>
            </SheetTrigger>
            <SheetContent side="bottom" class="max-h-screen overflow-y-auto pb-16">
              <TooltipProvider disable-hoverable-content>
                <SettingsPanel />
              </TooltipProvider>
              <DebugLogButton :busy="isExportingLog" @export="exportDebugLog" />
            </SheetContent>
          </Sheet>
          <Button
            variant="outline"
            class="cursor-pointer select-none"
            :disabled="controlsBusy"
            @click="isAlbumHistoryOpen = true"
          >
            Album History
          </Button>
          <Button variant="outline" class="cursor-pointer select-none" :disabled="controlsBusy || !selectedOption" @click="isUploadRecordsOpen = true">Upload Records</Button>

        </template>
      </template>
    </div>
    <div
      v-if="uploadState.completionVisible"
      class="w-full h-full"
    >
      <UploadComplete :account="selectedOption" />
    </div>
    <div
      v-else-if="uploadState.isUploading"
      class="w-full h-full"
    >
      <Upload :account="selectedOption" />
    </div>
    <GoogleAuthSetup
      v-model:open="isAccountSetupOpen"
      @account-added="refreshCredentials"
    >
      <DebugLogButton :busy="isExportingLog" @export="exportDebugLog" />
    </GoogleAuthSetup>
    <DebugLogButton v-if="!isSettingsOpen && !isAccountSetupOpen" :busy="isExportingLog" @export="exportDebugLog" />
    <Toaster
      position="bottom-center"
      :offset="64"
      :mobile-offset="64"
      rich-colors
      expand
      :visible-toasts="4"
    />
  </main>
</template>
