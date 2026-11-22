import React, { useState } from 'react'
import { createRoot } from 'react-dom/client'
import { LanguageProvider } from '../src/context/LanguageContext'
import { InboxActivity } from '../src/features/inbox/InboxActivity'
import './index.css'

export function Fixture() {
  const [hasStep, setHasStep] = useState(false)
  const now = new Date().toISOString()

  const runningRequest = {
    request_id: 'req-rotating-1',
    message_id: 'msg-user-1',
    reply_id: 'reply-1',
    agent_id: 'agent_cuan',
    state: 'running' as const,
    created_at: now,
    started_at: now,
    steps: hasStep
      ? [
          { seq: 1, kind: 'accepted', text: 'Message received.', at: now },
          { seq: 2, kind: 'runtime_tool_started', text: 'Membaca berkas konfigurasi proyek...', at: now },
        ]
      : [],
  }

  return (
    <LanguageProvider>
      <div className="min-h-screen bg-background text-foreground p-4 sm:p-6 flex flex-col justify-start items-center">
        <div className="w-full max-w-xl space-y-4">
          <div className="border-b border-border/40 pb-3 flex items-center justify-between">
            <span className="font-semibold text-sm">Inbox Activity Visual Verification</span>
            <button
              id="btn-toggle-step"
              className="text-xs px-2.5 py-1 rounded bg-muted hover:bg-muted/80 text-foreground cursor-pointer transition-colors"
              onClick={() => setHasStep((v) => !v)}
            >
              {hasStep ? 'Reset to Rotating' : 'Trigger Tool Step'}
            </button>
          </div>

          {/* User message matching user screenshot */}
          <div className="flex justify-end">
            <div className="wa-message-bubble wa-outgoing max-w-[85%] rounded-2xl rounded-tr-sm bg-primary text-primary-foreground px-4 py-2.5 text-sm shadow-xs">
              <p>Pindahkan + ke tengah: Inbox, Tasks, [+], Projects, Akun</p>
              <span className="block text-[10px] opacity-70 text-right mt-1">10:59 ✓</span>
            </div>
          </div>

          {/* Assistant Activity Container */}
          <div data-testid="inbox-activity-wrapper" className="max-w-[85%]">
            <InboxActivity
              request={runningRequest}
              bubble={true}
              onCancel={async () => {}}
            />
          </div>
        </div>
      </div>
    </LanguageProvider>
  )
}

const rootEl = document.getElementById('root')
if (rootEl) {
  createRoot(rootEl).render(<Fixture />)
}
