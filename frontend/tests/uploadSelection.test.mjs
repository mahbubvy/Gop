import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ref, nextTick } from 'vue'
import { useUploadSelection } from '../src/utils/uploadSelection.ts'

function setup(overrides = {}) {
  const calls = []
  const errors = []
  const selection = useUploadSelection({
    getAccount: () => 'test@example.com',
    isUploading: () => false,
    openFile: async options => {
      calls.push(['picker', options])
      return ['/photos/one.jpg', '/photos/two.mp4']
    },
    setSelected: async account => { calls.push(['account', account]) },
    setAlbumName: async name => { calls.push(['album', name]) },
    setAlbumAutoMode: async auto => { calls.push(['auto', auto]) },
    startUpload: async files => { calls.push(['upload', files]) },
    onError: error => errors.push(error),
    ...overrides,
  })
  return { selection, calls, errors }
}

test('local record review cancellation does not dispatch or leave selection locked', async () => {
  const { selection, calls } = setup({ reviewRecords: async () => null })
  await selection.drop(['/photo.jpg'])
  assert.equal(calls.some(call => call[0] === 'upload'), false)
  assert.equal(selection.starting.value, false)
  assert.equal(selection.busy.value, false)
})

test('record review blocks duplicate starts and forwards a one-batch bypass', async () => {
  let decide
  const uploads = []
  const { selection } = setup({ reviewRecords: () => new Promise(resolve => { decide = resolve }), startUpload: async (files, bypass) => uploads.push({ files, bypass }) })
  const pending = selection.drop(['/photo.jpg'])
  await new Promise(resolve => setTimeout(resolve, 0))
  await selection.drop(['/second.jpg'])
  assert.equal(uploads.length, 0)
  decide(true)
  await pending
  assert.deepEqual(uploads, [{ files: ['/photo.jpg'], bypass: true }])
})

test('recent albums and auto albums both pass through local record review', async () => {
  for (const mode of ['recent', 'auto']) {
    let reviewed = 0
    const { selection, calls } = setup({ reviewRecords: async () => { reviewed++; return false } })
    if (mode === 'recent') await selection.startRecentAlbum(['/photo.jpg'], 'AF1QipExisting')
    else { selection.mode.value = 'auto-album'; await selection.drop(['/photo.jpg']) }
    assert.equal(reviewed, 1)
    assert.equal(calls.filter(call => call[0] === 'upload').length, 1)
  }
})

test('auto review previews folders before any upload, for both picker and drop', async () => {
  for (const entry of ['picker', 'drop']) {
    const { selection, calls } = setup({ reviewAuto: true, previewAuto: async () => ['/photos/trip', '/photos/family'] })
    selection.mode.value = 'auto-album'
    if (entry === 'picker') await selection.choose('folder')
    else await selection.drop(['/photos'])
    assert.equal(calls.some(call => call[0] === 'upload'), false)
    assert.deepEqual(selection.previewFolders.value, ['/photos/trip', '/photos/family'])
    await selection.confirmAlbum()
    assert.equal(calls.some(call => call[0] === 'upload'), true)
    assert.deepEqual(calls.find(call => call[0] === 'auto'), ['auto', true])
  }
})

test('no candidates prevents configuration writes and upload for regular and recent entry points', async () => {
  for (const entry of ['regular', 'recent']) {
    const { selection, calls, errors } = setup({ countCandidates: async () => 0 })
    if (entry === 'regular') await selection.drop(['/empty'])
    else await selection.startRecentAlbum(['/empty'], 'AF1QipTest')
    assert.deepEqual(calls, [])
    assert.match(errors[0].message, /No supported files/)
  }
})

test('a second start is blocked while waiting for the backend start event', async () => {
  const { selection, calls } = setup()
  await selection.drop(['/one.jpg'])
  await selection.drop(['/two.jpg'])
  assert.equal(selection.starting.value, true)
  assert.equal(calls.filter(call => call[0] === 'upload').length, 1)
})

test('an empty auto preview does not start an upload or show a confirmation', async () => {
  const { selection, calls, errors } = setup({ reviewAuto: true, previewAuto: async () => [] })
  await selection.drop(['/empty'], 'auto-album')
  assert.deepEqual(calls, [])
  assert.deepEqual(selection.pendingFiles.value, [])
  assert.match(errors[0].message, /No eligible album folders/)
})

test('files picker allows multiple files, applies regular mode before starting', async () => {
  const { selection, calls } = setup()
  await selection.choose('files')
  assert.equal(calls[0][1].CanChooseFiles, true)
  assert.equal(calls[0][1].CanChooseDirectories, false)
  assert.equal(calls[0][1].AllowsMultipleSelection, true)
  assert.deepEqual(calls.slice(1), [
    ['account', 'test@example.com'], ['album', ''], ['auto', false],
    ['upload', ['/photos/one.jpg', '/photos/two.mp4']],
  ])
  assert.equal(selection.busy.value, false)
})

