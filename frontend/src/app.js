import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { createInputPump } from './input-pump.js'
import { createMouseInputCoalescer } from './mouse-input.js'
import { advanceOutputCursor, readResetCursor } from './output-cursor.js'
import { resolvePluginBase } from './plugin-base.js'
import { ensureHerdrMouseCapture } from './mouse-capture.js'
import './app.css'

const statusElement = document.getElementById('status')
const reconnectButton = document.getElementById('reconnect')
const errorElement = document.getElementById('error')
const terminalElement = document.getElementById('terminal')

const encoder = new TextEncoder()
let cursor = 0
let stopped = false
let resizeTimer = null
let errorTimer = null

const pluginBase = resolvePluginBase({
  apiURL: window.SPR_API_URL,
  pluginURI: window.SPR_PLUGIN?.URI,
  baseURI: document.baseURI
})

const endpoint = (path) => new URL(path, pluginBase).toString()

const setStatus = (state, label) => {
  statusElement.dataset.state = state
  statusElement.textContent = label
}

const showError = (message) => {
  errorElement.textContent = message
  errorElement.hidden = false
  clearTimeout(errorTimer)
  errorTimer = setTimeout(() => {
    errorElement.hidden = true
  }, 7000)
}

const tokyoNight = Object.freeze({
  background: '#1a1b26',
  foreground: '#a9b1d6',
  cursor: '#c0caf5',
  cursorAccent: '#1a1b26',
  selectionBackground: '#292e42',
  black: '#1a1b26',
  red: '#f7768e',
  green: '#9ece6a',
  yellow: '#e0af68',
  blue: '#7aa2f7',
  magenta: '#ad8ee6',
  cyan: '#449dab',
  white: '#a9b1d6',
  brightBlack: '#414868',
  brightRed: '#ff7a93',
  brightGreen: '#b9f27c',
  brightYellow: '#ff9e64',
  brightBlue: '#7da6ff',
  brightMagenta: '#bb9af7',
  brightCyan: '#0db9d7',
  brightWhite: '#c0caf5'
})

const terminalFontFamily = '"JetBrainsMono Nerd Font", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace'

const waitForTerminalFont = async () => {
  if (!document.fonts) return
  await Promise.race([
    Promise.all([
      document.fonts.load('400 13px "JetBrainsMono Nerd Font"'),
      document.fonts.load('700 13px "JetBrainsMono Nerd Font"')
    ]),
    new Promise((resolve) => setTimeout(resolve, 1500))
  ])
}

const applyShellTheme = () => {
  const root = document.documentElement
  root.style.colorScheme = 'dark'
  root.style.setProperty('--surface', tokyoNight.background)
  root.style.setProperty('--surface-raised', '#13141c')
  root.style.setProperty('--border', tokyoNight.selectionBackground)
  root.style.setProperty('--text', tokyoNight.foreground)
  root.style.setProperty('--muted', '#565f89')
  root.style.setProperty('--accent', tokyoNight.blue)
}

const terminal = new Terminal({
  allowProposedApi: false,
  convertEol: false,
  cursorBlink: true,
  cursorStyle: 'block',
  fontFamily: terminalFontFamily,
  fontSize: 13,
  fontWeight: 400,
  fontWeightBold: 700,
  lineHeight: 1,
  macOptionIsMeta: true,
  rightClickSelectsWord: false,
  scrollback: 10000,
  theme: tokyoNight
})
const fitAddon = new FitAddon()
terminal.loadAddon(fitAddon)
applyShellTheme()

const request = async (path, options = {}) => {
  const response = await fetch(endpoint(path), {
    cache: 'no-store',
    ...options
  })
  if (!response.ok && response.status !== 204) {
    const detail = (await response.text()).trim()
    throw new Error(detail || `HTTP ${response.status}`)
  }
  return response
}

const sendResize = async () => {
  try {
    await request('terminal/resize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ columns: terminal.cols, rows: terminal.rows })
    })
  } catch (error) {
    showError(`Resize failed: ${error.message}`)
  }
}

const fit = () => {
  try {
    fitAddon.fit()
  } catch (error) {
    return
  }
  clearTimeout(resizeTimer)
  resizeTimer = setTimeout(sendResize, 80)
}

