import { useState } from 'react'
import { IntegrationOverview } from './IntegrationOverview'
import { GitHubIntegration } from './GitHubIntegration'
import { Link, useLocation } from 'react-router-dom'

export function IntegrationsView() {
  const { pathname } = useLocation()
  const [copyState, setCopyState] = useState('')
  const snippet = `<script src="${window.location.origin}/chat/widget.js" data-product="your-website" defer></script>`
  if (pathname.endsWith('/github')) return <GitHubIntegration />
  if (!pathname.endsWith('/live-chat')) return <IntegrationOverview />
  return (
    <section
      className="min-w-0 max-w-3xl space-y-5"
      aria-label="Live Chat integration"
    >
      <Link to="/settings/integrations" className="action-secondary">
        ← Integrations
      </Link>
      <header>
        <p className="mt-2 text-sm text-muted-foreground">
          Add live chat to your website. Visitors appear in Contacts when they
          open the widget.
        </p>
      </header>
      <section className="min-w-0 space-y-3 rounded-lg border bg-card p-5">
        <h2 className="text-base font-semibold">1. Install the widget</h2>
        <p className="text-sm text-muted-foreground">
          Paste this code before your website's closing body tag. Replace
          your-website with your product or website name.
        </p>
        <pre className="max-w-full overflow-x-auto rounded-lg bg-muted p-3 text-sm select-all">
          <code>{snippet}</code>
        </pre>
        <button
          className="action-secondary"
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(snippet)
              setCopyState('Code copied.')
            } catch {
              setCopyState(
                'Could not copy. Select the code above and copy it manually.',
              )
            }
          }}
        >
          Copy widget code
        </button>
        {copyState && (
          <p role="status" className="text-sm">
            {copyState}
          </p>
        )}
      </section>
      <section className="min-w-0 space-y-3 rounded-lg border bg-card p-5">
        <h2 className="text-base font-semibold">
          2. Customer identity (optional)
        </h2>
        <p className="text-sm text-muted-foreground">
          Set data-email and data-name from the signed-in user, or run this code
          after the widget loads:
        </p>
        <pre className="max-w-full overflow-x-auto rounded-lg bg-muted p-3 text-sm select-all">
          <code>{`window.$dojoChat.identify({\n  email: 'customer@example.com',\n  name: 'Customer name'\n})`}</code>
        </pre>
        <p className="text-sm text-muted-foreground">
          This email is provided by the widget and is unverified. Matching
          emails do not merge contact histories. Do not use it for
          authentication or to separate accounts on shared browsers.
        </p>
      </section>
      <section className="space-y-3 rounded-lg border bg-card p-5">
        <h2 className="text-base font-semibold">3. Check your installation</h2>
        <ol className="list-decimal space-y-2 pl-5 text-sm">
          <li>Open your website and select Live Chat.</li>
          <li>Send a test message from the widget.</li>
          <li>
            Open Contacts, select the visitor, then choose Open conversation to
            reply.
          </li>
        </ol>
        <p className="text-sm text-muted-foreground">
          If the widget does not appear, check the installation code and whether
          your website allows scripts from {window.location.origin}.
        </p>
        <Link to="/contacts" className="action-primary">
          Open Contacts
        </Link>
      </section>
    </section>
  )
}