test('folder picker passes the directory intact to existing scanning logic', async () => {
  let options
  const { selection, calls } = setup({ openFile: async value => { options = value; return '/photos/trip' } })
  selection.mode.value = 'auto-album'
  await selection.choose('folder')
  assert.equal(options.CanChooseFiles, false)
  assert.equal(options.CanChooseDirectories, true)
  assert.equal(options.AllowsMultipleSelection, false)
  assert.deepEqual(calls, [
    ['account', 'test@example.com'], ['album', ''], ['auto', true], ['upload', ['/photos/trip']],
  ])
})

for (const result of ['', [], null]) {
  test(`cancelled picker (${JSON.stringify(result)}) does not change config or upload`, async () => {
    const { selection, calls } = setup({ openFile: async () => result })
    await selection.choose('folder')
    assert.deepEqual(calls, [])
    assert.deepEqual(selection.pendingFiles.value, [])
    assert.equal(selection.busy.value, false)
  })
}

test('album selection waits for a nonblank name and confirmation', async () => {
  const { selection, calls } = setup()
  selection.mode.value = 'album'
  await selection.choose('files')
  assert.equal(calls.length, 1)
  selection.albumName.value = '   '
  await selection.confirmAlbum()
  assert.equal(calls.length, 1)
  selection.albumName.value = '  My trip  '
  await selection.confirmAlbum()
  assert.deepEqual(calls.slice(1), [
    ['account', 'test@example.com'], ['album', 'My trip'], ['auto', false],
    ['upload', ['/photos/one.jpg', '/photos/two.mp4']],
  ])
  assert.deepEqual(selection.pendingFiles.value, [])
  assert.equal(selection.albumName.value, '')
})

test('cancelling album confirmation leaves backend untouched', async () => {
  const { selection, calls } = setup()
  await selection.drop(['/photos'], 'album')
  selection.cancelAlbum()
  await selection.confirmAlbum()
  assert.deepEqual(calls, [])
  assert.deepEqual(selection.pendingFiles.value, [])
})

test('Linux drop without a zone honors home mode; explicit zones still override it', async () => {
  const uploading = ref(false)
  const { selection, calls } = setup({ isUploading: () => uploading.value })
  selection.mode.value = 'auto-album'
  await selection.drop(['/photos'], '')
  assert.deepEqual(calls[2], ['auto', true])
  uploading.value = true
  await nextTick()
  uploading.value = false
  await nextTick()
  await selection.drop(['/photos'], 'regular')
  assert.deepEqual(calls[6], ['auto', false])
})

test('picker locks other selections and captures mode/account before awaiting', async () => {
  let resolvePicker
  let account = 'first@example.com'
  const { selection, calls } = setup({
    getAccount: () => account,
    openFile: () => new Promise(resolve => { resolvePicker = resolve }),
  })
  const pending = selection.choose('files')
  assert.equal(selection.busy.value, true)
  selection.mode.value = 'auto-album'
  account = 'second@example.com'
  await selection.choose('folder')
  await selection.drop(['/unexpected'])
  resolvePicker(['/chosen.jpg'])
  await pending
  assert.deepEqual(calls, [
    ['account', 'first@example.com'], ['album', ''], ['auto', false], ['upload', ['/chosen.jpg']],
  ])
})

test('pending album selection cannot be replaced by a drop or picker', async () => {
  const { selection, calls } = setup()
  await selection.drop(['/original'], 'album')
  await selection.drop(['/replacement'], 'regular')
  await selection.choose('files')
  assert.deepEqual(selection.pendingFiles.value, ['/original'])
  assert.deepEqual(calls, [])
})

test('configuration failure prevents upload and preserves album selection for retry', async () => {
  const failure = new Error('Could not configure album')
  const { selection, calls, errors } = setup({ setAlbumAutoMode: async () => { throw failure } })
  await selection.drop(['/photos'], 'album')
  selection.albumName.value = 'Trip'
  await selection.confirmAlbum()
  assert.deepEqual(errors, [failure])
  assert.ok(!calls.some(([name]) => name === 'upload'))
  assert.deepEqual(selection.pendingFiles.value, ['/photos'])
  assert.equal(selection.busy.value, false)
})

test('configuration promises finish before upload is emitted', async () => {
  let resolveConfig
  const { selection, calls } = setup({
    setAlbumAutoMode: () => new Promise(resolve => { resolveConfig = resolve }),
  })
  const pending = selection.drop(['/photos'])
  await new Promise(resolve => setImmediate(resolve))
  assert.ok(!calls.some(([name]) => name === 'upload'))
  resolveConfig()
  await pending
  assert.deepEqual(calls.at(-1), ['upload', ['/photos']])
})

test('picker and start errors are surfaced and release busy state', async () => {
  for (const method of ['openFile', 'startUpload']) {
    const failure = new Error(method)
    const { selection, errors } = setup({ [method]: async () => { throw failure } })
    await selection.choose('files')
    assert.deepEqual(errors, [failure])
    assert.equal(selection.busy.value, false)
  }
})

test('no account or active upload blocks both entry points', async () => {
  for (const overrides of [{ getAccount: () => '' }, { isUploading: () => true }]) {
    const { selection, calls } = setup(overrides)
    await selection.choose('files')
    await selection.drop(['/photos'])
    assert.deepEqual(calls, [])
  }
})
