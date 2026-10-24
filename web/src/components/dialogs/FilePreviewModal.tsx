import React, {
  useState,
  useEffect,
  useRef,
  useId,
  type ReactNode,
  isValidElement,
  cloneElement,
} from 'react'
import {
  X,
  Download,
  LoaderCircle,
  Copy,
  Check,
  FileText,
  FileCode,
  Image as ImageIcon,
  GitCompare,
  Terminal,
  Info,
  Lightbulb,
  AlertCircle,
  AlertTriangle,
  ShieldAlert,
  Eye,
  Code,
  FileQuestion,
  ZoomIn,
  ZoomOut,
  RefreshCw,
} from 'lucide-react'
import ReactMarkdown, { defaultUrlTransform } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { trapDialogFocus } from '../../lib/dialog'
import { getAuthToken } from '../../api/client'
import type { Attachment } from '../../features/inbox/Attachments'
import { useLanguage } from '../../context/LanguageContext'

export function isMarkdownFile(name: string): boolean {
  return /\.(md|markdown|mdown|mkdn)$/i.test(name)
}

export function isImageFile(name: string, type?: string): boolean {
  return (
    /\.(png|jpe?g|gif|webp|svg|ico|bmp|avif)$/i.test(name) ||
    (type ? type.startsWith('image/') : false)
  )
}

export function isDiffFile(name: string): boolean {
  return /\.(diff|patch)$/i.test(name)
}

export function isLogFile(name: string): boolean {
  return /\.(log)$/i.test(name)
}

export function isJsonFile(name: string, type?: string): boolean {
  return /\.json$/i.test(name) || (type ? type.includes('json') : false)
}

export function isCodeFile(name: string): boolean {
  return /\.(js|jsx|ts|tsx|mjs|cjs|go|py|rb|rs|c|cpp|h|hpp|java|kt|sh|bash|zsh|css|scss|sass|html|htm|sql|graphql|proto|yaml|yml|toml|xml)$/i.test(
    name,
  )
}

export function isTextFile(name: string, type?: string): boolean {
  return (
    /\.(txt|text|log|csv|tsv|env|ini|conf|config|diff|patch)$/i.test(name) ||
    (type ? type.startsWith('text/') : false)
  )
}

export function isPreviewableFile(name: string, type?: string): boolean {
  return (
    isMarkdownFile(name) ||
    isImageFile(name, type) ||
    isDiffFile(name) ||
    isLogFile(name) ||
    isJsonFile(name, type) ||
    isCodeFile(name) ||
    isTextFile(name, type)
  )
}