if (typeof ResizeObserver === 'function') {
  new ResizeObserver(fit).observe(document.getElementById('terminal-shell'))
}
window.addEventListener('resize', fit)

const inputPump = createInputPump({
  send: (body) => request('terminal/input', {
    method: 'POST',
    headers: { 'Content-Type': 'application/octet-stream' },
    body
  }),
  onError: (error) => {
    setStatus('error', 'Input offline')
    showError(`Input failed: ${error.message}`)
  }
})

const queueInput = (bytes) => inputPump.write(bytes)
const mouseInput = createMouseInputCoalescer({
  write: (data) => queueInput(encoder.encode(data))
})

terminal.onData((data) => mouseInput.write(data))
terminal.onBinary((data) => {
  const bytes = Uint8Array.from(data, (character) => character.charCodeAt(0) & 0xff)
  queueInput(bytes)
})

terminalElement.addEventListener('contextmenu', (event) => {
  if (terminal.modes.mouseTrackingMode !== 'none') event.preventDefault()
})

terminal.attachCustomKeyEventHandler((event) => {
  const modifier = navigator.platform.toLowerCase().includes('mac') ? event.metaKey : event.ctrlKey
  if (modifier && event.type === 'keydown' && event.key.toLowerCase() === 'c' && terminal.hasSelection()) {
    const text = terminal.getSelection()
    navigator.clipboard?.writeText(text).catch(() => {})
    return false
  }
  return true
})

const poll = async () => {
  let backoff = 250
  while (!stopped) {
    try {
      const response = await fetch(endpoint(`terminal/output?cursor=${cursor}`), {
        cache: 'no-store'
      })
      if (response.status === 409) {
        cursor = await readResetCursor(response, cursor)
        terminal.reset()
        ensureHerdrMouseCapture(terminal)
        setStatus('connecting', 'Refreshing terminal')
        await request('terminal/restart', { method: 'POST' })
        continue
      }
      if (!response.ok && response.status !== 204) {
        const detail = (await response.text()).trim()
        throw new Error(detail || `HTTP ${response.status}`)
      }
      if (response.status === 200) {
        const bytes = new Uint8Array(await response.arrayBuffer())
        if (bytes.length > 0) {
          await new Promise((resolve) => terminal.write(bytes, resolve))
          cursor = advanceOutputCursor(cursor, bytes.length)
        }
      }
      ensureHerdrMouseCapture(terminal)
      setStatus('connected', 'Connected')
      backoff = 250
    } catch (error) {
      setStatus('error', 'Reconnecting')
      await new Promise((resolve) => setTimeout(resolve, backoff))
      backoff = Math.min(backoff * 2, 5000)
    }
  }
}

const bootstrap = async () => {
  fit()
  terminal.focus()
  try {
    const response = await request('terminal/status')
    const status = await response.json()
    const baseCursor = Number.isSafeInteger(status.baseCursor) ? status.baseCursor : 0
    const nextCursor = Number.isSafeInteger(status.nextCursor) ? status.nextCursor : baseCursor
    cursor = baseCursor
    if (baseCursor > 0) {
      cursor = nextCursor
      terminal.reset()
      ensureHerdrMouseCapture(terminal)
      await request('terminal/restart', { method: 'POST' })
    }
    setStatus(status.running ? 'connected' : 'connecting', status.running ? `Herdr ${status.version}` : 'Starting Herdr')
  } catch (error) {
    setStatus('error', 'Reconnecting')
  }
  await poll()
}

reconnectButton.addEventListener('click', async () => {
  reconnectButton.disabled = true
  setStatus('connecting', 'Reattaching')
  try {
    await request('terminal/restart', { method: 'POST' })
  } catch (error) {
    showError(`Reattach failed: ${error.message}`)
  } finally {
    setTimeout(() => {
      reconnectButton.disabled = false
    }, 1200)
  }
})

window.addEventListener('beforeunload', () => {
  stopped = true
})

const start = async () => {
  await waitForTerminalFont()
  terminal.open(terminalElement)
  ensureHerdrMouseCapture(terminal)
  window.parent.postMessage(JSON.stringify({ type: 'spr:ready' }), '*')
  await bootstrap()
}

start().catch((error) => {
  setStatus('error', 'Terminal failed')
  showError(`Terminal failed: ${error.message || error}`)
  console.error('spr-herdr terminal bootstrap failed', error)
})
