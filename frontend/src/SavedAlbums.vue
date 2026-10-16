<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Dialogs } from '@wailsio/runtime'
import { ConfigManager } from '../bindings/app/backend'
import Button from './components/ui/button/Button.vue'
import { Input } from './components/ui/input'
import { savedAlbumsExport, sortSavedAlbums, type SavedAlbum, type SavedAlbumSort } from './utils/savedAlbums'

defineProps<{
  account: string
  selectedAlbumId: string
  dragDrop: boolean
  busy: boolean
}>()

const emit = defineEmits<{
  back: []
  select: [album: SavedAlbum]
  choose: [kind: 'files' | 'folder', album: SavedAlbum]
}>()

const albums = ref<SavedAlbum[]>([])
const sortBy = ref<SavedAlbumSort>('added')
const sortedAlbums = computed(() => sortSavedAlbums(albums.value, sortBy.value))
const selected = ref<SavedAlbum | null>(null)
const newAlbumId = ref('')
const newAlbumName = ref('')
const editingId = ref('')
const editedName = ref('')
const removingId = ref('')
const loading = ref(false)
const saving = ref(false)
const exporting = ref(false)
const error = ref('')
const notice = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    albums.value = await ConfigManager.GetSavedAlbums()
  } catch {
    albums.value = []
    error.value = 'Could not load saved albums. Try again.'
  } finally {
    loading.value = false
  }
}

async function add() {
  const albumId = newAlbumId.value.trim()
  const albumName = newAlbumName.value.trim()
  if (!albumId || !albumName || saving.value) return
  saving.value = true
  error.value = ''
  try {
    await ConfigManager.SaveSavedAlbum(albumId, albumName)
    albums.value = await ConfigManager.GetSavedAlbums()
    newAlbumId.value = ''
    newAlbumName.value = ''
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Could not save this album. Check the ID and try again.'
  } finally {
    saving.value = false
  }
}

function select(album: SavedAlbum) {
  selected.value = album
  emit('select', album)
}

function beginRename(album: SavedAlbum) {
  editingId.value = album.albumId
  editedName.value = album.albumName
  removingId.value = ''
}

async function rename(album: SavedAlbum) {
  const name = editedName.value.trim()
  if (!name || saving.value) return
  saving.value = true
  error.value = ''
  try {
    await ConfigManager.SaveSavedAlbum(album.albumId, name)
    album.albumName = name
    if (selected.value?.albumId === album.albumId) selected.value = album
    editingId.value = ''
  } catch {
    error.value = 'Could not save this display name. Try again.'
  } finally {
    saving.value = false
  }
}

async function remove(album: SavedAlbum) {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    await ConfigManager.RemoveSavedAlbum(album.albumId)
    albums.value = albums.value.filter(item => item.albumId !== album.albumId)
    if (selected.value?.albumId === album.albumId) selected.value = null
    removingId.value = ''
  } catch {
    error.value = 'Could not remove this saved shortcut. The Google Photos album was not changed.'
  } finally {
    saving.value = false
  }
}

