import assert from 'node:assert/strict'
import {
  advanceOutputCursor,
  readCursorHeader,
  readReplayCursor
} from './src/output-cursor.js'

const normal = new Response(new Uint8Array([1, 2, 3]), { status: 200 })
assert.equal(readCursorHeader(normal, 'X-Terminal-Next-Cursor', 10), 10)
assert.equal(advanceOutputCursor(10, 3), 13)

// SPR's sandbox fetch bridge may preserve status/body without exposing custom
// response headers. Recovery must still escape the stale cursor instead of
// repeatedly requesting cursor zero. Recovery starts at the oldest retained
// output so it can rebuild terminal state without restarting Herdr.
const strippedHeaders = new Response(JSON.stringify({
  reset: 'required',
  baseCursor: 3,
  nextCursor: 8
}), { status: 409 })
assert.equal(await readReplayCursor(strippedHeaders, 0), 3)

const legacyHeaders = new Response('terminal replay cursor expired', {
  status: 409,
  headers: { 'X-Terminal-Base-Cursor': '13' }
})
assert.equal(await readReplayCursor(legacyHeaders, 0), 13)

console.log('output cursor checks passed')
