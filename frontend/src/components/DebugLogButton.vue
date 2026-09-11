<script setup lang="ts">
import { Bug } from '@lucide/vue'
import Button from './ui/button/Button.vue'
defineProps<{ busy: boolean }>()
defineEmits<{ export: [] }>()
</script>

<template>
  <!-- Notifications use layer 40; sheets use 50. Keep modal instances inside
       their focus trap, with the debug control above warning cards. -->
  <Button
    variant="outline"
    size="icon"
    class="fixed bottom-3 right-3 z-[60] rounded-full shadow-lg"
    title="Export debug log (includes file and album names)"
    aria-label="Export debug log"
    :disabled="busy"
    :aria-busy="busy"
    @click="$emit('export')"
  >
    <Bug class="size-4" />
  </Button>
</template>
