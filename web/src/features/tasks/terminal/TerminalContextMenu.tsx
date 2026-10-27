import { useEffect, useRef } from 'react'
import { Copy, Clipboard, Ban, Trash2, CheckSquare, Maximize2, Minimize2, Eye } from 'lucide-react'

export function isLikelyFilePath(text: string): boolean {
  if (!text || text.length > 200 || text.includes('\n') || text.includes(' ')) return false
  const trimmed = text.trim().replace(/^['"`(]+|[)'"`]+$/g, '')
  const fileExtRegex = /\.(md|json|ts|tsx|js|jsx|go|py|html|css|txt|csv|png|jpg|jpeg|svg|webp|gif|sh|yml|yaml|sql)$/i
  return fileExtRegex.test(trimmed)
}

export type TerminalContextMenuProps = {
  isOpen: boolean
  x: number
  y: number
  hasSelection: boolean
  selectedText?: string
  isFullscreen: boolean
  onClose: () => void
  onCopy: () => void
  onPaste: () => void
  onSelectAll: () => void
  onClear: () => void
  onInterrupt: () => void
  onToggleFullscreen: () => void
  onPreviewFile?: (filePath: string) => void
  t: (key: string) => string
}

export function TerminalContextMenu({
  isOpen,
  x,
  y,
  hasSelection,
  selectedText = '',
  isFullscreen,
  onClose,
  onCopy,
  onPaste,
  onSelectAll,
  onClear,
  onInterrupt,
  onToggleFullscreen,
  onPreviewFile,
  t,
}: TerminalContextMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!isOpen) return
    const handleClickOutside = (e: MouseEvent | TouchEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        onClose()
      }
    }
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('touchstart', handleClickOutside)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('touchstart', handleClickOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen, onClose])

  if (!isOpen) return null

  // Ensure menu stays within window bounds
  const menuWidth = 200
  const menuHeight = 220
  const adjustedX = Math.min(Math.max(8, x), window.innerWidth - menuWidth - 8)
  const adjustedY = Math.min(Math.max(8, y), window.innerHeight - menuHeight - 8)

  return (
    <div
      ref={menuRef}
      style={{ left: `${adjustedX}px`, top: `${adjustedY}px` }}
      className="fixed z-50 min-w-[190px] rounded-lg border border-[#333] bg-[#1a1a1a] p-1.5 text-xs text-neutral-200 shadow-2xl animate-in fade-in-50 zoom-in-95 duration-100 select-none"
    >
      <button
        type="button"
        onClick={() => {
          onCopy()
          onClose()
        }}
        className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left transition-colors hover:bg-neutral-800 active:bg-primary active:text-primary-foreground"
      >
        <Copy className="h-3.5 w-3.5 text-neutral-400" />
        <span>{hasSelection ? t('Copy selection') : t('Copy all output')}</span>
      </button>

      {hasSelection && isLikelyFilePath(selectedText) && onPreviewFile && (
        <button
          type="button"
          onClick={() => {
            onPreviewFile(selectedText.trim().replace(/^['"`(]+|[)'"`]+$/g, ''))
            onClose()
          }}
          className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-emerald-400 font-medium transition-colors hover:bg-emerald-950/40 active:bg-emerald-800 active:text-white"
        >
          <Eye className="h-3.5 w-3.5" />
          <span className="truncate">Preview "{selectedText.trim().replace(/^['"`(]+|[)'"`]+$/g, '')}"</span>
        </button>
      )}

      <button
        type="button"
        onClick={() => {
          onPaste()
          onClose()
        }}
        className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left transition-colors hover:bg-neutral-800 active:bg-primary active:text-primary-foreground"
      >
        <Clipboard className="h-3.5 w-3.5 text-neutral-400" />
        <span>{t('Paste into terminal')}</span>
      </button>

      <button
        type="button"
        onClick={() => {
          onSelectAll()
          onClose()
        }}
        className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left transition-colors hover:bg-neutral-800 active:bg-primary active:text-primary-foreground"
      >
        <CheckSquare className="h-3.5 w-3.5 text-neutral-400" />
        <span>{t('Select all')}</span>
      </button>

      <div className="my-1 h-px bg-[#333]" />

      <button
        type="button"
        onClick={() => {
          onInterrupt()
          onClose()
        }}
        className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-rose-300 transition-colors hover:bg-rose-500/20 active:bg-rose-600 active:text-white"
      >
        <Ban className="h-3.5 w-3.5 text-rose-400" />
        <span>{t('Send Ctrl+C (Interrupt)')}</span>
      </button>

      <button
        type="button"
        onClick={() => {
          onClear()
          onClose()
        }}
        className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left transition-colors hover:bg-neutral-800 active:bg-primary active:text-primary-foreground"
      >
        <Trash2 className="h-3.5 w-3.5 text-neutral-400" />
        <span>{t('Clear terminal')}</span>
      </button>

      <div className="my-1 h-px bg-[#333]" />

      <button
        type="button"
        onClick={() => {
          onToggleFullscreen()
          onClose()
        }}
        className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left transition-colors hover:bg-neutral-800 active:bg-primary active:text-primary-foreground"
      >
        {isFullscreen ? (
          <>
            <Minimize2 className="h-3.5 w-3.5 text-neutral-400" />
            <span>{t('Exit fullscreen')}</span>
          </>
        ) : (
          <>
            <Maximize2 className="h-3.5 w-3.5 text-neutral-400" />
            <span>{t('Fullscreen')}</span>
          </>
        )}
      </button>
    </div>
  )
}
