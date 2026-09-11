import assert from 'node:assert/strict'
import fs from 'node:fs'
import vm from 'node:vm'
import { test } from 'node:test'
import ts from 'typescript'
import { computed, ref, watch, effectScope } from 'vue'

test('home screen reactive setup initializes without a reference error', () => {
  const source = fs.readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8')
  const setup = source.slice(source.indexOf('const isDraggingFiles'), source.indexOf('watch(selectedOption'))
  assert.ok(setup.includes('watch(currentScreen'))
  const scope = effectScope()
  try {
    scope.run(() => {
      const result = vm.runInNewContext(ts.transpile(setup + '\ncurrentScreen.value', { target: ts.ScriptTarget.ES2022 }), {
        ref, computed, watch, uploadState: {},
        useUploadSelection: () => ({ mode: ref('regular'), busy: ref(false), pendingFiles: ref([]), albumName: ref(''), previewFolders: ref([]) }),
      })
      assert.equal(result, 'home')
    })
  } finally { scope.stop() }
})
