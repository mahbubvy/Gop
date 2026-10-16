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
import SavedAlbums from './SavedAlbums.vue'
import UploadRecords from './UploadRecords.vue'
import { uploadManager } from './utils/UploadManager'
import { uploadModes, useUploadSelection } from './utils/uploadSelection'
import { findAlbumDisplayName, isAlbumIdentifier } from './utils/albumDisplayName'
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

interface SavedAlbum {
  albumId: string
  albumName: string
}

type RecentAlbumEntryMode = 'normal' | 'drag-drop'

// Drag state for the three drop zones
const isDraggingFiles = ref(false)

const selectedOption = ref('')
const options = ref<string[]>([])
const accountAvatars = ref<Record<string, string>>({})
const isManagingAvatar = ref(false)
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
const isSavedAlbumScreen = ref(false)
const savedAlbums = ref<SavedAlbum[]>([])
const selectedSavedAlbum = ref<SavedAlbum | null>(null)
const savedAlbumEntryMode = ref<RecentAlbumEntryMode>('normal')
const pendingSavedSources = ref<string[]>([])
const isSavingAlbum = ref(false)
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
const appBackgroundMode = ref('default')
const appBackgroundColor = ref('#101214')
const appBackgroundImage = ref('')
const appBackgroundStyle = computed(() => {
  if (appBackgroundMode.value === 'image' && appBackgroundImage.value) {
    return {
      backgroundColor: '#101214',
      backgroundImage: `linear-gradient(rgba(8, 10, 12, 0.56), rgba(8, 10, 12, 0.56)), url("${appBackgroundImage.value}")`,
      backgroundPosition: 'center',
      backgroundSize: 'cover',
    }
  }
  if (appBackgroundMode.value === 'color') {
    return {
      backgroundColor: appBackgroundColor.value,
      backgroundImage: 'linear-gradient(rgba(8, 10, 12, 0.48), rgba(8, 10, 12, 0.48))',
    }
  }
  return {}
})
const currentScreen = computed(() => isAccountSetupOpen.value ? 'account-setup' : isSettingsOpen.value ? 'settings' : uploadState.isUploading ? 'uploading' : uploadState.completionVisible ? 'upload-results' : recordReviewOpen.value ? 'record-review' : isUploadRecordsOpen.value ? 'upload-records' : isAlbumHistoryOpen.value ? 'album-history' : isSavedAlbumScreen.value ? 'saved-albums' : isRecentAlbumScreen.value ? 'recent-albums' : showAlbumInput.value ? 'selection-review' : 'home')

async function refreshAppBackground() {
  try {
    const config = await ConfigManager.GetSettings()
    appBackgroundMode.value = config.backgroundMode || 'default'
    appBackgroundColor.value = config.backgroundColor || '#101214'
    appBackgroundImage.value = appBackgroundMode.value === 'image'
      ? await ConfigManager.GetAppBackgroundImageDataURL()
      : ''
  } catch (error) {
    console.warn('Could not load saved app background:', error)
  }
}

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
  startUpload: (files, bypassLocalRecords, displayName) => {
    uploadManager.setNextAlbumDisplayName(isRecentAlbumScreen.value ? selectedRecentAlbum.value?.albumName || '' : isSavedAlbumScreen.value ? selectedSavedAlbum.value?.albumName || '' : displayName || '')
    return Events.Emit('startUpload', { files, bypassLocalRecords: !!bypassLocalRecords })
  },
  onError: error => toast.error('Could not prepare upload', {
    description: error instanceof Error ? error.message : String(error),
  }),
})
const { mode: uploadMode, busy: selectionBusy, pendingFiles, albumName: albumNameOrKey, albumDisplayName } = selection
const albumIdentifier = computed(() => isAlbumIdentifier(albumNameOrKey.value.trim()))
const albumDisplayNameEdited = ref(false)
let albumNameLookupTimer: ReturnType<typeof setTimeout> | undefined
let albumNameLookupGeneration = 0
const showAlbumInput = computed(() => pendingFiles.value.length > 0)
const controlsBusy = computed(() => selectionBusy.value || selection.starting.value || !!removingAccount.value || isExtractingTokenBinding.value || isLoadingRecentAlbums.value || isManagingAvatar.value)
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
watch([albumNameOrKey, selectedOption, uploadMode], ([rawIdentifier]) => {
  if (albumNameLookupTimer) clearTimeout(albumNameLookupTimer)
  const generation = ++albumNameLookupGeneration
  albumDisplayNameEdited.value = false
  albumDisplayName.value = ''
  const albumID = rawIdentifier.trim()
  const account = selectedOption.value
  if (uploadMode.value !== 'album' || !account || !isAlbumIdentifier(albumID)) return

  albumNameLookupTimer = setTimeout(async () => {
    try {
      await ConfigManager.SetSelected(account)
      if (generation !== albumNameLookupGeneration || selectedOption.value !== account) return
      const [recent, history, saved] = await Promise.all([ConfigManager.GetRecentAlbums(), ConfigManager.GetAlbumHistory(), ConfigManager.GetSavedAlbums()])
      if (generation !== albumNameLookupGeneration || albumNameOrKey.value.trim() !== albumID || albumDisplayNameEdited.value) return
      albumDisplayName.value = findAlbumDisplayName(albumID, recent, history, saved)
    } catch (error) {
      console.warn('Could not look up the saved local album name:', error)
    }
  }, 250)
})
watch(() => uploadState.completionVisible, visible => {
  if (visible) {
    isRecentAlbumScreen.value = false
    selectedRecentAlbum.value = null
    pendingRecentSources.value = []
    isSavedAlbumScreen.value = false
    selectedSavedAlbum.value = null
    pendingSavedSources.value = []
  }
})

