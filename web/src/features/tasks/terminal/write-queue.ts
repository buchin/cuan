import type { Terminal } from '@xterm/xterm'
import { scheduleTerminalWriteTask } from './schedule-write-task'
import { sanitizeTerminalOutput } from './sanitize-output'

/**
 * Coalesced, yield-friendly xterm writes (ported from Fural).
 *
 * Prevents main-thread starvation by:
 * - coalescing rapid chunks into a single pending buffer
 * - pausing parsing while tab is hidden
 * - draining in task-yielded chunks via MessageChannel
 * - capping hidden backlog so minimized tabs never OOM
 * - capping visible pending to newest tail so huge live dumps cannot freeze UI
 */

export const TERMINAL_WRITE_CHUNK_CHARS = 8_000
export const TERMINAL_LIVE_WRITE_CHUNK_CHARS = 1_024
export const TERMINAL_HIDDEN_BACKLOG_CAP = 96_000
export const TERMINAL_VISIBLE_BACKLOG_CAP = 256_000

export type TerminalWriteQueueOptions = {
  chunkSize?: number
  hiddenCap?: number
  visibleCap?: number
  onBacklogDropped?: () => void
  isActive?: () => boolean
  onDrained?: () => void
  onChunkWritten?: () => void
}

export type TerminalWriteQueue = {
  enqueue: (data: string) => void
  resume: () => void
  clear: () => void
  pendingChars: () => number
  isFlushing: () => boolean
}

function isDocumentHidden(): boolean {
  return typeof document !== 'undefined' && document.visibilityState === 'hidden'
}

export function trimPendingTail(pending: string, max: number): string {
  if (max <= 0) return ''
  if (pending.length <= max) return pending
  let tail = pending.slice(pending.length - max)
  const esc = tail.indexOf('\x1b')
  if (esc > 0 && esc < tail.length - 1) {
    tail = tail.slice(esc)
  }
  if (tail.includes('\x1b[?2026h') && !tail.includes('\x1b[?2026l')) {
    tail += '\x1b[?2026l'
  }
  return tail
}

export function createTerminalWriteQueue(
  terminal: Terminal,
  opts: TerminalWriteQueueOptions = {},
): TerminalWriteQueue {
  const chunkSize = opts.chunkSize ?? TERMINAL_LIVE_WRITE_CHUNK_CHARS
  const hiddenCap = opts.hiddenCap ?? TERMINAL_HIDDEN_BACKLOG_CAP
  const visibleCap = opts.visibleCap ?? TERMINAL_VISIBLE_BACKLOG_CAP
  const isActive = opts.isActive ?? (() => true)

  let pending = ''
  let flushing = false
  let droppedWhileHidden = false

  const pump = () => {
    if (flushing) return
    if (!pending) return
    if (isDocumentHidden()) return
    flushing = true

    const step = () => {
      if (!isActive()) {
        pending = ''
        flushing = false
        return
      }
      if (isDocumentHidden()) {
        flushing = false
        return
      }
      if (!pending) {
        flushing = false
        opts.onDrained?.()
        return
      }

      // When large backlog accumulates (e.g. initial connection history replay),
      // drain in fast bulk chunks (TERMINAL_WRITE_CHUNK_CHARS = 8000) so the screen stabilizes
      // immediately without an artificial slow typewriter replay effect.
      const adaptiveChunk =
        pending.length > 512 ? TERMINAL_WRITE_CHUNK_CHARS : chunkSize
      const chunk =
        pending.length <= adaptiveChunk ? pending : pending.slice(0, adaptiveChunk)
      pending =
        pending.length <= adaptiveChunk ? '' : pending.slice(adaptiveChunk)

      const afterWrite = () => {
        try {
          opts.onChunkWritten?.()
        } catch {}
        if (!pending) {
          flushing = false
          opts.onDrained?.()
          return
        }
        if (pending.length <= adaptiveChunk) {
          if (typeof queueMicrotask === 'function') {
            queueMicrotask(step)
          } else {
            setTimeout(step, 0)
          }
          return
        }
        scheduleTerminalWriteTask(step)
      }

      try {
        terminal.write(sanitizeTerminalOutput(chunk), afterWrite)
      } catch {
        pending = ''
        flushing = false
      }
    }

    step()
  }

  return {
    enqueue(data: string) {
      if (!data || !isActive()) return
      pending += data
      if (isDocumentHidden()) {
        if (pending.length > hiddenCap) {
          pending = ''
          if (!droppedWhileHidden) {
            droppedWhileHidden = true
            try {
              opts.onBacklogDropped?.()
            } catch {}
          }
        }
        return
      }
      if (pending.length > visibleCap) {
        pending = trimPendingTail(pending, visibleCap)
      }
      pump()
    },
    resume() {
      droppedWhileHidden = false
      if (!isDocumentHidden() && pending.length > visibleCap) {
        pending = trimPendingTail(pending, visibleCap)
      }
      pump()
    },
    clear() {
      pending = ''
      flushing = false
      droppedWhileHidden = false
    },
    pendingChars() {
      return pending.length
    },
    isFlushing() {
      return flushing
    },
  }
}
