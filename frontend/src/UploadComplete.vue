<script setup lang="ts">
import { computed, ref } from 'vue'
import { CheckCircle2, ClipboardCheck, FolderPlus, Home, AlertTriangle } from '@lucide/vue'
import Button from './components/ui/button/Button.vue'
import { uploadManager } from './utils/UploadManager'
import { resultHeading, mediaLabel } from './utils/uploadPresentation'

defineProps<{ account: string }>()

const { state } = uploadManager
const copyButtonText = ref('Copy results as JSON')

const hasIssues = computed(() => state.results.fail.length > 0 || state.albumErrors.length > 0)
const heading = computed(() => resultHeading(state.cancelRequested, state.results.success.length, state.results.fail.length, state.albumErrors.length, state.results.skipped.length))
const usesAlbums = computed(() => state.albumAutoMode || !!state.albumName)
const unfinishedCount = computed(() => Math.max(0, state.totalFiles - state.uploadedFiles))
const locallySkipped = computed(() => state.results.skipped.filter(item => item.code === 'local-record').length)

async function copyResults() {
  const copied = await uploadManager.copyResultsAsJson()
  copyButtonText.value = copied ? 'Copied' : 'Copy failed'
  window.setTimeout(() => { copyButtonText.value = 'Copy results as JSON' }, 1600)
}
</script>

<template>
  <div class="flex h-full w-full flex-col items-center overflow-y-auto px-4 pt-6 pb-16 md:px-8" style="--wails-draggable: none">
    <div class="w-full max-w-2xl space-y-4">
      <section class="rounded-xl border bg-card p-5 text-center shadow-sm">
        <p class="mb-3 text-xs text-muted-foreground break-all">{{ account }}</p>
        <AlertTriangle v-if="hasIssues || state.cancelRequested" class="mx-auto mb-2 size-8 text-amber-500" />
        <CheckCircle2 v-else-if="state.results.success.length" class="mx-auto mb-2 size-8 text-emerald-500" />
        <h1 class="text-xl font-semibold">{{ heading }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ state.results.success.length }} {{ mediaLabel(state.workPaths) }} uploaded to your Google Photos library.</p>
        <p v-if="state.cancelRequested" class="mt-2 text-sm text-muted-foreground">Already uploaded files remain in your library. {{ usesAlbums ? 'Album assignment may be incomplete.' : '' }}</p>
        <p v-if="!state.totalFiles && !state.results.fail.length && !state.cancelRequested" class="mt-2 text-sm text-muted-foreground">No eligible files found. Check supported formats, excluded files and recursive folder settings.</p>
        <p v-if="!usesAlbums" class="mt-2 text-sm text-muted-foreground">No album selected.</p>
        <p class="mt-2 text-xs text-muted-foreground">{{ new Date(state.finishedAt).toLocaleString() }}</p>
        <div class="mt-4 grid grid-cols-2 gap-2 text-sm sm:grid-cols-4">
          <div class="rounded-md bg-muted/50 px-2 py-2"><strong class="block text-base">{{ state.results.success.length }}</strong><span class="text-muted-foreground">Uploaded</span></div>
          <div class="rounded-md bg-muted/50 px-2 py-2"><strong class="block text-base">{{ state.results.fail.length }}</strong><span class="text-muted-foreground">Failed</span></div>
          <div v-if="state.cancelRequested || state.results.cancelled.length" class="rounded-md bg-muted/50 px-2 py-2"><strong class="block text-base">{{ state.results.cancelled.length }}</strong><span class="text-muted-foreground">Cancelled</span></div>
          <div v-if="unfinishedCount" class="rounded-md bg-muted/50 px-2 py-2"><strong class="block text-base">{{ unfinishedCount }}</strong><span class="text-muted-foreground">No final result</span></div>
          <div v-if="locallySkipped" class="rounded-md bg-muted/50 px-2 py-2"><strong class="block text-base">{{ locallySkipped }}</strong><span class="text-muted-foreground">Locally recorded</span></div>
          <div class="rounded-md bg-muted/50 px-2 py-2"><strong class="block text-base">{{ state.results.skipped.length - locallySkipped }}</strong><span class="text-muted-foreground">Other skipped</span></div>
          <div class="rounded-md bg-muted/50 px-2 py-2"><strong class="block text-base">{{ state.results.warnings.length }}</strong><span class="text-muted-foreground">Warnings</span></div>
        </div>
      </section>

      <section v-if="usesAlbums" class="rounded-xl border p-4">
        <div class="mb-3 flex items-center gap-2">
          <FolderPlus class="size-4 text-primary" />
          <h2 class="font-semibold">Albums updated</h2>
        </div>
        <p v-if="!state.albumsUpdated.length" class="text-sm text-muted-foreground">No album updates were confirmed.</p>
        <ul v-else class="space-y-2">
          <li v-for="(album, index) in state.albumsUpdated" :key="`${album.AlbumName}-${index}`" class="flex items-center justify-between gap-3 rounded-lg bg-muted/50 px-3 py-2">
            <span class="min-w-0 break-all text-sm font-medium">{{ album.AlbumName }}</span>
            <span class="shrink-0 text-xs text-muted-foreground">{{ album.ItemsAdded }} {{ album.ItemsAdded === 1 ? 'file' : 'files' }} added</span>
          </li>
        </ul>
      </section>

      <section v-if="state.albumErrors.length" class="rounded-xl border border-amber-500/40 bg-amber-500/10 p-4">
        <div class="mb-2 flex items-center gap-2 text-amber-700 dark:text-amber-300"><AlertTriangle class="size-4" /><h2 class="font-semibold">Album updates needing attention</h2></div>
        <p class="mb-2 text-sm">Successfully uploaded files are already in your library, but some could not be added to an album.</p>
        <p v-for="(error, index) in state.albumErrors" :key="`${error.AlbumName}-${index}`" class="text-sm text-muted-foreground"><span class="font-medium text-foreground">{{ error.AlbumName || 'Album' }}:</span> {{ error.Error }}</p>
      </section>

      <details v-if="state.results.fail.length || state.results.skipped.length || state.results.cancelled.length" class="rounded-xl border p-4 text-sm">
        <summary class="cursor-pointer">Review failed, cancelled and skipped files</summary>
        <p v-for="(path, index) in state.results.cancelled" :key="`cancel-${index}`" class="mt-2 break-all">{{ path.split(/[\\/]/).pop() }}: Cancelled</p>
        <p v-for="(failure, index) in state.results.fail" :key="`failure-${index}`" class="mt-2 break-all">{{ failure }}</p>
        <p v-for="(skip, index) in state.results.skipped" :key="`skip-${index}`" class="mt-2 break-all">{{ skip.paths.map(path => path.split(/[\\/]/).pop()).join(', ') }}: {{ skip.reason }}</p>
      </details>

      <div class="flex flex-col gap-2 pb-2 sm:flex-row sm:justify-end">
        <Button variant="outline" @click="copyResults"><ClipboardCheck class="size-4" />{{ copyButtonText }}</Button>
        <Button @click="uploadManager.dismissCompletion()"><Home class="size-4" />Go back home</Button>
      </div>
    </div>
  </div>
</template>