watch(selectedOption, async (newValue) => {
  isUploadRecordsOpen.value = false
  if (newValue) {
    closeRecentAlbums()
    closeSavedAlbums()
    try {
      await ConfigManager.SetSelected(newValue)
      await updateTokenBindingPrompt(newValue)
      await refreshAppBackground()
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
    const nextAccountAvatars: Record<string, string> = {}
    const nextOptions = state.accounts.map(account => {
      nextNeedsTokenBinding[account.email] = account.needsTokenBinding
      if (account.avatarDataURL) nextAccountAvatars[account.email] = account.avatarDataURL
      return account.email
    })

    accountNeedsTokenBinding.value = nextNeedsTokenBinding
    accountAvatars.value = nextAccountAvatars
    options.value = nextOptions
    selectedOption.value = state.selected || ''
    if (state.selected) await ConfigManager.SetSelected(state.selected)
    if (state.selected) {
      await updateTokenBindingPrompt(state.selected)
    } else {
      tokenBindingEmail.value = ''
    }
    await refreshAppBackground()
  } catch (error) {
    console.error('Failed to refresh Google accounts:', error)
    toast.error('Could not refresh Google accounts', {
      description: error instanceof Error ? error.message : String(error),
    })
  }
}

async function setAccountAvatar() {
  const email = selectedOption.value
  if (!email || isManagingAvatar.value) return

  isManagingAvatar.value = true
  try {
    const path = await Dialogs.OpenFile({
      Title: `Choose avatar for ${email}`,
      CanChooseFiles: true,
      CanChooseDirectories: false,
      AllowsMultipleSelection: false,
      Filters: [{ DisplayName: 'Image files (PNG, JPEG, GIF)', Pattern: '*.png;*.jpg;*.jpeg;*.gif' }],
    })
    if (!path || Array.isArray(path)) return

    await ConfigManager.SetAccountAvatar(email, path)
    await refreshCredentials()
    toast.success('Account avatar saved on this PC.')
  } catch (error) {
    toast.error('Could not save account avatar', {
      description: error instanceof Error ? error.message : String(error),
    })
  } finally {
    isManagingAvatar.value = false
  }
}

async function clearAccountAvatar() {
  const email = selectedOption.value
  if (!email || isManagingAvatar.value) return

  isManagingAvatar.value = true
  try {
    await ConfigManager.ClearAccountAvatar(email)
    await refreshCredentials()
    toast.success('Account avatar removed.')
  } catch (error) {
    toast.error('Could not remove account avatar', {
      description: error instanceof Error ? error.message : String(error),
    })
  } finally {
    isManagingAvatar.value = false
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
    await refreshCredentials()
    const avatarCleanupFailed = error instanceof Error && error.message.startsWith('account was removed, but its local avatar could not be deleted:')
    toast.error(avatarCleanupFailed ? 'Account removed, but its local avatar could not be deleted.' : 'Failed to remove credentials.', {
      description: error instanceof Error ? error.message : String(error),
    })
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
    const [recent, saved] = await Promise.all([ConfigManager.GetRecentAlbums(), ConfigManager.GetSavedAlbums()])
    recentAlbums.value = recent
    savedAlbums.value = saved
    isRecentAlbumScreen.value = true
  } catch (error) {
    console.error('Failed to load recent albums:', error)
    toast.error('Could not load recent albums')
    pendingRecentSources.value = []
  } finally {
    isLoadingRecentAlbums.value = false
  }
}

async function openSavedAlbums(entryMode: RecentAlbumEntryMode = 'normal', sources: string[] = []) {
  if (controlsBusy.value || !selectedOption.value || entryMode === 'drag-drop' && !sources.length) return
  closeRecentAlbums()
  selectedSavedAlbum.value = null
  savedAlbumEntryMode.value = entryMode
  pendingSavedSources.value = [...sources]
  isSavedAlbumScreen.value = true
}

function closeRecentAlbums() {
  if (selectionBusy.value) return
  isRecentAlbumScreen.value = false
  selectedRecentAlbum.value = null
  pendingRecentSources.value = []
}

function closeSavedAlbums() {
  if (selectionBusy.value) return
  isSavedAlbumScreen.value = false
  selectedSavedAlbum.value = null
  pendingSavedSources.value = []
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

async function saveAlbumShortcut(albumId: string, albumName: string) {
  await ConfigManager.SetSelected(selectedOption.value)
  await ConfigManager.SaveSavedAlbum(albumId, albumName)
  savedAlbums.value = await ConfigManager.GetSavedAlbums()
  toast.success('Album saved on this PC.')
}

async function saveAlbumIdentifier() {
  if (isSavingAlbum.value || !albumIdentifier.value || !albumNameOrKey.value.trim() || !albumDisplayName.value.trim()) return
  isSavingAlbum.value = true
  try {
    await saveAlbumShortcut(albumNameOrKey.value.trim(), albumDisplayName.value.trim())
  } catch (error) {
    toast.error('Could not save this album locally.', { description: error instanceof Error ? error.message : String(error) })
  } finally {
    isSavingAlbum.value = false
  }
}

async function saveRecentAlbum(album: RecentAlbum) {
  if (isSavingAlbum.value || savedAlbums.value.some(item => item.albumId === album.albumId)) return
  isSavingAlbum.value = true
  try {
    await saveAlbumShortcut(album.albumId, album.albumName || 'Saved album')
  } catch (error) {
    toast.error('Could not save this album locally.', { description: error instanceof Error ? error.message : String(error) })
  } finally {
    isSavingAlbum.value = false
  }
}

async function selectSavedAlbum(album: SavedAlbum) {
  selectedSavedAlbum.value = album
  if (savedAlbumEntryMode.value !== 'drag-drop' || !pendingSavedSources.value.length) return
  uploadManager.setNextAlbumDisplayName(album.albumName)
  await selection.startRecentAlbum(pendingSavedSources.value, album.albumId)
  if (uploadState.isUploading) closeSavedAlbums()
}

async function chooseForSavedAlbum(kind: 'files' | 'folder', album: SavedAlbum) {
  selectedSavedAlbum.value = album
  uploadManager.setNextAlbumDisplayName(album.albumName)
  await selection.chooseRecentAlbum(kind, album.albumId)
  if (uploadState.isUploading) closeSavedAlbums()
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
    if (isSavedAlbumScreen.value) {
      if (selectedSavedAlbum.value) {
        uploadManager.setNextAlbumDisplayName(selectedSavedAlbum.value.albumName)
        void selection.startRecentAlbum(files, selectedSavedAlbum.value.albumId)
      } else {
        pendingSavedSources.value = files
        savedAlbumEntryMode.value = 'drag-drop'
      }
      return
    }
    if (dropZone === 'recent-album') {
      void openRecentAlbums('drag-drop', files)
      return
    }
    if (dropZone === 'saved-album') {
      void openSavedAlbums('drag-drop', files)
      return
    }
    void selection.drop(files, dropZone)
  })
})

