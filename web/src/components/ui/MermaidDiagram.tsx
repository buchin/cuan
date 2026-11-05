import { useEffect, useRef, useState, type PointerEvent } from 'react'
import {
  Check,
  Code2,
  Copy,
  LoaderCircle,
  Maximize2,
  RotateCcw,
  ZoomIn,
  ZoomOut,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { useTheme } from '../../context/ThemeContext'
import { SlideDrawer } from '../layout/SlideDrawer'
import { SurfaceHeader } from '../layout/SurfaceHeader'
import { renderMermaid } from './mermaid-renderer'

const actionClass =
  'inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-muted-foreground hover:bg-muted/60 hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-40 aria-pressed:bg-muted aria-pressed:text-foreground'

export function MermaidDiagram({ code }: { code: string }) {
  const { t } = useLanguage()
  const { resolvedTheme } = useTheme()
  const [image, setImage] = useState<{
    url: string
    width: number
    height: number
  } | null>(null)
  const [failed, setFailed] = useState(false)
  const [showSource, setShowSource] = useState(false)
  const [viewer, setViewer] = useState(false)
  const [zoom, setZoom] = useState(1)
  const [copyState, setCopyState] = useState<'idle' | 'copied' | 'failed'>(
    'idle',
  )
  const copyTimer = useRef<ReturnType<typeof setTimeout>>()
  const expand = useRef<HTMLButtonElement>(null)
  const drag = useRef<{
    x: number
    y: number
    left: number
    top: number
  } | null>(null)
  useEffect(() => () => clearTimeout(copyTimer.current), [])
  useEffect(() => {
    let current = true
    setImage(null)
    setFailed(false)
    renderMermaid(code, resolvedTheme === 'dark').then(
      (result) => {
        if (current) setImage(result)
      },
      () => {
        if (current) setFailed(true)
      },
    )
    return () => {
      current = false
    }
  }, [code, resolvedTheme])

  async function copy() {
    clearTimeout(copyTimer.current)
    try {
      await navigator.clipboard.writeText(code)
      setCopyState('copied')
    } catch {
      setCopyState('failed')
      setShowSource(true)
    }
    copyTimer.current = setTimeout(() => setCopyState('idle'), 2500)
  }
  function closeViewer() {
    setViewer(false)
    requestAnimationFrame(() => expand.current?.focus())
  }
  function pan(event: PointerEvent<HTMLDivElement>) {
    if (!drag.current) return
    event.currentTarget.scrollLeft =
      drag.current.left - (event.clientX - drag.current.x)
    event.currentTarget.scrollTop =
      drag.current.top - (event.clientY - drag.current.y)
  }
  const source = (
    <pre
      tabIndex={0}
      aria-label={t('Kode diagram')}
      className="m-0 max-h-80 overflow-auto whitespace-pre rounded-lg bg-muted/30 p-3 text-xs leading-relaxed"
    >
      <code>{code}</code>
    </pre>
  )

  return (
    <figure className="my-3 min-w-0" data-mermaid-diagram>
      <SurfaceHeader
        variant="card"
        title={t('Diagram')}
        className="px-0 py-0"
        actions={
          <>
            <button
              type="button"
              className={actionClass}
              aria-label={t('Lihat kode diagram')}
              title={t('Lihat kode diagram')}
              aria-pressed={showSource}
              onClick={() => setShowSource((value) => !value)}
            >
              <Code2 className="h-4 w-4" />
            </button>
            <button
              type="button"
              className={actionClass}
              aria-label={t(
                copyState === 'copied' ? 'Tersalin' : 'Salin kode diagram',
              )}
              title={t('Salin kode diagram')}
              onClick={copy}
            >
              {copyState === 'copied' ? (
                <Check className="h-4 w-4" />
              ) : (
                <Copy className="h-4 w-4" />
              )}
            </button>
            {image && (
              <button
                ref={expand}
                type="button"
                className={actionClass}
                aria-label={t('Perbesar diagram')}
                title={t('Perbesar diagram')}
                onClick={() => {
                  setZoom(1)
                  setViewer(true)
                }}
              >
                <Maximize2 className="h-4 w-4" />
              </button>
            )}
          </>
        }
      />
      {copyState === 'failed' && (
        <p role="status" className="my-1 text-xs text-muted-foreground">
          {t('Tidak dapat menyalin. Pilih dan salin kode diagram.')}
        </p>
      )}
      {failed && (
        <p role="status" className="my-2 text-xs text-muted-foreground">
          {t('Diagram tidak dapat ditampilkan.')}
        </p>
      )}
      {showSource || failed ? (
        source
      ) : image ? (
        <div
          className="max-h-96 overflow-auto rounded-lg bg-muted/20 p-3"
          style={{ WebkitOverflowScrolling: 'touch' }}
        >
          <img
            src={image.url}
            alt={t('Diagram')}
            className="mx-auto block h-auto w-auto max-w-full"
          />
        </div>
      ) : (
        <div
          role="status"
          aria-label={t('Memuat diagram')}
          className="flex min-h-20 items-center justify-center text-muted-foreground"
        >
          <LoaderCircle className="h-4 w-4 animate-spin motion-reduce:animate-none" />
        </div>
      )}
      <SlideDrawer
        isOpen={viewer}
        onClose={closeViewer}
        title={t('Diagram')}
        size="full"
        contentClassName="p-0 sm:p-0 overflow-hidden"
        headerActions={
          <>
            <button
              type="button"
              className={actionClass}
              aria-label={t('Perkecil')}
              disabled={zoom <= 1}
              onClick={() => setZoom((value) => Math.max(1, value - 0.5))}
            >
              <ZoomOut className="h-4 w-4" />
            </button>
            <button
              type="button"
              className={actionClass}
              aria-label={t('Atur ulang zoom')}
              onClick={() => setZoom(1)}
            >
              <RotateCcw className="h-4 w-4" />
            </button>
            <button
              type="button"
              className={actionClass}
              aria-label={t('Perbesar')}
              disabled={zoom >= 4}
              onClick={() => setZoom((value) => Math.min(4, value + 0.5))}
            >
              <ZoomIn className="h-4 w-4" />
            </button>
          </>
        }
      >
        <div
          tabIndex={0}
          aria-label={t('Geser diagram')}
          className="h-full w-full cursor-grab overflow-auto overscroll-contain active:cursor-grabbing"
          style={{ WebkitOverflowScrolling: 'touch' }}
          onPointerDown={(event) => {
            if (event.pointerType !== 'mouse' || event.button !== 0) return
            drag.current = {
              x: event.clientX,
              y: event.clientY,
              left: event.currentTarget.scrollLeft,
              top: event.currentTarget.scrollTop,
            }
            event.currentTarget.setPointerCapture(event.pointerId)
          }}
          onPointerMove={pan}
          onPointerUp={() => {
            drag.current = null
          }}
          onPointerCancel={() => {
            drag.current = null
          }}
        >
          <div className="flex min-h-full min-w-full w-max justify-center p-4 sm:p-6">
            {image && (
              <img
                src={image.url}
                alt={t('Diagram')}
                draggable={false}
                style={{ width: image.width * zoom }}
                className="block h-auto max-w-none shrink-0 select-none self-start"
              />
            )}
          </div>
        </div>
      </SlideDrawer>
    </figure>
  )
}
