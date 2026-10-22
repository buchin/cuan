import { Link } from 'react-router-dom'
import { ChatMarkdown } from './ChatMarkdown'

// Promote a trailing paragraph of known navigation links. Never interpret model
// text as an executable action or turn arbitrary destinations into primary CTAs.
export function messageNavigation(content: string) {
  const boundary = content.lastIndexOf('\n\n')
  const tail = boundary >= 0 ? content.slice(boundary + 2) : content
  const links = [...tail.matchAll(/\[([^\]\n]+)\]\((\/[^\s)]+)\)/g)]
  if (!links.length || links.length > 3 || tail.replace(/\[([^\]\n]+)\]\((\/[^\s)]+)\)/g, '').replace(/[\s·•|]/g, '')) return { body: content, actions: [] }
  const actions = links.map(([, label, href]) => {
    const path = href.split(/[?#]/)[0]
    if (path === '/settings/integrations/github') return { label: 'Connect GitHub', href, primary: true }
    if (path === '/settings/integrations/live-chat') return { label: 'Connect Live Chat', href, primary: true }
    if (/^\/tasks\/[A-Za-z0-9_-]+$/.test(path)) return { label, href, primary: false }
    return null
  })
  if (actions.some((action) => !action)) return { body: content, actions: [] }
  return { body: boundary >= 0 ? content.slice(0, boundary).trimEnd() : '', actions: actions.filter((action) => action !== null) }
}

export function MessageContent({
  content,
  hideTaskActions = false,
}: {
  content: string
  hideTaskActions?: boolean
}) {
  const { body, actions } = messageNavigation(content)
  const visibleActions = hideTaskActions
    ? actions.filter(
        (action) =>
          !/^\/tasks\/[A-Za-z0-9_-]+$/.test(action.href.split(/[?#]/)[0]),
      )
    : actions
  return <>
    {body && <ChatMarkdown content={body} />}
    {visibleActions.length > 0 && <div className="mt-3 flex flex-wrap items-center gap-2" aria-label="Message actions">
      {visibleActions.map((action) => <Link key={action.href} to={action.href} className={action.primary
        ? 'inline-flex min-h-11 items-center justify-center rounded-lg bg-primary px-3 text-sm font-medium text-primary-foreground hover:opacity-90 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring'
        : 'inline-flex min-h-11 items-center px-2 text-sm underline underline-offset-4 hover:opacity-80 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring'}>
        {action.label}
      </Link>)}
    </div>}
  </>
}
