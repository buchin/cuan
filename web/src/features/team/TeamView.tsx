import { useLanguage } from '../../context/LanguageContext'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Bot, Plus, RefreshCw, Trash2, MoreVertical, X } from 'lucide-react'
import { connectionStatus } from '../agents/connection'
import {
  accountsApi,
  type AccountUsage,
  type ProviderAccount,
} from '../../api/accounts'
import { agentsApi, type Agent, type HarnessAuth } from '../../api/agents'

function AgentActions({
  name,
  onRemove,
  disabled,
}: {
  name: string
  onRemove: () => void
  disabled: boolean
}) {
  const [expanded, setExpanded] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (!expanded) return
    const outside = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setExpanded(false)
    }
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setExpanded(false)
        trigger.current?.focus()
      }
    }
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', key)
    return () => {
      document.removeEventListener('pointerdown', outside)
      document.removeEventListener('keydown', key)
    }
  }, [expanded])
  return (
    <div
      ref={root}
      className="relative"
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node))
          setExpanded(false)
      }}
    >
      <button
        ref={trigger}
        className="icon-action"
        aria-label={`Actions ${name}`}
        aria-expanded={expanded}
        onClick={() => setExpanded(!expanded)}
        disabled={disabled}
      >
        <MoreVertical className="h-5 w-5" />
      </button>
      {expanded && (
        <div className="absolute right-0 top-full z-20 mt-1 min-w-44 rounded-lg border bg-popover p-1 text-popover-foreground shadow-md">
          <button
            className="flex min-h-11 w-full items-center gap-2 rounded-md px-3 text-sm text-destructive hover:bg-accent"
            onClick={() => {
              setExpanded(false)
              trigger.current?.focus()
              if (window.confirm(`Delete agent ${name}?`)) onRemove()
            }}
          >
            <Trash2 className="h-4 w-4" />
            Delete {name}
          </button>
        </div>
      )}
    </div>
  )
}

type Step = 'name' | 'google' | 'save'

