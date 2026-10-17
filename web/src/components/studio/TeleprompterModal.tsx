import { useState, useEffect, useRef } from 'react'
import { Button } from '../ui/button'
import {
  Play,
  Pause,
  RotateCcw,
  X,
  Type,
  FlipHorizontal,
  HelpCircle,
  Video
} from 'lucide-react'

interface TeleprompterModalProps {
  isOpen: boolean
  onClose: () => void
  title?: string
  content: string
}

export function TeleprompterModal({
  isOpen,
  onClose,
  title = 'Naskah Teleprompter',
  content,
}: TeleprompterModalProps) {
  const [isPlaying, setIsPlaying] = useState(false)
  const [scrollSpeed, setScrollSpeed] = useState(3) // 1 to 10
  const [fontSize, setFontSize] = useState(42) // px
  const [isMirrored, setIsMirrored] = useState(false)
  const [showControls, setShowControls] = useState(true)
  const [showShortcutsHelp, setShowShortcutsHelp] = useState(false)

  const scrollContainerRef = useRef<HTMLDivElement>(null)
  const scrollPosRef = useRef<number>(0)
  const animationFrameRef = useRef<number | null>(null)

  // Reset when opening
  useEffect(() => {
    if (isOpen) {
      scrollPosRef.current = 0
      setIsPlaying(false)
      if (scrollContainerRef.current) {
        scrollContainerRef.current.scrollTop = 0
      }
    }
  }, [isOpen])

  // Keyboard shortcut listener
  useEffect(() => {
    if (!isOpen) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return
      }

      switch (e.code) {
        case 'Space':
          e.preventDefault()
          setIsPlaying((prev) => !prev)
          break
        case 'ArrowUp':
          e.preventDefault()
          setScrollSpeed((prev) => Math.min(prev + 1, 10))
          break
        case 'ArrowDown':
          e.preventDefault()
          setScrollSpeed((prev) => Math.max(prev - 1, 1))
          break
        case 'Equal': // + key
        case 'NumpadAdd':
          e.preventDefault()
          setFontSize((prev) => Math.min(prev + 4, 72))
          break
        case 'Minus': // - key
        case 'NumpadSubtract':
          e.preventDefault()
          setFontSize((prev) => Math.max(prev - 4, 20))
          break
        case 'KeyM':
          e.preventDefault()
          setIsMirrored((prev) => !prev)
          break
        case 'KeyR':
          e.preventDefault()
          handleReset()
          break
        case 'KeyH':
          e.preventDefault()
          setShowControls((prev) => !prev)
          break
        case 'Escape':
          e.preventDefault()
          onClose()
          break
        default:
          break
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onClose])

  // Smooth auto-scroll loop
  useEffect(() => {
    if (!isOpen || !isPlaying) {
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current)
        animationFrameRef.current = null
      }
      return
    }

    let lastTimestamp: number | null = null

    const step = (timestamp: number) => {
      if (!lastTimestamp) lastTimestamp = timestamp
      const elapsed = timestamp - lastTimestamp
      lastTimestamp = timestamp

      const container = scrollContainerRef.current
      if (container) {
        // speed scale: 1 -> 20px/s, 10 -> 200px/s
        const pxPerSecond = scrollSpeed * 20
        const delta = (pxPerSecond * elapsed) / 1000

        scrollPosRef.current += delta
        container.scrollTop = scrollPosRef.current

        // Stop if reached the end
        if (container.scrollTop + container.clientHeight >= container.scrollHeight - 5) {
          setIsPlaying(false)
          return
        }
      }

      animationFrameRef.current = requestAnimationFrame(step)
    }

    animationFrameRef.current = requestAnimationFrame(step)

    return () => {
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current)
        animationFrameRef.current = null
      }
    }
  }, [isOpen, isPlaying, scrollSpeed])

  const handleReset = () => {
    setIsPlaying(false)
    scrollPosRef.current = 0
    if (scrollContainerRef.current) {
      scrollContainerRef.current.scrollTop = 0
    }
  }

  // Handle manual scroll update
  const handleScroll = () => {
    if (scrollContainerRef.current) {
      // If user scrolls manually while playing or paused, keep scrollPosRef synced
      scrollPosRef.current = scrollContainerRef.current.scrollTop
    }
  }

  if (!isOpen) return null

  // Clean text from Markdown headers or formatting for clean speech readability
  const formattedText = content || 'Tidak ada naskah yang dimuat untuk task ini.'

  return (
    <div className="fixed inset-0 z-[100] bg-slate-950 text-slate-100 flex flex-col select-none overflow-hidden animate-in fade-in duration-200">
      {/* Reading Guide Line (Center Horizon) */}
      <div className="absolute top-1/3 left-0 right-0 h-16 border-y border-emerald-500/20 bg-emerald-500/5 pointer-events-none z-10" />

      {/* Floating HUD Controls Bar */}
      <div
        className={`fixed top-4 left-1/2 -translate-x-1/2 z-50 transition-all duration-300 ${
          showControls ? 'opacity-100 translate-y-0' : 'opacity-0 -translate-y-4 pointer-events-none'
        }`}
      >
        <div className="flex items-center gap-2 px-4 py-2.5 rounded-2xl bg-slate-900/90 backdrop-blur-md border border-slate-700/80 shadow-2xl text-xs">
          {/* Play/Pause */}
          <Button
            size="sm"
            onClick={() => setIsPlaying(!isPlaying)}
            className={`font-bold gap-1.5 px-3 h-8 shadow-sm ${
              isPlaying
                ? 'bg-amber-500 hover:bg-amber-400 text-slate-950'
                : 'bg-emerald-500 hover:bg-emerald-400 text-slate-950'
            }`}
          >
            {isPlaying ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4 fill-current" />}
            <span>{isPlaying ? 'Pause' : 'Play'}</span>
          </Button>

          {/* Speed Controls */}
          <div className="flex items-center gap-1.5 px-2 py-1 bg-slate-800/80 rounded-xl border border-slate-700">
            <span className="text-[10px] text-slate-400 font-mono">Kecepatan:</span>
            <button
              onClick={() => setScrollSpeed((s) => Math.max(s - 1, 1))}
              className="h-6 w-6 rounded-lg bg-slate-700 hover:bg-slate-600 flex items-center justify-center font-bold"
            >
              -
            </button>
            <span className="font-mono font-bold text-emerald-400 w-5 text-center">{scrollSpeed}x</span>
            <button
              onClick={() => setScrollSpeed((s) => Math.min(s + 1, 10))}
              className="h-6 w-6 rounded-lg bg-slate-700 hover:bg-slate-600 flex items-center justify-center font-bold"
            >
              +
            </button>
          </div>

          {/* Font Size Controls */}
          <div className="flex items-center gap-1.5 px-2 py-1 bg-slate-800/80 rounded-xl border border-slate-700">
            <Type className="h-3.5 w-3.5 text-slate-400" />
            <button
              onClick={() => setFontSize((f) => Math.max(f - 4, 20))}
              className="h-6 w-6 rounded-lg bg-slate-700 hover:bg-slate-600 flex items-center justify-center font-bold"
            >
              -
            </button>
            <span className="font-mono font-bold text-slate-200 w-8 text-center">{fontSize}px</span>
            <button
              onClick={() => setFontSize((f) => Math.min(f + 4, 72))}
              className="h-6 w-6 rounded-lg bg-slate-700 hover:bg-slate-600 flex items-center justify-center font-bold"
            >
              +
            </button>
          </div>

          {/* Mirror Toggle */}
          <Button
            variant="outline"
            size="sm"
            onClick={() => setIsMirrored(!isMirrored)}
            className={`h-8 px-2.5 gap-1 border-slate-700 ${
              isMirrored ? 'bg-purple-600 text-white' : 'text-slate-300 hover:text-white'
            }`}
            title="Mirror Horizontal (untuk kaca teleprompter fisik)"
          >
            <FlipHorizontal className="h-3.5 w-3.5" />
            <span>Mirror</span>
          </Button>

          {/* Reset */}
          <Button
            variant="outline"
            size="sm"
            onClick={handleReset}
            className="h-8 px-2.5 text-slate-300 hover:text-white border-slate-700"
            title="Reset ke awal (R)"
          >
            <RotateCcw className="h-3.5 w-3.5" />
          </Button>

          {/* Shortcuts Help */}
          <button
            onClick={() => setShowShortcutsHelp(!showShortcutsHelp)}
            className="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800"
            title="Shortcut Keyboard"
          >
            <HelpCircle className="h-4 w-4" />
          </button>

          {/* Close */}
          <button
            onClick={onClose}
            className="p-1.5 text-slate-400 hover:text-red-400 rounded-lg hover:bg-slate-800 ml-1"
            title="Tutup Teleprompter (Esc)"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Floating Info Header (Title & Status) */}
      <div className="absolute top-4 left-6 z-40 hidden sm:flex items-center gap-3 opacity-60 hover:opacity-100 transition-opacity">
        <div className="flex items-center gap-2">
          <div className="h-8 w-8 rounded-xl bg-red-600 text-white flex items-center justify-center">
            <Video className="h-4 w-4" />
          </div>
          <div>
            <h3 className="text-xs font-bold text-white tracking-wide">{title}</h3>
            <span className="text-[10px] text-slate-400 font-mono">Teleprompter Studio Mode</span>
          </div>
        </div>
      </div>

      {/* Keyboard Shortcuts Overlay Modal */}
      {showShortcutsHelp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 max-w-sm w-full space-y-4 shadow-2xl">
            <div className="flex items-center justify-between">
              <h4 className="text-sm font-bold text-white flex items-center gap-2">
                <HelpCircle className="h-4 w-4 text-emerald-400" />
                <span>Shortcut Keyboard Teleprompter</span>
              </h4>
              <button
                onClick={() => setShowShortcutsHelp(false)}
                className="text-slate-400 hover:text-white"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="space-y-2 text-xs text-slate-300">
              <div className="flex items-center justify-between py-1 border-b border-slate-800">
                <span>Play / Pause</span>
                <kbd className="px-2 py-0.5 rounded bg-slate-800 font-mono text-emerald-400">Space</kbd>
              </div>
              <div className="flex items-center justify-between py-1 border-b border-slate-800">
                <span>Kecepatan +/-</span>
                <kbd className="px-2 py-0.5 rounded bg-slate-800 font-mono text-emerald-400">↑ / ↓</kbd>
              </div>
              <div className="flex items-center justify-between py-1 border-b border-slate-800">
                <span>Ukuran Font +/-</span>
                <kbd className="px-2 py-0.5 rounded bg-slate-800 font-mono text-emerald-400">+ / -</kbd>
              </div>
              <div className="flex items-center justify-between py-1 border-b border-slate-800">
                <span>Mirror Horizontal</span>
                <kbd className="px-2 py-0.5 rounded bg-slate-800 font-mono text-emerald-400">M</kbd>
              </div>
              <div className="flex items-center justify-between py-1 border-b border-slate-800">
                <span>Reset ke Awal</span>
                <kbd className="px-2 py-0.5 rounded bg-slate-800 font-mono text-emerald-400">R</kbd>
              </div>
              <div className="flex items-center justify-between py-1 border-b border-slate-800">
                <span>Sembunyikan Menu HUD</span>
                <kbd className="px-2 py-0.5 rounded bg-slate-800 font-mono text-emerald-400">H</kbd>
              </div>
              <div className="flex items-center justify-between py-1">
                <span>Keluar Fullscreen</span>
                <kbd className="px-2 py-0.5 rounded bg-slate-800 font-mono text-emerald-400">Esc</kbd>
              </div>
            </div>
            <Button
              size="sm"
              onClick={() => setShowShortcutsHelp(false)}
              className="w-full bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs"
            >
              Mengerti
            </Button>
          </div>
        </div>
      )}

      {/* Main Content Area */}
      <div
        ref={scrollContainerRef}
        onScroll={handleScroll}
        className={`flex-1 overflow-y-auto px-6 sm:px-16 md:px-28 lg:px-44 pt-44 pb-96 transition-all scroll-smooth ${
          isMirrored ? 'scale-x-[-1]' : ''
        }`}
        style={{
          scrollBehavior: 'auto',
        }}
      >
        <div
          className="max-w-4xl mx-auto leading-relaxed font-semibold text-slate-100 whitespace-pre-wrap tracking-wide"
          style={{ fontSize: `${fontSize}px`, lineHeight: 1.6 }}
        >
          {formattedText}
        </div>
      </div>
    </div>
  )
}
