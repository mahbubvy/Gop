import assert from 'node:assert/strict'
import fs from 'node:fs'
import { test } from 'node:test'
import { parse } from '@vue/compiler-sfc'
import { parse as parseTemplate } from '@vue/compiler-dom'

test('record folders use native disclosures closed by default with files inside', () => {
  const source = fs.readFileSync(new URL('../src/UploadRecords.vue', import.meta.url), 'utf8')
  const { descriptor } = parse(source)
  const tree = parseTemplate(descriptor.template.content)
  const elements = []
  function visit(node) {
    if (node.type === 1) elements.push(node)
    for (const child of node.children || []) visit(child)
  }
  visit(tree)
  const folder = elements.find(node => node.tag === 'details')
  assert.ok(folder)
  assert.equal(folder.props.some(prop => prop.name === 'open' || prop.arg?.content === 'open'), false)
  const summary = folder.children.find(node => node.tag === 'summary')
  assert.ok(summary, 'native summary provides mouse and keyboard toggling')
  assert.match(summary.loc.source, /group.folder/)
  assert.match(summary.loc.source, /group.files.length/)
  assert.ok(folder.children.some(node => node.tag === 'label'), 'file selection remains inside the disclosure')
})
