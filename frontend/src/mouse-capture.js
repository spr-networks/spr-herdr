// Crossterm's EnableMouseCapture enables all-motion tracking and SGR reports.
// Reassert the equivalent xterm modes after replay because a detached client
// can leave a disable sequence as the last mouse-mode state in the output ring.
export const herdrMouseCaptureSequence = '\x1b[?1000h\x1b[?1002h\x1b[?1003h\x1b[?1006h'

export const ensureHerdrMouseCapture = (terminal) => {
  if (terminal.modes?.mouseTrackingMode !== 'none') return false
  terminal.write(herdrMouseCaptureSequence)
  return true
}
