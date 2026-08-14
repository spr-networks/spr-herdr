import assert from 'node:assert/strict'
import { createInputPump } from './src/input-pump.js'

const batches = []
const bounded = createInputPump({
  delayMs: 60_000,
  maxBatchBytes: 4,
  send: async (body) => batches.push([...body])
})
bounded.write(Uint8Array.from([1, 2, 3]))
bounded.write(Uint8Array.from([4, 5]))
await bounded.flush()
assert.deepEqual(batches, [[1, 2, 3, 4], [5]])

let releaseFirst
const firstPending = new Promise((resolve) => { releaseFirst = resolve })
const coalescedBatches = []
const coalesced = createInputPump({
  delayMs: 60_000,
  send: async (body) => {
    coalescedBatches.push([...body])
    if (coalescedBatches.length === 1) await firstPending
  }
})
coalesced.write(Uint8Array.from([1]))
const flushing = coalesced.flush()
await Promise.resolve()
coalesced.write(Uint8Array.from([2]))
coalesced.write(Uint8Array.from([3]))
releaseFirst()
await flushing
assert.deepEqual(coalescedBatches, [[1], [2, 3]])

console.log('input pump checks passed')
