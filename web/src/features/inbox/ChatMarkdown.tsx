import {
  ExtensionBoundary,
  fenceRenderer,
  markdownParsers,
  useExtensionRevision,
} from '../../extensions/sdk'
import {
  memo,
  isValidElement,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import ReactMarkdown, { defaultUrlTransform } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Check, Copy, ChevronDown, Workflow } from 'lucide-react'
import { cn } from '../../lib/utils'
import { TaskCard } from './TaskCard'
import { RoomCard } from './RoomCard'
import type { Task } from '../../api/types'
import {
  extractConversationRoomId,
  extractTaskId,
} from './RoomCard'
export { ChatPreviewCard } from './RoomCard'
import './chat-markdown.css'

// Messages can contain untrusted URLs. File paths remain readable text until
// project-aware file previews exist; never navigate to a server-local file URL.
function safeLinkUrl(url: string) {
  const safe = defaultUrlTransform(url)
  if (/[\u0000-\u0020\\]/.test(safe)) return ''
  return /^(https?:\/\/|mailto:|\/(?!\/)|#)/i.test(safe) ? safe : ''
}

function CodeBlock({ children }: { children?: ReactNode }) {
  const extensionRevision = useExtensionRevision()
  const element = isValidElement<{ children?: ReactNode; className?: string }>(
    children,
  )
    ? children
    : null
  const text = String(element?.props.children ?? '').replace(/\n$/, '')
  const language = element?.props.className?.replace(/^language-/, '') || 'text'
  const [status, setStatus] = useState<'idle' | 'copied' | 'failed'>('idle')
  const [open, setOpen] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout>>()
  useEffect(() => () => clearTimeout(timer.current), [])

  async function copy() {
    clearTimeout(timer.current)
    let copied = false
    try {
      await navigator.clipboard.writeText(text)
      copied = true
    } catch {
      const previous = document.activeElement as HTMLElement | null
      const field = document.createElement('textarea')
      field.value = text
      field.style.cssText = 'position:fixed;left:-9999px;top:0'
      document.body.appendChild(field)
      try {
        field.select()
        copied = document.execCommand('copy')
      } catch {
        /* Show actionable failure below. */
      } finally {
        field.remove()
        previous?.focus()
      }
    }
    setStatus(copied ? 'copied' : 'failed')
    timer.current = setTimeout(() => setStatus('idle'), 2500)
  }

  const Renderer = fenceRenderer(language)
  if (Renderer)
    return (
      <ExtensionBoundary
        resetKey={language + ':' + text + ':' + extensionRevision}
        fallback={
          <pre>
            <code>{text}</code>
          </pre>
        }
      >
        <Renderer code={text} language={language} />
      </ExtensionBoundary>
    )

  const isMermaid = language === 'mermaid' || language === 'flowchart'
  if (isMermaid) {
    const lineCount = text.split('\n').filter((l) => l.trim()).length
    return (
      <div className="chat-code-mermaid my-2.5 rounded-xl border border-border/80 bg-muted/30 overflow-hidden shadow-xs">
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          className="flex min-h-11 w-full cursor-pointer select-none items-center justify-between gap-3 px-3.5 py-2.5 text-left transition-colors hover:bg-muted/50 focus-visible:outline-2 focus-visible:outline-ring"
        >
          <div className="flex items-center gap-2.5 min-w-0">
            <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Workflow className="h-4 w-4" />
            </span>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-xs font-semibold text-foreground truncate">
                  Diagram Alur (Mermaid)
                </span>
                <span className="text-[10px] font-mono font-medium px-1.5 py-0.5 rounded bg-muted text-muted-foreground border border-border/50">
                  {lineCount} baris
                </span>
              </div>
              <p className="text-[11px] text-muted-foreground truncate">
                {open ? 'Sembunyikan diagram alur' : 'Klik untuk melihat diagram alur'}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <span className="text-xs font-medium text-primary hidden sm:inline">
              {open ? 'Tutup' : 'Lihat'}
            </span>
            <ChevronDown
              className={cn(
                'h-4 w-4 text-muted-foreground transition-transform duration-200',
                open && 'rotate-180 text-primary',
              )}
            />
          </div>
        </button>
        {open && (
          <div className="border-t border-border/60 bg-muted/10 p-3">
            <div className="flex items-center justify-between pb-2 text-[11px] text-muted-foreground border-b border-border/40 mb-2">
              <span className="font-mono">{language}</span>
              <button
                type="button"
                onClick={copy}
                aria-label="Copy code"
                className="inline-flex min-h-11 min-w-11 shrink-0 items-center justify-center gap-1.5 rounded px-2 py-1 text-xs hover:bg-muted focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
              >
                {status === 'copied' ? (
                  <>
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    <span className="text-emerald-600 dark:text-emerald-400">Tersalin</span>
                  </>
                ) : (
                  <>
                    <Copy className="h-3.5 w-3.5" />
                    <span>Salin</span>
                  </>
                )}
              </button>
            </div>
            <pre tabIndex={0} aria-label="Code block" className="overflow-x-auto rounded-lg bg-background/80 p-3 font-mono text-xs leading-relaxed border border-border/40">
              <code className={element?.props.className}>{text}</code>
            </pre>
          </div>
        )}
      </div>
    )
  }
  return (
    <div className="chat-code rounded-lg border border-border bg-muted/50">
      <div className="flex min-w-0 items-center justify-between gap-3 border-b border-border px-3 py-1.5 text-xs text-muted-foreground">
        <span className="truncate font-mono">{language}</span>
        <button
          type="button"
          onClick={copy}
          aria-label="Copy code"
          className="inline-flex min-h-11 min-w-11 shrink-0 items-center gap-1.5 rounded px-1.5 py-1 hover:bg-muted focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
        >
          {status === 'copied' ? (
            <Check className="h-3.5 w-3.5" />
          ) : (
            <Copy className="h-3.5 w-3.5" />
          )}
          {status === 'copied' ? 'Copied' : 'Copy'}
        </button>
      </div>
      <pre tabIndex={0} aria-label="Code block">
        <code className={element?.props.className}>{text}</code>
      </pre>
      <span
        role="status"
        className={status === 'failed' ? 'block px-3 pb-2 text-xs' : 'sr-only'}
      >
        {status === 'failed'
          ? 'Could not copy. Select the code and copy manually.'
          : status === 'copied'
            ? 'Code copied'
            : ''}
      </span>
    </div>
  )
}

function autolinkConversationPaths(raw: string): string {
  if (!raw) return ''
  // Split around code blocks and existing markdown links to avoid altering them
  const parts = raw.split(/(```[\s\S]*?```|`[^`\n]+`|\[[^\]]+\]\([^)]+\))/g)
  return parts
    .map((part, index) => {
      if (index % 2 === 1) return part
      return part.replace(
        /(^|[\s(])(\/(?:inbox|chat)\/[A-Za-z0-9_-]+)(?=[)\s.,;!?]|$)/g,
        (_match, prefix, path) => `${prefix}[${path}](${path})`
      )
    })
    .join('')
}

function prepareChatMarkdown(raw: string): string {
  if (!raw) return ''
  // Separate a block of > quotes from an immediately following reply text that lacks a blank line
  const unquoted = raw.replace(/(^>.*)(\r?\n)([^>\r\n])/gm, '$1$2\n$3')
  return autolinkConversationPaths(unquoted)
}

export const ChatMarkdown = memo(function ChatMarkdown({
  content,
  className,
  onTaskClick,
}: {
  content: string
  className?: string
  onTaskClick?: (task: Task) => void
}) {
  const extensionRevision = useExtensionRevision()
  const preparedContent = prepareChatMarkdown(content)
  return (
    <div
      className={
        className
          ? `chat-markdown min-w-0 max-w-full leading-relaxed ${className}`
          : 'chat-markdown min-w-0 max-w-full text-base leading-relaxed'
      }
    >
      <ExtensionBoundary
        resetKey={content + ':' + extensionRevision}
        fallback={
          <div className="whitespace-pre-wrap break-words">{content}</div>
        }
      >
        <ReactMarkdown
          remarkPlugins={[remarkGfm, ...markdownParsers()]}
          skipHtml
          urlTransform={safeLinkUrl}
          components={{
            pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
            blockquote: ({ children }) => (
              <blockquote
                className="chat-reply-quote"
                role="region"
                aria-label="Kutipan pesan"
              >
                <div className="chat-reply-body">{children}</div>
              </blockquote>
            ),
            a: ({ href, children }) => {
              if (!href) return <span>{children}</span>
              const taskMatch = href.split(/[?#]/)[0].match(/^\/tasks\/([A-Za-z0-9_-]+)$/)
              const taskId = taskMatch ? taskMatch[1] : extractTaskId(href)
              if (taskId) {
                return <TaskCard taskId={taskId} className="my-2" onClick={onTaskClick} />
              }
              const inboxMatch = href.split(/[?#]/)[0].match(/^\/inbox\/([A-Za-z0-9_-]+)$/)
              const chatMatch = href.split(/[?#]/)[0].match(/^\/chat\/([A-Za-z0-9_-]+)$/)
              const roomId = inboxMatch ? inboxMatch[1] : chatMatch ? chatMatch[1] : extractConversationRoomId(href)
              if (roomId) {
                return <RoomCard roomId={roomId} className="my-2" />
              }
              return (
                <a
                  href={href}
                  target={/^https?:/i.test(href) ? '_blank' : undefined}
                  rel="noopener noreferrer"
                >
                  {children}
                </a>
              )
            },
            // Avoid loading arbitrary remote images/tracking pixels from agent output.
            img: ({ alt }) => (
              <span className="text-muted-foreground">
                {alt ? `[Image: ${alt}]` : '[Image]'}
              </span>
            ),
            table: ({ children }) => (
              <div
                className="chat-table"
                role="region"
                aria-label="Message table"
                tabIndex={0}
              >
                <table>{children}</table>
              </div>
            ),
          }}
        >
          {preparedContent}
        </ReactMarkdown>
      </ExtensionBoundary>
    </div>
  )
})
