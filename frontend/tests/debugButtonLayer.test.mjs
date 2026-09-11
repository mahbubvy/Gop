import assert from 'node:assert/strict'
import fs from 'node:fs'
import { test } from 'node:test'

test('debug export stays above notifications and is present inside modal focus traps', () => {
  const read = path => fs.readFileSync(new URL(path, import.meta.url), 'utf8')
  const button = read('../src/components/DebugLogButton.vue')
  const toaster = read('../src/components/ui/sonner/Sonner.vue')
  const app = read('../src/App.vue')
  assert.match(button, /fixed bottom-3 right-3 z-\[60\]/)
  assert.match(toaster, /zIndex: 40/)
  assert.match(app, /:offset="64"/)
  assert.match(app, /:mobile-offset="64"/)
  assert.match(app, /<SheetContent[^>]*>[\s\S]*?<DebugLogButton[^>]*[\s\S]*?<\/SheetContent>/)
  assert.match(app, /<GoogleAuthSetup[\s\S]*?<DebugLogButton[\s\S]*?<\/GoogleAuthSetup>/)
})
