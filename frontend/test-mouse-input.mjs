import assert from 'node:assert/strict'
import {
  createMouseInputCoalescer,
  isSgrMouseMotion
} from './src/mouse-input.js'

assert.equal(isSgrMouseMotion('\x1b[<35;42;53M'), true)
assert.equal(isSgrMouseMotion('\x1b[<32;42;53M'), true)
assert.equal(isSgrMouseMotion('\x1b[<0;42;53M'), false)
assert.equal(isSgrMouseMotion('\x1b[<0;42;53m'), false)
assert.equal(isSgrMouseMotion('a'), false)

const output = []
const coalescer = createMouseInputCoalescer({
  delayMs: 60_000,
  write: (data) => output.push(data)
})
coalescer.write('\x1b[<35;1;1M')
coalescer.write('\x1b[<35;2;2M')
coalescer.flush()
assert.deepEqual(output, ['\x1b[<35;2;2M'])

coalescer.write('\x1b[<32;3;3M')
coalescer.write('\x1b[<0;3;3m')
assert.deepEqual(output.slice(1), ['\x1b[<32;3;3M', '\x1b[<0;3;3m'])

console.log('mouse input checks passed')