export function TeamView({
  selectedRole = 'all',
  showHeading = true,
  showCreateAction = true,
  createRequested = false,
  onCreateHandled,
  onAgentAdded,
  focusAgentId,
}: {
  showHeading?: boolean
  showCreateAction?: boolean
  createRequested?: boolean
  onCreateHandled?: () => void
  focusAgentId?: string
  selectedRole?: string
  onAgentAdded?: () => void
}) {
  const { t } = useLanguage()
  const [usage, setUsage] = useState<Record<string, AccountUsage>>({})
  const [usageBusy, setUsageBusy] = useState('')
  const [accounts, setAccounts] = useState<ProviderAccount[]>([])
  const [backupBusy, setBackupBusy] = useState('')
  const [backupError, setBackupError] = useState('')
  const [search, setSearch] = useState('')
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [open, setOpen] = useState(false)
  const [step, setStep] = useState<Step>('name')
  const [handle, setHandle] = useState('@dev')
  const [auth, setAuth] = useState<HarnessAuth | null>(null)
  const [code, setCode] = useState('')
  const [busy, setBusy] = useState(false)
  const [formError, setFormError] = useState('')
  const [notice, setNotice] = useState('')
  const [checking, setChecking] = useState('')
  const [removing, setRemoving] = useState('')
  const [now, setNow] = useState(Date.now())
  // Expire visible success even when this roster stays open or resumes from sleep.
  useEffect(() => {
    const update = () => setNow(Date.now())
    const timer = window.setInterval(update, 1000)
    window.addEventListener('focus', update)
    document.addEventListener('visibilitychange', update)
    return () => {
      window.clearInterval(timer)
      window.removeEventListener('focus', update)
      document.removeEventListener('visibilitychange', update)
    }
  }, [])
  const active = useRef<{ accountId: string; sessionId: string } | null>(null)
  const accountScope = useRef('')
  const epoch = useRef(0)
  const dialogActive = useRef(false)
  const addButton = useRef<HTMLButtonElement>(null)
  const createTrigger = useRef<HTMLElement | null>(null)

  const cancelSession = async () => {
    const session = active.current
    active.current = null
    if (session)
      await accountsApi
        .cancel(session.accountId, session.sessionId)
        .catch(() => {})
  }
  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setAgents(await agentsApi.list())
      try {
        setAccounts(await accountsApi.list())
        setBackupError('')
      } catch {
        setBackupError(
          'Backup account settings could not be loaded. Try again.',
        )
      }
    } catch {
      setError('Could not load agents. Try again.')
    } finally {
      setLoading(false)
    }
  }, [])
  useEffect(() => {
    void load()
  }, [load])
  useEffect(
    () => () => {
      dialogActive.current = false
      epoch.current++
      void cancelSession()
    },
    [],
  )
  useEffect(() => {
    if (!auth?.session_id || !auth.expires_at) return
    const sessionId = auth.session_id
    const timer = window.setTimeout(
      () => {
        if (active.current?.sessionId !== sessionId) return
        void cancelSession()
        setAuth(null)
        setCode('')
        setFormError('Sign-in timed out. Create a new link to continue.')
      },
      Math.max(0, Date.parse(auth.expires_at) - Date.now()),
    )
    return () => window.clearTimeout(timer)
  }, [auth?.session_id, auth?.expires_at])

  const close = () => {
    epoch.current++
    dialogActive.current = false
    void cancelSession()
    setOpen(false)
    setCode('')
    setAuth(null)
    setBusy(false)
    window.setTimeout(
      () =>
        (createTrigger.current?.isConnected
          ? createTrigger.current
          : addButton.current
        )?.focus(),
      0,
    )
  }
  const begin = () => {
    createTrigger.current = document.activeElement as HTMLElement | null
    epoch.current++
    dialogActive.current = true
    accountScope.current = ''
    setStep('name')
    setOpen(true)
    setAuth(null)
    setCode('')
    setFormError('')
    setNotice('')
    setBusy(false)
    let suffix = 1
    let next = '@dev'
    while (
      agents.some(
        (a) =>
          a.name.split(' ')[0].toLowerCase() === next ||
          a.id === 'agent_' + next.slice(1),
      )
    )
      next = `@dev${++suffix}`
    setHandle(next)
  }
  useEffect(() => {
    if (createRequested) {
      begin()
      onCreateHandled?.()
    }
  }, [createRequested])
  const connect = async () => {
    if (busy || handle.length < 2) return
    const currentEpoch = epoch.current
    setBusy(true)
    setFormError('')
    setCode('')
    try {
      await cancelSession()
      if (!dialogActive.current || epoch.current !== currentEpoch) return
      if (!accountScope.current) {
        const account = await accountsApi.create(`${handle} · Google`)
        if (!dialogActive.current || epoch.current !== currentEpoch) return
        accountScope.current = account.id
      }
      const accountId = accountScope.current
      const session = await accountsApi.login(accountId)
      if (!dialogActive.current || epoch.current !== currentEpoch) {
        if (session.session_id)
          await accountsApi
            .cancel(accountId, session.session_id)
            .catch(() => {})
        return
      }
      if (!session.auth_url || !session.session_id)
        throw new Error('The Google sign-in link is unavailable. Try again.')
      active.current = { accountId, sessionId: session.session_id }
      setAuth(session)
      setStep('google')
    } catch (err) {
      if (dialogActive.current && epoch.current === currentEpoch)
        setFormError(
          err instanceof Error
            ? err.message
            : 'Could not connect to Google. Try again.',
        )
    } finally {
      if (dialogActive.current && epoch.current === currentEpoch) setBusy(false)
    }
  }
  const finish = async (currentEpoch: number) => {
    await agentsApi.connectCLI({
      name: handle,
      handle,
      provider: 'agy',
      account_id: accountScope.current,
    })
    if (!dialogActive.current || epoch.current !== currentEpoch) return
    close()
    setNotice(`${handle} was added.`)
    await load()
    onAgentAdded?.()
  }
  const retrySave = async () => {
    const currentEpoch = epoch.current
    setBusy(true)
    setFormError('')
    try {
      await finish(currentEpoch)
    } catch (e) {
      if (dialogActive.current && epoch.current === currentEpoch)
        setFormError(
          e instanceof Error
            ? e.message
            : 'Could not save the agent. Try again.',
        )
    } finally {
      if (dialogActive.current && epoch.current === currentEpoch) setBusy(false)
    }
  }
  const check = async (id: string) => {
    setChecking(id)
    setError('')
    setNotice('')
    try {
      const result = await agentsApi.verifyConnection(id)
      setAgents((current) =>
        current.map((a) =>
          a.id === id
            ? {
                ...a,
                connection_state: result.connection_state,
                last_checked_at: result.last_checked_at,
                verified_at: result.verified_at,
              }
            : a,
        ),
      )
      const messages: Record<string, string> = {
        verified:
          'The provider responded successfully. This result is valid for five minutes.',
        quota_limited:
          'The agent has reached its quota. Try again when it resets.',
        login_required: 'The agent needs to sign in again.',
        unavailable: 'Connection interrupted. Try again later.',
      }
      setNotice(
        messages[result.connection_state] ||
          'Could not confirm the connection. Try again later.',
      )
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : 'Could not check the connection. Try again.',
      )
    } finally {
      setChecking('')
    }
  }
  const verify = async () => {
    if (!active.current || !code.trim() || busy) return
    const currentEpoch = epoch.current
    const session = active.current
    setBusy(true)
    setFormError('')
    try {
      const account = await accountsApi.code(
        session.accountId,
        session.sessionId,
        code.trim(),
      )
      if (!dialogActive.current || epoch.current !== currentEpoch) return
      if (!account.credentials_present)
        throw new Error(
          'Sign-in is incomplete. Open Google and copy the code provided.',
        )
      active.current = null
      setAuth(null)
      setStep('save')
      await finish(currentEpoch)
    } catch (err) {
      if (dialogActive.current && epoch.current === currentEpoch)
        setFormError(
          err instanceof Error
            ? err.message
            : 'Could not verify the code. Try again or create a new link.',
        )
    } finally {
      if (dialogActive.current && epoch.current === currentEpoch) {
        setCode('')
        setBusy(false)
      }
    }
  }
  const remove = async (id: string) => {
    if (removing) return
    setRemoving(id)
    setError('')
    try {
      await agentsApi.delete(id)
      setAgents((current) => current.filter((a) => a.id !== id))
      onAgentAdded?.()
    } catch {
      setError('Could not delete the agent. Try again.')
    } finally {
      setRemoving('')
    }
  }
  useEffect(() => {
    if (focusAgentId && !loading)
      document
        .getElementById(`settings-agent-${focusAgentId}`)
        ?.scrollIntoView({ block: 'center' })
  }, [focusAgentId, loading])
  const visibleAgents =
    selectedRole === 'human' || selectedRole === 'product_owner' ? [] : agents
  const filteredAgents = visibleAgents.filter((a) =>
    [a.name, a.harness]
      .join(' ')
      .toLowerCase()
      .includes(search.trim().toLowerCase()),
  )
  const secondary =
    'inline-flex items-center justify-center gap-2 rounded-md border border-border px-4 py-2.5 text-sm hover:bg-muted disabled:opacity-50'
  const primary =
    'inline-flex items-center justify-center gap-2 rounded-md bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50'

  return (
    <div className="mx-auto max-w-4xl space-y-5 pb-8">
      <div className="flex items-center justify-between gap-3">
        <div>
          {showHeading && <h2 className="text-lg font-semibold">Agent</h2>}
          <p className="mt-1 text-sm text-muted-foreground">
            {t(
              'Each agent keeps its identity. Allowed backup accounts can help when a provider reaches its usage limit.',
            )}
          </p>
        </div>
        {showCreateAction && (
          <button
            ref={addButton}
            onClick={() => begin()}
            className={`${primary} shrink-0`}
          >
            <Plus className="h-4 w-4" />
            Add agent
          </button>
        )}
      </div>
      {visibleAgents.length > 0 && (
        <div className="flex flex-wrap items-center gap-2">
          <input
            className="field-control min-w-0 flex-1"
            aria-label="Search agents"
            placeholder="Search by name or agent type"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          {search.trim() && (
            <button className="action-secondary" onClick={() => setSearch('')}>
              Clear search
            </button>
          )}
        </div>
      )}
      {notice && (
        <p role="status" className="text-sm">
          {notice}
        </p>
      )}
      {error && (
        <div role="alert" className="text-sm text-destructive">
          {error}{' '}
          <button onClick={load} className="underline">
            Try again
          </button>
        </div>
      )}
      {backupError && (
        <p role="alert" className="text-sm text-destructive">
          {t(backupError)}{' '}
          <button className="underline" onClick={() => void load()}>
            {t('Try again')}
          </button>
        </p>
      )}
      {loading ? (
        <p role="status" className="text-sm text-muted-foreground">
          Loading agents…
        </p>
      ) : visibleAgents.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border p-8 text-center">
          <Bot className="mx-auto h-8 w-8 text-muted-foreground" />
          <h2 className="mt-3 font-medium">No agents yet</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Add your first agent, then connect a Google account.
          </p>
          {showCreateAction && (
            <button onClick={() => begin()} className={`${primary} mt-4`}>
              Add your first agent
            </button>
          )}
        </div>
      ) : filteredAgents.length === 0 ? (
        <div className="rounded-lg border p-5 text-center">
          <p>No matching agents.</p>
          <button
            className="action-secondary mt-3"
            onClick={() => setSearch('')}
          >
            Show all agents
          </button>
        </div>
      ) : (
        <div
          role="list"
          aria-label="Agent list"
          className="divide-y divide-border rounded-lg border border-border"
        >
          {visibleAgents.map((agent) => {
            const account = accounts.find(
              (item) => item.agent_id === agent.id && !item.legacy,
            )
            const expiry = agent.verified_at
              ? Date.parse(agent.verified_at) + 5 * 60_000
              : NaN
            const state = connectionStatus(agent, now)
            const a = { ...agent, connection_state: state.state }
            const details: Record<string, string> = {
              verified: 'The provider responded successfully.',
              unknown:
                'No current result. Check the connection without signing in again.',
              login_required: 'Sign-in required. Your history is saved.',
              quota_limited:
                'Provider quota reached. Wait for it to reset; you do not need to sign in again.',
              unavailable: 'The provider is unreachable. Check again later.',
            }
            return (
              <div
                role="listitem"
                aria-label={`Agent ${a.name}`}
                key={a.id}
                id={`settings-agent-${a.id}`}
                className="p-4"
              >
                <div className="grid grid-cols-[minmax(0,1fr)_44px] items-start gap-3 sm:grid-cols-[minmax(0,1fr)_auto_44px]">
                  <div className="flex min-w-0 gap-3">
                    <Bot className="mt-1 h-5 w-5 shrink-0 text-muted-foreground" />
                    <div className="min-w-0">
                      <h3 className="break-words text-base font-medium">
                        {a.name}
                      </h3>
                      <p
                        className={`mt-1 text-sm ${state.verified ? 'text-emerald-700 dark:text-emerald-400' : 'text-muted-foreground'}`}
                      >
                        {state.state === 'unknown'
                          ? a.last_checked_at || a.verified_at
                            ? 'Connection status needs to be checked again'
                            : 'Connection status has not been checked'
                          : state.label}
                      </p>
                    </div>
                  </div>
                  <div className="col-start-2 row-start-1 sm:col-start-3">
                    <AgentActions
                      name={a.name}
                      disabled={!!checking || !!removing}
                      onRemove={() => void remove(a.id)}
                    />
                  </div>
                  {(a.harness === 'agy' ||
                    a.provider === 'agy' ||
                    a.provider === 'antigravity') && (
                    <button
                      onClick={() => void check(a.id)}
                      disabled={!!checking || !!removing}
                      aria-label={`Check connection ${a.name}`}
                      className="action-secondary col-span-2 justify-self-start sm:col-span-1 sm:col-start-2 sm:row-start-1"
                    >
                      <RefreshCw
                        className={`h-4 w-4 ${checking === a.id ? 'animate-spin' : ''}`}
                      />
                      {checking === a.id ? 'Checking…' : 'Check connection'}
                    </button>
                  )}
                </div>
                {account && (
                  <div className="mt-3 border-t border-border pt-3">
                    <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm">
                      <input
                        type="checkbox"
                        checked={!!account.allow_failover}
                        disabled={!!backupBusy}
                        onChange={async (e) => {
                          const enabled = e.target.checked
                          setBackupBusy(account.id)
                          setBackupError('')
                          try {
                            const updated = await accountsApi.failover(
                              account.id,
                              enabled,
                            )
                            setAccounts((old) =>
                              old.map((item) =>
                                item.id === updated.id ? updated : item,
                              ),
                            )
                          } catch {
                            setBackupError(
                              'Could not save backup account settings. Try again.',
                            )
                          } finally {
                            setBackupBusy('')
                          }
                        }}
                      />
                      {t('Use this AI account as a backup')}
                    </label>
                    <p className="text-xs text-muted-foreground">
                      {t(
                        'Allows your agents to use this account for Inbox replies and new task sessions. Cuan checks available model quota before starting. Primary connections stay unchanged.',
                      )}
                    </p>
                    <label className="mt-2 flex min-h-11 cursor-pointer items-center gap-3 text-sm">
                      <input
                        type="checkbox"
                        checked={!!account.allow_model_fallback}
                        disabled={!!backupBusy}
                        onChange={async (e) => {
                          const enabled = e.target.checked
                          setBackupBusy(account.id)
                          setBackupError('')
                          try {
                            const updated = await accountsApi.modelFallback(
                              account.id,
                              enabled,
                            )
                            setAccounts((old) =>
                              old.map((item) =>
                                item.id === updated.id ? updated : item,
                              ),
                            )
                          } catch {
                            setBackupError(
                              'Could not save backup account settings. Try again.',
                            )
                          } finally {
                            setBackupBusy('')
                          }
                        }}
                      />
                      {t('Allow available text models on this account')}
                    </label>
                    <p className="text-xs text-muted-foreground">
                      {t(
                        'Cuan can choose another Gemini or Claude text model using reported quota. Model capabilities and usage limits may differ. Your agent stays the same.',
                      )}
                    </p>
                    <button
                      type="button"
                      className="action-secondary mt-3"
                      disabled={!!usageBusy}
                      onClick={async () => {
                        setUsageBusy(account.id)
                        setBackupError('')
                        try {
                          setUsage((old) => ({
                            ...old,
                            [account.id]: {
                              available: false,
                              checked_at: '',
                              models: [],
                              message: '',
                            },
                          }))
                          const result = await accountsApi.usage(account.id)
                          setUsage((old) => ({ ...old, [account.id]: result }))
                        } catch {
                          setBackupError(
                            'Could not check model usage. Try again.',
                          )
                        } finally {
                          setUsageBusy('')
                        }
                      }}
                    >
                      {usageBusy === account.id
                        ? t('Checking…')
                        : t('Check model usage')}
                    </button>
                    {usage[account.id] && !usageBusy && (
                      <div
                        className="mt-3 space-y-2 text-xs text-muted-foreground"
                        aria-label={t('Model usage')}
                      >
                        {usage[account.id].available ? (
                          usage[account.id].models.map((model) => (
                            <div
                              key={model.model}
                              className="flex min-w-0 flex-wrap justify-between gap-2"
                            >
                              <span className="min-w-0 break-all">
                                {model.model}
                              </span>
                              <span>
                                {Math.round(model.remaining_percent)}%{' '}
                                {t('remaining')}
                              </span>
                              {model.reset_at && (
                                <span className="w-full">
                                  {t('Resets')}:{' '}
                                  {new Date(model.reset_at).toLocaleString()}
                                </span>
                              )}
                            </div>
                          ))
                        ) : (
                          <p>
                            {t(
                              'Model usage is unavailable. No remaining quota is assumed.',
                            )}
                          </p>
                        )}
                        {usage[account.id].checked_at && (
                          <p>
                            {t('Checked')}:{' '}
                            {new Date(
                              usage[account.id].checked_at,
                            ).toLocaleString()}
                          </p>
                        )}
                      </div>
                    )}
                  </div>
                )}
                {(a.last_checked_at || a.verified_at) && (
                  <details className="mt-2 text-xs text-muted-foreground">
                    <summary className="inline-flex min-h-11 cursor-pointer items-center text-sm underline underline-offset-2">
                      Connection details
                    </summary>
                    <div className="space-y-2 pb-2">
                      <p className="text-sm">
                        {state.state === 'disabled'
                          ? 'This agent is disabled.'
                          : details[a.connection_state || 'unknown'] ||
                            details.unknown}
                      </p>
                      {a.last_checked_at && (
                        <p>
                          Checked:{' '}
                          <time dateTime={a.last_checked_at}>
                            {new Date(a.last_checked_at).toLocaleString(
                              'en-US',
                            )}
                          </time>
                        </p>
                      )}
                      {a.verified_at && Number.isFinite(expiry) && (
                        <p>
                          {state.verified ? 'Valid until' : 'Result expired at'}
                          :{' '}
                          <time dateTime={new Date(expiry).toISOString()}>
                            {new Date(expiry).toLocaleString('en-US')}
                          </time>
                        </p>
                      )}
                    </div>
                  </details>
                )}
              </div>
            )
          })}
        </div>
      )}
      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm">
          <section
            role="dialog"
            aria-modal="true"
            aria-labelledby="add-agent-title"
            className="max-h-[90dvh] w-full max-w-md overflow-y-auto rounded-xl border border-border bg-card shadow-xl"
            onKeyDown={(event) => {
              if (event.key === 'Escape') close()
              if (event.key !== 'Tab') return
              const items = Array.from(
                event.currentTarget.querySelectorAll<HTMLElement>(
                  'button:not(:disabled), input:not(:disabled), a[href]',
                ),
              )
              const first = items[0],
                last = items[items.length - 1]
              if (event.shiftKey && document.activeElement === first) {
                event.preventDefault()
                last?.focus()
              } else if (!event.shiftKey && document.activeElement === last) {
                event.preventDefault()
                first?.focus()
              }
            }}
          >
            <header className="flex items-center justify-between border-b border-border px-5 py-4">
              <div>
                <h2 id="add-agent-title" className="font-semibold">
                  Add agent
                </h2>
                {step !== 'save' && (
                  <p className="mt-1 text-xs text-muted-foreground">
                    {step === 'name'
                      ? 'Step 1 of 2 · Choose a name'
                      : 'Step 2 of 2 · Connect Google'}
                  </p>
                )}
              </div>
              <button
                onClick={close}
                aria-label="Close"
                className="icon-action rounded p-2 hover:bg-muted"
              >
                <X className="h-4 w-4" />
              </button>
            </header>
            <form
              onSubmit={(event) => {
                event.preventDefault()
                if (step === 'save') void retrySave()
                else if (step === 'name' || !auth?.auth_url) void connect()
                else void verify()
              }}
            >
              <div className="space-y-4 p-5">
                {step === 'name' && (
                  <>
                    <div>
                      <label
                        htmlFor="agent-handle"
                        className="text-sm font-medium"
                      >
                        Agent name
                      </label>
                      <input
                        id="agent-handle"
                        autoFocus
                        required
                        maxLength={49}
                        value={handle}
                        onChange={(event) =>
                          setHandle(
                            '@' +
                              event.target.value
                                .replace(/^@+/, '')
                                .toLowerCase()
                                .replace(/[^a-z0-9_-]/g, ''),
                          )
                        }
                        className="mt-2 w-full rounded-md border border-input bg-background px-3 py-2.5 text-base"
                        aria-describedby="agent-name-help"
                      />
                      <p
                        id="agent-name-help"
                        className="mt-2 text-xs text-muted-foreground"
                      >
                        For example: @writer or @developer.
                      </p>
                    </div>
                    <p className="text-sm text-muted-foreground">
                      Next, sign in to Google to connect this agent. Other agent
                      connections are preserved.
                    </p>
                  </>
                )}
                {step === 'google' && (
                  <>
                    <p className="text-sm">
                      Connect Google for <strong>{handle}</strong>.
                    </p>
                    {auth?.auth_url ? (
                      <>
                        <div className="space-y-2">
                          <p className="text-sm font-medium">
                            1. Sign in to your Google account
                          </p>
                          <a
                            href={auth.auth_url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className={`${secondary} w-full`}
                          >
                            Open Google ↗
                          </a>
                          <p className="text-xs text-muted-foreground">
                            Choose the account you want to use, then return
                            here.
                          </p>
                        </div>
                        <div>
                          <label
                            htmlFor="oauth-code"
                            className="text-sm font-medium"
                          >
                            2. Paste the code from Google
                          </label>
                          <input
                            id="oauth-code"
                            type="password"
                            autoComplete="off"
                            value={code}
                            onChange={(event) => setCode(event.target.value)}
                            placeholder="Paste your code here"
                            className="mt-2 w-full rounded-md border border-input bg-background px-3 py-2.5 text-base"
                          />
                          <p className="mt-2 text-xs text-muted-foreground">
                            Paste the code here only. The link expires in five
                            minutes.
                          </p>
                        </div>
                        <button
                          type="button"
                          disabled={busy}
                          onClick={() => void connect()}
                          className="text-xs text-muted-foreground underline"
                        >
                          Create a new link
                        </button>
                      </>
                    ) : (
                      <p className="text-sm text-muted-foreground">
                        Create a new link to continue signing in to Google.
                      </p>
                    )}
                  </>
                )}
                {step === 'save' && (
                  <p className="text-sm">
                    Google is connected. Save <strong>{handle}</strong> to start
                    using it.
                  </p>
                )}
                {formError && (
                  <p
                    role="alert"
                    className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
                  >
                    {formError}
                  </p>
                )}
                {busy && (
                  <p role="status" className="text-sm text-muted-foreground">
                    {step === 'save'
                      ? 'Adding agent…'
                      : step === 'name' || !auth?.auth_url
                        ? 'Preparing Google sign-in…'
                        : 'Checking code…'}
                  </p>
                )}
              </div>
              <footer className="flex items-center justify-end gap-2 border-t border-border p-4">
                {step !== 'save' && (
                  <button type="button" onClick={close} className={secondary}>
                    Cancel
                  </button>
                )}
                <button
                  type="submit"
                  className={primary}
                  disabled={
                    busy ||
                    (step === 'name' && handle.length < 2) ||
                    (step === 'google' && !!auth?.auth_url && !code.trim())
                  }
                >
                  {busy && <RefreshCw className="h-4 w-4 animate-spin" />}
                  {step === 'save'
                    ? 'Save agent'
                    : step === 'name'
                      ? 'Connect Google'
                      : auth?.auth_url
                        ? 'Confirm sign-in'
                        : 'Create a new link'}
                </button>
              </footer>
            </form>
          </section>
        </div>
      )}
    </div>
  )
}

export function TeamSubPanel({
  onAddAgentClick,
}: {
  selectedRole: string
  onSelectRole: (role: string) => void
  onAddAgentClick?: () => void
}) {
  return (
    <div className="space-y-3 p-4">
      <h2 className="text-sm font-medium">Agent</h2>
      <p className="text-xs text-muted-foreground">
        Each agent keeps its identity. Allowed backup accounts can help when a
        provider reaches its usage limit.
      </p>
      {onAddAgentClick && (
        <button
          onClick={onAddAgentClick}
          className="flex items-center gap-2 text-sm text-primary"
        >
          <Plus className="h-4 w-4" />
          Add agent
        </button>
      )}
    </div>
  )
}
