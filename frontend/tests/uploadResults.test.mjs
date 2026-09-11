import assert from 'node:assert/strict'
import { test } from 'node:test'
import { recordUploadResult } from '../src/utils/uploadResults.ts'

test('a local match with an album media key is skipped, not counted as a new upload', () => {
  const results = { success: [], fail: [], cancelled: [], skipped: [], warnings: [] }
  const processed = recordUploadResult(results, { Path: '/photo.jpg', Paths: ['/photo.jpg'], MediaKey: 'saved-key', IsError: false, Skipped: true, SkipCode: 'local-record', SkipReason: 'Previously uploaded' })
  assert.equal(processed, 1)
  assert.equal(results.success.length, 0)
  assert.equal(results.fail.length, 0)
  assert.equal(results.skipped[0].code, 'local-record')
})

test('confirmed cancellation is separate from genuine failures and successful uploads', () => {
  const results = { success: [], fail: [], cancelled: [], skipped: [], warnings: [] }
  const event = { Path: '/photo.jpg', Paths: [], MediaKey: '', IsError: true, Skipped: false, ErrorMessage: 'context canceled', SkipCode: '', SkipReason: '' }
  recordUploadResult(results, { ...event, Cancelled: true })
  recordUploadResult(results, { ...event, Path: '/other.jpg', ErrorMessage: 'connection reset', Cancelled: false })
  recordUploadResult(results, { ...event, Path: '/done.jpg', IsError: false, MediaKey: 'media' })
  assert.deepEqual(results.cancelled, ['/photo.jpg'])
  assert.equal(results.fail.length, 1)
  assert.equal(results.success.length, 1)
})

test('cancellation messages alone do not relabel genuine failures', () => {
  const results = { success: [], fail: [], cancelled: [], skipped: [], warnings: [] }
  recordUploadResult(results, { Path: 'cancelled holiday.jpg', IsError: true, ErrorMessage: 'bad file', Cancelled: false })
  assert.equal(results.fail.length, 1)
  assert.equal(results.cancelled.length, 0)
})
