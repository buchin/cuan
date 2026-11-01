import { useCallback, useEffect, useRef, useState, type PointerEvent } from 'react'
import {
  EMPTY_STICKY_MODS,
  MOBILE_KEYBOARD_ROW1,
  MOBILE_KEYBOARD_ROW2,
  applyStickyModifiers,
  startKeyRepeat,
  toggleStickyMod,
  type KeyRepeatHandle,
  type MobileKeyDef,
  type StickyMods,
} from './mobile-keyboard'
import { CornerDownLeft, Copy, Clipboard, Ban, Keyboard, Trash2, ChevronDown, ChevronUp, Send, X } from 'lucide-react'

export type MobileTerminalKeyboardProps = {
  onInject: (data: string) => void
  onCopy?: () => void
  onPaste?: () => void
  onClear?: () => void
  onToggleKeyboard?: () => void
  onClose?: () => void
  onScrollToBottom?: () => void
  showPromptInput?: boolean
  compact?: boolean
  disabled?: boolean
  className?: string
}

export function MobileTerminalKeyboard({
  onInject,
  onCopy,
  onPaste,
  onClear,
  onToggleKeyboard,
  onClose,
  showPromptInput = false,
  compact = false,
  disabled = false,
  className = '',
}: MobileTerminalKeyboardProps) {
  const [mods, setMods] = useState<StickyMods>(EMPTY_STICKY_MODS)
  const [showExtended, setShowExtended] = useState(compact ? true : false)
  const [promptText, setPromptText] = useState('')
  const [history, setHistory] = useState<string[]>([])
  const [historyIdx, setHistoryIdx] = useState(-1)
  const promptInputRef = useRef<HTMLInputElement>(null)
  const modsRef = useRef(mods)
  modsRef.current = mods

  const repeatRef = useRef<KeyRepeatHandle | null>(null)
  const stopRepeat = useCallback(() => {
    repeatRef.current?.stop()
    repeatRef.current = null
  }, [])

  useEffect(() => () => stopRepeat(), [stopRepeat])

  const handleSendPrompt = useCallback(
    (e?: React.FormEvent) => {
      if (e) e.preventDefault()
      if (disabled) return
      const text = promptText
      if (!text.trim()) {
        onInject('\r')
        return
      }
      onInject(text)
      setTimeout(() => {
        onInject('\r')
      }, 40)
      setHistory((prev) => [text, ...prev.filter((h) => h !== text)].slice(0, 50))
      setHistoryIdx(-1)
      setPromptText('')
    },
    [disabled, onInject, promptText],
  )

  const handlePromptKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === 'Enter') {
        e.preventDefault()
        handleSendPrompt()
      } else if (e.key === 'ArrowUp') {
        if (history.length > 0) {
          e.preventDefault()
          const nextIdx = Math.min(historyIdx + 1, history.length - 1)
          setHistoryIdx(nextIdx)
          setPromptText(history[nextIdx] || '')
        }
      } else if (e.key === 'ArrowDown') {
        if (historyIdx > 0) {
          e.preventDefault()
          const nextIdx = historyIdx - 1
          setHistoryIdx(nextIdx)
          setPromptText(history[nextIdx] || '')
        } else if (historyIdx === 0) {
          e.preventDefault()
          setHistoryIdx(-1)
          setPromptText('')
        }
      }
    },
    [history, historyIdx, handleSendPrompt],
  )

  const handleToggleKeyboard = useCallback(() => {
    onToggleKeyboard?.()
    promptInputRef.current?.focus()
  }, [onToggleKeyboard])

  const fireKey = useCallback(
    (key: MobileKeyDef) => {
      if (disabled) return

      if (key.kind === 'mod' && key.mod) {
        setMods((prev) => toggleStickyMod(prev, key.mod!))
        return
      }

      if (key.kind === 'action') {
        if (key.action === 'copy') onCopy?.()
        else if (key.action === 'paste') onPaste?.()
        else if (key.action === 'clear') onClear?.()
        else if (key.action === 'enter') onInject('\r')
        return
      }

      const seq = key.seq ?? ''
      const res = applyStickyModifiers(seq, modsRef.current)
      if (res.consumed) {
        setMods(res.nextMods)
      }
      if (res.clipboardAction === 'paste') {
        onPaste?.()
        return
      }
      if (res.payload) {
        onInject(res.payload)
      }
    },
    [disabled, onInject, onCopy, onPaste, onClear],
  )

  const handlePointerDown = useCallback(
    (key: MobileKeyDef, event: PointerEvent) => {
      if (disabled) return
      // Prevent focus stealing from xterm hidden textarea so soft keyboard stays open
      event.preventDefault()
      event.stopPropagation()

      stopRepeat()

      if (key.kind === 'mod' || key.kind === 'action') {
        fireKey(key)
        return
      }

      fireKey(key)

      // Hold to repeat for navigation and sequence keys
      repeatRef.current = startKeyRepeat(() => {
        fireKey(key)
      })
    },
    [disabled, fireKey, stopRepeat],
  )

  return (
    <div
      role="toolbar"
      aria-label="Terminal Mobile Toolbar"
      className={`sm:hidden shrink-0 select-none touch-manipulation border-t border-[#262626] bg-[#161616] pb-[max(0.75rem,env(safe-area-inset-bottom))] ${className}`}
    >
      {/* Mobile Floating Prompt Bar - Isolated from xterm re-render stream (when standalone) */}
      {showPromptInput && (
        <form
          onSubmit={handleSendPrompt}
          className="flex items-center gap-1.5 px-2 py-1.5 bg-[#1b1b1b] border-b border-[#2d2d2d]"
        >
          <span className="text-emerald-400 font-mono text-xs font-bold pl-0.5 select-none shrink-0">&gt;</span>
          <div className="relative flex-1 min-w-0">
            <input
              ref={promptInputRef}
              type="text"
              value={promptText}
              onChange={(e) => setPromptText(e.target.value)}
              onKeyDown={handlePromptKeyDown}
              placeholder="Type command or input..."
              disabled={disabled}
              autoCapitalize="none"
              autoCorrect="off"
              autoComplete="off"
              spellCheck={false}
              className="w-full h-8 bg-[#111] border border-[#333] focus:border-emerald-500 rounded px-2.5 pr-7 text-xs font-mono text-neutral-100 placeholder:text-neutral-500 outline-none transition-colors"
            />
            {promptText && (
              <button
                type="button"
                onPointerDown={(e) => {
                  e.preventDefault()
                  setPromptText('')
                  promptInputRef.current?.focus()
                }}
                className="absolute right-1.5 top-1/2 -translate-y-1/2 p-0.5 text-neutral-400 hover:text-white"
                title="Clear input"
              >
                <X className="h-3 w-3" />
              </button>
            )}
          </div>
          <button
            type="submit"
            disabled={disabled || !promptText.trim()}
            onPointerDown={(e) => {
              if (promptText.trim()) {
                e.preventDefault()
                handleSendPrompt()
              }
            }}
            className="inline-flex h-8 shrink-0 items-center justify-center gap-1 rounded bg-emerald-600 hover:bg-emerald-500 active:bg-emerald-700 text-white px-2.5 text-xs font-semibold font-mono transition-colors disabled:opacity-35 disabled:cursor-not-allowed shadow-xs"
            title="Send input to terminal"
            aria-label="Send input"
          >
            <Send className="h-3.5 w-3.5" />
            <span>Send</span>
          </button>
        </form>
      )}

      {/* Quick Actions Row - Hidden in compact mode (embedded with TerminalComposer) to eliminate redundancy */}
      {!compact && (
        <div className="flex items-center gap-1 overflow-x-auto px-1.5 pt-1 pb-0.5 select-none [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
        {/* Kbd Drawer / Extended Keys Toggle - hidden in compact mode to avoid duplicate with composer */}
        {!compact && (
          <button
            type="button"
            disabled={disabled}
            onPointerDown={(e) => {
              e.preventDefault()
              setShowExtended((prev) => !prev)
            }}
            className={`inline-flex h-8 shrink-0 items-center gap-1 rounded border px-2.5 font-mono text-xs font-semibold transition-colors disabled:opacity-40 ${
              showExtended
                ? 'border-primary bg-primary text-primary-foreground shadow-xs'
                : 'border-[#333] bg-[#222] text-neutral-200 active:bg-zinc-700'
            }`}
            title="Toggle extra keys drawer (ESC, TAB, arrows, CTRL, ALT)"
          >
            <Keyboard className="h-3.5 w-3.5" />
            <span>Kbd</span>
            {showExtended ? (
              <ChevronUp className="h-3 w-3 opacity-70" />
            ) : (
              <ChevronDown className="h-3 w-3 opacity-70" />
            )}
          </button>
        )}

        {/* Ctrl+C SIGINT Quick Interrupt - hidden in compact mode because TerminalComposer already renders it */}
        {!compact && (
          <button
            type="button"
            disabled={disabled}
            onPointerDown={(e) => {
              e.preventDefault()
              onInject('\x03')
            }}
            className="inline-flex h-8 shrink-0 items-center gap-1 rounded border border-rose-500/30 bg-rose-500/15 px-2.5 font-mono text-xs font-semibold text-rose-300 transition-colors active:bg-rose-600 active:text-white disabled:opacity-40"
            title="Send Ctrl+C (Interrupt)"
          >
            <Ban className="h-3.5 w-3.5" />
            <span>Ctrl+C</span>
          </button>
        )}

        {/* Enter key */}
        <button
          type="button"
          disabled={disabled}
          onPointerDown={(e) => {
            e.preventDefault()
            onInject('\r')
          }}
          className="inline-flex h-8 shrink-0 items-center gap-1 rounded border border-[#333] bg-[#222] px-2.5 font-mono text-xs font-medium text-neutral-200 transition-colors active:bg-primary active:text-primary-foreground disabled:opacity-40"
          title="Enter (Return)"
        >
          <CornerDownLeft className="h-3.5 w-3.5" />
          <span>Enter</span>
        </button>

        {/* Copy selection / buffer */}
        {onCopy && (
          <button
            type="button"
            disabled={disabled}
            onPointerDown={(e) => {
              e.preventDefault()
              onCopy()
            }}
            className="inline-flex h-8 shrink-0 items-center gap-1 rounded border border-[#333] bg-[#222] px-2 font-mono text-xs font-medium text-neutral-300 transition-colors active:bg-zinc-700 disabled:opacity-40"
            title="Copy selection"
          >
            <Copy className="h-3.5 w-3.5" />
            <span>Copy</span>
          </button>
        )}

        {/* Paste from clipboard */}
        {onPaste && (
          <button
            type="button"
            disabled={disabled}
            onPointerDown={(e) => {
              e.preventDefault()
              onPaste()
            }}
            className="inline-flex h-8 shrink-0 items-center gap-1 rounded border border-[#333] bg-[#222] px-2 font-mono text-xs font-medium text-neutral-300 transition-colors active:bg-zinc-700 disabled:opacity-40"
            title="Paste into terminal"
          >
            <Clipboard className="h-3.5 w-3.5" />
            <span>Paste</span>
          </button>
        )}

        {/* Clear buffer */}
        {onClear && (
          <button
            type="button"
            disabled={disabled}
            onPointerDown={(e) => {
              e.preventDefault()
              onClear()
            }}
            className="inline-flex h-8 shrink-0 items-center gap-1 rounded border border-[#333] bg-[#222] px-2 font-mono text-xs font-medium text-neutral-400 transition-colors active:bg-zinc-700 disabled:opacity-40"
            title="Clear terminal screen"
          >
            <Trash2 className="h-3.5 w-3.5" />
            <span>Clear</span>
          </button>
        )}

        {/* Quick keys when extended drawer is collapsed */}
        {!showExtended && (
          <>
            <button
              type="button"
              disabled={disabled}
              onPointerDown={(e) => {
                e.preventDefault()
                onInject('\x1b')
              }}
              className="inline-flex h-8 shrink-0 items-center justify-center rounded border border-[#2c2c2c] bg-[#1e1e1e] px-2 font-mono text-xs font-medium text-neutral-300 transition-colors active:bg-primary active:text-primary-foreground disabled:opacity-40"
              title="Escape"
            >
              ESC
            </button>
            <button
              type="button"
              disabled={disabled}
              onPointerDown={(e) => {
                e.preventDefault()
                onInject('\t')
              }}
              className="inline-flex h-8 shrink-0 items-center justify-center rounded border border-[#2c2c2c] bg-[#1e1e1e] px-2 font-mono text-xs font-medium text-neutral-300 transition-colors active:bg-primary active:text-primary-foreground disabled:opacity-40"
              title="Tab"
            >
              TAB
            </button>
            <button
              type="button"
              disabled={disabled}
              onPointerDown={(e) => {
                e.preventDefault()
                onInject('\x1b[A')
              }}
              className="inline-flex h-8 shrink-0 items-center justify-center rounded border border-[#2c2c2c] bg-[#1e1e1e] px-2 font-mono text-xs font-medium text-neutral-300 transition-colors active:bg-primary active:text-primary-foreground disabled:opacity-40"
              title="Up arrow"
            >
              ↑
            </button>
            <button
              type="button"
              disabled={disabled}
              onPointerDown={(e) => {
                e.preventDefault()
                onInject('\x1b[B')
              }}
              className="inline-flex h-8 shrink-0 items-center justify-center rounded border border-[#2c2c2c] bg-[#1e1e1e] px-2 font-mono text-xs font-medium text-neutral-300 transition-colors active:bg-primary active:text-primary-foreground disabled:opacity-40"
              title="Down arrow"
            >
              ↓
            </button>
          </>
        )}

        {/* Soft keyboard toggle */}
        {onToggleKeyboard && (
          <button
            type="button"
            disabled={disabled}
            onPointerDown={(e) => {
              e.preventDefault()
              handleToggleKeyboard()
            }}
            className="ml-auto inline-flex h-8 shrink-0 items-center justify-center rounded border border-[#333] bg-[#222] px-2 font-mono text-xs text-neutral-400 transition-colors hover:text-foreground active:bg-zinc-700 disabled:opacity-40"
            title="Toggle soft keyboard"
          >
            <Keyboard className="h-4 w-4" />
          </button>
        )}

        {/* Hide / Collapse Toolbar */}
        {onClose && (
          <button
            type="button"
            disabled={disabled}
            onPointerDown={(e) => {
              e.preventDefault()
              onClose()
            }}
            className={`${onToggleKeyboard ? '' : 'ml-auto'} inline-flex h-8 shrink-0 items-center justify-center rounded border border-[#333] bg-[#222] px-2 font-mono text-xs text-neutral-400 transition-colors hover:text-rose-300 active:bg-zinc-700 disabled:opacity-40`}
            title="Hide keyboard toolbar"
            aria-label="Hide keyboard toolbar"
          >
            <ChevronDown className="h-4 w-4" />
          </button>
        )}
      </div>
      )}

      {/* Extended Drawer Keys: Row 1 & Row 2 */}
      {showExtended && (
        <div className="space-y-0.5 pt-0.5">
          {compact && onClose && (
            <div className="flex items-center justify-between px-2 pt-0.5 pb-0.5 text-[11px] text-neutral-400 font-mono select-none">
              <span className="text-[10px] uppercase tracking-wider text-neutral-500 font-semibold">Tombol Terminal</span>
              <button
                type="button"
                onPointerDown={(e) => {
                  e.preventDefault()
                  onClose()
                }}
                className="inline-flex items-center gap-1 text-neutral-400 hover:text-white px-1.5 py-0.5 rounded hover:bg-neutral-800 transition-colors"
                title="Tutup tombol terminal"
              >
                <span>Tutup</span>
                <ChevronDown className="h-3 w-3" />
              </button>
            </div>
          )}
          {/* Row 1 Keys: ESC, TAB, /, -, ↑, END, PGUP */}
          <div className="grid grid-cols-7 gap-1 px-1.5 py-0.5">
            {MOBILE_KEYBOARD_ROW1.map((key) => (
              <button
                key={key.id}
                type="button"
                disabled={disabled}
                onPointerDown={(e) => handlePointerDown(key, e)}
                onPointerUp={stopRepeat}
                onPointerLeave={stopRepeat}
                onPointerCancel={stopRepeat}
                className="flex h-9 min-w-0 items-center justify-center rounded border border-[#2c2c2c] bg-[#1e1e1e] font-mono text-[13px] font-medium text-neutral-200 transition-colors active:bg-primary active:text-primary-foreground disabled:opacity-30"
              >
                {key.label}
              </button>
            ))}
          </div>

          {/* Row 2 Keys: CTRL, ALT, SHIFT, ←, ↓, →, PGDN */}
          <div className="grid grid-cols-7 gap-1 px-1.5 py-0.5">
            {MOBILE_KEYBOARD_ROW2.map((key) => {
              const isModActive = key.kind === 'mod' && key.mod ? mods[key.mod] : false
              return (
                <button
                  key={key.id}
                  type="button"
                  disabled={disabled}
                  onPointerDown={(e) => handlePointerDown(key, e)}
                  onPointerUp={stopRepeat}
                  onPointerLeave={stopRepeat}
                  onPointerCancel={stopRepeat}
                  className={`flex h-9 min-w-0 items-center justify-center rounded border font-mono text-[12px] font-semibold transition-colors disabled:opacity-30 ${
                    isModActive
                      ? 'border-primary bg-primary text-primary-foreground shadow-sm'
                      : 'border-[#2c2c2c] bg-[#1e1e1e] text-neutral-200 active:bg-primary active:text-primary-foreground'
                  }`}
                >
                  {key.label}
                </button>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
