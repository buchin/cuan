import { Link } from 'react-router-dom'
import { CheckSquare, ArrowRight } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { ChatMarkdown } from './ChatMarkdown'
import { StatusReportCard, parseTaskReportMessage } from '../../components/ui/StatusReportCard'
import { TaskCard } from './TaskCard'
import { RoomCard } from './RoomCard'
import {
  ChatPreviewCard,
  extractConversationRoomId,
  extractTaskId,
} from './RoomCard'
import { ReceiptPreviewCard, parseReceiptExtraction } from './ReceiptPreviewCard'
import type { ReceiptExtraction } from '../../api/appsSkills'

// Promote a trailing paragraph of known navigation links. Never interpret model
// text as an executable action or turn arbitrary destinations into primary CTAs.
export function messageNavigation(content: string) {
  const boundary = content.lastIndexOf('\n\n')
  const tail = boundary >= 0 ? content.slice(boundary + 2) : content
  const linkRegex = /\[([^\]\n]+)\]\((https?:\/\/[^\s)]+|\/[^\s)]+)\)/g
  const links = [...tail.matchAll(linkRegex)]
  if (!links.length || links.length > 3 || tail.replace(linkRegex, '').replace(/[\s·•|]/g, '')) return { body: content, actions: [] }
  const actions = links.map(([, label, href]) => {
    const path = href.split(/[?#]/)[0]
    if (path === '/settings/integrations/github') return { label: 'Connect GitHub', href, primary: true }
    if (path === '/settings/integrations/live-chat') return { label: 'Connect Live Chat', href, primary: true }
    if (/^\/tasks\/[A-Za-z0-9_-]+$/.test(path) || extractTaskId(href)) return { label, href, primary: false }
    if (/^\/inbox\/[A-Za-z0-9_-]+$/.test(path) || /^\/chat\/[A-Za-z0-9_-]+$/.test(path) || extractConversationRoomId(href)) return { label, href, primary: false }
    return null
  })
  if (actions.some((action) => !action)) return { body: content, actions: [] }
  return { body: boundary >= 0 ? content.slice(0, boundary).trimEnd() : '', actions: actions.filter((action) => action !== null) }
}

export function MessageContent({
  content,
  hideTaskActions = false,
  receiptExtraction,
}: {
  content: string
  hideTaskActions?: boolean
  receiptExtraction?: ReceiptExtraction
}) {
  const { language, t } = useLanguage()
  const { body, actions } = messageNavigation(content)
  const visibleActions = hideTaskActions
    ? actions.filter(
        (action) =>
          !extractTaskId(action.href) &&
          !/^\/tasks\/[A-Za-z0-9_-]+$/.test(action.href.split(/[?#]/)[0]),
      )
    : actions

  const report = body ? parseTaskReportMessage(body, language) : null
  const extractedReceipt = receiptExtraction || (body ? parseReceiptExtraction(body) : null)
  const displayBody = extractedReceipt
    ? body.replace(/```(?:cuan-receipt|receipt)\s*[\s\S]*?```/, '').trim()
    : body

  return (
    <>
      {report ? (
        <div className="space-y-2.5 w-full min-w-0 max-w-full">
          <StatusReportCard report={report} />
          {report.dodDetails?.remainingText && (
            <div className="pt-1">
              <ChatMarkdown content={report.dodDetails.remainingText} />
            </div>
          )}
        </div>
      ) : (
        displayBody && <ChatMarkdown content={displayBody} />
      )}
      {extractedReceipt && (
        <ReceiptPreviewCard extraction={extractedReceipt} />
      )}
      {visibleActions.length > 0 && (
        <div
          className="mt-3 flex flex-col gap-2.5 w-full max-w-full"
          aria-label="Message actions"
        >
          {visibleActions.map((action) => {
            const taskMatch = action.href.split(/[?#]/)[0].match(/^\/tasks\/([A-Za-z0-9_-]+)$/)
            const taskId = taskMatch ? taskMatch[1] : extractTaskId(action.href)
            if (taskId) {
              return <TaskCard key={action.href} taskId={taskId} />
            }
            const inboxMatch = action.href.split(/[?#]/)[0].match(/^\/inbox\/([A-Za-z0-9_-]+)$/)
            const chatMatch = action.href.split(/[?#]/)[0].match(/^\/chat\/([A-Za-z0-9_-]+)$/)
            const roomId = inboxMatch ? inboxMatch[1] : chatMatch ? chatMatch[1] : extractConversationRoomId(action.href)
            if (roomId) {
              return <RoomCard key={action.href} roomId={roomId} />
            }
            return (
              <div key={action.href} className="flex flex-wrap items-center gap-2">
                <Link
                  to={action.href}
                  className={
                    action.primary
                      ? 'inline-flex min-h-11 min-w-[44px] items-center justify-center rounded-lg bg-primary px-3.5 text-xs font-semibold text-primary-foreground hover:opacity-90 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring shadow-xs'
                      : 'inline-flex min-h-11 min-w-[44px] items-center gap-1.5 rounded-lg border border-border/80 bg-background/80 px-3 py-1.5 text-xs font-semibold text-foreground hover:bg-muted/70 transition-colors shadow-xs'
                  }
                >
                  {!action.primary && (
                    <CheckSquare className="h-3.5 w-3.5 text-primary" />
                  )}
                  <span>{t(action.label)}</span>
                  {!action.primary && (
                    <ArrowRight className="h-3 w-3 opacity-60 ml-0.5" />
                  )}
                </Link>
              </div>
            )
          })}
        </div>
      )}
    </>
  )
}
