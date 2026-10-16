<script setup lang="ts">
import { nextTick, ref, onMounted, watch } from 'vue'
import { Dialogs } from '@wailsio/runtime'
import { ConfigManager } from '../bindings/app/backend'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import Button from './components/ui/button/Button.vue'
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip'
import { Info } from '@lucide/vue'

import {
    NumberField,
    NumberFieldContent,
    NumberFieldDecrement,
    NumberFieldIncrement,
    NumberFieldInput,
} from '@/components/ui/number-field'

interface Settings {
    proxy: string
    useQuota: boolean
    saver: boolean
    recursive: boolean
    forceUpload: boolean
    pairLivePhotos: boolean
    skipIncompleteLivePhotos: boolean
    updateExistingPhotosToLive: boolean
    deleteFromHost: boolean
    disableUnsupportedFilesFilter: boolean
    setDateFromFilename: boolean
    uploadThreads: number
}

type BooleanSetting = Exclude<keyof Settings, 'proxy' | 'uploadThreads'>

const settings = ref<Settings>({
    proxy: '',
    useQuota: false,
    saver: false,
    recursive: false,
    forceUpload: false,
    pairLivePhotos: false,
    skipIncompleteLivePhotos: true,
    updateExistingPhotosToLive: false,
    deleteFromHost: false,
    disableUnsupportedFilesFilter: false,
    setDateFromFilename: false,
    uploadThreads: 0
})
const isHydrating = ref(true)
const backgroundMode = ref('default')
const backgroundColor = ref('#101214')
const backgroundBusy = ref(false)
const backgroundError = ref('')
const backgroundAccount = ref('')
const emit = defineEmits<{ backgroundChanged: [] }>()

const toggleSetting = (setting: BooleanSetting, enabled = true) => {
    if (!enabled) return
    settings.value[setting] = !settings.value[setting]
}

onMounted(async () => {
    try {
        const config = await ConfigManager.GetSettings()
        settings.value = {
            proxy: config.proxy || '',
            useQuota: config.useQuota || false,
            saver: config.saver || false,
            recursive: config.recursive || false,
            forceUpload: config.forceUpload || false,
            pairLivePhotos: config.pairLivePhotos || false,
            skipIncompleteLivePhotos: config.skipIncompleteLivePhotos ?? true,
            updateExistingPhotosToLive: config.updateExistingPhotosToLive || false,
            deleteFromHost: config.deleteFromHost || false,
            disableUnsupportedFilesFilter: config.disableUnsupportedFilesFilter || false,
            setDateFromFilename: config.setDateFromFilename || false,
            uploadThreads: config.uploadThreads || 1
        }
        backgroundMode.value = config.backgroundMode || 'default'
        backgroundColor.value = config.backgroundColor || '#101214'
        backgroundAccount.value = config.selected || ''
    } finally {
        await nextTick()
        isHydrating.value = false
    }
})

async function setBackgroundColor() {
    backgroundBusy.value = true
    backgroundError.value = ''
    try {
        await ConfigManager.SetAppBackgroundColor(backgroundColor.value)
        backgroundMode.value = 'color'
        emit('backgroundChanged')
    } catch {
        backgroundError.value = 'Could not save the background color.'
    } finally {
        backgroundBusy.value = false
    }
}

async function chooseBackgroundImage() {
    backgroundError.value = ''
    try {
        const path = await Dialogs.OpenFile({
            Title: 'Choose app background image',
            CanChooseFiles: true,
            CanChooseDirectories: false,
            AllowsMultipleSelection: false,
            Filters: [{ DisplayName: 'Image files (PNG, JPEG, GIF)', Pattern: '*.png;*.jpg;*.jpeg;*.gif' }],
        })
        if (!path || Array.isArray(path)) return
        backgroundBusy.value = true
        await ConfigManager.SetAppBackgroundImage(path)
        backgroundMode.value = 'image'
        emit('backgroundChanged')
    } catch {
        backgroundError.value = 'Could not save that image. Choose a valid PNG, JPEG, or GIF under 5 MiB.'
    } finally {
        backgroundBusy.value = false
    }
}

async function resetBackground() {
    backgroundBusy.value = true
    backgroundError.value = ''
    try {
        await ConfigManager.ResetAppBackground()
        backgroundMode.value = 'default'
        backgroundColor.value = '#101214'
        emit('backgroundChanged')
    } catch {
        backgroundError.value = 'Could not reset the background.'
    } finally {
        backgroundBusy.value = false
    }
}

// Watch for changes to proxy value and update backend
watch(() => settings.value.proxy, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetProxy(newValue)
})

