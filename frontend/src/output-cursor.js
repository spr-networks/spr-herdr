const validCursor = (value) => Number.isSafeInteger(value) && value >= 0

export const readCursorHeader = (response, name, fallback) => {
  const parsed = Number.parseInt(response.headers.get(name) || '', 10)
  return validCursor(parsed) ? parsed : fallback
}

export const readResetCursor = async (response, fallback) => {
  let cursor = readCursorHeader(response, 'X-Terminal-Next-Cursor', fallback)
  try {
    const payload = await response.json()
    if (validCursor(payload?.nextCursor)) cursor = payload.nextCursor
  } catch {
    // Older servers only supplied cursor metadata as response headers.
  }
  return cursor
}

export const advanceOutputCursor = (cursor, byteLength) => {
  if (!validCursor(cursor) || !validCursor(byteLength)) return cursor
  const next = cursor + byteLength
  return validCursor(next) ? next : cursor
}
