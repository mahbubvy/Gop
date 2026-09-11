import { ref, watch } from 'vue'
import type { Dialogs } from '@wailsio/runtime'

export type UploadMode = 'regular' | 'album' | 'auto-album'

export const uploadModes: { value: UploadMode; title: string; description: string }[] = [
  { value: 'regular', title: 'Upload Only', description: 'Upload without adding to an album.' },
  { value: 'album', title: 'New album', description: 'Choose files, then name the album.' },
  { value: 'auto-album', title: 'Auto Album', description: 'Create albums using each file’s folder name.' },
]

interface UploadSelectionDependencies {
  getAccount: () => string
  isUploading: () => boolean
  openFile: (options: Dialogs.OpenFileDialogOptions) => Promise<string | string[] | null>
  setSelected: (account: string) => Promise<unknown>
  setAlbumName: (name: string) => Promise<unknown>
  setAlbumAutoMode: (auto: boolean) => Promise<unknown>
  startUpload: (files: string[], bypassLocalRecords?: boolean) => Promise<unknown>
  reviewRecords?: (files: string[]) => Promise<boolean | null>
  onError: (error: unknown) => void
  reviewAuto?: boolean
  previewAuto?: (paths: string[]) => Promise<string[]>
  countCandidates?: (paths: string[]) => Promise<number>
}

// Pickers and drops share the same configuration/start sequence. Only paths are
// passed to the existing uploader; scanning and upload settings stay there.
export function useUploadSelection(deps: UploadSelectionDependencies) {
  const mode = ref<UploadMode>('regular')
  const busy = ref(false)
  const starting = ref(false)
  watch(deps.isUploading, uploading => { if (uploading) starting.value = false })
  const pendingFiles = ref<string[]>([])
  const albumName = ref('')
  const previewFolders = ref<string[]>([])
  let pendingAccount = ''

  function cancelAlbum() {
    if (busy.value) return
    pendingFiles.value = []
    pendingAccount = ''
    albumName.value = ''
    previewFolders.value = []
  }

  async function start(files: string[], selectedMode: UploadMode, account: string) {
    await startWithAlbum(files, selectedMode, account, selectedMode === 'album' ? albumName.value.trim() : '')
  }

  async function startWithAlbum(files: string[], selectedMode: UploadMode, account: string, selectedAlbumName: string) {
    await deps.setSelected(account)
    await deps.setAlbumName(selectedMode === 'album' ? selectedAlbumName : '')
    await deps.setAlbumAutoMode(selectedMode === 'auto-album')
    const bypass = deps.reviewRecords ? await deps.reviewRecords(files) : false
    if (bypass === null) return
    starting.value = true
    try {
      await deps.startUpload(files, bypass)
      if (deps.isUploading()) starting.value = false
    } catch (error) {
      starting.value = false
      throw error
    }
  }

  async function validateSources(files: string[]) {
    if (deps.countCandidates && await deps.countCandidates(files) === 0) {
      throw new Error('No supported files found. Check the selected folder, file formats, excluded files and Recursive Directory Upload in Settings.')
    }
  }

  async function accept(files: string[], selectedMode: UploadMode, account: string) {
    if (!files.length) return
    await validateSources(files)
    mode.value = selectedMode
    if (selectedMode === 'auto-album' && deps.reviewAuto) {
      previewFolders.value = await deps.previewAuto?.(files) || []
      if (deps.previewAuto && !previewFolders.value.length) throw new Error('No eligible album folders found. Check file formats and Live Photo settings.')
    }
    if (selectedMode === 'album' || (selectedMode === 'auto-album' && deps.reviewAuto)) {
      pendingFiles.value = files
      pendingAccount = account
      albumName.value = ''
    } else {
      await start(files, selectedMode, account)
    }
  }

  function canSelect() {
    return !busy.value && !starting.value && !deps.isUploading() && !pendingFiles.value.length && !!deps.getAccount()
  }

  async function choose(kind: 'files' | 'folder') {
    if (!canSelect()) return
    const selectedMode = mode.value
    const account = deps.getAccount()
    busy.value = true
    try {
      const selection = await deps.openFile({
        Title: kind === 'folder' ? 'Choose folder to upload' : 'Choose files to upload',
        CanChooseFiles: kind === 'files',
        CanChooseDirectories: kind === 'folder',
        AllowsMultipleSelection: kind === 'files',
        ButtonText: selectedMode === 'album' ? 'Choose' : 'Upload',
      })
      const files = (Array.isArray(selection) ? selection : [selection]).filter((path): path is string => !!path)
      await accept(files, selectedMode, account)
    } catch (error) {
      deps.onError(error)
    } finally {
      busy.value = false
    }
  }

  async function drop(files: string[], dropZone?: string) {
    if (!canSelect()) return
    const selectedMode = uploadModes.find(option => option.value === dropZone)?.value ?? mode.value
    busy.value = true
    try {
      await accept(files, selectedMode, deps.getAccount())
    } catch (error) {
      deps.onError(error)
    } finally {
      busy.value = false
    }
  }

  async function confirmAlbum() {
    if (busy.value || starting.value || deps.isUploading() || !pendingFiles.value.length || (mode.value === 'album' && !albumName.value.trim())) return
    busy.value = true
    try {
      await start([...pendingFiles.value], mode.value, pendingAccount)
      pendingFiles.value = []
      pendingAccount = ''
      albumName.value = ''
    } catch (error) {
      deps.onError(error)
    } finally {
      busy.value = false
    }
  }

  async function chooseRecentAlbum(kind: 'files' | 'folder', albumKey: string) {
    if (!canSelect() || !albumKey.trim()) return
    const account = deps.getAccount()
    busy.value = true
    try {
      const selection = await deps.openFile({
        Title: kind === 'folder' ? 'Choose folder to upload' : 'Choose files to upload',
        CanChooseFiles: kind === 'files',
        CanChooseDirectories: kind === 'folder',
        AllowsMultipleSelection: kind === 'files',
        ButtonText: 'Upload',
      })
      const files = (Array.isArray(selection) ? selection : [selection]).filter((path): path is string => !!path)
      if (files.length) {
        await validateSources(files)
        await startWithAlbum(files, 'album', account, albumKey)
      }
    } catch (error) {
      deps.onError(error)
    } finally {
      busy.value = false
    }
  }

  async function startRecentAlbum(files: string[], albumKey: string) {
    if (!canSelect() || !files.length || !albumKey.trim()) return
    const account = deps.getAccount()
    busy.value = true
    try {
      await validateSources(files)
      await startWithAlbum(files, 'album', account, albumKey)
    } catch (error) {
      deps.onError(error)
    } finally {
      busy.value = false
    }
  }

  return { mode, busy, starting, pendingFiles, albumName, previewFolders, choose, drop, confirmAlbum, cancelAlbum, chooseRecentAlbum, startRecentAlbum }
}