// Create individual watchers for each boolean setting
watch(() => settings.value.useQuota, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetUseQuota(newValue)
})

watch(() => settings.value.saver, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetSaver(newValue)
})

watch(() => settings.value.recursive, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetRecursive(newValue)
})

watch(() => settings.value.forceUpload, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetForceUpload(newValue)
})

watch(() => settings.value.pairLivePhotos, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetPairLivePhotos(newValue)
    if (newValue && !settings.value.skipIncompleteLivePhotos) {
        settings.value.skipIncompleteLivePhotos = true
        await ConfigManager.SetSkipIncompleteLivePhotos(true)
    }
})

watch(() => settings.value.skipIncompleteLivePhotos, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetSkipIncompleteLivePhotos(newValue)
})

watch(() => settings.value.updateExistingPhotosToLive, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetUpdateExistingPhotosToLive(newValue)
})

watch(() => settings.value.deleteFromHost, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetDeleteFromHost(newValue)
})

watch(() => settings.value.disableUnsupportedFilesFilter, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetDisableUnsupportedFilesFilter(newValue)
})

watch(() => settings.value.setDateFromFilename, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetSetDateFromFilename(newValue)
})

watch(() => settings.value.uploadThreads, async (newValue) => {
    if (isHydrating.value) return
    if (newValue < 1) {
        settings.value.uploadThreads = 1
    } else {
        await ConfigManager.SetUploadThreads(newValue)
    }
})
</script>

