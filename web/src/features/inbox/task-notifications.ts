/**
 * task-notifications.ts
 *
 * Web Audio harmonic chime synthesizer and background document title notification badge.
 * Provides instant audio/visual feedback when tasks complete with zero-friction autoplay fallback.
 */

export const CHIME_FREQUENCIES = {
  C6: 1046.5,
  E6: 1318.5,
} as const

export const CHIME_DECAY_DURATION = 0.4

export const TASK_COMPLETED_BADGE_TEMPLATE = (count: number = 1): string =>
  `(${Math.max(1, Math.floor(count))}) Tugas Selesai! - Cuan`

let originalDocumentTitle: string | null = null
let isBadgeActive = false
let focusListenerAttached = false

/**
 * Play a gentle, harmonic Web Audio chime without external binary assets.
 * Combines C6 (1046.5 Hz) and E6 (1318.5 Hz) sine waves with an exponential decay of 0.4s.
 * Catches browser autoplay restrictions silently (zero-friction fallback).
 */
export async function playTaskCompletionChime(): Promise<boolean> {
  try {
    if (typeof window === 'undefined') return false
    const AudioCtx =
      window.AudioContext ||
      (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
    if (!AudioCtx) return false

    const ctx = new AudioCtx()
    if (ctx.state === 'suspended') {
      await ctx.resume().catch(() => {})
    }

    const now = ctx.currentTime
    const duration = CHIME_DECAY_DURATION

    // Master volume with exponential decay
    const gainNode = ctx.createGain()
    gainNode.gain.setValueAtTime(0.18, now)
    gainNode.gain.exponentialRampToValueAtTime(0.0001, now + duration)
    gainNode.connect(ctx.destination)

    // Harmonic sine wave 1: C6 (1046.5 Hz)
    const osc1 = ctx.createOscillator()
    osc1.type = 'sine'
    osc1.frequency.setValueAtTime(CHIME_FREQUENCIES.C6, now)
    osc1.connect(gainNode)

    // Harmonic sine wave 2: E6 (1318.5 Hz)
    const osc2 = ctx.createOscillator()
    osc2.type = 'sine'
    osc2.frequency.setValueAtTime(CHIME_FREQUENCIES.E6, now)
    osc2.connect(gainNode)

    osc1.start(now)
    osc2.start(now)
    osc1.stop(now + duration)
    osc2.stop(now + duration)

    // Clean up AudioContext resources after playback
    setTimeout(() => {
      ctx.close().catch(() => {})
    }, (duration + 0.1) * 1000)

    return true
  } catch {
    // Zero-friction silent fallback if browser blocks autoplay or Web Audio is unavailable
    return false
  }
}

/**
 * Restores original document title and resets title badge state.
 */
export function restoreOriginalTitle(): void {
  if (typeof document === 'undefined') return
  if (originalDocumentTitle !== null) {
    document.title = originalDocumentTitle
    originalDocumentTitle = null
  }
  isBadgeActive = false
}

/**
 * Sets document title badge: '(1) Tugas Selesai! - Cuan' when tab is in background,
 * and automatically restores the original title when window gains focus.
 */
export function showTaskCompletedTitleBadge(count: number = 1): void {
  if (typeof document === 'undefined') return

  if (originalDocumentTitle === null) {
    originalDocumentTitle = document.title || 'Cuan'
  }

  document.title = TASK_COMPLETED_BADGE_TEMPLATE(count)
  isBadgeActive = true

  if (!focusListenerAttached && typeof window !== 'undefined') {
    focusListenerAttached = true

    const onFocusOrVisible = () => {
      restoreOriginalTitle()
      window.removeEventListener('focus', onFocusOrVisible)
      document.removeEventListener('visibilitychange', onVisibilityChange)
      focusListenerAttached = false
    }

    const onVisibilityChange = () => {
      if (!document.hidden) {
        onFocusOrVisible()
      }
    }

    window.addEventListener('focus', onFocusOrVisible)
    document.addEventListener('visibilitychange', onVisibilityChange)
  }
}

/**
 * Check if the document title badge is currently active.
 */
export function isTitleBadgeActive(): boolean {
  return isBadgeActive
}

/**
 * Get the currently remembered original title.
 */
export function getOriginalTitle(): string | null {
  return originalDocumentTitle
}

/**
 * Notify task completion: plays audio chime, and if tab is in the background,
 * displays the document title badge until the tab is focused.
 */
export function notifyTaskCompleted(options?: { count?: number }): void {
  void playTaskCompletionChime()

  if (typeof document !== 'undefined' && document.hidden) {
    showTaskCompletedTitleBadge(options?.count || 1)
  }
}