async function exportAlbums() {
  if (exporting.value || !albums.value.length) return
  exporting.value = true
  error.value = ''
  notice.value = ''
  try {
    const path = await Dialogs.SaveFile({
      Title: 'Export saved albums',
      Filename: 'gotohp-saved-albums.json',
      CanCreateDirectories: true,
      Filters: [{ DisplayName: 'JSON', Pattern: '*.json' }],
    })
    if (!path) return
    const data = { exportedAt: new Date().toISOString(), ...savedAlbumsExport(sortedAlbums.value) }
    await ConfigManager.ExportDebugLog(path, JSON.stringify(data, null, 2))
    notice.value = 'Saved album names and identifiers exported.'
  } catch {
    error.value = 'Could not export saved albums. Try another location or filename.'
  } finally {
    exporting.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="w-full max-w-2xl space-y-4 pt-6 pb-16" style="--wails-draggable: none">
    <div class="flex items-center justify-between gap-3">
      <h1 class="text-xl font-semibold">Saved Albums</h1>
      <Button variant="outline" :disabled="busy || saving" @click="emit('back')">Back to Home</Button>
    </div>
    <p class="break-all text-xs text-muted-foreground">{{ account }} · Saved on this PC</p>
    <p class="text-sm text-muted-foreground">Saved albums stay here until you remove them. Their names are local labels; uploads use the album ID.</p>
    <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
    <p
      v-if="notice"
      role="status"
      class="text-sm text-emerald-500"
    >
      {{ notice }}
    </p>
    <form class="space-y-2 rounded-lg border p-3" @submit.prevent="add">
      <label for="saved-album-id" class="block text-sm font-medium">Album identifier</label>
      <Input id="saved-album-id" v-model="newAlbumId" placeholder="AF1Qip…" :disabled="saving || busy" autocomplete="off" />
      <label for="saved-album-name" class="block text-sm font-medium">Display name</label>
      <Input id="saved-album-name" v-model="newAlbumName" placeholder="e.g. Family photos" :disabled="saving || busy" autocomplete="off" />
      <Button type="submit" :disabled="saving || busy || !newAlbumId.trim() || !newAlbumName.trim()">Save album</Button>
    </form>
    <p v-if="loading" role="status" class="text-sm text-muted-foreground">Loading saved albums…</p>
    <p v-else-if="!albums.length && !error" role="status" class="rounded-lg border p-4 text-sm text-muted-foreground">
      No saved albums yet. Add an album ID above, or save one from Recent Albums.
    </p>

    <div
      v-if="albums.length"
      class="flex flex-wrap items-end justify-between gap-2"
    >
      <label for="saved-album-sort" class="grid gap-1 text-sm">
        Sort by
        <select
          id="saved-album-sort"
          v-model="sortBy"
          class="h-9 rounded-md border bg-background px-3 text-sm"
          :disabled="loading || busy || saving || exporting"
        >
          <option value="added">Recently added</option>
          <option value="alphabetical">Alphabetical</option>
        </select>
      </label>
      <Button
        variant="outline"
        :disabled="loading || busy || saving || exporting"
        @click="exportAlbums"
      >
        Export saved albums
      </Button>
    </div>

    <div
      v-for="album in sortedAlbums"
      :key="album.albumId"
      class="space-y-2 rounded-lg border p-3 transition-colors"
      :class="selectedAlbumId === album.albumId ? 'border-emerald-500 bg-emerald-500/15 ring-1 ring-emerald-500/40' : 'border-border'"
    >
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="min-w-0 flex-1 rounded px-1 py-1 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          :aria-pressed="selectedAlbumId === album.albumId"
          :disabled="busy || saving"
          @click="select(album)"
        >
          <span class="block break-all text-sm font-medium">{{ album.albumName }}</span>
          <span class="block break-all text-xs text-muted-foreground">{{ album.albumId }}</span>
        </button>
        <Button variant="outline" size="sm" :disabled="busy || saving || exporting" @click="beginRename(album)">Rename</Button>
        <Button variant="outline" size="sm" :disabled="busy || saving || exporting" @click="removingId = album.albumId; editingId = ''">Remove</Button>
      </div>

      <div v-if="editingId === album.albumId" class="flex flex-wrap gap-2">
        <Input v-model="editedName" :aria-label="`Display name for ${album.albumId}`" :disabled="saving" class="min-w-40 flex-1" @keydown.enter.prevent="rename(album)" />
        <Button :disabled="saving || !editedName.trim()" @click="rename(album)">Save name</Button>
        <Button variant="outline" :disabled="saving" @click="editingId = ''">Cancel</Button>
      </div>

      <div v-if="removingId === album.albumId" role="alert" class="space-y-2 text-sm">
        <p>Remove this shortcut from this PC? The Google Photos album will not be deleted.</p>
        <Button variant="destructive" :disabled="saving" @click="remove(album)">Remove shortcut</Button>
        <Button variant="outline" :disabled="saving" @click="removingId = ''">Cancel</Button>
      </div>
    </div>

    <div v-if="selected && !dragDrop" class="grid w-full max-w-md grid-cols-2 gap-2">
      <Button :disabled="busy || saving" @click="emit('choose', 'files', selected)">Choose files</Button>
      <Button variant="outline" :disabled="busy || saving" @click="emit('choose', 'folder', selected)">Choose folder</Button>
    </div>
    <p v-if="dragDrop && albums.length" class="text-xs text-center text-muted-foreground">Drop the files onto an album above to start uploading.</p>
  </section>
</template>
