/**
 * Word-level editing keys for web terminals (adapted from Fural).
 * Matches iTerm / macOS Terminal natural text editing so Alt+Arrow and Alt+Backspace
 * work naturally in readline, bash/zsh, and interactive agents like agy:
 *
 *   Alt+Left      → ESC b   (backward-word)
 *   Alt+Right     → ESC f   (forward-word)
 *   Alt+Backspace → ESC DEL (backward-kill-word)
 */

export const WORD_LEFT = '\x1bb'
export const WORD_RIGHT = '\x1bf'
export const WORD_BACKSPACE = '\x1b\x7f'

export function desktopWordEditSequence(
  event: Pick<
    KeyboardEvent,
    'type' | 'key' | 'code' | 'altKey' | 'ctrlKey' | 'shiftKey' | 'metaKey'
  >,
): string | null {
  if (event.type !== 'keydown') return null
  if (event.metaKey) return null

  const key = event.key
  const alt = event.altKey
  const ctrl = event.ctrlKey
  const shift = event.shiftKey

  // Alt+Backspace / Ctrl+Backspace → kill previous word
  if (key === 'Backspace' && (alt || ctrl) && !shift) {
    return WORD_BACKSPACE
  }

  // Alt+Left / Alt+Right (no Ctrl) → word jump
  if (alt && !ctrl && !shift) {
    if (key === 'ArrowLeft' || key === 'Left') return WORD_LEFT
    if (key === 'ArrowRight' || key === 'Right') return WORD_RIGHT
  }

  return null
}
