// Adapted from Fural's terminal mobile-input pipeline. Kept independent of
// React, sockets and provider identity so the input contract can be tested.
/**
 * Mobile IMEs (esp. Android predictive keyboards) often re-emit the *entire*
 * current word/line via `insertText` / composition on every keystroke, instead
 * of a single new character. If we forward that raw to a PTY, the agent TUI
 * inserts the full string again → staircase of growing lines:
 *
 *   q
 *   qw
 *   qwr
 *
 * Track what we already sent and only forward the delta.
 *
 * Desktop browsers can also coalesce fast keystrokes into one multi-char
 * `input` event (e.g. buffer "hel" then data "lo"). That is *not* an IME
 * full-buffer resend — treating it as autocorrect would send DELs and erase
 * the line. Only enable full-word replace/shorten heuristics after we have
 * observed the classic growing full-buffer IME pattern on this line.
 *
 * A separate case is iOS/Android double-space → ". " (or other small trailing
 * edits) when keystrokes were single-char so far: xterm's textarea still holds
 * the whole line, and `_handleAnyTextareaChanges` may re-emit that full value
 * (equal-length rewrite, or `newValue.replace(oldValue, "")` failing when the
 * trailing space became ". "). Appending it would duplicate every word already
 * on the PTY. Detect near full-line rewrites via common-prefix / body similarity
 * and only rewrite the trailing delta. Callers should clear the helper
 * textarea only when this normalizer drops or rewrites an event; keeping
 * ordinary input provides sentence context to Android IMEs such as SwiftKey.
 */
export class MobileInputNormalizer {
  private lineBuffer = ""
  /** True once this line saw a multi-char full-buffer IME resend. */
  private imeMode = false

  /** Reset on connect / Enter so a new prompt starts clean. */
  reset() {
    this.lineBuffer = ""
    this.imeMode = false
  }

  /**
   * Whether this line is in mobile IME full-buffer mode.
   * Callers may clear the helper textarea whenever normalizing is on; this
   * flag remains useful for diagnostics and tests.
   */
  get isImeMode() {
    return this.imeMode
  }

  /** Characters already forwarded on the current logical line (for tests). */
  get trackedLine() {
    return this.lineBuffer
  }

  /**
   * Re-seed the tracked line from the retained helper textarea after a socket
   * reconnect (park/unpark, soft reconnect, mobile network blip). The PTY
   * prompt still holds this text and the textarea kept it for IME sentence
   * context — `reset()` alone forgets it, so the next IME full-line
   * re-emission (double-space period, autocorrect) falls through to the
   * plain-append path and sends the whole sentence to the PTY a second time.
   * Only the current logical line (after the last line break) is still
   * pending on the prompt; anything earlier was already submitted.
   */
  reseed(raw: string) {
    const cut = Math.max(raw.lastIndexOf("\n"), raw.lastIndexOf("\r"))
    const line = cut >= 0 ? raw.slice(cut + 1) : raw
    this.lineBuffer = hasControlChars(line) ? "" : line
    this.imeMode = false
  }