function safeLinkUrl(url: string) {
  const safe = defaultUrlTransform(url)
  if (/[\u0000-\u0020\\]/.test(safe)) return ''
  return /^(https?:\/\/|mailto:|\/(?!\/)|#)/i.test(safe) ? safe : ''
}

// Token-based syntax highlighter for common programming languages & JSON
function highlightCodeTokens(code: string): ReactNode[] {
  const tokenRegex =
    /(\/\/[^\n]*|\/\*[\s\S]*?\*\/|#[^\n]*)|("(?:\\.|[^"\\])*"|'[^']*'|`[^`]*`)|(\b(?:const|let|var|function|return|if|else|for|while|import|export|from|type|interface|class|package|func|struct|def|async|await|true|false|null|nil|SELECT|FROM|WHERE|INSERT|UPDATE|DELETE|default|case|switch|break|continue|try|catch|finally|throw|new|this|typeof|instanceof)\b)|(\b\d+(?:\.\d+)?\b)|(\b[A-Z][a-zA-Z0-9_]*\b)/g

  const nodes: ReactNode[] = []
  let lastIndex = 0
  let match: RegExpExecArray | null
  let key = 0

  while ((match = tokenRegex.exec(code)) !== null) {
    if (match.index > lastIndex) {
      nodes.push(
        <span key={key++}>{code.slice(lastIndex, match.index)}</span>,
      )
    }
    if (match[1]) {
      // Comment
      nodes.push(
        <span key={key++} className="italic text-muted-foreground">
          {match[1]}
        </span>,
      )
    } else if (match[2]) {
      // String
      nodes.push(
        <span
          key={key++}
          className="text-emerald-600 dark:text-emerald-400"
        >
          {match[2]}
        </span>,
      )
    } else if (match[3]) {
      // Keyword
      nodes.push(
        <span
          key={key++}
          className="font-semibold text-purple-600 dark:text-purple-400"
        >
          {match[3]}
        </span>,
      )
    } else if (match[4]) {
      // Number
      nodes.push(
        <span key={key++} className="text-amber-600 dark:text-amber-400">
          {match[4]}
        </span>,
      )
    } else if (match[5]) {
      // Type or Class
      nodes.push(
        <span
          key={key++}
          className="text-sky-600 dark:text-sky-400 font-medium"
        >
          {match[5]}
        </span>,
      )
    }
    lastIndex = tokenRegex.lastIndex
  }

  if (lastIndex < code.length) {
    nodes.push(<span key={key++}>{code.slice(lastIndex)}</span>)
  }

  return nodes
}

// Code block with syntax highlighting and copy button
function CodeBlock({ children }: { children?: ReactNode }) {
  const element = isValidElement<{ children?: ReactNode; className?: string }>(
    children,
  )
    ? children
    : null
  const rawText = String(element?.props.children ?? '').replace(/\n$/, '')
  const language =
    element?.props.className?.replace(/^language-/, '') || 'text'
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => () => clearTimeout(timer.current), [])

  const handleCopy = async () => {
    clearTimeout(timer.current)
    try {
      await navigator.clipboard.writeText(rawText)
      setCopied(true)
    } catch {
      const field = document.createElement('textarea')
      field.value = rawText
      field.style.cssText = 'position:fixed;left:-9999px;top:0'
      document.body.appendChild(field)
      try {
        field.select()
        document.execCommand('copy')
        setCopied(true)
      } catch {
        // Fallback failed
      } finally {
        field.remove()
      }
    }
    timer.current = setTimeout(() => setCopied(false), 2500)
  }

  return (
    <div className="my-3 overflow-hidden rounded-lg border border-border bg-muted/40 text-xs">
      <div className="flex items-center justify-between border-b border-border bg-muted/60 px-3 py-1.5 font-mono text-muted-foreground">
        <span className="font-semibold uppercase tracking-wider text-[11px]">
          {language}
        </span>
        <button
          type="button"
          onClick={handleCopy}
          aria-label={copied ? 'Code copied' : 'Copy code'}
          className="inline-flex min-h-8 min-w-8 items-center gap-1.5 rounded px-2 py-0.5 text-xs text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
        >
          {copied ? (
            <>
              <Check className="h-3.5 w-3.5 text-emerald-500" />
              <span className="text-emerald-600 dark:text-emerald-400 font-sans">
                Copied
              </span>
            </>
          ) : (
            <>
              <Copy className="h-3.5 w-3.5" />
              <span className="font-sans">Copy</span>
            </>
          )}
        </button>
      </div>
      <pre
        tabIndex={0}
        aria-label={`${language} code block`}
        className="overflow-x-auto p-3.5 font-mono text-xs leading-relaxed"
      >
        <code>{highlightCodeTokens(rawText)}</code>
      </pre>
    </div>
  )
}

// GitHub-style Alert Callout blockquote parser & renderer
function CalloutBlockquote({ children }: { children?: ReactNode }) {
  const childArray = React.Children.toArray(children)
  const firstElement = childArray.find((c) => isValidElement(c)) as
    | React.ReactElement<{ children?: ReactNode }>
    | undefined

  if (firstElement && firstElement.props && firstElement.props.children) {
    const pChildren = React.Children.toArray(firstElement.props.children)
    const firstText = pChildren[0]
    if (typeof firstText === 'string') {
      const match = firstText.match(
        /^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\](?:\s*\n|\s+)?([\s\S]*)$/i,
      )
      if (match) {
        const type = match[1].toLowerCase() as
          | 'note'
          | 'tip'
          | 'important'
          | 'warning'
          | 'caution'
        const remainder = match[2]
        const newPChildren = remainder
          ? [remainder, ...pChildren.slice(1)]
          : pChildren.slice(1)
        const updatedFirst =
          newPChildren.length > 0
            ? cloneElement(firstElement, {}, ...newPChildren)
            : null
        const restElements = childArray.filter((c) => c !== firstElement)

        const alertStyles = {
          note: {
            container:
              'border-l-4 border-blue-500 bg-blue-50/70 dark:bg-blue-950/30 text-blue-950 dark:text-blue-100',
            header: 'text-blue-600 dark:text-blue-400',
            icon: <Info className="h-4 w-4 shrink-0 text-blue-500" />,
            title: 'Note',
          },
          tip: {
            container:
              'border-l-4 border-emerald-500 bg-emerald-50/70 dark:bg-emerald-950/30 text-emerald-950 dark:text-emerald-100',
            header: 'text-emerald-600 dark:text-emerald-400',
            icon: <Lightbulb className="h-4 w-4 shrink-0 text-emerald-500" />,
            title: 'Tip',
          },
          important: {
            container:
              'border-l-4 border-purple-500 bg-purple-50/70 dark:bg-purple-950/30 text-purple-950 dark:text-purple-100',
            header: 'text-purple-600 dark:text-purple-400',
            icon: (
              <AlertCircle className="h-4 w-4 shrink-0 text-purple-500" />
            ),
            title: 'Important',
          },
          warning: {
            container:
              'border-l-4 border-amber-500 bg-amber-50/70 dark:bg-amber-950/30 text-amber-950 dark:text-amber-100',
            header: 'text-amber-600 dark:text-amber-400',
            icon: (
              <AlertTriangle className="h-4 w-4 shrink-0 text-amber-500" />
            ),
            title: 'Warning',
          },
          caution: {
            container:
              'border-l-4 border-destructive bg-destructive/10 text-destructive-foreground',
            header: 'text-destructive',
            icon: <ShieldAlert className="h-4 w-4 shrink-0 text-destructive" />,
            title: 'Caution',
          },
        }[type]

        return (
          <div
            role="region"
            aria-label={`${alertStyles.title} callout`}
            className={`my-4 rounded-r-lg p-3 text-xs leading-relaxed ${alertStyles.container}`}
          >
            <div
              className={`mb-1.5 flex items-center gap-1.5 font-semibold uppercase tracking-wider text-[11px] ${alertStyles.header}`}
            >
              {alertStyles.icon}
              <span>{alertStyles.title}</span>
            </div>
            <div className="space-y-1.5 [&>p]:leading-relaxed">
              {updatedFirst}
              {restElements}
            </div>
          </div>
        )
      }
    }
  }

  return (
    <blockquote className="my-3 border-l-4 border-muted-foreground/30 pl-4 py-1 italic text-muted-foreground text-xs leading-relaxed">
      {children}
    </blockquote>
  )
}

