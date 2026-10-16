import assert from 'node:assert/strict'
import { test } from 'node:test'
import { findAlbumDisplayName, isAlbumIdentifier } from '../src/utils/albumDisplayName.ts'

test('recognizes the album identifier format accepted by the uploader', () => {
  assert.equal(isAlbumIdentifier('AF1QipAlbumKey'), true)
  assert.equal(isAlbumIdentifier('Summer trip'), false)
  assert.equal(isAlbumIdentifier('AF1Qip'), false)
})

test('prefills an account album name from recent albums before history', () => {
  assert.equal(findAlbumDisplayName('AF1QipOne', [
    { albumId: 'AF1QipOne', albumName: 'Recent name' },
  ], [
    { albumId: 'AF1QipOne', albumName: 'History name' },
  ]), 'Recent name')
})

test('falls back to the newest matching history entry and ignores other album IDs', () => {
  assert.equal(findAlbumDisplayName('AF1QipOne', [], [
    { albumId: 'AF1QipOther', albumName: 'Other name' },
    { albumId: 'AF1QipOne', albumName: 'Newest name' },
    { albumId: 'AF1QipOne', albumName: 'Older name' },
  ]), 'Newest name')
})

test('prefers the saved local name when recent or history labels differ', () => {
  assert.equal(findAlbumDisplayName('AF1QipOne', [
    { albumId: 'AF1QipOne', albumName: 'Recent name' },
  ], [
    { albumId: 'AF1QipOne', albumName: 'History name' },
  ], [
    { albumId: 'AF1QipOne', albumName: 'Saved name' },
  ]), 'Saved name')
})
