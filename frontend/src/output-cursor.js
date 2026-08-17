const validCursor = (value) => Number.isSafeInteger(value) && value >= 0

export const readCursorHeader = (response, name, fallback) => {
  const parsed = Number.parseInt(response.headers.get(name) || '', 10)
  return validCursor(parsed) ? parsed : fallback
}

export const readReplayCursor = async (response, fallback) => {
  let cursor = readCursorHeader(response, 'X-Terminal-Base-Cursor', fallback)
  try {
    const payload = await response.json()
    if (validCursor(payload?.baseCursor)) cursor = payload.baseCursor
  } catch {
    // Servers may supply cursor metadata only through response headers.
  }
  return cursor
}

export const advanceOutputCursor = (cursor, byteLength) => {
  if (!validCursor(cursor) || !validCursor(byteLength)) return cursor
  const next = cursor + byteLength
  return validCursor(next) ? next : cursor
}
