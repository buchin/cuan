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
import { CornerDownLeft, Copy, Clipboard, Ban, Keyboard, Trash2, ChevronDown, ChevronUp } from 'lucide-react'

export type MobileTerminalKeyboardProps = {
  onInject: (data: string) => void
  onCopy?: () => void
  onPaste?: () => void
  onClear?: () => void
  onToggleKeyboard?: () => void
  onClose?: () => void
  onScrollToBottom?: () => void
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
  disabled = false,
  className = '',
}: MobileTerminalKeyboardProps) {
  const [mods, setMods] = useState<StickyMods>(EMPTY_STICKY_MODS)
  const [showExtended, setShowExtended] = useState(false)
  const modsRef = useRef(mods)
  modsRef.current = mods

  const repeatRef = useRef<KeyRepeatHandle | null>(null)
  const stopRepeat = useCallback(() => {
    repeatRef.current?.stop()
    repeatRef.current = null
  }, [])

  useEffect(() => () => stopRepeat(), [stopRepeat])

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
      className={`sm:hidden shrink-0 select-none touch-manipulation border-t border-[#262626] bg-[#161616] pb-1 pt-1 ${className}`}
      onMouseDown={(e) => e.preventDefault()}
      onPointerDown={(e) => e.preventDefault()}
    >
      {/* Quick Actions Row */}
      <div className="flex items-center gap-1 overflow-x-auto px-1.5 pb-1 select-none [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
        {/* Kbd Drawer / Extended Keys Toggle */}
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

        {/* Ctrl+C SIGINT Quick Interrupt */}
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
              onToggleKeyboard()
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

      {/* Extended Drawer Keys: Row 1 & Row 2 */}
      {showExtended && (
        <div className="space-y-0.5 pt-0.5">
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
