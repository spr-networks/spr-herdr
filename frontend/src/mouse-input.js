const sgrMousePattern = /^\x1b\[<(\d+);\d+;\d+M$/

export const isSgrMouseMotion = (data) => {
  if (typeof data !== 'string') return false
  const match = sgrMousePattern.exec(data)
  return match !== null && (Number.parseInt(match[1], 10) & 32) !== 0
}

export const createMouseInputCoalescer = ({ write, delayMs = 16 }) => {
  let pendingMotion = null
  let timer = null

  const flush = () => {
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
    if (pendingMotion === null) return
    const motion = pendingMotion
    pendingMotion = null
    write(motion)
  }

  return {
    write(data) {
      if (!isSgrMouseMotion(data)) {
        flush()
        write(data)
        return
      }
      pendingMotion = data
      if (timer === null) {
        timer = setTimeout(flush, delayMs)
      }
    },
    flush
  }
}
