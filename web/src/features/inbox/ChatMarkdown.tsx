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
import { Check, Copy } from 'lucide-react'
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

function prepareChatMarkdown(raw: string): string {
  if (!raw) return ''
  // Separate a block of > quotes from an immediately following reply text that lacks a blank line
  return raw.replace(/(^>.*)(\r?\n)([^>\r\n])/gm, '$1$2\n$3')
}

export const ChatMarkdown = memo(function ChatMarkdown({
  content,
  className,
}: {
  content: string
  className?: string
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
            a: ({ href, children }) =>
              href ? (
                <a
                  href={href}
                  target={/^https?:/i.test(href) ? '_blank' : undefined}
                  rel="noopener noreferrer"
                >
                  {children}
                </a>
              ) : (
                <span>{children}</span>
              ),
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