  /**
   * @returns bytes to send to the PTY, or null to drop (empty / pure duplicate)
   */
  normalize(data: string): string | null {
    if (!data) return null

    // Enter / submit ends the logical line.
    if (data === "\r" || data === "\n" || data === "\r\n") {
      this.lineBuffer = ""
      this.imeMode = false
      return data === "\n" ? "\r" : data
    }

    // Split trailing line endings so "hello\r" still resets after the word.
    if (data.endsWith("\r") || data.endsWith("\n")) {
      const body = data.replace(/[\r\n]+$/g, "")
      const nl = data.endsWith("\r\n")
        ? "\r\n"
        : data.endsWith("\r")
          ? "\r"
          : "\r"
      const head = body ? this.normalize(body) : null
      this.lineBuffer = ""
      this.imeMode = false
      if (head) return head + nl
      return nl
    }

    // Backspace / DEL
    if (data === "\x7f" || data === "\b") {
      if (this.lineBuffer.length > 0) {
        this.lineBuffer = this.lineBuffer.slice(0, -1)
      }
      return data
    }

    // Terminal input escapes (CSI mouse, arrows, SS3, …) and bare SGR mouse
    // reports must pass through to the PTY unchanged and must NOT pollute the
    // IME line buffer — otherwise a drag can poison full-buffer delta logic
    // and strip the ESC prefix from later cells ("<35;…" instead of ESC[<…).
    if (isTerminalInputEscape(data) || isBareSgrMouseReport(data)) {
      return data
    }

    // Full-buffer resend: data is previous line + new suffix.
    // Classic mobile IME staircase pattern — enter IME mode.
    if (
      this.lineBuffer.length > 0 &&
      data.startsWith(this.lineBuffer) &&
      data.length > this.lineBuffer.length
    ) {
      this.imeMode = true
      const delta = data.slice(this.lineBuffer.length)
      this.lineBuffer = data
      return delta || null
    }

    // Full-buffer resend of the exact same text (duplicate key event).
    // Only multi-char or IME mode: a single-char exact match is often the user
    // typing the same letter/space again ("aa", double-space), not an echo.
    if (
      data === this.lineBuffer &&
      (this.imeMode || data.length > 1)
    ) {
      return null
    }

    // Near full-line rewrite without prior staircase (e.g. iOS double-space
    // period: buffer "hello world ", event "hello world. "). Desktop coalesced
    // "hel"+"lo" has commonPrefix 0 → falls through to append.
    // Also catches autocorrect mid-line + period in one event (low char prefix
    // but high word/body overlap) which used to append the whole sentence.
    if (
      this.lineBuffer.length > 0 &&
      data.length >= 2 &&
      !hasControlChars(data) &&
      !data.startsWith(this.lineBuffer) &&
      !this.lineBuffer.startsWith(data)
    ) {
      const rewritten = tryNearLineRewrite(this.lineBuffer, data)
      if (rewritten != null) {
        this.imeMode = true
        this.lineBuffer = data
        return rewritten
      }
    }

    // IME replaced the word (e.g. autocorrect "teh" → "the").
    // Only when we already saw full-buffer IME behavior — otherwise a
    // multi-char batch from fast desktop typing would erase the line.
    if (
      this.imeMode &&
      this.lineBuffer.length > 0 &&
      data.length > 0 &&
      !data.startsWith(this.lineBuffer) &&
      !this.lineBuffer.startsWith(data) &&
      data.length >= 2 &&
      !hasControlChars(data)
    ) {
      const backs = "\x7f".repeat(this.lineBuffer.length)
      this.lineBuffer = data
      return backs + data
    }

    // Shortening without DEL (IME cancelled composition).
    if (
      this.imeMode &&
      this.lineBuffer.length > 0 &&
      this.lineBuffer.startsWith(data) &&
      data.length < this.lineBuffer.length
    ) {
      const n = this.lineBuffer.length - data.length
      this.lineBuffer = data
      return "\x7f".repeat(n)
    }

    // Line-killing control characters (Ctrl+C ^C, Ctrl+U, Ctrl+W, Ctrl+K)
    // clear or truncate the pending prompt line on the PTY side in ways we
    // cannot mirror client-side. Anything already tracked is no longer a
    // reliable prefix of the pending prompt text — drop it instead of
    // poisoning later full-buffer comparisons (a poisoned buffer makes the
    // next IME re-emission fall through to plain append and duplicate the
    // whole sentence). Callers must also clear the helper textarea.
    if (isLineKillControlInput(data)) {
      this.lineBuffer = ""
      this.imeMode = false
      return data
    }

    // Normal single char, paste, or coalesced desktop keystrokes. Control
    // bytes that reach this point (cursor movers ^A/^E, redraw ^L, Tab) do
    // not add text to the pending line — appending them would poison the
    // tracked prefix and make the next IME re-emission mis-diff.
    if (!hasControlChars(data)) {
      this.lineBuffer += data
    }
    return data
  }
}

/**
 * Raw input bytes that invalidate the tracked pending line. Plain cursor
 * movers / Tab / redraw (^A ^E ^L arrows) keep the line content and must NOT
 * reset tracking; only characters that clear or kill part of the prompt line
 * do. Exported so the input pipeline can also reset the helper textarea.
 */
