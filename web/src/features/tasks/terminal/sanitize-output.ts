/**
 * Normalize PTY output glyphs that break in browser xterm.js (ported from Fural).
 *
 * Mappings:
 *   U+23FA BLACK CIRCLE FOR RECORD              → U+25CF ●
 *   U+23FA + VS16 emoji presentation            → U+25CF ●
 *   U+2759 MEDIUM VERTICAL BAR (collapsed accent) → U+2503 ┃
 *   U+21E3 DOWNWARDS ARROW WITH TIP DOWNWARDS     → U+2193 ↓
 *   Braille spinner frames (U+280B…U+283C)        → U+2022 •
 */

const RECORD_CIRCLE = '\u23FA'
const RECORD_CIRCLE_EMOJI = '\u23FA\uFE0F'
const BLACK_CIRCLE = '\u25CF'

const MEDIUM_VERTICAL_BAR = '\u2759'
const BOX_HEAVY_VERTICAL = '\u2503'

const ARROW_TIP_DOWN = '\u21E3'
const ARROW_DOWN = '\u2193'

// All braille spinner & pattern frames (U+2800–U+28FF)
const BRAILLE_SPINNER = /[\u2800-\u28FF]/g
const BULLET = '\u2022'

function needsSanitize(text: string): boolean {
  if (text.length === 1 && text.charCodeAt(0) < 0x2000) {
    return false
  }
  return (
    text.includes(RECORD_CIRCLE) ||
    text.includes(MEDIUM_VERTICAL_BAR) ||
    text.includes(ARROW_TIP_DOWN) ||
    /[\u2800-\u28FF]/.test(text)
  )
}

export function sanitizeTerminalOutput(text: string): string {
  if (!needsSanitize(text)) return text
  return text
    .replace(new RegExp(RECORD_CIRCLE_EMOJI, 'g'), BLACK_CIRCLE)
    .replace(new RegExp(RECORD_CIRCLE, 'g'), BLACK_CIRCLE)
    .replace(new RegExp(MEDIUM_VERTICAL_BAR, 'g'), BOX_HEAVY_VERTICAL)
    .replace(new RegExp(ARROW_TIP_DOWN, 'g'), ARROW_DOWN)
    .replace(BRAILLE_SPINNER, BULLET)
}
