import assert from 'node:assert/strict'
import { test } from 'node:test'
import { errorCategory, safeName } from '../src/utils/debugLog.ts'

test('error categories explain cancellation and network problems without exposing raw text', () => {
  assert.equal(errorCategory('context canceled C:\\private\\a.jpg'), 'cancelled')
  assert.equal(errorCategory('dial tcp: connection reset https://private/?token=secret'), 'network')
  assert.equal(errorCategory('401 Authorization: Bearer secret'), 'authentication-or-permission')
  assert.equal(errorCategory('unrecognized secret payload'), 'other-error')
})

test('names omit directory prefixes, album keys and email addresses', () => {
  assert.equal(safeName('C:\\Users\\private\\photo.jpg'), 'photo.jpg')
  assert.equal(safeName('/home/private/movie.mp4'), 'movie.mp4')
  assert.equal(safeName('AF1QipSecretKey'), '[album key]')
  assert.equal(safeName('person@example.com'), '[email]')
})