export function isLineKillControlInput(data: string): boolean {
  return (
    data.length >= 1 &&
    (data === "\x03" /* ^C */ ||
      data === "\x15" /* ^U kill line */ ||
      data === "\x17" /* ^W kill word */ ||
      data === "\x0b" /* ^K kill to EOL */ ||
      data.startsWith("\x03") ||
      data.startsWith("\x15") ||
      data.startsWith("\x17") ||
      data.startsWith("\x0b"))
  )
}

/**
 * Input that makes whatever the xterm helper textarea still holds invalid as
 * the pending prompt line: a submitted line (Enter from any path — xterm only
 * clears its textarea on physical Enter keydowns, so IME/soft-keyboard
 * "enter" paths leave the submitted sentence behind to be re-emitted by the
 * next double-space/autocorrect event) or a line-killing control.
 */
export function terminalInputInvalidatesHelperTextarea(data: string): boolean {
  return (
    data.includes("\r") || data.includes("\n") || isLineKillControlInput(data)
  )
}

/**
 * Ready-to-wire mobile input filter for terminal sockets that do not use the
 * full `useTerminalWebSocket` pipeline (auth/login terminals). Applies the
 * same three protections: textarea hygiene on line submit / line kill,
 * IME full-line re-emission delta/drop, and reconnect re-seeding via sync().
 */
export class TerminalHelperTextareaInputFilter {
  private norm = new MobileInputNormalizer()
  private getTextarea: () => HTMLTextAreaElement | null | undefined

  constructor(getTextarea: () => HTMLTextAreaElement | null | undefined) {
    this.getTextarea = getTextarea
  }

  /**
   * Re-seed the tracked line from the retained helper textarea after a socket
   * (re)connect — the PTY prompt still holds that text, so the tracker must
   * too or the next IME re-emission duplicates the sentence. An empty
   * textarea (cleared after an IME rewrite) keeps the tracker: staircase IMEs
   * still re-emit the full line from keyboard state.
   */
  sync() {
    try {
      const retained = this.getTextarea()?.value ?? ""
      if (retained) {
        this.norm.reseed(retained)
      }
    } catch {
      /* keep the existing tracker */
    }
  }

  /**
   * Forget the tracked line and the textarea after input that invalidates it
   * (submitted line / line-kill) arrived from outside the xterm event path.
   */
  reset() {
    this.norm.reset()
    this.clearTextarea()
  }

  private clearTextarea() {
    try {
      const ta = this.getTextarea()
      if (ta && ta.value) {
        ta.value = ""
      }
    } catch {
      /* ignore */
    }
  }

  /** @returns PTY payload, or null when the event must be dropped. */
  filter(data: string): string | null {
    if (terminalInputInvalidatesHelperTextarea(data)) {
      this.clearTextarea()
    }
    const next = this.norm.normalize(data)
    if (next === null || next !== data) {
      // A dropped/rewritten IME event must not leave stale text in the
      // textarea for the next event to re-emit.
      this.clearTextarea()
    }
    return next
  }
}

function hasControlChars(s: string): boolean {
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i)
    if (c < 0x20 || c === 0x7f) return true
  }
  return false
}

function commonPrefixLength(a: string, b: string): number {
  const n = Math.min(a.length, b.length)
  let i = 0
  while (i < n && a.charCodeAt(i) === b.charCodeAt(i)) i++
  return i
}

/**
 * Strip trailing whitespace / sentence-ending punctuation so we can compare
 * the "body" of a line when mobile double-space converts " " → ". ".
 */
function lineBody(s: string): string {
  return s.replace(/[\s.!?…]+$/u, "")
}

/**
 * iOS/Android double-space (or smart punctuation) turned the trailing spaces
 * into ". " / "! " / etc. Bodies match (optionally case-insensitive for the
 * rare first-letter recase); length stays in the same ballpark.
 */
function isTrailingPeriodConversion(prev: string, data: string): boolean {
  if (prev.length < 1 || data.length < 2) return false
  // Must look like a sentence terminator was introduced/adjusted at the end.
  if (!/[.!?…]\s*$/u.test(data)) return false
  const pb = lineBody(prev)
  const db = lineBody(data)
  if (pb.length < 1) return false
  if (pb !== db && pb.toLowerCase() !== db.toLowerCase()) return false
  return Math.abs(data.length - prev.length) <= 4
}

