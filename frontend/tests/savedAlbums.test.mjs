import assert from 'node:assert/strict'
import fs from 'node:fs'
import { test } from 'node:test'
import { parse } from '@vue/compiler-sfc'
import { sortSavedAlbums, savedAlbumsExport } from '../src/utils/savedAlbums.ts'

const albums = [
  { albumId: 'AF1Qip1', albumName: 'Zoo' },
  { albumId: 'AF1Qip2', albumName: 'family' },
  { albumId: 'AF1Qip3', albumName: 'Beach' },
]

test('added sort shows the most recently saved album first without mutating source order', () => {
  assert.deepEqual(sortSavedAlbums(albums, 'added'), [albums[2], albums[1], albums[0]])
  assert.deepEqual(albums.map(album => album.albumId), ['AF1Qip1', 'AF1Qip2', 'AF1Qip3'])
})

test('alphabetical sort is case-insensitive by display name', () => {
  assert.deepEqual(sortSavedAlbums(albums, 'alphabetical').map(album => album.albumName), ['Beach', 'family', 'Zoo'])
})

test('export includes readable names paired with the original album IDs', () => {
  assert.deepEqual(savedAlbumsExport(albums), {
    schemaVersion: 1,
    albums: [
      { albumId: 'AF1Qip1', albumName: 'Zoo' },
      { albumId: 'AF1Qip2', albumName: 'family' },
      { albumId: 'AF1Qip3', albumName: 'Beach' },
    ],
  })
})

test('Saved Albums exposes sort and export controls and highlights the full selected card', () => {
  const source = fs.readFileSync(new URL('../src/SavedAlbums.vue', import.meta.url), 'utf8')
  const { descriptor } = parse(source)
  assert.match(descriptor.template.content, /Recently added/)
  assert.match(descriptor.template.content, /Alphabetical/)
  assert.match(descriptor.template.content, /Export saved albums/)
  assert.match(descriptor.template.content, /aria-pressed="selectedAlbumId === album\.albumId"/)
  assert.match(descriptor.template.content, /selectedAlbumId === album\.albumId \? 'border-emerald-500 bg-emerald-500\/15 ring-1 ring-emerald-500\/40'/)
})