// Diff Viewer component with line numbers and unified additions/deletions styling
function DiffViewer({ content }: { content: string }) {
  const lines = content.split('\n')
  return (
    <div className="overflow-x-auto rounded-lg border border-border bg-muted/20 font-mono text-xs">
      <table className="w-full border-collapse">
        <tbody>
          {lines.map((line, index) => {
            const isChunk = line.startsWith('@@')
            const isAdd = line.startsWith('+') && !line.startsWith('+++')
            const isDel = line.startsWith('-') && !line.startsWith('---')
            const isHeader =
              line.startsWith('diff --git') ||
              line.startsWith('index ') ||
              line.startsWith('--- ') ||
              line.startsWith('+++ ')

            let lineStyle = 'text-foreground'
            let rowStyle = 'hover:bg-muted/40 transition-colors'
            let marker = ' '

            if (isChunk) {
              rowStyle = 'bg-sky-500/10 text-sky-700 dark:text-sky-300 font-semibold'
              lineStyle = 'text-sky-700 dark:text-sky-300'
            } else if (isAdd) {
              rowStyle = 'bg-emerald-500/15 dark:bg-emerald-950/40 text-emerald-800 dark:text-emerald-200'
              lineStyle = 'text-emerald-800 dark:text-emerald-200'
              marker = '+'
            } else if (isDel) {
              rowStyle = 'bg-rose-500/15 dark:bg-rose-950/40 text-rose-800 dark:text-rose-200'
              lineStyle = 'text-rose-800 dark:text-rose-200'
              marker = '-'
            } else if (isHeader) {
              rowStyle = 'bg-muted/60 text-muted-foreground font-semibold'
              lineStyle = 'text-muted-foreground'
            }

            return (
              <tr key={index} className={`${rowStyle} border-b border-border/30 last:border-b-0`}>
                <td className="w-10 select-none py-0.5 pr-2 pl-3 text-right text-[11px] text-muted-foreground/60 border-r border-border/40 font-mono">
                  {index + 1}
                </td>
                <td className="w-4 select-none px-1 text-center font-bold font-mono opacity-70">
                  {marker}
                </td>
                <td className="py-0.5 px-2 font-mono whitespace-pre leading-relaxed select-text">
                  <span className={lineStyle}>{line.substring(marker !== ' ' ? 1 : 0)}</span>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

// Code & Text Viewer with line numbers
function CodeWithLineNumbers({ content, language }: { content: string; language?: string }) {
  const lines = content.split('\n')
  return (
    <div
      aria-label={language ? `${language} code block` : 'code block'}
      className="overflow-x-auto rounded-lg border border-border bg-muted/20 font-mono text-xs"
    >
      <table className="w-full border-collapse">
        <tbody>
          {lines.map((line, index) => (
            <tr key={index} className="hover:bg-muted/40 transition-colors border-b border-border/20 last:border-b-0">
              <td className="w-10 select-none py-0.5 pr-2 pl-3 text-right text-[11px] text-muted-foreground/50 border-r border-border/40 font-mono">
                {index + 1}
              </td>
              <td className="py-0.5 px-3 font-mono whitespace-pre leading-relaxed select-text">
                <code>{highlightCodeTokens(line)}</code>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function FilePreviewModal({
  open,
  attachment,
  onClose,
  onDownload,
}: {
  open: boolean
  attachment: Attachment
  onClose: () => void
  onDownload?: () => void
}) {
  const { t } = useLanguage()
  const dialogRef = useRef<HTMLDialogElement>(null)
  const headingId = useId()

  const [content, setContent] = useState('')
  const [imageUrl, setImageUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState(false)
  const [viewMode, setViewMode] = useState<'preview' | 'raw'>('preview')
  const [isZoomed, setIsZoomed] = useState(false)
  const copyTimer = useRef<ReturnType<typeof setTimeout>>()
  const imageBlobUrl = useRef<string>('')

  const isMd = isMarkdownFile(attachment.name)
  const isImg = isImageFile(attachment.name, attachment.type)
  const isDiff = isDiffFile(attachment.name)
  const isLog = isLogFile(attachment.name)
  const isJson = isJsonFile(attachment.name, attachment.type)
  const isCode = isCodeFile(attachment.name)
  const isSupportedPreview = isPreviewableFile(attachment.name, attachment.type)

  const fetchContent = async () => {
    setLoading(true)
    setError('')
    try {
      const sep = attachment.url.includes('?') ? '&' : '?'
      const previewUrl = `${attachment.url}${sep}preview=1`
      const res = await fetch(previewUrl, {
        headers: { Authorization: 'Bearer ' + getAuthToken() },
      })
      if (!res.ok) {
        throw new Error(
          t('Could not load file preview.') + ` (Status: ${res.status})`,
        )
      }

      if (isImg) {
        const blob = await res.blob()
        if (imageBlobUrl.current) URL.revokeObjectURL(imageBlobUrl.current)
        const url = URL.createObjectURL(blob)
        imageBlobUrl.current = url
        setImageUrl(url)
      } else {
        const text = await res.text()
        setContent(text)
      }
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : t('File unavailable. Try downloading again.'),
      )
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!open) return
    setContent('')
    setImageUrl('')
    setError('')
    setIsZoomed(false)
    setViewMode('preview')

    if (isSupportedPreview) {
      void fetchContent()
    }

    const prevActive = document.activeElement as HTMLElement | null
    const dialog = dialogRef.current
    dialog?.showModal()

    return () => {
      dialog?.close()
      if (prevActive?.isConnected) prevActive.focus()
      clearTimeout(copyTimer.current)
      if (imageBlobUrl.current) {
        URL.revokeObjectURL(imageBlobUrl.current)
        imageBlobUrl.current = ''
      }
    }
  }, [open, attachment.id, attachment.url])

  const handleDownload = () => {
    if (onDownload) {
      onDownload()
    } else {
      const link = document.createElement('a')
      link.href = attachment.url
      link.download = attachment.name
      link.target = '_blank'
      link.click()
    }
  }

  const handleCopyContent = async () => {
    clearTimeout(copyTimer.current)
    const textToCopy = isJson && content ? getFormattedJson(content) : content
    try {
      await navigator.clipboard.writeText(textToCopy)
      setCopied(true)
    } catch {
      const ta = document.createElement('textarea')
      ta.value = textToCopy
      ta.style.cssText = 'position:fixed;left:-9999px;top:0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
      setCopied(true)
    }
    copyTimer.current = setTimeout(() => setCopied(false), 2000)
  }

  function getFormattedJson(str: string): string {
    try {
      const parsed = JSON.parse(str)
      return JSON.stringify(parsed, null, 2)
    } catch {
      return str
    }
  }

  if (!open) return null

  // Format file size
  const formattedSize =
    attachment.size < 1024
      ? `${attachment.size} B`
      : attachment.size < 1024 * 1024
        ? `${Math.round(attachment.size / 1024)} KB`
        : `${(attachment.size / (1024 * 1024)).toFixed(1)} MB`

  const displayedRawContent = isJson && content ? getFormattedJson(content) : content

  const fileTypeLabel = isMd
    ? 'Markdown'
    : isImg
      ? 'Image'
      : isDiff
        ? 'Diff'
        : isLog
          ? 'Log'
          : isJson
            ? 'JSON'
            : isCode
              ? 'Code'
              : isSupportedPreview
                ? 'Text'
                : 'Binary'

  const fileTypeIcon = isImg ? (
    <ImageIcon className="h-4 w-4" />
  ) : isDiff ? (
    <GitCompare className="h-4 w-4" />
  ) : isLog ? (
    <Terminal className="h-4 w-4" />
  ) : isCode ? (
    <FileCode className="h-4 w-4" />
  ) : isSupportedPreview ? (
    <FileText className="h-4 w-4" />
  ) : (
    <FileQuestion className="h-4 w-4" />
  )

  return (
    <dialog
      ref={dialogRef}
      aria-labelledby={headingId}
      onKeyDown={(e) => {
        e.stopPropagation()
        trapDialogFocus(e)
      }}
      onCancel={(e) => {
        e.preventDefault()
        e.stopPropagation()
        onClose()
      }}
      className="m-0 h-dvh max-h-dvh w-full max-w-full overflow-hidden border-0 bg-background text-foreground shadow-2xl backdrop:bg-black/60 sm:m-auto sm:h-auto sm:max-h-[90dvh] sm:w-[calc(100%_-_32px)] sm:max-w-4xl lg:max-w-5xl sm:rounded-2xl sm:border sm:border-border sm:bg-card sm:text-card-foreground"
    >
      <div className="flex h-full flex-col min-h-0 sm:max-h-[90dvh]">
        {/* Header */}
        <header className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-border bg-card/90 px-3 py-2.5 backdrop-blur-sm sm:px-5 sm:py-3">
          <div className="flex min-w-0 flex-1 items-center gap-2">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
              {fileTypeIcon}
            </div>
            <div className="min-w-0 flex-1">
              <h2
                id={headingId}
                className="truncate text-sm font-semibold sm:text-base"
                title={attachment.name}
              >
                {attachment.name}
              </h2>
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <span>{formattedSize}</span>
                <span>·</span>
                <span className="uppercase font-medium tracking-wider text-[11px] bg-muted/60 px-1.5 py-0.5 rounded border border-border/50">
                  {fileTypeLabel}
                </span>
              </div>
            </div>
          </div>

          <div className="flex shrink-0 items-center gap-1.5 sm:gap-2">
            {/* View mode toggle for Markdown */}
            {isMd && (
              <div className="flex items-center rounded-lg border border-border bg-muted/40 p-0.5 text-xs">
                <button
                  type="button"
                  onClick={() => setViewMode('preview')}
                  className={`inline-flex min-h-8 items-center gap-1 rounded px-2.5 py-1 text-xs font-medium transition-colors ${
                    viewMode === 'preview'
                      ? 'bg-background text-foreground shadow-xs'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                  aria-pressed={viewMode === 'preview'}
                >
                  <Eye className="h-3.5 w-3.5" />
                  <span>{t('Preview')}</span>
                </button>
                <button
                  type="button"
                  onClick={() => setViewMode('raw')}
                  className={`inline-flex min-h-8 items-center gap-1 rounded px-2.5 py-1 text-xs font-medium transition-colors ${
                    viewMode === 'raw'
                      ? 'bg-background text-foreground shadow-xs'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                  aria-pressed={viewMode === 'raw'}
                >
                  <Code className="h-3.5 w-3.5" />
                  <span>{t('Raw')}</span>
                </button>
              </div>
            )}

            {/* Copy Content Button (when text content is available) */}
            {content && !isImg && (
              <button
                type="button"
                onClick={handleCopyContent}
                className="action-secondary inline-flex min-h-9 items-center gap-1.5 text-xs font-medium"
                title={t('Copy content')}
                aria-label={t('Copy content')}
              >
                {copied ? (
                  <>
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    <span className="text-emerald-600 dark:text-emerald-400 font-sans">
                      {t('Copied')}
                    </span>
                  </>
                ) : (
                  <>
                    <Copy className="h-3.5 w-3.5" />
                    <span className="hidden sm:inline">{t('Copy content')}</span>
                  </>
                )}
              </button>
            )}

            {/* Image zoom toggle */}
            {isImg && imageUrl && (
              <button
                type="button"
                onClick={() => setIsZoomed(!isZoomed)}
                className="action-secondary inline-flex min-h-9 items-center gap-1.5 text-xs font-medium"
                title={isZoomed ? 'Fit to screen' : 'Zoom in'}
                aria-label={isZoomed ? 'Fit to screen' : 'Zoom in'}
              >
                {isZoomed ? (
                  <>
                    <ZoomOut className="h-3.5 w-3.5" />
                    <span className="hidden sm:inline">Fit</span>
                  </>
                ) : (
                  <>
                    <ZoomIn className="h-3.5 w-3.5" />
                    <span className="hidden sm:inline">Zoom</span>
                  </>
                )}
              </button>
            )}

            {/* Download Button */}
            <button
              type="button"
              onClick={handleDownload}
              className="action-primary inline-flex min-h-9 items-center gap-1.5 text-xs font-medium"
              title={t('Download')}
              aria-label={t('Download')}
            >
              <Download className="h-3.5 w-3.5 shrink-0" />
              <span>{t('Download')}</span>
            </button>

            {/* Close Button */}
            <button
              type="button"
              onClick={onClose}
              className="icon-action min-h-11 min-w-11 text-muted-foreground hover:text-foreground"
              aria-label={t('Close')}
              title={t('Close')}
            >
              <X className="h-5 w-5" />
            </button>
          </div>
        </header>

        {/* Content Area */}
        <div className="min-h-0 flex-1 overflow-y-auto p-4 sm:p-6">
          {loading && (
            <div
              role="status"
              className="flex h-64 flex-col items-center justify-center gap-3 text-sm text-muted-foreground"
            >
              <LoaderCircle className="h-8 w-8 animate-spin text-primary" />
              <p>{t('Loading preview…')}</p>
            </div>
          )}

          {error && (
            <div
              role="alert"
              className="flex h-64 flex-col items-center justify-center gap-3 rounded-lg border border-destructive/20 bg-destructive/5 p-6 text-center text-sm"
            >
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="font-medium text-destructive">{error}</p>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => void fetchContent()}
                  className="action-secondary inline-flex items-center gap-1.5 text-xs"
                >
                  <RefreshCw className="h-3.5 w-3.5" />
                  <span>Retry</span>
                </button>
                <button
                  type="button"
                  onClick={handleDownload}
                  className="action-primary inline-flex items-center gap-1.5 text-xs"
                >
                  <Download className="h-3.5 w-3.5" />
                  <span>{t('Download')}</span>
                </button>
              </div>
            </div>
          )}

          {/* Unsupported binary format fallback */}
          {!isSupportedPreview && !loading && !error && (
            <div className="flex h-64 flex-col items-center justify-center gap-4 rounded-xl border border-border bg-muted/20 p-8 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
                <FileQuestion className="h-8 w-8" />
              </div>
              <div className="max-w-md space-y-1">
                <h3 className="font-semibold text-foreground text-sm sm:text-base">
                  {attachment.name}
                </h3>
                <p className="text-xs text-muted-foreground">
                  {t('Preview unavailable for binary file.')} ({formattedSize})
                </p>
              </div>
              <button
                type="button"
                onClick={handleDownload}
                className="action-primary inline-flex min-h-11 items-center gap-2 px-5 py-2.5 text-sm font-medium"
              >
                <Download className="h-4 w-4" />
                <span>{t('Download file')}</span>
              </button>
            </div>
          )}

          {!loading && !error && isSupportedPreview && (
            <>
              {/* 1. Image Preview */}
              {isImg && (
                <div className="flex flex-col items-center justify-center min-h-[300px] w-full overflow-auto rounded-lg border border-border bg-muted/10 p-4">
                  {imageUrl ? (
                    <img
                      src={imageUrl}
                      alt={attachment.name}
                      className={`max-w-full rounded-md object-contain transition-all duration-200 ${
                        isZoomed
                          ? 'max-h-none w-auto cursor-zoom-out'
                          : 'max-h-[65dvh] w-auto cursor-zoom-in'
                      }`}
                      onClick={() => setIsZoomed(!isZoomed)}
                    />
                  ) : (
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      <LoaderCircle className="h-4 w-4 animate-spin" />
                      <span>{t('Loading preview…')}</span>
                    </div>
                  )}
                </div>
              )}

              {/* 2. Markdown Rich Preview */}
              {isMd && viewMode === 'preview' && (
                <article className="prose prose-sm dark:prose-invert max-w-none space-y-3 leading-relaxed text-foreground select-text">
                  <ReactMarkdown
                    remarkPlugins={[remarkGfm]}
                    urlTransform={safeLinkUrl}
                    components={{
                      h1: ({ children }) => (
                        <h1 className="border-b border-border pb-2 text-xl font-bold tracking-tight text-foreground sm:text-2xl mt-2 mb-4">
                          {children}
                        </h1>
                      ),
                      h2: ({ children }) => (
                        <h2 className="border-b border-border pb-1.5 text-lg font-semibold tracking-tight text-foreground sm:text-xl mt-6 mb-3">
                          {children}
                        </h2>
                      ),
                      h3: ({ children }) => (
                        <h3 className="text-base font-semibold text-foreground sm:text-lg mt-5 mb-2">
                          {children}
                        </h3>
                      ),
                      h4: ({ children }) => (
                        <h4 className="text-sm font-semibold text-foreground sm:text-base mt-4 mb-2">
                          {children}
                        </h4>
                      ),
                      p: ({ children }) => (
                        <p className="my-2 text-xs sm:text-sm leading-relaxed text-foreground/90">
                          {children}
                        </p>
                      ),
                      ul: ({ children }) => (
                        <ul className="my-2 ml-5 list-disc space-y-1 text-xs sm:text-sm text-foreground/90">
                          {children}
                        </ul>
                      ),
                      ol: ({ children }) => (
                        <ol className="my-2 ml-5 list-decimal space-y-1 text-xs sm:text-sm text-foreground/90">
                          {children}
                        </ol>
                      ),
                      li: ({ children }) => (
                        <li className="leading-relaxed">{children}</li>
                      ),
                      code: ({ className, children }) => {
                        const isInline = !className
                        if (isInline) {
                          return (
                            <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-[11px] sm:text-xs text-foreground font-medium">
                              {children}
                            </code>
                          )
                        }
                        return <CodeBlock>{children}</CodeBlock>
                      },
                      pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
                      blockquote: ({ children }) => (
                        <CalloutBlockquote>{children}</CalloutBlockquote>
                      ),
                      table: ({ children }) => (
                        <div className="my-4 overflow-x-auto rounded-lg border border-border">
                          <table className="w-full border-collapse text-left text-xs sm:text-sm">
                            {children}
                          </table>
                        </div>
                      ),
                      thead: ({ children }) => (
                        <thead className="border-b border-border bg-muted/60 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                          {children}
                        </thead>
                      ),
                      th: ({ children }) => (
                        <th className="px-3 py-2 font-semibold text-foreground">
                          {children}
                        </th>
                      ),
                      td: ({ children }) => (
                        <td className="border-t border-border px-3 py-2 text-foreground/90">
                          {children}
                        </td>
                      ),
                      tr: ({ children }) => (
                        <tr className="hover:bg-muted/20 transition-colors">
                          {children}
                        </tr>
                      ),
                      hr: () => <hr className="my-6 border-border" />,
                      a: ({ href, children }) =>
                        href ? (
                          <a
                            href={href}
                            target={
                              /^https?:/i.test(href) ? '_blank' : undefined
                            }
                            rel="noopener noreferrer"
                            className="font-medium text-primary underline underline-offset-2 hover:opacity-80"
                          >
                            {children}
                          </a>
                        ) : (
                          <span>{children}</span>
                        ),
                      input: ({ type, checked, disabled }) => {
                        if (type === 'checkbox') {
                          return (
                            <input
                              type="checkbox"
                              checked={checked}
                              disabled={disabled}
                              className="mr-2 h-4 w-4 rounded border-border accent-primary align-middle"
                            />
                          )
                        }
                        return null
                      },
                    }}
                  >
                    {content}
                  </ReactMarkdown>
                </article>
              )}

              {/* 3. Diff Viewer */}
              {isDiff && viewMode === 'preview' && (
                <div className="space-y-2">
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span className="font-mono">{attachment.name}</span>
                    <button
                      type="button"
                      onClick={handleCopyContent}
                      className="action-secondary inline-flex items-center gap-1.5 text-xs"
                      aria-label={t('Copy content')}
                    >
                      {copied ? (
                        <>
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                          <span className="text-emerald-600 dark:text-emerald-400">
                            {t('Copied')}
                          </span>
                        </>
                      ) : (
                        <>
                          <Copy className="h-3.5 w-3.5" />
                          <span>{t('Copy content')}</span>
                        </>
                      )}
                    </button>
                  </div>
                  <DiffViewer content={content} />
                </div>
              )}

              {/* 4. Code / Text / Raw Viewer with Line Numbers */}
              {((isMd && viewMode === 'raw') ||
                (isDiff && viewMode === 'raw') ||
                (!isMd && !isDiff && !isImg)) && (
                <div className="space-y-2">
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span className="font-mono">
                      {isLog
                        ? 'Console/Log View'
                        : isJson
                          ? 'Formatted JSON'
                          : isCode
                            ? 'Source Code'
                            : 'Text'}
                    </span>
                    <button
                      type="button"
                      onClick={handleCopyContent}
                      className="action-secondary inline-flex items-center gap-1.5 text-xs"
                      aria-label={t('Copy content')}
                    >
                      {copied ? (
                        <>
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                          <span className="text-emerald-600 dark:text-emerald-400">
                            {t('Copied')}
                          </span>
                        </>
                      ) : (
                        <>
                          <Copy className="h-3.5 w-3.5" />
                          <span>{t('Copy content')}</span>
                        </>
                      )}
                    </button>
                  </div>
                  <CodeWithLineNumbers
                    content={displayedRawContent}
                    language={isJson ? 'json' : isCode ? 'code' : isLog ? 'log' : 'text'}
                  />
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </dialog>
  )
}