/**
 * True when `data` is almost certainly a mobile keyboard re-emit of the same
 * line (double-space period, light autocorrect, first-letter recase) rather
 * than a desktop coalesced batch ("hel"+"lo") or a genuine paste.
 */
function isLikelyLineResend(prev: string, data: string): boolean {
  if (prev.length < 2 || data.length < 2) return false
  if (hasControlChars(data)) return false
  // Large paste of new content — not a resend.
  if (data.length > prev.length + 8 && data.length > prev.length * 1.3) {
    return false
  }

  if (isTrailingPeriodConversion(prev, data)) return true

  const prefix = commonPrefixLength(prev, data)
  const shorter = Math.min(prev.length, data.length)
  // Majority of the shared line is still the same characters.
  if (shorter >= 4 && prefix >= Math.ceil(shorter * 0.6)) return true

  const pb = lineBody(prev).toLowerCase()
  const db = lineBody(data).toLowerCase()
  if (pb.length < 4 || db.length < 4) return false

  const bodyPrefix = commonPrefixLength(pb, db)
  if (bodyPrefix >= Math.ceil(Math.min(pb.length, db.length) * 0.6)) {
    return true
  }

  // Mid-line autocorrect + trailing period: char prefix may be tiny, but most
  // whitespace-separated tokens still match ("teh fox" → "the fox. ").
  if (Math.abs(pb.length - db.length) <= 6) {
    const pw = pb.split(/\s+/).filter(Boolean)
    const dw = db.split(/\s+/).filter(Boolean)
    if (pw.length >= 2 && dw.length >= 2) {
      const set = new Set(dw)
      const hits = pw.filter((w) => set.has(w)).length
      if (hits >= Math.ceil(pw.length * 0.6)) return true
    }
  }

  return false
}

/**
 * If `data` is almost the same as `prev` (trailing edit / small rewrite of a
 * long shared prefix / double-space period / light autocorrect), return DEL*
 * + new suffix. Otherwise null (caller appends).
 */
function tryNearLineRewrite(prev: string, data: string): string | null {
  const prefix = commonPrefixLength(prev, data)
  // At most two trailing chars differ (double-space → ". ", typo fix at end).
  const almostSameEnd = prefix >= Math.max(1, prev.length - 2)
  // Longer shared body with similar length (period conversion, capitalization).
  const substantialRewrite =
    prefix >= Math.ceil(prev.length * 0.75) &&
    Math.abs(data.length - prev.length) <= 3
  // Explicit double-space → ". " even when more than 2 trailing chars changed
  // (e.g. "  " → ". ", or first-letter recase + period).
  const periodConversion = isTrailingPeriodConversion(prev, data)
  // Broader mobile resend (mid-line autocorrect + period, moderate prefix).
  const likelyResend = isLikelyLineResend(prev, data)
  if (
    !almostSameEnd &&
    !substantialRewrite &&
    !periodConversion &&
    !likelyResend
  ) {
    return null
  }
  // Rewrite from the shared prefix (0 if none — full DEL*prev + data).
  // Covers first-letter recase, mid-line autocorrect, and trailing period.
  const backs = "\x7f".repeat(prev.length - prefix)
  const add = data.slice(prefix)
  const out = backs + add
  return out || null
}

/** ESC-prefixed terminal input (CSI / SS3 / OSC leftovers / single ESC). */
function isTerminalInputEscape(data: string): boolean {
  return data.length > 0 && data.charCodeAt(0) === 0x1b
}

/**
 * Bare xterm SGR mouse cell(s) when the leading ESC was already lost:
 * `<btn;col;rowM` optionally repeated. Keep out of the IME line buffer.
 */
const BARE_SGR_MOUSE =
  /^(?:<\d{1,4};\d{1,4};\d{1,4}[Mm]?)+$/

function isBareSgrMouseReport(data: string): boolean {
  if (!data || data.length > 256 || data.charCodeAt(0) !== 0x3c /* < */) {
    return false
  }
  return BARE_SGR_MOUSE.test(data)
}