<template>
  <div class="flex flex-col gap-2.5 m-4">
    <section class="space-y-3 rounded-xl border border-border p-4">
      <div>
        <h2 class="text-sm font-semibold">App background</h2>
        <p class="text-xs text-muted-foreground">{{ backgroundAccount ? `For ${backgroundAccount}` : 'No account selected · background fallback' }} · Saved on this PC. Images: PNG, JPEG, or GIF under 5 MiB.</p>
      </div>
      <div class="flex items-center gap-3">
        <input
          v-model="backgroundColor"
          type="color"
          aria-label="Choose app background color"
          class="size-10 cursor-pointer rounded border-0 bg-transparent p-0"
        >
        <Button variant="outline" :disabled="backgroundBusy" @click="setBackgroundColor">Use color</Button>
        <span v-if="backgroundMode === 'color'" class="text-xs text-muted-foreground">Active</span>
      </div>
      <div class="flex flex-wrap gap-2">
        <Button variant="outline" :disabled="backgroundBusy" @click="chooseBackgroundImage">Choose image</Button>
        <Button variant="ghost" :disabled="backgroundBusy || backgroundMode === 'default'" @click="resetBackground">Reset to default</Button>
        <span v-if="backgroundMode === 'image'" class="self-center text-xs text-muted-foreground">Image active</span>
      </div>
      <p v-if="backgroundError" role="alert" class="text-xs text-destructive">{{ backgroundError }}</p>
    </section>
    <NumberField
      v-model="settings.uploadThreads"
      class="flex items-center justify-between"
    >
      <Label
        for="upload-threads"
        class="size-full"
      >Upload Threads</Label>
      <NumberFieldContent>
        <NumberFieldDecrement
          class="cursor-pointer"
          :disabled="settings.uploadThreads <= 1"
        />
        <NumberFieldInput />
        <NumberFieldIncrement class="cursor-pointer" />
      </NumberFieldContent>
    </NumberField>
    <div class="flex items-center justify-between">
      <Label
        for="use-quota"
        class="size-full cursor-pointer"
      >Use Quota</Label>
      <Switch
        id="use-quota"
        v-model="settings.useQuota"
      />
    </div>
    <div class="flex items-center justify-between">
      <Label
        for="saver-mode"
        class="size-full cursor-pointer"
      >Storage Saver Quality</Label>
      <Switch
        id="saver-mode"
        v-model="settings.saver"
      />
    </div>
    <div class="flex items-center justify-between">
      <Label
        for="recursive"
        class="size-full cursor-pointer"
      >Recursive Directory Upload</Label>
      <Switch
        id="recursive"
        v-model="settings.recursive"
      />
    </div>
    <div class="flex items-center justify-between">
      <div
        class="flex min-w-0 flex-1 cursor-pointer items-center gap-1.5"
        @click.self="toggleSetting('forceUpload')"
      >
        <Label
          for="force-upload"
          class="cursor-pointer"
        >Force Upload</Label>
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              class="inline-flex size-5 shrink-0 cursor-help items-center justify-center rounded-full text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
              aria-label="About Force Upload"
            >
              <Info class="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent class="max-w-72">
            Applies to single files. Live Photo pairs always check both components for duplicates.
          </TooltipContent>
        </Tooltip>
      </div>
      <Switch
        id="force-upload"
        v-model="settings.forceUpload"
      />
    </div>
    <div class="flex items-center justify-between">
      <div
        class="flex min-w-0 flex-1 cursor-pointer items-center gap-1.5 pr-4"
        @click.self="toggleSetting('pairLivePhotos')"
      >
        <Label
          for="pair-live-photos"
          class="cursor-pointer"
        >Pair Apple Live Photos</Label>
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              class="inline-flex size-5 shrink-0 cursor-help items-center justify-center rounded-full text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
              aria-label="About Pair Apple Live Photos"
            >
              <Info class="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent class="max-w-72">
            Identify Apple Live Photo image and MOV pairs via embedded metadata and upload them as one item.
          </TooltipContent>
        </Tooltip>
      </div>
      <Switch
        id="pair-live-photos"
        v-model="settings.pairLivePhotos"
      />
    </div>
    <div
      class="flex items-center justify-between pl-4 transition-opacity"
      :class="settings.pairLivePhotos ? 'opacity-100' : 'opacity-45'"
    >
      <div
        class="flex min-w-0 flex-1 items-center gap-1.5 pr-4"
        :class="settings.pairLivePhotos ? 'cursor-pointer' : 'cursor-not-allowed'"
        @click.self="toggleSetting('skipIncompleteLivePhotos', settings.pairLivePhotos)"
      >
        <Label
          for="skip-incomplete-live-photos"
          :class="settings.pairLivePhotos ? 'cursor-pointer' : 'cursor-not-allowed'"
        >Skip Incomplete Live Photos</Label>
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              class="inline-flex size-5 shrink-0 cursor-help items-center justify-center rounded-full text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
              aria-label="About Skip Incomplete Live Photos"
            >
              <Info class="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent class="max-w-72">
            Only upload Live Photos when both photo and video are available. If either is missing,
            don't upload the remaining file separately.
          </TooltipContent>
        </Tooltip>
      </div>
      <Switch
        id="skip-incomplete-live-photos"
        v-model="settings.skipIncompleteLivePhotos"
        :disabled="!settings.pairLivePhotos"
      />
    </div>
    <div
      class="flex items-center justify-between pl-4 transition-opacity"
      :class="settings.pairLivePhotos ? 'opacity-100' : 'opacity-45'"
    >
      <div
        class="flex min-w-0 flex-1 items-center gap-1.5 pr-4"
        :class="settings.pairLivePhotos ? 'cursor-pointer' : 'cursor-not-allowed'"
        @click.self="toggleSetting('updateExistingPhotosToLive', settings.pairLivePhotos)"
      >
        <Label
          for="update-existing-photos-to-live"
          :class="settings.pairLivePhotos ? 'cursor-pointer' : 'cursor-not-allowed'"
        >Update Existing Photos to Live Photos</Label>
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              class="inline-flex size-5 shrink-0 cursor-help items-center justify-center rounded-full text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
              aria-label="About Update Existing Photos to Live Photos"
            >
              <Info class="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent class="max-w-72">
            If a static photo was already uploaded, re-upload it alongside its MOV companion to upgrade it to a Live Photo. Both local photo and video files are required.
          </TooltipContent>
        </Tooltip>
      </div>
      <Switch
        id="update-existing-photos-to-live"
        v-model="settings.updateExistingPhotosToLive"
        :disabled="!settings.pairLivePhotos"
      />
    </div>
    <div class="flex items-center justify-between">
      <Label
        for="filter-unsupported"
        class="size-full cursor-pointer"
      >Disable Unsupported Files Filter</Label>
      <Switch
        id="filter-unsupported"
        v-model="settings.disableUnsupportedFilesFilter"
      />
    </div>
    <div class="flex items-center justify-between">
      <Label
        for="set-date-from-filename"
        class="size-full cursor-pointer"
      >Set Upload Date from Filename</Label>
      <Switch
        id="set-date-from-filename"
        v-model="settings.setDateFromFilename"
      />
    </div>
    <div class="flex items-center justify-between">
      <Label
        for="delete-host"
        class="size-full cursor-pointer"
      >Delete From Host After Upload</Label>
      <Switch
        id="delete-host"
        v-model="settings.deleteFromHost"
        variant="destructive"
      />
    </div>
    <div>
      <Input
        v-model="settings.proxy"
        type="text"
        placeholder="Proxy URL (optional)"
      />
    </div>
  </div>
</template>