onUnmounted(() => {
  if (albumNameLookupTimer) clearTimeout(albumNameLookupTimer)
  albumNameLookupGeneration++
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
    :style="[{ '--wails-draggable': 'drag' }, appBackgroundStyle]"
  >
    <!-- Drop zones shown when dragging files -->
    <div
      v-if="!uploadState.isUploading && !uploadState.completionVisible && isDraggingFiles && options.length > 0 && !showAlbumInput && !controlsBusy && !isAccountSetupOpen && !isAlbumHistoryOpen && !isUploadRecordsOpen && !isRecentAlbumScreen && !isSavedAlbumScreen"
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
      <div
        data-file-drop-target
        data-drop-zone="saved-album"
        class="col-span-2 flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-primary/60 p-3 drop-zone"
      >
        <h2 class="text-xl font-semibold select-none">Saved Albums</h2>
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
              >{{ albumIdentifier ? 'Album identifier' : 'Album name' }}</Label>
              <Input
                id="album-input"
                v-model="albumNameOrKey"
                :placeholder="albumIdentifier ? 'AF1Qip…' : 'e.g. Summer trip'"
                :disabled="selectionBusy"
                autofocus
                @keydown.enter.prevent="selection.confirmAlbum"
              />
              <template v-if="albumIdentifier">
                <Label for="album-display-name" class="text-muted-foreground text-sm">Display name</Label>
                <Input
                  id="album-display-name"
                  v-model="albumDisplayName"
                  placeholder="e.g. Family photos"
                  :disabled="selectionBusy"
                  autocomplete="off"
                  @update:model-value="albumDisplayNameEdited = true"
                  @keydown.enter.prevent="selection.confirmAlbum"
                />
                <p class="text-xs text-muted-foreground">This name is saved on this computer for this account. It only labels the album in this app; Google Photos keeps its current title.</p>
                <Button
                  variant="outline"
                  class="self-start"
                  :disabled="selectionBusy || isSavingAlbum || !albumDisplayName.trim()"
                  @click="saveAlbumIdentifier"
                >
                  {{ isSavingAlbum ? 'Saving…' : 'Save to Saved Albums' }}
                </Button>
              </template>
              <p v-else class="text-xs text-muted-foreground">A name creates a new album, even if that name exists. Use Recent Album to reuse one.</p>
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
                :disabled="(uploadMode === 'album' && (!albumNameOrKey.trim() || (albumIdentifier && !albumDisplayName.trim()))) || selectionBusy"
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

        <template v-else-if="isSavedAlbumScreen">
          <SavedAlbums
            :account="selectedOption"
            :selected-album-id="selectedSavedAlbum?.albumId || ''"
            :drag-drop="savedAlbumEntryMode === 'drag-drop'"
            :busy="controlsBusy"
            @back="closeSavedAlbums"
            @select="selectSavedAlbum"
            @choose="chooseForSavedAlbum"
          />
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
              <div v-for="album in recentAlbums" :key="album.albumId" class="flex items-center gap-2 rounded-lg border p-2">
                <button
                  type="button"
                  class="flex min-w-0 flex-1 items-center justify-between gap-3 rounded px-1 py-1 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  :class="selectedRecentAlbum?.albumId === album.albumId ? 'text-primary' : ''"
                  :disabled="selectionBusy"
                  @click="selectRecentAlbum(album)"
                >
                  <span class="min-w-0 break-all text-sm font-medium">{{ savedAlbums.find(item => item.albumId === album.albumId)?.albumName || album.albumName || 'Saved album' }}</span>
                  <span class="shrink-0 text-xs text-muted-foreground">Use</span>
                </button>
                <Button
                  v-if="!savedAlbums.some(item => item.albumId === album.albumId)"
                  variant="outline"
                  size="sm"
                  :disabled="selectionBusy || isSavingAlbum"
                  @click="saveRecentAlbum(album)"
                >Save</Button>
                <span v-else class="px-2 text-xs text-muted-foreground">Saved</span>
              </div>
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
            :avatars="accountAvatars"
            :removing-account="removingAccount"
            :disabled="controlsBusy"
            @item-removed="removeCredentials"
            @set-avatar="setAccountAvatar"
            @remove-avatar="clearAccountAvatar"
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
            class="mt-3 w-full max-w-3xl"
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
                <SettingsPanel @background-changed="refreshAppBackground" />
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
          <Button variant="outline" class="cursor-pointer select-none" :disabled="controlsBusy || !selectedOption" @click="openSavedAlbums()">Saved Albums</Button>
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
