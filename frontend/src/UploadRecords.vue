<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Dialogs } from '@wailsio/runtime'
import { ConfigManager } from '../bindings/app/backend'
import Button from './components/ui/button/Button.vue'

const props = defineProps<{ account: string }>()
const emit = defineEmits<{ back: [] }>()
const rows = ref<{ path: string; folder: string; size: number; recordedAt: string; origin: string }[]>([])
const search = ref('')
const appliedSearch = ref('')
const offset = ref(0)
const hasMore = ref(false)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const selected = ref<string[]>([])
const confirming = ref(false)
const recordUploads = ref(true)
const skipRecordedUploads = ref(false)
const ready = ref(false)
const bypassedBySafety = ref(false)
const groups = computed(() => {
  const groups = new Map<string, typeof rows.value>()
  for (const row of rows.value) groups.set(row.folder, [...(groups.get(row.folder) || []), row])
  return [...groups].map(([folder, files]) => ({ folder, files }))
})

async function load() {
  busy.value = true; error.value = ''; selected.value = []; confirming.value = false
  try {
    const page = await ConfigManager.GetUploadRecords(props.account, appliedSearch.value, offset.value)
    rows.value = page.rows; hasMore.value = page.hasMore
  } catch { error.value = 'Could not read local records. Nothing was removed. Try again.'; rows.value = [] }
  finally { busy.value = false }
}
async function initialize() {
  busy.value = true; error.value = ''
  try {
    const settings = await ConfigManager.GetSettings()
    recordUploads.value = settings.recordUploads
    skipRecordedUploads.value = settings.skipRecordedUploads
    bypassedBySafety.value = settings.forceUpload || settings.deleteFromHost
    ready.value = true
    await load()
  } catch { error.value = 'Could not load record settings. Try again.' }
  finally { busy.value = false }
}
async function toggle(kind: 'record' | 'skip') {
  busy.value = true; error.value = ''
  try {
    if (kind === 'record') { await ConfigManager.SetRecordUploads(!recordUploads.value); recordUploads.value = !recordUploads.value }
    else { await ConfigManager.SetSkipRecordedUploads(!skipRecordedUploads.value); skipRecordedUploads.value = !skipRecordedUploads.value }
  } catch { error.value = 'Could not save the setting. Please retry.' }
  finally { busy.value = false }
}
async function find() { offset.value = 0; appliedSearch.value = search.value; await load() }
async function page(direction: number) { offset.value = Math.max(0, offset.value + direction * 50); await load() }
async function forget() {
  busy.value = true; error.value = ''; notice.value = ''
  try {
    await ConfigManager.ForgetUploadRecords(props.account, [...selected.value])
    notice.value = 'Selected local records forgotten. Your files and Google Photos were not changed.'
    offset.value = 0; await load()
  } catch { error.value = 'Could not forget the selected records. Try again.' }
  finally { busy.value = false; confirming.value = false }
}
async function exportRecords() {
  busy.value = true; error.value = ''; notice.value = ''
  try {
    const path = await Dialogs.SaveFile({ Title: 'Export all upload records for this account', Filename: 'gotohp-upload-records.csv', Filters: [{ DisplayName: 'CSV', Pattern: '*.csv' }] })
    if (path) { await ConfigManager.ExportUploadRecords(props.account, path); notice.value = 'All records for this account exported, including full file paths.' }
  } catch { error.value = 'Export failed. Try a new filename or another location.' }
  finally { busy.value = false }
}
onMounted(initialize)
</script>

<template>
  <section class="w-full max-w-2xl space-y-4 pt-6 pb-16" style="--wails-draggable: none">
    <div class="flex items-center justify-between gap-3"><h1 class="text-xl font-semibold">Upload Records</h1><Button variant="outline" :disabled="busy" @click="emit('back')">Back</Button></div>
    <p class="break-all text-xs text-muted-foreground">{{ account }} · This PC only</p>
    <p class="text-sm text-muted-foreground">These records show past upload success, not whether files still exist in Google Photos. Renamed, moved or changed files are checked as new candidates.</p>
    <div class="rounded-xl border p-4 space-y-3">
      <label class="flex gap-3 text-sm"><input type="checkbox" :checked="recordUploads" :disabled="busy || !ready" @change="toggle('record')"><span>Keep local upload records<br><span class="text-xs text-muted-foreground">Turning this off keeps existing records.</span></span></label>
      <label class="flex gap-3 text-sm"><input type="checkbox" :checked="skipRecordedUploads" :disabled="busy || !ready" @change="toggle('skip')"><span>Skip files recorded as uploaded<br><span class="text-xs text-muted-foreground">Matches use account, full path, size and modified time. Existing records can still be used when recording is off.</span></span></label>
      <p v-if="bypassedBySafety" class="text-xs text-amber-500">Force Upload or Delete From Host is enabled in Settings, so local skipping is currently bypassed.</p>
    </div>
    <form class="flex gap-2" @submit.prevent="find"><input v-model="search" class="min-w-0 flex-1 rounded-md border bg-background px-3 py-2 text-sm" placeholder="Search file or folder path" aria-label="Search file or folder path" :disabled="busy"><Button :disabled="busy">Search</Button></form>
    <div class="flex flex-wrap gap-2"><Button variant="outline" :disabled="busy" @click="exportRecords">Export all as CSV</Button><Button variant="outline" :disabled="busy || !selected.length" @click="confirming = true">Forget selected ({{ selected.length }})</Button></div>
    <div v-if="confirming" role="alert" class="rounded-lg border p-3 space-y-3 text-sm"><p>Forget {{ selected.length }} local records? This does not delete any files or Google Photos media. These files may upload again.</p><Button :disabled="busy" @click="forget">Confirm forget</Button> <Button variant="outline" :disabled="busy" @click="confirming = false">Cancel</Button></div>
    <p v-if="notice" role="status" class="text-sm">{{ notice }}</p>
    <div v-if="error" role="alert" class="text-sm space-y-2"><p>{{ error }}</p><Button variant="outline" :disabled="busy" @click="initialize">Try again</Button></div>
    <p v-if="busy" role="status" class="text-sm text-muted-foreground">Working…</p>
    <p v-else-if="!rows.length && !error" class="text-sm text-muted-foreground">{{ appliedSearch ? 'No matching records.' : 'No recorded uploads for this account yet. Earlier uploads are not imported automatically.' }}</p>
    <div v-for="group in groups" :key="group.folder" class="rounded-xl border p-3 space-y-3">
      <h2 class="break-all text-sm font-semibold">{{ group.folder }}</h2><p class="text-xs text-muted-foreground">{{ group.files.length }} records on this page</p>
      <label v-for="row in group.files" :key="row.path" class="flex items-start gap-2 text-sm"><input v-model="selected" type="checkbox" :value="row.path" :disabled="busy" :aria-label="`Select ${row.path}`"><span class="min-w-0 break-all">{{ row.path.split(/[\\/]/).pop() }}<span class="block text-xs text-muted-foreground">{{ new Date(row.recordedAt).toLocaleString() }} · {{ (row.size / 1048576).toFixed(2) }} MB · {{ row.origin === 'remote-match' ? 'Already in library' : 'Uploaded' }}</span></span></label>
    </div>
    <div class="flex items-center justify-between gap-2"><Button variant="outline" :disabled="busy || offset === 0" @click="page(-1)">Previous</Button><span class="text-xs">Page {{ offset / 50 + 1 }} · up to 50 records</span><Button variant="outline" :disabled="busy || !hasMore || !!error" @click="page(1)">Next</Button></div>
  </section>
</template>
