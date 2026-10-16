import assert from 'node:assert/strict'
import fs from 'node:fs'
import { test } from 'node:test'

test('pending local album name survives both uploadStart events until uploadStop', () => {
  const source = fs.readFileSync(new URL('../src/utils/UploadManager.ts', import.meta.url), 'utf8')
  const startHandler = source.slice(source.indexOf('Events.On("uploadStart"'), source.indexOf('Events.On("uploadStop"'))
  const stopHandler = source.slice(source.indexOf('Events.On("uploadStop"'), source.indexOf('Events.On("albumProgress"'))

  assert.doesNotMatch(startHandler, /pendingAlbumDisplayName\s*=\s*''/)
  assert.match(stopHandler, /pendingAlbumDisplayName\s*=\s*''/)
})
