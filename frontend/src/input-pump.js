const defaultMaxBatchBytes = 60 * 1024

export const createInputPump = ({
  send,
  onError = () => {},
  delayMs = 8,
  maxBatchBytes = defaultMaxBatchBytes
}) => {
  let chunks = []
  let pendingBytes = 0
  let timer = null
  let drainPromise = null

  const takeBatch = () => {
    const size = Math.min(pendingBytes, maxBatchBytes)
    const body = new Uint8Array(size)
    let offset = 0

    while (offset < size) {
      const chunk = chunks[0]
      const take = Math.min(chunk.length, size - offset)
      body.set(chunk.subarray(0, take), offset)
      offset += take
      pendingBytes -= take
      if (take === chunk.length) {
        chunks.shift()
      } else {
        chunks[0] = chunk.subarray(take)
      }
    }
    return body
  }

  const schedule = () => {
    if (timer !== null || drainPromise !== null || pendingBytes === 0) return
    timer = setTimeout(() => {
      timer = null
      void drain()
    }, delayMs)
  }

  const drain = () => {
    if (drainPromise !== null) return drainPromise
    if (pendingBytes === 0) return Promise.resolve()

    drainPromise = (async () => {
      try {
        while (pendingBytes > 0) {
          await send(takeBatch())
        }
      } catch (error) {
        onError(error)
      } finally {
        drainPromise = null
        schedule()
      }
    })()
    return drainPromise
  }

  return {
    write(bytes) {
      if (!(bytes instanceof Uint8Array) || bytes.length === 0) return
      chunks.push(bytes)
      pendingBytes += bytes.length
      schedule()
    },
    async flush() {
      if (timer !== null) {
        clearTimeout(timer)
        timer = null
      }
      await drain()
    }
  }
}
