/**
 * Mobile terminal accessory keyboard layout and sticky modifier state machine.
 * Provides developer-grade terminal control on mobile touch screens (Blink / Developer style).
 */

export type StickyMods = {
  ctrl: boolean
  alt: boolean
  shift: boolean
}

export const EMPTY_STICKY_MODS: StickyMods = {
  ctrl: false,
  alt: false,
  shift: false,
}

export type MobileKeyKind = 'seq' | 'mod' | 'action'
export type MobileKeyAction = 'copy' | 'paste' | 'clear' | 'enter'

export type MobileKeyDef = {
  id: string
  label: string
  kind: MobileKeyKind
  seq?: string
  mod?: keyof StickyMods
  action?: MobileKeyAction
}

/** Row 1: ESC, TAB, /, -, ↑, END, PGUP */
export const MOBILE_KEYBOARD_ROW1: readonly MobileKeyDef[] = [
  { id: 'esc', label: 'ESC', kind: 'seq', seq: '\x1b' },
  { id: 'tab', label: 'TAB', kind: 'seq', seq: '\t' },
  { id: 'slash', label: '/', kind: 'seq', seq: '/' },
  { id: 'dash', label: '-', kind: 'seq', seq: '-' },
  { id: 'up', label: '↑', kind: 'seq', seq: '\x1b[A' },
  { id: 'end', label: 'END', kind: 'seq', seq: '\x1b[F' },
  { id: 'pgup', label: 'PGUP', kind: 'seq', seq: '\x1b[5~' },
] as const

/** Row 2: CTRL, ALT, SHIFT, ←, ↓, →, PGDN */
export const MOBILE_KEYBOARD_ROW2: readonly MobileKeyDef[] = [
  { id: 'ctrl', label: 'CTRL', kind: 'mod', mod: 'ctrl' },
  { id: 'alt', label: 'ALT', kind: 'mod', mod: 'alt' },
  { id: 'shift', label: 'SHIFT', kind: 'mod', mod: 'shift' },
  { id: 'left', label: '←', kind: 'seq', seq: '\x1b[D' },
  { id: 'down', label: '↓', kind: 'seq', seq: '\x1b[B' },
  { id: 'right', label: '→', kind: 'seq', seq: '\x1b[C' },
  { id: 'pgdn', label: 'PGDN', kind: 'seq', seq: '\x1b[6~' },
] as const

export function toggleStickyMod(mods: StickyMods, mod: keyof StickyMods): StickyMods {
  return { ...mods, [mod]: !mods[mod] }
}

export type StickyApplyResult = {
  payload: string
  nextMods: StickyMods
  consumed: boolean
  clipboardAction?: 'paste'
}

/**
 * Applies sticky modifiers to incoming data (from accessory bar or soft keyboard).
 */
export function applyStickyModifiers(data: string, mods: StickyMods): StickyApplyResult {
  if (!data || (!mods.ctrl && !mods.alt && !mods.shift)) {
    return { payload: data, nextMods: mods, consumed: false }
  }

  // Sticky CTRL + v -> paste from clipboard
  if (mods.ctrl && !mods.alt && data.length === 1 && data.toLowerCase() === 'v') {
    return {
      payload: '',
      nextMods: { ...EMPTY_STICKY_MODS },
      consumed: true,
      clipboardAction: 'paste',
    }
  }

  // Navigation sequences: keep sticky mods latched so repeated arrows keep word-jumping
  if (data.startsWith('\x1b[') || data === '\x1b') {
    return { payload: data, nextMods: mods, consumed: false }
  }

  const nextMods: StickyMods = { ...EMPTY_STICKY_MODS }
  let payload = data

  if (mods.shift && payload.length === 1) {
    const ch = payload
    if (ch >= 'a' && ch <= 'z') {
      payload = ch.toUpperCase()
    }
  }

  if (mods.ctrl && payload.length === 1) {
    const code = payload.toUpperCase().charCodeAt(0)
    if (code >= 0x40 && code <= 0x5f) {
      payload = String.fromCharCode(code & 0x1f)
    } else if (payload === ' ') {
      payload = '\0'
    } else if (payload === '?') {
      payload = '\x7f'
    }
  }

  if (mods.alt) {
    payload = '\x1b' + payload
  }

  return { payload, nextMods, consumed: true }
}

/**
 * Key repeat runner for holding sequence keys.
 */
export type KeyRepeatHandle = {
  stop: () => void
}

export function startKeyRepeat(
  fn: () => void,
  initialDelayMs = 350,
  repeatIntervalMs = 60
): KeyRepeatHandle {
  let timer: ReturnType<typeof setTimeout> | null = null
  let interval: ReturnType<typeof setInterval> | null = null

  timer = setTimeout(() => {
    fn()
    interval = setInterval(fn, repeatIntervalMs)
  }, initialDelayMs)

  return {
    stop() {
      if (timer) {
        clearTimeout(timer)
        timer = null
      }
      if (interval) {
        clearInterval(interval)
        interval = null
      }
    },
  }
}
