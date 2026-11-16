/**
 * Adapted from Fural's xterm.js auto-response filtering & wire protocol.
 * Drops xterm.js auto-probes (XTVERSION, OSC colors, Primary/Secondary DA)
 * so they are never injected into the PTY stdin stream as garbage characters.
 */

export const MSG_DATA = 0x00
export const MSG_RESIZE = 0x01

export function encodeTerminalDataFrame(data: string): ArrayBuffer {
  const bytes = new TextEncoder().encode(data)
  const frame = new Uint8Array(bytes.length + 1)
  frame[0] = MSG_DATA
  frame.set(bytes, 1)
  return frame.buffer
}

export function encodeTerminalResizeFrame(cols: number, rows: number): ArrayBuffer {
  const frame = new Uint8Array(5)
  frame[0] = MSG_RESIZE
  frame[1] = (cols >> 8) & 0xff
  frame[2] = cols & 0xff
  frame[3] = (rows >> 8) & 0xff
  frame[4] = rows & 0xff
  return frame.buffer
}

/** Full DCS XTVERSION reply from stock xterm.js (ST = ESC \ or BEL). */
const XTERM_JS_XTVERSION_DCS = /^\x1bP>\|xterm\.js\([^)]*\)(?:\x1b\\|\x07)$/
const XTERM_JS_XTVERSION_BARE = /^>\|xterm\.js\([^)]*\)$/

/** OSC 10/11/12 color report: ESC ] {10|11|12} ; rgb:... ST|BEL */
const XTERM_JS_OSC_COLOR_REPORT =
  /^\x1b\](10|11|12);rgb:[0-9a-fA-F]{1,4}\/[0-9a-fA-F]{1,4}\/[0-9a-fA-F]{1,4}(?:\x1b\\|\x07)$/
const XTERM_JS_OSC_COLOR_REPORT_BARE =
  /^\](10|11|12);rgb:[0-9a-fA-F]{1,4}\/[0-9a-fA-F]{1,4}\/[0-9a-fA-F]{1,4}$/

/** Primary DA reply: ESC [ ? 1 ; 2 c (or VT100 variants) */
const XTERM_JS_PRIMARY_DA = /^\x1b\[\?(?:1;2|1;0|0|1|6)c$/
const XTERM_JS_PRIMARY_DA_BARE = /^\?(?:1;2|1;0|0|1|6)c$/

/** Secondary DA reply: ESC [ > 0 ; 276 ; 0 c */
const XTERM_JS_SECONDARY_DA = /^\x1b\[>[0-9]+;[0-9]+;[0-9]+c$/
const XTERM_JS_SECONDARY_DA_BARE = /^>[0-9]+;[0-9]+;[0-9]+c$/

export function isXtermJsAutoResponse(data: string): boolean {
  if (!data || data.length > 96) return false
  return (
    XTERM_JS_XTVERSION_DCS.test(data) ||
    XTERM_JS_XTVERSION_BARE.test(data) ||
    XTERM_JS_OSC_COLOR_REPORT.test(data) ||
    XTERM_JS_OSC_COLOR_REPORT_BARE.test(data) ||
    XTERM_JS_PRIMARY_DA.test(data) ||
    XTERM_JS_PRIMARY_DA_BARE.test(data) ||
    XTERM_JS_SECONDARY_DA.test(data) ||
    XTERM_JS_SECONDARY_DA_BARE.test(data)
  )
}
