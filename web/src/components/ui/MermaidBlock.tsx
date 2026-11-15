import {
  useState,
  useEffect,
  useRef,
  useCallback,
} from 'react'
import { createPortal } from 'react-dom'
import {
  Workflow,
  Copy,
  Check,
  Maximize2,
  ZoomIn,
  ZoomOut,
  RotateCcw,
  Code,
  AlertTriangle,
  X,
  Eye,
} from 'lucide-react'
import { cn } from '../../lib/utils'

export interface MermaidBlockProps {
  code: string
  className?: string
  caption?: string
}

let idCounter = 0
function generateMermaidId(): string {
  idCounter += 1
  return `mermaid-${Date.now()}-${idCounter}`
}

function normalizeMermaidCode(raw: string): string {
  let cleaned = (raw || '').trim()
  if (cleaned.startsWith('```mermaid')) {
    cleaned = cleaned.replace(/^```mermaid\s*\n?/, '')
  } else if (cleaned.startsWith('```')) {
    cleaned = cleaned.replace(/^```[a-z]*\s*\n?/, '')
  }
  if (cleaned.endsWith('```')) {
    cleaned = cleaned.replace(/\n?```$/, '')
  }
  return cleaned.trim()
}

let mermaidPromise: Promise<any> | null = null

function loadMermaid(): Promise<any> {
  if (typeof window === 'undefined') {
    return Promise.reject(new Error('Window is not defined'))
  }
  if ((window as any).mermaid) {
    return Promise.resolve((window as any).mermaid)
  }
  if (!mermaidPromise) {
    mermaidPromise = new Promise((resolve, reject) => {
      const existing = document.querySelector('script[src*="mermaid.min.js"]') as HTMLScriptElement
      if (existing) {
        if ((window as any).mermaid) {
          resolve((window as any).mermaid)
        } else {
          existing.addEventListener('load', () => resolve((window as any).mermaid))
          existing.addEventListener('error', (e) => reject(e))
        }
        return
      }

      const script = document.createElement('script')
      script.src = '/vendor/mermaid/mermaid.min.js'
      script.async = true
      script.onload = () => {
        if ((window as any).mermaid) {
          resolve((window as any).mermaid)
        } else {
          reject(new Error('Pustaka Mermaid tidak terdefinisi'))
        }
      }
      script.onerror = () => {
        mermaidPromise = null
        reject(new Error('Gagal memuat berkas script Mermaid'))
      }
      document.head.appendChild(script)
    })
  }
  return mermaidPromise
}

async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    const previous = document.activeElement as HTMLElement | null
    const field = document.createElement('textarea')
    field.value = text
    field.style.cssText = 'position:fixed;left:-9999px;top:0'
    document.body.appendChild(field)
    try {
      field.select()
      return document.execCommand('copy')
    } catch {
      return false
    } finally {
      field.remove()
      previous?.focus()
    }
  }
}

export function MermaidBlock({
  code,
  className,
  caption,
}: MermaidBlockProps) {
  const normalizedCode = normalizeMermaidCode(code)
  const [svg, setSvg] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [viewMode, setViewMode] = useState<'diagram' | 'code'>('diagram')
  const [copied, setCopied] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [zoom, setZoom] = useState(1)
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const [isPanning, setIsPanning] = useState(false)
  const panStartRef = useRef({ x: 0, y: 0 })
  const copyTimerRef = useRef<ReturnType<typeof setTimeout>>()

  const lineCount = normalizedCode.split('\n').filter((l) => l.trim()).length

  useEffect(() => {
    return () => {
      if (copyTimerRef.current) clearTimeout(copyTimerRef.current)
    }
  }, [])

  const handleCopy = useCallback(async () => {
    if (copyTimerRef.current) clearTimeout(copyTimerRef.current)
    const success = await copyToClipboard(normalizedCode)
    if (success) {
      setCopied(true)
      copyTimerRef.current = setTimeout(() => setCopied(false), 2200)
    }
  }, [normalizedCode])

  useEffect(() => {
    let isCancelled = false
    const id = generateMermaidId()

    async function renderMermaid() {
      if (!normalizedCode) {
        setSvg(null)
        setError(null)
        setLoading(false)
        return
      }

      setLoading(true)
      setError(null)

      try {
        const mermaid = await loadMermaid()
        const isDark =
          typeof document !== 'undefined' &&
          document.documentElement.classList.contains('dark')

        mermaid.initialize({
          startOnLoad: false,
          securityLevel: 'strict',
          fontFamily: 'inherit',
          theme: isDark ? 'dark' : 'neutral',
          logLevel: 'error',
        })

        const result = await mermaid.render(id, normalizedCode)
        if (!isCancelled) {
          setSvg(result.svg)
          setError(null)
          setLoading(false)
        }
      } catch (err: any) {
        if (!isCancelled) {
          setError(err?.message || 'Sintaks diagram Mermaid tidak valid')
          setSvg(null)
          setLoading(false)
        }
      } finally {
        if (typeof document !== 'undefined') {
          const stray1 = document.getElementById(id)
          const stray2 = document.getElementById('d' + id)
          stray1?.remove()
          stray2?.remove()
        }
      }
    }

    renderMermaid()

    return () => {
      isCancelled = true
    }
  }, [normalizedCode])

  // Reset zoom & pan when fullscreen opens or closes
  const openFullscreen = useCallback(() => {
    setZoom(1)
    setPan({ x: 0, y: 0 })
    setIsFullscreen(true)
  }, [])

  const closeFullscreen = useCallback(() => {
    setIsFullscreen(false)
    setZoom(1)
    setPan({ x: 0, y: 0 })
  }, [])

  // Listen for Esc key in fullscreen
  useEffect(() => {
    if (!isFullscreen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        closeFullscreen()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isFullscreen, closeFullscreen])

  // Mouse pan handlers for fullscreen
  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    setIsPanning(true)
    panStartRef.current = { x: e.clientX - pan.x, y: e.clientY - pan.y }
  }, [pan])

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    if (!isPanning) return
    setPan({
      x: e.clientX - panStartRef.current.x,
      y: e.clientY - panStartRef.current.y,
    })
  }, [isPanning])

  const handleMouseUp = useCallback(() => {
    setIsPanning(false)
  }, [])

  return (
    <div
      className={cn(
        'chat-code-mermaid my-3 rounded-xl border border-border/80 bg-muted/20 dark:bg-muted/10 overflow-hidden shadow-xs transition-colors',
        className
      )}
      role="figure"
      aria-label="Diagram alur Mermaid"
    >
      {/* Header Bar */}
      <div className="flex min-h-11 items-center justify-between gap-2 border-b border-border/60 bg-muted/40 px-3 py-2 text-xs">
        <div className="flex items-center gap-2 min-w-0">
          <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Workflow className="h-4 w-4" />
          </span>
          <div className="min-w-0 flex items-center gap-2">
            <span className="font-semibold text-foreground truncate">
              {caption || 'Diagram Alur (Mermaid)'}
            </span>
            <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground border border-border/40 shrink-0">
              {lineCount} baris
            </span>
          </div>
        </div>

        {/* Action Controls */}
        <div className="flex items-center gap-1 shrink-0">
          {/* Toggle View Mode: Diagram vs Kode */}
          {!error && (
            <div className="inline-flex rounded-lg border border-border/60 bg-background/80 p-0.5 text-[11px]">
              <button
                type="button"
                onClick={() => setViewMode('diagram')}
                className={cn(
                  'inline-flex min-h-8 items-center gap-1 rounded-md px-2 py-1 font-medium transition-colors cursor-pointer',
                  viewMode === 'diagram'
                    ? 'bg-muted text-foreground shadow-2xs font-semibold'
                    : 'text-muted-foreground hover:text-foreground'
                )}
                aria-pressed={viewMode === 'diagram'}
              >
                <Eye className="h-3.5 w-3.5" />
                <span className="hidden sm:inline">Visual</span>
              </button>
              <button
                type="button"
                onClick={() => setViewMode('code')}
                className={cn(
                  'inline-flex min-h-8 items-center gap-1 rounded-md px-2 py-1 font-medium transition-colors cursor-pointer',
                  viewMode === 'code'
                    ? 'bg-muted text-foreground shadow-2xs font-semibold'
                    : 'text-muted-foreground hover:text-foreground'
                )}
                aria-pressed={viewMode === 'code'}
              >
                <Code className="h-3.5 w-3.5" />
                <span className="hidden sm:inline">Sintaks</span>
              </button>
            </div>
          )}

          {/* Copy Button */}
          <button
            type="button"
            onClick={handleCopy}
            aria-label="Salin kode diagram"
            className="inline-flex min-h-9 min-w-9 items-center justify-center gap-1.5 rounded-lg px-2.5 py-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
            title="Salin sintaks Mermaid"
          >
            {copied ? (
              <>
                <Check className="h-3.5 w-3.5 text-emerald-500" />
                <span className="text-emerald-600 dark:text-emerald-400 font-medium hidden sm:inline">
                  Tersalin
                </span>
              </>
            ) : (
              <>
                <Copy className="h-3.5 w-3.5" />
                <span className="hidden sm:inline">Salin</span>
              </>
            )}
          </button>

          {/* Pop-up / Fullscreen Viewer Button */}
          {svg && !error && (
            <button
              type="button"
              onClick={openFullscreen}
              aria-label="Perbesar diagram (fullscreen)"
              className="inline-flex min-h-9 min-w-9 items-center justify-center gap-1 rounded-lg px-2 py-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
              title="Perbesar / Buka Viewer Interaktif"
            >
              <Maximize2 className="h-3.5 w-3.5" />
              <span className="hidden sm:inline">Perbesar</span>
            </button>
          )}
        </div>
      </div>

      {/* Body Area */}
      {error ? (
        /* Graceful Fallback on Invalid Syntax */
        <div className="p-3 space-y-2.5">
          <div className="flex items-center gap-2 rounded-lg bg-amber-500/10 border border-amber-500/20 px-3 py-2 text-xs text-amber-700 dark:text-amber-400">
            <AlertTriangle className="h-4 w-4 shrink-0 text-amber-500" />
            <div className="min-w-0">
              <p className="font-semibold">Sintaks diagram Mermaid tidak valid</p>
              <p className="text-[11px] opacity-80 truncate">
                {error.split('\n')[0] || 'Tidak dapat merender diagram visual.'}
              </p>
            </div>
          </div>
          <pre
            tabIndex={0}
            aria-label="Mermaid raw syntax fallback"
            className="overflow-x-auto rounded-lg bg-background/90 p-3 font-mono text-xs leading-relaxed border border-border/50 text-foreground"
          >
            <code>{normalizedCode}</code>
          </pre>
        </div>
      ) : loading ? (
        /* Calm Loading Skeleton */
        <div className="flex items-center justify-center p-8 text-xs text-muted-foreground animate-pulse gap-2">
          <Workflow className="h-4 w-4 animate-spin text-primary/70" />
          <span>Menyiapkan diagram visual...</span>
        </div>
      ) : viewMode === 'code' ? (
        /* Raw Syntax Code View */
        <div className="bg-background/80 p-3">
          <pre
            tabIndex={0}
            aria-label="Mermaid syntax"
            className="overflow-x-auto rounded-lg bg-muted/40 p-3 font-mono text-xs leading-relaxed border border-border/40 text-foreground"
          >
            <code>{normalizedCode}</code>
          </pre>
        </div>
      ) : (
        /* Responsive Visual SVG Diagram */
        <div
          className="mermaid-scroll-wrapper w-full max-w-full overflow-x-auto p-4 flex justify-center items-center"
          style={{
            WebkitOverflowScrolling: 'touch',
            touchAction: 'pan-x pan-y',
          }}
        >
          <div
            className="mermaid-svg-container w-full flex justify-center [&>svg]:max-w-full [&>svg]:h-auto [&>svg]:mx-auto"
            dangerouslySetInnerHTML={{ __html: svg || '' }}
          />
        </div>
      )}

      {/* Interactive Fullscreen / Pop-up Viewer Modal */}
      {isFullscreen &&
        typeof document !== 'undefined' &&
        createPortal(
          <div
            className="fixed inset-0 z-50 flex flex-col bg-background/95 backdrop-blur-md"
            role="dialog"
            aria-modal="true"
            aria-label="Pratinjau Diagram Mermaid"
          >
            {/* Modal Header */}
            <div className="flex min-h-12 items-center justify-between border-b border-border/60 bg-muted/30 px-4 py-2.5">
              <div className="flex items-center gap-2.5">
                <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
                  <Workflow className="h-4 w-4" />
                </span>
                <div>
                  <h2 className="text-sm font-semibold text-foreground">
                    {caption || 'Diagram Alur (Mermaid)'}
                  </h2>
                  <p className="text-[11px] text-muted-foreground">
                    Mode Viewer Interaktif (Zoom & Geser)
                  </p>
                </div>
              </div>

              {/* Center Zoom Controls */}
              <div className="flex items-center gap-1 rounded-xl border border-border/60 bg-background/90 px-2 py-1 shadow-xs">
                <button
                  type="button"
                  onClick={() => setZoom((z) => Math.max(0.4, Number((z - 0.2).toFixed(1))))}
                  disabled={zoom <= 0.4}
                  aria-label="Perkecil zoom"
                  className="inline-flex min-h-9 min-w-9 items-center justify-center rounded-lg text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-40 transition-colors cursor-pointer"
                  title="Perkecil (Zoom Out)"
                >
                  <ZoomOut className="h-4 w-4" />
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setZoom(1)
                    setPan({ x: 0, y: 0 })
                  }}
                  aria-label="Atur ulang zoom 100%"
                  className="inline-flex min-h-9 items-center gap-1 rounded-lg px-2 text-xs font-mono font-medium text-foreground hover:bg-muted transition-colors cursor-pointer"
                  title="Reset Zoom (100%)"
                >
                  <RotateCcw className="h-3.5 w-3.5 text-muted-foreground" />
                  <span>{Math.round(zoom * 100)}%</span>
                </button>
                <button
                  type="button"
                  onClick={() => setZoom((z) => Math.min(3, Number((z + 0.2).toFixed(1))))}
                  disabled={zoom >= 3}
                  aria-label="Perbesar zoom"
                  className="inline-flex min-h-9 min-w-9 items-center justify-center rounded-lg text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-40 transition-colors cursor-pointer"
                  title="Perbesar (Zoom In)"
                >
                  <ZoomIn className="h-4 w-4" />
                </button>
              </div>

              {/* Right Controls */}
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={handleCopy}
                  aria-label="Salin sintaks diagram"
                  className="inline-flex min-h-9 min-w-9 items-center justify-center gap-1.5 rounded-lg border border-border/60 bg-background px-3 py-1 text-xs font-medium text-foreground hover:bg-muted transition-colors cursor-pointer"
                >
                  {copied ? (
                    <>
                      <Check className="h-3.5 w-3.5 text-emerald-500" />
                      <span className="text-emerald-600 dark:text-emerald-400 hidden sm:inline">
                        Tersalin
                      </span>
                    </>
                  ) : (
                    <>
                      <Copy className="h-3.5 w-3.5" />
                      <span className="hidden sm:inline">Salin</span>
                    </>
                  )}
                </button>
                <button
                  type="button"
                  onClick={closeFullscreen}
                  aria-label="Tutup pratinjau"
                  className="inline-flex min-h-9 min-w-9 items-center justify-center rounded-lg border border-border/60 bg-background text-muted-foreground hover:bg-muted hover:text-foreground transition-colors cursor-pointer"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
            </div>

            {/* Modal Body Canvas */}
            <div
              className="relative flex-1 overflow-auto p-4 sm:p-8 flex items-center justify-center cursor-grab active:cursor-grabbing select-none"
              onMouseDown={handleMouseDown}
              onMouseMove={handleMouseMove}
              onMouseUp={handleMouseUp}
              onMouseLeave={handleMouseUp}
              style={{
                WebkitOverflowScrolling: 'touch',
                touchAction: 'none',
              }}
            >
              <div
                className="transition-transform duration-75 flex justify-center items-center max-w-none"
                style={{
                  transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
                  transformOrigin: 'center center',
                }}
                dangerouslySetInnerHTML={{ __html: svg || '' }}
              />
            </div>

            {/* Modal Footer Hint */}
            <div className="flex items-center justify-between border-t border-border/40 bg-muted/20 px-4 py-2 text-[11px] text-muted-foreground">
              <span>Gunakan kontrol zoom atau seret mouse/layar sentuh untuk menggeser.</span>
              <span className="font-mono hidden sm:inline">Tekan Esc untuk menutup</span>
            </div>
          </div>,
          document.body
        )}
    </div>
  )
}
