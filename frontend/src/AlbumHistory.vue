<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ConfigManager } from '../bindings/app/backend'
import Button from './components/ui/button/Button.vue'

interface AlbumHistoryEntry { albumId: string; albumName: string; itemsAdded: number; operation: string; createdAt: number }
const entries = ref<AlbumHistoryEntry[]>([])
const loading = ref(true)
const error = ref('')
defineProps<{ account: string }>()
const emit = defineEmits<{ back: [] }>()

function formatTime(timestamp: number) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(timestamp * 1000))
}

async function loadHistory() {
  loading.value = true
  error.value = ''
  try { entries.value = await ConfigManager.GetAlbumHistory() }
  catch { error.value = 'Could not load album history. Please try again.' }
  finally { loading.value = false }
}
onMounted(loadHistory)
</script>

<template>
  <div class="flex w-full max-w-2xl flex-col gap-4 pt-8" style="--wails-draggable: none">
    <div class="flex items-center justify-between gap-3">
      <div><h1 class="text-xl font-semibold">Album history</h1><p class="text-sm text-muted-foreground">Albums created or updated by uploads.</p></div>
      <Button variant="outline" size="sm" @click="emit('back')">Back</Button>
    </div>
    <p class="text-xs text-muted-foreground break-all">{{ account }} · Last 40 album operations</p>
    <p v-if="loading" role="status" class="rounded-lg border p-4 text-sm text-muted-foreground">Loading history…</p>
    <div v-else-if="error" role="alert" class="rounded-lg border p-4 space-y-3">
      <p class="text-sm">{{ error }}</p>
      <Button variant="outline" @click="loadHistory">Try again</Button>
    </div>
    <p v-else-if="!entries.length" class="rounded-lg border p-4 text-sm text-muted-foreground">No album history yet.</p>
    <div v-else class="space-y-2">
      <div v-for="(entry, index) in entries" :key="`${entry.albumId}-${entry.createdAt}-${index}`" class="rounded-md border px-3 py-2.5">
        <p class="break-all text-sm font-medium">{{ entry.albumName || 'Saved album' }}</p>
        <p class="mt-1 text-xs text-muted-foreground">{{ entry.operation }} · {{ entry.itemsAdded }} {{ entry.itemsAdded === 1 ? 'file' : 'files' }} added</p>
        <time :datetime="new Date(entry.createdAt * 1000).toISOString()" class="mt-1 block text-xs text-muted-foreground">{{ formatTime(entry.createdAt) }}</time>
      </div>
    </div>
  </div>
</template>
