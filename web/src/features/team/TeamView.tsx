import { useLanguage } from '../../context/LanguageContext'
import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Bot,
  Plus,
  RefreshCw,
  Trash2,
  MoreVertical,
  X,
  BarChart2,
  Search,
  AlertCircle,
  Key,
} from 'lucide-react'
import { connectionStatus } from '../agents/connection'
import {
  accountsApi,
  type AccountUsage,
  type ProviderAccount,
} from '../../api/accounts'
import { agentsApi, type Agent, type HarnessAuth, type ModelInfo, isModelVisionCapable } from '../../api/agents'

function formatVerificationTime(isoString?: string): string {
  if (!isoString) return ''
  const date = new Date(isoString)
  if (isNaN(date.getTime())) return isoString
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  if (diffMs >= 0 && diffMs < 60_000) return 'Just now'
  if (diffMs >= 0 && diffMs < 3600_000) {
    const mins = Math.floor(diffMs / 60_000)
    return `${mins}m ago`
  }
  const isToday = date.toDateString() === now.toDateString()
  const timeStr = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  if (isToday) return `Today, ${timeStr}`
  return `${date.toLocaleDateString([], { month: 'short', day: 'numeric' })}, ${timeStr}`
}

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
        className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground disabled:opacity-50 transition-colors"
        aria-label={`Actions ${name}`}
        aria-expanded={expanded}
        onClick={() => setExpanded(!expanded)}
        disabled={disabled}
      >
        <MoreVertical className="h-4 w-4" />
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
  onAgentConnected,
  focusAgentId,
  connectionOnly = false,
}: {
  connectionOnly?: boolean
  showHeading?: boolean
  showCreateAction?: boolean
  createRequested?: boolean
  onCreateHandled?: () => void
  focusAgentId?: string
  selectedRole?: string
  onAgentAdded?: () => void
  onAgentConnected?: () => void
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
  const [auth, setAuth] = useState<HarnessAuth | null>(() => {
    try {
      const raw = window.localStorage.getItem('cuan.commandcode.auth')
      return raw ? (JSON.parse(raw) as HarnessAuth) : null
    } catch { return null }
  })
  const [code, setCode] = useState('')
  const [busy, setBusy] = useState(false)
  const [formError, setFormError] = useState('')
  const [notice, setNotice] = useState('')
  const [checking, setChecking] = useState('')
  const [removing, setRemoving] = useState('')
  const [lifecycleBusy, setLifecycleBusy] = useState('')
  const [replaceAgentId, setReplaceAgentId] = useState('')
  const [replacementKey, setReplacementKey] = useState('')
  const [now, setNow] = useState(Date.now())
  const [provider, setProvider] = useState<'agy' | 'commandcode'>('agy')
  const [commandcodeMode, setCommandcodeMode] = useState<'subscription' | 'byok'>('byok')
  const [commandcodeModel, setCommandcodeModel] = useState('commandcode-default')
  const [availableModels, setAvailableModels] = useState<ModelInfo[]>([])
  const [byokKey, setByokKey] = useState('')
  const [subscriptionKey, setSubscriptionKey] = useState('')
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
  const active = useRef<{ accountId: string; sessionId: string; harness?: 'commandcode' } | null>(null)
  const accountScope = useRef('')
  useEffect(() => {
    if (auth?.session_id && auth.expires_at && Date.parse(auth.expires_at) > Date.now()) {
      active.current = { accountId: '', sessionId: auth.session_id, harness: 'commandcode' }
    } else if (!auth) {
      window.localStorage.removeItem('cuan.commandcode.auth')
    }
  }, [auth])
  const epoch = useRef(0)
  const dialogActive = useRef(false)
  const autoChecked = useRef(new Set<string>())
  const addButton = useRef<HTMLButtonElement>(null)
  const createTrigger = useRef<HTMLElement | null>(null)

  const cancelSession = async () => {
    const session = active.current
    active.current = null
    if (session) {
      if (session.harness === 'commandcode') await agentsApi.authCancel(session.sessionId).catch(() => {})
      else await accountsApi.cancel(session.accountId, session.sessionId).catch(() => {})
    }
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

    setProvider('agy')
    setCommandcodeMode('byok')
    setCommandcodeModel('commandcode-default')
    setByokKey('')
    setSubscriptionKey('')
    agentsApi.listModels('commandcode').then(setAvailableModels).catch(() => {})
    setStep(connectionOnly ? 'google' : 'name')
    setOpen(true)
    setAuth(null)
    setCode('')
    setFormError('')
    setNotice('')
    setBusy(false)
    let suffix = 1
    const base = connectionOnly ? '@cuan' : '@dev'
    let next = base
    while (
      agents.some(
        (a) =>
          a.name.split(' ')[0].toLowerCase() === next ||
          a.id === 'agent_' + next.slice(1),
      )
    )
      next = `${base}${++suffix}`
    setHandle(next)
    if (connectionOnly) void connect(next)
  }
  useEffect(() => {
    if (createRequested) {
      begin()
      onCreateHandled?.()
    }
  }, [createRequested])
  const connect = async (name = handle) => {
    if (busy || name.length < 2) return
    if (provider === 'commandcode') return connectCommandCode(name)
    const currentEpoch = epoch.current
    setBusy(true)
    setFormError('')
    setCode('')
    try {
      await cancelSession()
      if (!dialogActive.current || epoch.current !== currentEpoch) return
      if (!accountScope.current) {
        const account = await accountsApi.create(`${name} · Google`)
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

  const connectCommandCode = async (name = handle) => {
    if (busy || name.length < 2) return
    const currentEpoch = epoch.current
    setBusy(true)
    setFormError('')
    try {
      if (!dialogActive.current || epoch.current !== currentEpoch) return
      accountScope.current = ''
      if (commandcodeMode === 'subscription') {
        // Start or resume the provider-owned login. The server keeps one pending session per owner.
        const session = await agentsApi.authInit('commandcode')
        if (!dialogActive.current || epoch.current !== currentEpoch) return
        if (!session.session_id) throw new Error('The CommandCode sign-in session is unavailable. Try again.')
        active.current = { accountId: '', sessionId: session.session_id, harness: 'commandcode' }
        window.localStorage.setItem('cuan.commandcode.auth', JSON.stringify(session))
        setAuth(session)
        setStep('google')
        return
      }
      // Custom provider BYOK: save API key encrypted in ProviderAccount.
      const key = byokKey.trim()
      if (key.length < 8) {
        throw new Error('CommandCode API key is required (at least 8 characters)')
      }
      await agentsApi.connectCLI({
        name,
        handle,
        provider: 'commandcode',
        auth_type: 'byok',
        session_token: key,
        model_id: commandcodeModel,
      })
      if (!dialogActive.current || epoch.current !== currentEpoch) return
      close()
      setNotice(`${handle} was added.`)
      await load()
      onAgentAdded?.()
      onAgentConnected?.()
    } catch (err) {
      if (dialogActive.current && epoch.current === currentEpoch)
        setFormError(
          err instanceof Error
            ? err.message
            : 'Could not connect to CommandCode. Try again.',
        )
    } finally {
      if (dialogActive.current && epoch.current === currentEpoch) setBusy(false)
    }
  }
  const finish = async (currentEpoch: number) => {
    if (provider === 'commandcode' && commandcodeMode === 'byok') {
      // BYOK already saved in connectCommandCode via connectCLI.
      close()
      setNotice(`${handle} was added.`)
      await load()
      onAgentAdded?.()
      onAgentConnected?.()
      return
    }
    if (accountScope.current) {
      try {
        await accountsApi.failover(accountScope.current, true)
        await accountsApi.modelFallback(accountScope.current, true)
      } catch {
        // failover default non-fatal
      }
    }
    await agentsApi.connectCLI({
      name: handle,
      handle,
      provider: provider === 'commandcode' ? 'commandcode' : 'agy',
      auth_type: provider === 'commandcode' ? (subscriptionKey.trim() ? 'byok' : 'subscription') : undefined,
      session_token: subscriptionKey.trim() || undefined,
      model_id: provider === 'commandcode' ? commandcodeModel : undefined,
      account_id: accountScope.current,
    })
    if (!dialogActive.current || epoch.current !== currentEpoch) return
    close()
    setNotice(`${handle} was added.`)
    await load()
    onAgentAdded?.()
    onAgentConnected?.()
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
  const check = async (id: string, silent = false) => {
    setChecking(id)
    if (!silent) {
      setError('')
      setNotice('')
    }
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
      if (!silent) {
        setNotice(
          messages[result.connection_state] ||
            'Could not confirm the connection. Try again later.',
        )
      }
    } catch (e) {
      if (!silent) {
        setError(
          e instanceof Error
            ? e.message
            : 'Could not check the connection. Try again.',
        )
      }
    } finally {
      setChecking((prev) => (prev === id ? '' : prev))
    }
  }
  // Refresh stale/unknown statuses sequentially once when the roster opens.
  // Sequential execution avoids overloading host processes or triggering false
  // connection errors across accounts.
  useEffect(() => {
    if (loading) return
    const stale = agents.filter((agent) => {
      if (autoChecked.current.has(agent.id) || agent.enabled === false || agent.status === 'inactive') return false
      const checked = Date.parse(agent.last_checked_at || agent.verified_at || '')
      return !Number.isFinite(checked) || checked + 5 * 60_000 <= Date.now()
    })
    if (stale.length === 0) return
    let cancelled = false
    const runSequential = async () => {
      for (const agent of stale.slice(0, 3)) {
        if (cancelled) break
        autoChecked.current.add(agent.id)
        await check(agent.id, true)
      }
    }
    void runSequential()
    return () => {
      cancelled = true
    }
  }, [agents, loading])
  const verify = async () => {
    if (!active.current || (provider !== 'commandcode' && !code.trim()) || busy) return
    const currentEpoch = epoch.current
    const session = active.current
    setBusy(true)
    setFormError('')
    try {
      if (session.harness === 'commandcode') {
        if (subscriptionKey.trim()) {
          active.current = null
          setAuth(null)
          setStep('save')
          await finish(currentEpoch)
          return
        }
        const result = await agentsApi.authCode(session.sessionId, code.trim() || 'browser-completed')
        if (!result.authenticated && !result.verified) throw new Error(result.message || 'CommandCode sign-in is incomplete. Try again.')
        active.current = null
        window.localStorage.removeItem('cuan.commandcode.auth')
        setAuth(null)
        setStep('save')
        await finish(currentEpoch)
        return
      }
      let account
      try { account = await accountsApi.code(
        session.accountId,
        session.sessionId,
        code.trim(),
        true,
      ) } catch (err) {
        if (!(err instanceof Error) || !/setup is (not complete|incomplete)/i.test(err.message)) throw err
        account = await accountsApi.finishSetup(session.accountId)
      }
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
  const reconnect = async (agent: Agent, account?: ProviderAccount) => {
    if (lifecycleBusy) return
    setLifecycleBusy(agent.id); setError('')
    try {
      // Subscription agents need the same resumable provider-owned auth flow as
      // Add agent. Reconnect used to only open Google account links, leaving
      // CommandCode agents stuck with a failed roster check.
      if (agent.provider === 'commandcode' && agent.auth_type === 'subscription') {
        epoch.current++
        dialogActive.current = true
        setProvider('commandcode')
        setCommandcodeMode('subscription')
        setHandle(agent.name.split(' ')[0] || '@dev')
        setOpen(true)
        setStep('google')
        setCode('')
        setFormError('')
        const session = await agentsApi.authInit('commandcode')
        if (!session.session_id) throw new Error('The CommandCode sign-in session is unavailable. Try again.')
        active.current = { accountId: '', sessionId: session.session_id, harness: 'commandcode' }
        window.localStorage.setItem('cuan.commandcode.auth', JSON.stringify(session))
        setAuth(session)
        return
      }
      if (account && (account.provider === 'agy' || account.provider === 'antigravity')) {
        const session = await accountsApi.login(account.id)
        if (session.auth_url) window.open(session.auth_url, '_blank', 'noopener,noreferrer')
      }
      await agentsApi.patch(agent.id, { enabled: true, status: 'active' })
      await agentsApi.verifyConnection(agent.id); await load()
      setNotice(`${agent.name} connection check started.`)
    } catch (e) { setError(e instanceof Error ? e.message : 'Could not reconnect. Try again.') }
    finally { setLifecycleBusy('') }
  }
  const replaceBYOK = async (agent: Agent) => {
    const key = replacementKey.trim()
    if (key.length < 8 || lifecycleBusy) return
    setLifecycleBusy(agent.id); setError('')
    try { await agentsApi.replaceCredentials(agent.id, key); setReplacementKey(''); setReplaceAgentId(''); await load(); setNotice(`${agent.name} BYOK credential was replaced.`) }
    catch (e) { setError(e instanceof Error ? e.message : 'Could not replace the BYOK credential. Try again.') }
    finally { setLifecycleBusy('') }
  }
  const disconnect = async (agent: Agent) => {
    if (lifecycleBusy) return
    setLifecycleBusy(agent.id); setError('')
    try { await agentsApi.patch(agent.id, { enabled: false, status: 'inactive' }); await load(); setNotice(`${agent.name} was disconnected.`) }
    catch (e) { setError(e instanceof Error ? e.message : 'Could not disconnect. Try again.') }
    finally { setLifecycleBusy('') }
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
    <div className={connectionOnly ? 'contents' : 'mx-auto max-w-4xl space-y-5 pb-8'}>
      <div hidden={connectionOnly}>
      <div className="flex items-center justify-between gap-3">
        <div>
          {showHeading && <h2 className="text-lg font-semibold">Agent</h2>}
          <p className="mt-1 text-sm text-muted-foreground hidden sm:block">
            {t(
              'Connection status, remaining quota and work progress are separate. A connected account can still have limited quota.',
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
            {t('Add agent')}
          </button>
        )}
      </div>
      {visibleAgents.length > 0 && (
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <input
              className="field-control pl-9 w-full"
              aria-label="Search agents"
              placeholder={t('Search by name or agent type')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          {search.trim() && (
            <button className="action-secondary text-xs px-2.5 py-1.5" onClick={() => setSearch('')}>
              {t('Clear')}
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
        <div
          role="alert"
          className="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive flex items-center justify-between gap-2"
        >
          <div className="flex items-center gap-2 min-w-0">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span className="truncate">{t(backupError)}</span>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <button className="underline font-medium hover:opacity-80" onClick={() => void load()}>
              {t('Try again')}
            </button>
            <button
              className="text-muted-foreground hover:text-foreground p-0.5"
              onClick={() => setBackupError('')}
              aria-label="Dismiss error"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
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
            const account =
              accounts.find(
                (item) =>
                  !item.legacy &&
                  item.agent_id === agent.id &&
                  (agent.connection_id
                    ? item.connection_id === agent.connection_id
                    : true),
              ) ||
              accounts.find(
                (item) => !item.legacy && item.agent_id === agent.id,
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
                className="p-4 sm:p-5"
              >
                {/* 1. Header: Bot Icon + Agent Name + Status Badge (Left) & Actions Menu (Right) */}
                <div className="flex items-start justify-between gap-3">
                  <div className="flex min-w-0 items-start gap-3">
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20 mt-0.5">
                      <Bot className="h-5 w-5" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-1.5 sm:gap-2">
                        <h3 className="break-words text-sm sm:text-base font-semibold text-foreground">
                          {a.name}
                        </h3>
                        <span
                          className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[10px] font-semibold border shrink-0 ${
                            state.verified
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                              : state.state === 'quota_limited'
                                ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
                                : state.state === 'login_required' || state.state === 'unavailable'
                                  ? 'bg-destructive/10 text-destructive border-destructive/20'
                                  : 'bg-muted text-muted-foreground border-border'
                          }`}
                        >
                          {state.verified && (
                            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                          )}
                          {state.state === 'unknown'
                            ? a.last_checked_at || a.verified_at
                              ? t('Check needed')
                              : t('Not checked')
                            : t(state.label)}
                        </span>
                      </div>
                      {account?.email && (
                        <p className="text-xs text-muted-foreground font-mono mt-0.5 truncate">
                          {account.email}
                        </p>
                      )}
                    </div>
                  </div>
                  {/* Dedicated top-right corner actions menu only: NEVER collides with name/badge */}
                  <div className="shrink-0">
                    <AgentActions
                      name={a.name}
                      disabled={!!checking || !!removing || !!lifecycleBusy}
                      onRemove={() => void remove(a.id)}
                    />
                  </div>
                </div>

                {/* 2. Metadata Chips: Clean, structured badges eliminating hanging dots */}
                <div className="mt-3 flex flex-wrap items-center gap-1.5">
                  <span className="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-[11px] font-medium border border-border/50">
                    <span className="text-muted-foreground">Source:</span>
                    <span className="font-semibold text-foreground">
                      {a.auth_type === 'byok'
                        ? 'BYOK'
                        : a.auth_type === 'subscription'
                          ? 'Subscription'
                          : account?.provider === 'agy'
                            ? 'Google account'
                            : 'Provider'}
                    </span>
                  </span>
                  <span className="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-[11px] font-medium border border-border/50">
                    <span className="text-muted-foreground">Provider:</span>
                    <span className="font-semibold text-foreground">
                      {a.provider || account?.provider || 'Unknown'}
                    </span>
                  </span>
                  {a.model_id && (
                    <span className="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-[11px] font-medium border border-border/50">
                      <span className="text-muted-foreground">Model:</span>
                      <span className="font-semibold text-foreground">{a.model_id}</span>
                    </span>
                  )}
                  {(a.verified_at || a.last_checked_at || account?.verified_at || account?.last_checked_at) && (
                    <span
                      className="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-[11px] font-medium border border-border/50"
                      title={a.verified_at || a.last_checked_at || account?.verified_at || account?.last_checked_at}
                    >
                      <span className="text-muted-foreground">Last verified:</span>
                      <span className="font-semibold text-foreground font-mono">
                        {formatVerificationTime(
                          a.verified_at || a.last_checked_at || account?.verified_at || account?.last_checked_at,
                        )}
                      </span>
                    </span>
                  )}
                </div>

                {/* Alerts if quota limited or unavailable */}
                {state.state === 'quota_limited' && (
                  <div className="mt-3 rounded-md border border-amber-500/30 bg-amber-500/10 p-2.5 text-xs text-amber-800 dark:text-amber-300">
                    <p className="font-medium">{t('Individual account quota reached')}</p>
                    <p className="mt-0.5 text-muted-foreground">
                      {account?.message?.includes('Resets in')
                        ? account.message
                        : t(
                            'Google has paused CLI access for this Google account until its reset period. Per-model token capacity may still show 100%, but this account cannot execute queries until it resets or is upgraded.',
                          )}
                    </p>
                    {account?.quota_retry_at && (
                      <p className="mt-1.5 font-mono text-[11px] text-amber-900 dark:text-amber-200">
                        {t('Resets')}: {new Date(account.quota_retry_at).toLocaleString()}
                      </p>
                    )}
                  </div>
                )}
                {state.state === 'unavailable' && (
                  <div className="mt-3 flex items-center justify-between gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-2.5 py-2 text-xs">
                    <span className="min-w-0 truncate text-destructive">
                      {t('Agent unavailable')} · {t('Inbox will not route new work here.')}
                    </span>
                    <span className="shrink-0 text-muted-foreground">
                      {t('Check connection')}
                    </span>
                  </div>
                )}

                {/* 3. Primary Actions Row: Dedicated full-width container with comfortable touch targets */}
                <div className="mt-3.5 flex flex-wrap items-center gap-2 pt-2 border-t border-border/40">
                  {state.state !== 'verified' && (
                    <button
                      type="button"
                      className="inline-flex min-h-11 items-center justify-center rounded-lg border border-border bg-background px-3.5 py-2 text-xs font-medium text-foreground hover:bg-muted active:scale-[0.98] transition-colors disabled:opacity-50"
                      disabled={!!lifecycleBusy || !!removing}
                      onClick={() => void reconnect(a, account)}
                      aria-label={`Reconnect ${a.name}`}
                    >
                      <RefreshCw
                        className={`mr-1.5 h-3.5 w-3.5 shrink-0 ${lifecycleBusy === a.id ? 'animate-spin' : ''}`}
                      />
                      <span>{lifecycleBusy === a.id ? 'Working…' : 'Reconnect'}</span>
                    </button>
                  )}
                  {a.enabled !== false && (
                    <button
                      type="button"
                      className="inline-flex min-h-11 items-center justify-center rounded-lg border border-border bg-background px-3.5 py-2 text-xs font-medium text-foreground hover:bg-muted active:scale-[0.98] transition-colors disabled:opacity-50"
                      disabled={!!lifecycleBusy || !!removing}
                      onClick={() => void disconnect(a)}
                      aria-label={`Disconnect ${a.name}`}
                    >
                      Disconnect
                    </button>
                  )}
                  {(a.auth_type === 'byok' || a.provider === 'commandcode') && (
                    <button
                      type="button"
                      className="inline-flex min-h-11 items-center justify-center rounded-lg border border-border bg-background px-3.5 py-2 text-xs font-medium text-foreground hover:bg-muted active:scale-[0.98] transition-colors disabled:opacity-50"
                      disabled={!!lifecycleBusy || !!removing}
                      onClick={() => setReplaceAgentId(replaceAgentId === a.id ? '' : a.id)}
                      aria-label={`Replace BYOK credential for ${a.name}`}
                    >
                      <Key className="mr-1.5 h-3.5 w-3.5 shrink-0" />
                      <span>Replace BYOK</span>
                    </button>
                  )}
                </div>

                {/* Inline Replace BYOK form */}
                {replaceAgentId === a.id && (a.auth_type === 'byok' || a.provider === 'commandcode') && (
                  <div className="mt-3 flex flex-col gap-2 rounded-lg border border-border bg-muted/40 p-3 sm:flex-row">
                    <label className="sr-only" htmlFor={`replace-key-${a.id}`}>
                      New API key
                    </label>
                    <input
                      id={`replace-key-${a.id}`}
                      type="password"
                      autoComplete="off"
                      value={replacementKey}
                      onChange={(e) => setReplacementKey(e.target.value)}
                      placeholder="Enter a new API key"
                      className="min-h-11 min-w-0 flex-1 rounded-md border border-input bg-background px-3 text-sm"
                    />
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        className="min-h-11 rounded-md bg-primary px-3 text-xs text-primary-foreground disabled:opacity-50"
                        disabled={replacementKey.trim().length < 8 || !!lifecycleBusy}
                        onClick={() => void replaceBYOK(a)}
                      >
                        {lifecycleBusy === a.id ? 'Saving…' : 'Save key'}
                      </button>
                      <button
                        type="button"
                        className="min-h-11 rounded-md border border-border bg-background px-3 text-xs text-foreground hover:bg-muted"
                        disabled={!!lifecycleBusy}
                        onClick={() => {
                          setReplaceAgentId('')
                          setReplacementKey('')
                        }}
                      >
                        Cancel
                      </button>
                    </div>
                  </div>
                )}
                {account && (
                  <details className="mt-3 border-t border-border/60 pt-2">
                    <summary className="flex min-h-11 cursor-pointer items-center justify-between text-sm font-medium text-foreground">
                      <span>{t('Agent details')}</span>
                      <span className="text-xs text-muted-foreground">{t('Quota, routing and usage')}</span>
                    </summary>
                  <div className="space-y-2.5 pb-1">
                    <label className="flex cursor-pointer items-start gap-2.5 text-xs font-medium text-foreground">
                      <input
                        type="checkbox"
                        className="mt-0.5 rounded border-input"
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
                      <div className="min-w-0">
                        <span>{t('Use as backup')}</span>
                        <p className="text-[11px] text-muted-foreground font-normal leading-normal">
                          {t('Auto-failover for Inbox & tasks when primary quota is low.')}
                        </p>
                      </div>
                    </label>

                    <label className="flex cursor-pointer items-start gap-2.5 text-xs font-medium text-foreground">
                      <input
                        type="checkbox"
                        className="mt-0.5 rounded border-input"
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
                      <div className="min-w-0">
                        <span>{t('Allow model fallback')}</span>
                        <p className="text-[11px] text-muted-foreground font-normal leading-normal">
                          {t('Auto-switch between available Gemini and Claude models.')}
                        </p>
                      </div>
                    </label>

                    {/* Compact action buttons grid */}
                    <div className="grid grid-cols-2 gap-2 pt-2 border-t border-border/40">
                      {(a.harness === 'agy' ||
                        a.provider === 'agy' ||
                        a.provider === 'antigravity') && (
                        <button
                          onClick={() => void check(a.id)}
                          disabled={!!checking || !!removing}
                          aria-label={`Check connection ${a.name}`}
                          className="action-secondary text-xs py-2 px-2 inline-flex items-center justify-center gap-1.5"
                        >
                          <RefreshCw
                            className={`h-3.5 w-3.5 shrink-0 ${checking === a.id ? 'animate-spin' : ''}`}
                          />
                          <span className="truncate">{checking === a.id ? t('Checking…') : t('Check connection')}</span>
                        </button>
                      )}
                      <button
                        type="button"
                        className="action-secondary text-xs py-2 px-2 inline-flex items-center justify-center gap-1.5"
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
                        <BarChart2 className="h-3.5 w-3.5" />
                        <span>{usageBusy === account.id ? t('Checking…') : t('Check model usage')}</span>
                      </button>
                    </div>
                    {usage[account.id] && !usageBusy && (
                      <div
                        className="mt-3 space-y-2 text-xs text-muted-foreground"
                        aria-label={t('Model usage')}
                      >
                        {state.state === 'quota_limited' && (
                          <div className="rounded-md border border-amber-500/30 bg-amber-500/10 p-2 text-xs text-amber-800 dark:text-amber-300">
                            <p>{t('Note: The percentages below reflect per-model token capacity. Execution for this account is currently paused due to Google individual subscription quota.')}</p>
                            {account?.quota_retry_at && (
                              <p className="mt-1 font-mono text-[11px]">
                                {t('Resets')}: {new Date(account.quota_retry_at).toLocaleString()}
                              </p>
                            )}
                          </div>
                        )}
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
                  </details>
                )}
                {(a.last_checked_at || a.verified_at || state.state === 'unknown' || state.state === 'unavailable' || state.state === 'login_required') && (
                  <details className="mt-2 text-xs text-muted-foreground">
                    <summary className="inline-flex min-h-11 cursor-pointer items-center text-sm underline underline-offset-2">
                      Connection details
                    </summary>
                    <div className="space-y-2 pb-2">
                      <p className="text-sm">
                        {state.state === 'disabled'
                          ? 'This agent is disabled.'
                          : state.state === 'unknown'
                            ? (a.last_checked_at || a.verified_at
                              ? 'The last connection check has expired. Check the connection before routing new work here.'
                              : 'This agent has not been checked yet. Check the connection to confirm whether it is available.')
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
      </div>
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
                  {connectionOnly ? 'Connect account' : 'Add agent'}
                </h2>
                {!connectionOnly && step !== 'save' && (
                  <p className="mt-1 text-xs text-muted-foreground">
                    {step === 'name'
                      ? 'Step 1 of 2 · Choose a name'
                      : 'Step 2 of 2 · Connect account'}
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
                    {!connectionOnly && (
                      <div>
                        <label
                          htmlFor="agent-provider"
                          className="text-sm font-medium"
                        >
                          Provider
                        </label>
                        <div
                          id="agent-provider"
                          role="radiogroup"
                          aria-label="Provider"
                          className="mt-2 flex gap-3"
                        >
                          <label className="flex items-center gap-2 cursor-pointer rounded-md border px-3 py-2">
                            <input
                              type="radio"
                              name="provider"
                              value="agy"
                              checked={provider === 'agy'}
                              onChange={() => setProvider('agy')}
                              className="shrink-0"
                            />
                            Google (agy)
                          </label>
                          <label className="flex items-center gap-2 cursor-pointer rounded-md border px-3 py-2">
                            <input
                              type="radio"
                              name="provider"
                              value="commandcode"
                              checked={provider === 'commandcode'}
                              onChange={() => setProvider('commandcode')}
                              className="shrink-0"
                            />
                            CommandCode
                          </label>
                        </div>
                        {provider === 'commandcode' && (commandcodeMode === 'byok' || !!auth?.authenticated || !!auth?.verified) && (
                          <div className="mt-2 flex gap-3">
                            <label className="flex items-center gap-2 cursor-pointer rounded-md border px-3 py-2">
                              <input
                                type="radio"
                                name="commandcode-mode"
                                value="subscription"
                                checked={commandcodeMode === 'subscription'}
                                onChange={() => setCommandcodeMode('subscription')}
                                className="shrink-0"
                              />
                              Subscription connect
                            </label>
                            <label className="flex items-center gap-2 cursor-pointer rounded-md border px-3 py-2">
                              <input
                                type="radio"
                                name="commandcode-mode"
                                value="byok"
                                checked={commandcodeMode === 'byok'}
                                onChange={() => setCommandcodeMode('byok')}
                                className="shrink-0"
                              />
                              Custom BYOK
                            </label>
                          </div>
                        )}
                        {provider === 'commandcode' && commandcodeMode === 'byok' && (
                          <div>
                            <label
                              htmlFor="commandcode-api-key"
                              className="text-sm font-medium"
                            >
                              CommandCode API key
                            </label>
                            <input
                              id="commandcode-api-key"
                              type="password"
                              autoComplete="off"
                              value={byokKey}
                              onChange={(event) => setByokKey(event.target.value)}
                              placeholder="cc_live_..."
                              className="mt-2 w-full rounded-md border border-input bg-background px-3 py-2.5 text-base"
                            />
                            <p className="mt-1 text-xs text-muted-foreground">
                              Your API key is stored encrypted and used only for
                              your account.
                            </p>
                          </div>
                        )}
                        {provider === 'commandcode' && (commandcodeMode === 'byok' || !!auth?.authenticated || !!auth?.verified) && (
                          <div className="mt-3">
                            <label
                              htmlFor="commandcode-model"
                              className="text-sm font-medium"
                            >
                              Model
                            </label>
                            <select
                              id="commandcode-model"
                              value={commandcodeModel}
                              onChange={(e) => setCommandcodeModel(e.target.value)}
                              className="mt-2 w-full rounded-md border border-input bg-background px-3 py-2.5 text-sm"
                            >
                              {availableModels.length > 0 ? (
                                availableModels.map((m) => (
                                  <option key={m.id} value={m.id}>
                                    {m.display_name || m.name} ({m.supports_vision ? 'Vision' : 'Text-only'})
                                  </option>
                                ))
                              ) : (
                                <>
                                  <option value="commandcode-default">CommandCode Default (Text-only)</option>
                                  <option value="claude-sonnet-5-5">Claude Sonnet 5.5 (Vision)</option>
                                  <option value="google/gemini-3.8-flash">Gemini 3.8 Flash (Vision)</option>
                                  <option value="deepseek/deepseek-v4-pro">DeepSeek V4 Pro (Text-only)</option>
                                  <option value="deepseek/deepseek-v4-flash-vision-exp">DeepSeek V4 Flash Vision (Vision)</option>
                                  <option value="moonshotai/kimi-k2.7-code">Kimi K2.7 Code (Vision)</option>
                                  <option value="moonshotai/kimi-k3.1-pro">Kimi K3.1 Pro (Text-only)</option>
                                  <option value="qwen/qwen3.8-omni-flash">Qwen 3.8 Omni Flash (Vision)</option>
                                  <option value="xai/grok-4.5">xAI Grok 4.5 (Text-only)</option>
                                  <option value="reflection/beam:free">Reflection Beam (Free, Text-only)</option>
                                </>
                              )}
                            </select>
                            <p className="mt-1 text-xs text-muted-foreground">
                              {isModelVisionCapable(commandcodeModel, 'commandcode')
                                ? '✓ Model ini mendukung input multimodal (gambar & dokumen).'
                                : 'ℹ Model teks saja (tidak mendukung input gambar / vision).'}
                            </p>
                          </div>
                        )}
                        {!connectionOnly && provider === 'commandcode' && commandcodeMode === 'subscription' && (<p className="mt-2 text-xs text-muted-foreground">Sign in securely with your CommandCode subscription. Cuan never displays or stores subscription secrets.</p>)}
                      </div>
                    )}
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
                      Next, connect your AI account to use this agent. Other agent
                      connections are preserved.
                    </p>
                  </>
                )}
                {step === 'google' && (
                  <>
                    <p className="text-sm">
                      {connectionOnly ? 'Connect your AI account to get started.' : <>Connect your account for <strong>{handle}</strong>.</>}
                    </p>
                    {auth?.auth_url ? (
                      <>
                        <div className="space-y-2">
                          <p className="text-sm font-medium">
                            1. {provider === 'commandcode' ? 'Sign in to CommandCode' : 'Sign in to your provider account'}
                          </p>
                          <a
                            href={auth.auth_url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className={`${secondary} w-full`}
                          >
                            Open sign-in ↗
                          </a>
                          <p className="text-xs text-muted-foreground">
                            Complete sign-in, then return
                            here.
                          </p>
                        </div>
                        {provider !== 'commandcode' && <div>
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
                        </div>}
                        {provider === 'commandcode' && (
                          <div className="mt-4 space-y-2">
                            <label htmlFor="commandcode-subscription-key" className="text-sm font-medium">
                              API key from CommandCode (fallback)
                            </label>
                            <input
                              id="commandcode-subscription-key"
                              type="password"
                              autoComplete="off"
                              value={subscriptionKey}
                              onChange={(event) => setSubscriptionKey(event.target.value)}
                              placeholder="Paste the key shown once by CommandCode"
                              className="mt-2 w-full rounded-md border border-input bg-background px-3 py-2.5 text-base"
                            />
                            <p className="text-xs text-muted-foreground">Stored encrypted for this agent. It is never shown again.</p>
                          </div>
                        )}
                        {provider !== 'commandcode' && <p className="text-xs leading-relaxed text-muted-foreground">
                          By confirming sign-in, you agree to the{' '}
                          <a href="https://antigravity.google/terms" target="_blank" rel="noreferrer" className="underline">Antigravity terms</a>{' '}
                          and acknowledge Google's{' '}
                          <a href="https://policies.google.com/privacy" target="_blank" rel="noreferrer" className="underline">privacy policy</a>.
                          Cuan won't opt you into optional interaction sharing.
                        </p>}
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
                        ? (provider === 'commandcode' ? 'Preparing CommandCode sign-in…' : 'Preparing Google sign-in…')
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
                    (step === 'name' &&
                      handle.length < 2) ||
                    (step === 'name' &&
                      provider === 'commandcode' &&
                      commandcodeMode === 'byok' &&
                      byokKey.trim().length < 8) ||
                    (step === 'google' && provider !== 'commandcode' && !!auth?.auth_url && !code.trim())
                  }
                >
                  {busy && <RefreshCw className="h-4 w-4 animate-spin" />}
                  {step === 'save'
                    ? 'Save agent'
                    : step === 'name' && provider === 'commandcode'
                      ? 'Add agent'
                      : step === 'name'
                        ? 'Connect Google'
                        : auth?.auth_url
                          ? provider === 'commandcode' ? 'I finished signing in' : 'Confirm sign-in'
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
