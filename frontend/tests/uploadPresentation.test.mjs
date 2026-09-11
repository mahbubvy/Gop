import assert from 'node:assert/strict'
import { test } from 'node:test'
import { mediaLabel, resultHeading, folderCounts, recordAlbumProgress } from '../src/utils/uploadPresentation.ts'

test('results distinguish cancellation, failure, partial success, skips and album errors', () => {
  assert.equal(resultHeading(true, 2, 0, 0, 0), 'Upload cancelled')
  assert.equal(resultHeading(false, 0, 2, 0, 0), 'Upload failed')
  assert.equal(resultHeading(false, 1, 2, 0, 0), 'Upload finished with issues')
  assert.equal(resultHeading(false, 2, 0, 1, 0), 'Upload finished with issues')
  assert.equal(resultHeading(false, 0, 0, 0, 3), 'Files skipped')
  assert.equal(resultHeading(false, 0, 0, 0, 0), 'No files uploaded')
  assert.equal(resultHeading(false, 3, 0, 0, 0), 'Upload complete')
})

test('folder progress separates equal names and nested folders', () => {
  const paths = ['C:\\a\\trip\\a.jpg', 'C:\\b\\trip\\b.jpg', 'C:\\a\\trip\\nested\\c.jpg']
  assert.deepEqual(folderCounts('C:\\a\\trip', paths, [paths[0], paths[1]]), {total: 1, processed: 1})
  assert.deepEqual(folderCounts('C:\\a\\trip\\nested', paths, [paths[0]]), {total: 1, processed: 0})
})

test('media headings handle photo, video, mixed and unknown selections', () => {
  assert.equal(mediaLabel(['a.JPG', 'b.heic']), 'photos')
  assert.equal(mediaLabel(['a.mp4']), 'videos')
  assert.equal(mediaLabel(['a.jpg', 'b.mp4']), 'files')
  assert.equal(mediaLabel([]), 'files')
})

test('album results retain partial additions and separate split album counts', () => {
  const albums = []
  const event = {AlbumKeys: ['first'], AlbumName: 'Trip (1)', ItemsAdded: 500, TotalItems: 20500, IsComplete: false}
  recordAlbumProgress(albums, event)
  assert.equal(albums[0].ItemsAdded, 500)
  recordAlbumProgress(albums, {...event, ItemsAdded: 20000})
  recordAlbumProgress(albums, {...event, AlbumKeys: ['first', 'second'], AlbumName: 'Trip (2)', ItemsAdded: 20500})
  assert.deepEqual(albums.map(album => [album.AlbumName, album.ItemsAdded]), [['Trip (1)', 20000], ['Trip (2)', 500]])
})
