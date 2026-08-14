import assert from 'node:assert/strict'
import {
  ensureHerdrMouseCapture,
  herdrMouseCaptureSequence
} from './src/mouse-capture.js'

const writes = []
const inactiveTerminal = {
  modes: { mouseTrackingMode: 'none' },
  write: (value) => writes.push(value)
}

assert.equal(ensureHerdrMouseCapture(inactiveTerminal), true)
assert.deepEqual(writes, [herdrMouseCaptureSequence])
assert.match(herdrMouseCaptureSequence, /\x1b\[\?1003h/)
assert.match(herdrMouseCaptureSequence, /\x1b\[\?1006h/)

const activeWrites = []
const activeTerminal = {
  modes: { mouseTrackingMode: 'any' },
  write: (value) => activeWrites.push(value)
}

assert.equal(ensureHerdrMouseCapture(activeTerminal), false)
assert.deepEqual(activeWrites, [])

console.log('mouse capture checks passed')
