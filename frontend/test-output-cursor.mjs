import assert from 'node:assert/strict'
import {
  advanceOutputCursor,
  readCursorHeader,
  readResetCursor
} from './src/output-cursor.js'

const normal = new Response(new Uint8Array([1, 2, 3]), { status: 200 })
assert.equal(readCursorHeader(normal, 'X-Terminal-Next-Cursor', 10), 10)
assert.equal(advanceOutputCursor(10, 3), 13)

// SPR's sandbox fetch bridge may preserve status/body without exposing custom
// response headers. Recovery must still escape the stale cursor instead of
// repeatedly requesting cursor zero and restarting the terminal.
const strippedHeaders = new Response(JSON.stringify({
  reset: 'required',
  baseCursor: 3,
  nextCursor: 8
}), { status: 409 })
assert.equal(await readResetCursor(strippedHeaders, 0), 8)

const legacyHeaders = new Response('terminal replay cursor expired', {
  status: 409,
  headers: { 'X-Terminal-Next-Cursor': '21' }
})
assert.equal(await readResetCursor(legacyHeaders, 0), 21)

console.log('output cursor checks passed')
