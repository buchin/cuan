import { useLanguage } from '../../context/LanguageContext'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
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
  Pencil,
  Eye,
  FileText,
  Users,
  UserPlus,
  Shield,
  User,
  Clock,
  UserX,
  Cpu,
} from 'lucide-react'
import { useAuth } from '../../context/AuthContext'
import { teamApi, type TeamMember, type UserInvitation } from '../../api/team'
import { InviteMemberModal } from './InviteMemberModal'
import { connectionStatus } from '../agents/connection'
import {
  accountsApi,
  type AccountUsage,
  type ProviderAccount,
} from '../../api/accounts'
import { agentsApi, type Agent, type HarnessAuth, type ModelInfo, isModelVisionCapable } from '../../api/agents'
import { EditAgentModal } from './EditAgentModal'
import { SlideDrawer } from '../../components/layout/SlideDrawer'
import { StatusDot } from '../../components/layout/StatusDot'
import { HairlineList, HairlineListItem } from '../../components/layout/HairlineList'

function getAgentHandle(agent: { name: string; id: string }): string {
  if (agent.name.startsWith('@')) return agent.name
  if (agent.id.startsWith('agent_')) return `@${agent.id.slice(6)}`
  return `@${agent.name.toLowerCase().replace(/\s+/g, '_').replace(/[^a-z0-9_-]/g, '')}`
}

function formatJoinedDate(isoString?: string): string {
  if (!isoString) return '-'
  const date = new Date(isoString)
  if (isNaN(date.getTime())) return isoString
  return date.toLocaleDateString('id-ID', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

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
  onEdit,
  onRemove,
  disabled,
}: {
  name: string
  onEdit?: () => void
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
        className="inline-flex min-h-11 min-w-11 h-9 w-9 sm:h-9 sm:w-9 sm:min-h-9 sm:min-w-9 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground disabled:opacity-50 transition-colors"
        aria-label={`Actions ${name}`}
        aria-expanded={expanded}
        onClick={() => setExpanded(!expanded)}
        disabled={disabled}
      >
        <MoreVertical className="h-4 w-4" />
      </button>
      {expanded && (
        <div className="absolute right-0 top-full z-20 mt-1 min-w-44 rounded-lg border bg-popover p-1 text-popover-foreground shadow-md">
          {onEdit && (
            <button
              className="flex min-h-11 w-full items-center gap-2 rounded-md px-3 text-sm text-foreground hover:bg-accent"
              onClick={() => {
                setExpanded(false)
                trigger.current?.focus()
                onEdit()
              }}
            >
              <Pencil className="h-4 w-4" />
              Edit {name}
            </button>
          )}
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
  selectedRole: _selectedRole = 'all',
  showHeading: _showHeading = true,
  showCreateAction = true,
  createRequested = false,
  onCreateHandled,
  onClose,
  onAgentAdded,
  onAgentConnected,
  focusAgentId,
  connectionOnly = false,
  dialogOnly = false,
}: {
  dialogOnly?: boolean
  connectionOnly?: boolean
  showHeading?: boolean
  showCreateAction?: boolean
  createRequested?: boolean
  onCreateHandled?: () => void
  onClose?: () => void
  focusAgentId?: string
  selectedRole?: string
  onAgentAdded?: () => void
  onAgentConnected?: () => void
}) {
  const { t } = useLanguage()
  const { user, isOwner } = useAuth()
  const canManageAgents = isOwner
  const [activeTab, setActiveTab] = useState<'agents' | 'members'>('agents')
  const [members, setMembers] = useState<TeamMember[]>([])
  const [invitations, setInvitations] = useState<UserInvitation[]>([])
  const [teamLoading, setTeamLoading] = useState(false)
  const [teamError, setTeamError] = useState('')
  const [isInviteModalOpen, setIsInviteModalOpen] = useState(false)
  const [memberActionBusy, setMemberActionBusy] = useState('')
  const [memberSearch, setMemberSearch] = useState('')

  const loadTeamMembers = useCallback(async () => {
    setTeamLoading(true)
    setTeamError('')
    try {
      const data = await teamApi.listMembers()
      setMembers(data.members || [])
      setInvitations(data.invitations || [])
    } catch (err) {
      setTeamError(
        err instanceof Error ? err.message : t('Gagal memuat daftar anggota tim.')
      )
    } finally {
      setTeamLoading(false)
    }
  }, [t])

  useEffect(() => {
    if (!dialogOnly && !connectionOnly) {
      loadTeamMembers()
    }
  }, [loadTeamMembers, dialogOnly, connectionOnly])

  const handleRemoveMember = async (memberId: string, memberName: string) => {
    if (!isOwner) return
    if (
      !window.confirm(
        t(
          `Hapus akses untuk ${memberName}? Akun staf ini tidak akan dapat masuk lagi.`
        )
      )
    )
      return
    setMemberActionBusy(memberId)
    try {
      await teamApi.deleteMember(memberId)
      setNotice(t(`Akses untuk ${memberName} berhasil dihapus.`))
      await loadTeamMembers()
    } catch (err) {
      setTeamError(
        err instanceof Error
          ? err.message
          : t('Gagal menghapus anggota tim.')
      )
    } finally {
      setMemberActionBusy('')
    }
  }

  const handleRevokeInvite = async (inviteId: string, inviteEmail: string) => {
    if (!isOwner) return
    if (
      !window.confirm(
        t(
          `Cabut undangan untuk ${inviteEmail}? Tautan undangan ini akan kedaluwarsa seketika.`
        )
      )
    )
      return
    setMemberActionBusy(inviteId)
    try {
      await teamApi.deleteInvite(inviteId)
      setNotice(t(`Undangan untuk ${inviteEmail} berhasil dicabut.`))
      await loadTeamMembers()
    } catch (err) {
      setTeamError(
        err instanceof Error ? err.message : t('Gagal mencabut undangan.')
      )
    } finally {
      setMemberActionBusy('')
    }
  }

  const [usage, setUsage] = useState<Record<string, AccountUsage>>({})
  const [usageBusy, setUsageBusy] = useState('')
  const [accounts, setAccounts] = useState<ProviderAccount[]>([])
  const [backupBusy, setBackupBusy] = useState('')
  const [backupError, setBackupError] = useState('')
  const [agents, setAgents] = useState<Agent[]>([])
  const [editingAgent, setEditingAgent] = useState<Agent | null>(null)
  const [drawerAgent, setInternalDrawerAgent] = useState<Agent | null>(null)
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
  const [refreshingModels, setRefreshingModels] = useState(false)
  const [isCustomCommandcodeModel, setIsCustomCommandcodeModel] = useState(false)
  const [byokKey, setByokKey] = useState('')
  const [subscriptionKey, setSubscriptionKey] = useState('')

  const loadModels = useCallback(async (p: string, refresh = false) => {
    setRefreshingModels(true)
    try {
      const list = await agentsApi.listModels(p, refresh)
      setAvailableModels(list)
      if (list.length > 0 && !isCustomCommandcodeModel) {
        setCommandcodeModel((curr) => {
          if (curr && curr !== 'commandcode-default' && list.some((m) => m.id === curr)) return curr
          const def = list.find((m) => m.is_default) || list[0]
          return def ? def.id : curr
        })
      }
    } catch {
      // retain existing
    } finally {
      setRefreshingModels(false)
    }
  }, [isCustomCommandcodeModel])
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
    onClose?.()
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
    if (!canManageAgents) return
    createTrigger.current = document.activeElement as HTMLElement | null
    epoch.current++
    dialogActive.current = true
    accountScope.current = ''

    setProvider('agy')
    setCommandcodeMode('byok')
    setCommandcodeModel('commandcode-default')
    setIsCustomCommandcodeModel(false)
    setByokKey('')
    setSubscriptionKey('')
    void loadModels('commandcode', false)
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
    if (createRequested && canManageAgents) {
      begin()
      onCreateHandled?.()
    }
  }, [createRequested, canManageAgents])
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
  const primaryAgent = agents.find((a) => a.id === 'agent_cuan' || a.name === '@cuan') || agents[0]
  const primaryState = primaryAgent ? connectionStatus(primaryAgent, now) : null
  const primaryAccount = primaryAgent
    ? accounts.find(
        (item) =>
          !item.legacy &&
          item.agent_id === primaryAgent.id &&
          (primaryAgent.connection_id
            ? item.connection_id === primaryAgent.connection_id
            : true),
      ) || accounts.find((item) => !item.legacy && item.agent_id === primaryAgent.id)
    : null
  const workerAgents = agents.filter((a) => a.id !== primaryAgent?.id)

  const filteredMembers = members.filter((m) => {
    if (!memberSearch.trim()) return true
    const q = memberSearch.toLowerCase().trim()
    return (
      (m.name || '').toLowerCase().includes(q) ||
      (m.username || '').toLowerCase().includes(q) ||
      m.email.toLowerCase().includes(q) ||
      m.role.toLowerCase().includes(q)
    )
  })
  const filteredInvitations = invitations.filter((inv) => {
    if (!memberSearch.trim()) return true
    const q = memberSearch.toLowerCase().trim()
    return inv.email.toLowerCase().includes(q) || inv.role.toLowerCase().includes(q)
  })

  const [searchParams, setSearchParams] = useSearchParams()
  const agentParam = searchParams.get('agent') || ''

  const selectAgent = useCallback((agent: Agent | null) => {
    setInternalDrawerAgent(agent)
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (agent) {
          next.set('agent', agent.id)
        } else {
          next.delete('agent')
        }
        return next
      },
      { replace: true }
    )
  }, [setSearchParams])
  const setDrawerAgent = selectAgent

  const activeDrawerAgent =
    (agentParam ? agents.find((ag) => ag.id === agentParam) : null) ||
    (drawerAgent ? agents.find((ag) => ag.id === drawerAgent.id) || drawerAgent : null)
  const activeDrawerAccount = activeDrawerAgent
    ? accounts.find(
        (item) =>
          !item.legacy &&
          item.agent_id === activeDrawerAgent.id &&
          (activeDrawerAgent.connection_id
            ? item.connection_id === activeDrawerAgent.connection_id
            : true),
      ) || accounts.find((item) => !item.legacy && item.agent_id === activeDrawerAgent.id)
    : undefined
  const activeDrawerExpiry = activeDrawerAgent?.verified_at
    ? Date.parse(activeDrawerAgent.verified_at) + 5 * 60_000
    : NaN
  const activeDrawerState = activeDrawerAgent ? connectionStatus(activeDrawerAgent, now) : null
  const activeDrawerA =
    activeDrawerAgent && activeDrawerState
      ? { ...activeDrawerAgent, connection_state: activeDrawerState.state }
      : null
  const details: Record<string, string> = {
    verified: 'The provider responded successfully.',
    unknown:
      'No current result. Check the connection without signing in again.',
    login_required: 'Sign-in required. Your history is saved.',
    quota_limited:
      'Provider quota reached. Wait for it to reset; you do not need to sign in again.',
    unavailable: 'The provider is unreachable. Check again later.',
  }
  const secondary =
    'inline-flex items-center justify-center gap-2 rounded-md border border-border px-4 py-2.5 text-sm hover:bg-muted disabled:opacity-50'
  const primary =
    'inline-flex items-center justify-center gap-2 rounded-md bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50'

  // Focus specific agent if focusAgentId is specified in URL query
  useEffect(() => {
    if (focusAgentId && agents.length > 0) {
      const match = agents.find((a) => a.id === focusAgentId)
      if (match) selectAgent(match)
    }
  }, [focusAgentId, agents, selectAgent])

  return (
    <div className={connectionOnly || dialogOnly ? 'contents' : 'w-full space-y-5 pb-8'}>
      <div hidden={connectionOnly || dialogOnly}>
        {/* Team View Tabs: Asisten AI vs Anggota Tim */}
        <div className="flex border-b border-border mb-5" role="tablist" aria-label={t('Manajemen Tim')}>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'agents'}
            onClick={() => setActiveTab('agents')}
            className={`flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
              activeTab === 'agents'
                ? 'border-primary text-primary font-semibold'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            <Bot className="h-4 w-4" />
            <span>{t('Asisten AI')}</span>
            <span className="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground font-mono">
              {primaryAgent ? 1 : 0}
            </span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'members'}
            onClick={() => setActiveTab('members')}
            className={`flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
              activeTab === 'members'
                ? 'border-primary text-primary font-semibold'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            <Users className="h-4 w-4" />
            <span>{t('Anggota Tim')}</span>
            <span className="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground font-mono">
              {members.length + invitations.length}
            </span>
          </button>
        </div>

        {activeTab === 'agents' ? (
          <div className="space-y-6 max-w-3xl">
            {notice && (
              <p role="status" className="text-sm">
                {notice}
              </p>
            )}
            {error && (
              <div role="alert" className="text-sm text-destructive">
                {error}{' '}
                <button onClick={load} className="underline">
                  {t('Try again')}
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
                {t('Memuat asisten…')}
              </p>
            ) : !primaryAgent ? (
              <div className="rounded-xl border border-dashed border-border p-8 text-center bg-card/20">
                <Bot className="mx-auto h-8 w-8 text-muted-foreground" />
                <h3 className="mt-3 font-medium text-foreground">{t('Belum Ada Asisten AI')}</h3>
                <p className="mt-1 text-sm text-muted-foreground">
                  {t('Hubungkan akun AI Anda di Penyedia AI untuk mengaktifkan asisten.')}
                </p>
                {showCreateAction && canManageAgents && (
                  <button ref={addButton} onClick={() => begin()} className={`${primary} mt-4`}>
                    <Plus className="h-4 w-4" />
                    {t('Add agent')}
                  </button>
                )}
              </div>
            ) : (
              <div className="space-y-6">
                {/* 1. Profil Asisten Bisnis Tunggal (@cuan) */}
                {(() => {
                  const a = primaryAgent
                  const account = primaryAccount
                  const state = primaryState
                  return (
                    <div className="rounded-2xl border border-border/40 bg-card/30 p-5 sm:p-6 space-y-5 transition-colors">
                      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                        <div className="flex items-center gap-3.5 min-w-0">
                          <div className="relative shrink-0">
                            <div className="flex h-14 w-14 sm:h-16 sm:w-16 items-center justify-center rounded-2xl bg-primary/10 text-primary border border-primary/20 text-2xl font-semibold">
                              {a.avatar ? (
                                <span>{a.avatar}</span>
                              ) : (
                                <Bot className="h-7 w-7" />
                              )}
                            </div>
                            <div className="absolute -bottom-1 -right-1">
                              <StatusDot
                                status={
                                  state?.verified
                                    ? 'healthy'
                                    : state?.state === 'quota_limited'
                                      ? 'warning'
                                      : state?.state === 'login_required' || state?.state === 'unavailable'
                                        ? 'exception'
                                        : 'neutral'
                                }
                                size="md"
                              />
                            </div>
                          </div>
                          <div className="min-w-0 space-y-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <h3 className="text-lg sm:text-xl font-bold tracking-tight text-foreground truncate">
                                {a.name}
                              </h3>
                              <span className="font-mono text-xs text-muted-foreground">
                                {getAgentHandle(a)}
                              </span>
                              <span
                                className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-semibold border ${
                                  state?.verified
                                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                                    : 'bg-muted text-muted-foreground border-border'
                                }`}
                              >
                                {state?.verified && <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />}
                                <span>{state?.verified ? t('Aktif & Siap') : t('Menunggu Verifikasi')}</span>
                              </span>
                            </div>
                            <p className="text-xs sm:text-sm text-muted-foreground truncate">
                              {a.role || t('Cuan Universal Assistant')}
                            </p>
                          </div>
                        </div>

                        {canManageAgents && (
                          <button
                            type="button"
                            onClick={() => setEditingAgent(a)}
                            className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-border bg-background px-4 py-2 text-xs sm:text-sm font-medium text-foreground hover:bg-muted active:scale-[0.98] transition-colors shrink-0"
                            aria-label={`Edit ${a.name}`}
                          >
                            <Pencil className="h-4 w-4" />
                            <span>{t('Edit Profil Asisten')}</span>
                          </button>
                        )}
                      </div>

                      {/* Persona & Instruksi Kerja */}
                      <div className="space-y-1.5 pt-2 border-t border-border/40">
                        <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider block">
                          {t('Peran & Instruksi Kerja')}
                        </label>
                        <div className="p-3.5 rounded-xl border border-border/30 bg-muted/20 text-xs sm:text-sm text-foreground/90 leading-relaxed">
                          {a.system_prompt || a.role || t('Tidak ada instruksi khusus. Menggunakan persona asisten bisnis terpadu default.')}
                        </div>
                      </div>

                      {/* Model AI & Status Mesin */}
                      <div className="space-y-2 pt-2 border-t border-border/40">
                        <div className="flex items-center justify-between">
                          <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider block">
                            {t('Model AI & Konfigurasi')}
                          </label>
                          <button
                            type="button"
                            onClick={() => setDrawerAgent(a)}
                            className="text-xs text-muted-foreground hover:text-foreground font-medium underline-offset-2 hover:underline"
                          >
                            {t('Detail Teknis & Kuota')} ›
                          </button>
                        </div>
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-xs font-medium border border-border/50">
                            <span className="text-muted-foreground">Provider:</span>
                            <span className="font-semibold text-foreground">
                              {a.provider || account?.provider || 'Google'}
                            </span>
                          </span>
                          {a.model_id && (
                            <span className="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-xs font-medium border border-border/50">
                              <span className="text-muted-foreground">Model:</span>
                              <span className="font-semibold text-foreground">{a.model_id}</span>
                              {isModelVisionCapable(a.model_id, a.provider) ? (
                                <span className="inline-flex items-center gap-0.5 px-1 py-0.2 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                                  <Eye className="h-2.5 w-2.5" />
                                  <span>Vision</span>
                                </span>
                              ) : (
                                <span className="inline-flex items-center gap-0.5 px-1 py-0.2 rounded text-[10px] font-medium bg-muted text-muted-foreground border border-border/40">
                                  <span>Text</span>
                                </span>
                              )}
                            </span>
                          )}
                          {a.fallback_model_id && (
                            <span className="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-xs font-medium border border-border/50">
                              <span className="text-muted-foreground">Fallback:</span>
                              <span className="font-semibold text-foreground">{a.fallback_model_id}</span>
                              {isModelVisionCapable(a.fallback_model_id, a.provider) ? (
                                <span className="inline-flex items-center gap-0.5 px-1 py-0.2 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                                  <Eye className="h-2.5 w-2.5" />
                                  <span>Vision</span>
                                </span>
                              ) : (
                                <span className="inline-flex items-center gap-0.5 px-1 py-0.2 rounded text-[10px] font-medium bg-muted text-muted-foreground border border-border/40">
                                  <span>Text</span>
                                </span>
                              )}
                            </span>
                          )}
                          {(a.verified_at || a.last_checked_at) && (
                            <span className="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-xs font-medium border border-border/50">
                              <span className="text-muted-foreground">Verified:</span>
                              <span className="font-semibold text-foreground font-mono">
                                {formatVerificationTime(a.verified_at || a.last_checked_at)}
                              </span>
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  )
                })()}

                {/* 2. Mesin Eksekusi Latar Belakang (Worker Pool) - Progressive Disclosure */}
                <details className="group rounded-2xl border border-border/40 bg-card/20 p-4 sm:p-5 transition-colors">
                  <summary className="flex min-h-11 cursor-pointer items-center justify-between text-sm font-medium text-foreground select-none">
                    <div className="flex items-center gap-3">
                      <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-muted text-muted-foreground shrink-0">
                        <Cpu className="h-4 w-4" />
                      </div>
                      <div className="min-w-0">
                        <span className="font-semibold text-foreground">
                          {t('Mesin Eksekusi Latar Belakang (Worker Pool)')}
                        </span>
                        <p className="text-xs text-muted-foreground font-normal mt-0.5 truncate">
                          {workerAgents.length > 0
                            ? `${workerAgents.length} ${t('worker internal siap mengeksekusi tugas coding & vision otomatis')}`
                            : t('Semua tugas dialirkan ke mesin AI default')}
                        </p>
                      </div>
                    </div>
                    <span className="text-xs text-muted-foreground transition-transform group-open:rotate-90 shrink-0 ml-2">
                      ▸
                    </span>
                  </summary>

                  <div className="mt-4 pt-3 border-t border-border/40 space-y-3">
                    <p className="text-xs text-muted-foreground leading-relaxed">
                      {t('Akun-akun worker internal ini digunakan secara otonom oleh @cuan di balik layar untuk mengeksekusi tugas khusus (coding, terminal, vision) dan auto-failover saat kuota menipis.')}
                    </p>

                    {workerAgents.length > 0 ? (
                      <div className="rounded-xl border border-border/40 bg-card/40 overflow-hidden">
                        <HairlineList role="list" aria-label="Worker pool list">
                          {workerAgents.map((wa) => {
                            const a = wa
                            const waState = connectionStatus(a, now)
                            return (
                              <HairlineListItem
                                role="listitem"
                                aria-label={`Worker ${a.name}`}
                                key={a.id}
                                interactive
                                onClick={() => setDrawerAgent(a)}
                                leading={
                                  <div className="relative shrink-0">
                                    <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-muted/50 text-muted-foreground font-semibold text-sm">
                                      {a.avatar ? <span>{a.avatar}</span> : <Bot className="h-4 w-4" />}
                                    </div>
                                    <div className="absolute -bottom-0.5 -right-0.5">
                                      <StatusDot status={waState.verified ? 'healthy' : 'neutral'} size="sm" />
                                    </div>
                                  </div>
                                }
                                title={
                                  <div className="flex items-center gap-2">
                                    <span className="font-medium text-foreground text-sm">{a.name}</span>
                                    <span className="font-mono text-xs text-muted-foreground">{getAgentHandle(a)}</span>
                                  </div>
                                }
                                description={a.role || t('Worker Pool Engine')}
                                trailing={
                                  <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                                    <span>{a.provider || a.harness || 'engine'}</span>
                                    <span>›</span>
                                  </div>
                                }
                              />
                            )
                          })}
                        </HairlineList>
                      </div>
                    ) : null}

                    <div className="pt-2 flex items-center justify-between text-xs text-muted-foreground">
                      <span>{t('Butuh menambah engine AI atau kuota baru?')}</span>
                      <Link to="/settings/ai-providers" className="text-primary hover:underline font-medium">
                        {t('Buka Penyedia AI')} →
                      </Link>
                    </div>
                  </div>
                </details>
              </div>
            )}

            {/* Progressive Disclosure SlideDrawer for Deep Technical Details & Actions */}
            <SlideDrawer
              isOpen={!!activeDrawerAgent}
        onClose={() => {
          selectAgent(null)
          setReplaceAgentId('')
          setReplacementKey('')
        }}
        title={activeDrawerAgent ? activeDrawerAgent.name : t('Agent details')}
        description={
          activeDrawerAgent
            ? `${getAgentHandle(activeDrawerAgent)} · ${activeDrawerAgent.role || (activeDrawerAgent.system_prompt ? activeDrawerAgent.system_prompt.slice(0, 80) : '') || t('Konfigurasi teknis & routing model')}`
            : undefined
        }
        size="md"
        badge={
          activeDrawerAgent && (
            <span className="text-[10px] uppercase font-mono px-2 py-0.5 bg-primary/10 text-primary border border-primary/20 rounded">
              {activeDrawerAgent.provider || activeDrawerAccount?.provider || 'AI Engine'}
            </span>
          )
        }
        footer={
          activeDrawerAgent && (
            <div className="flex items-center justify-between w-full gap-2">
              {canManageAgents && (
                <button
                  type="button"
                  className="inline-flex min-h-11 items-center justify-center rounded-lg bg-primary px-3.5 py-2 text-xs font-semibold text-primary-foreground hover:bg-primary/90 transition-colors"
                  onClick={() => {
                    setEditingAgent(activeDrawerAgent)
                    selectAgent(null)
                  }}
                >
                  <Pencil className="mr-1.5 h-3.5 w-3.5" />
                  <span>{t('Edit Config')}</span>
                </button>
              )}
              <button
                type="button"
                className="action-secondary min-h-11 px-3 text-xs"
                onClick={() => selectAgent(null)}
              >
                {t('Tutup')}
              </button>
            </div>
          )
        }
      >
        {activeDrawerAgent && activeDrawerState && activeDrawerA && (() => {
          const a = activeDrawerA
          const account = activeDrawerAccount
          const expiry = activeDrawerExpiry
          const state = activeDrawerState
          return (
          <div className="space-y-5 p-1">
            {/* 1. Header: Bot Icon + Agent Name + Status Badge (Left) & Actions Menu (Right) */}
            <div className="flex items-start justify-between gap-3 p-3.5 rounded-xl border border-border/60 bg-muted/20">
              <div className="flex min-w-0 items-center gap-3">
                <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20">
                  {a.avatar ? (
                    <span className="text-lg">{a.avatar}</span>
                  ) : (
                    <Bot className="h-6 w-6" />
                  )}
                </div>
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <h4 className="text-base font-semibold text-foreground truncate">{a.name}</h4>
                    {!(a.name.replace(/^@/, '').toLowerCase().trim() === getAgentHandle(a).replace(/^@/, '').toLowerCase().trim() || a.name.startsWith('@')) && (
                      <span className="font-mono text-xs text-muted-foreground">{getAgentHandle(a)}</span>
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground truncate mt-0.5">
                    {account?.email || a.role || t('Asisten AI')}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <span
                  className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold border ${
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
                {canManageAgents && (
                  <AgentActions
                    name={a.name}
                    disabled={!!checking || !!removing || !!lifecycleBusy}
                    onEdit={() => {
                      setEditingAgent(a)
                      selectAgent(null)
                    }}
                    onRemove={() => void remove(a.id)}
                  />
                )}
              </div>
            </div>

            {/* 2. Metadata Chips: Clean, structured badges eliminating hanging dots */}
            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider block">
                {t('Konfigurasi Model & Provider')}
              </label>
              <div className="flex flex-wrap items-center gap-1.5">
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
                    {isModelVisionCapable(a.model_id, a.provider) ? (
                      <span className="inline-flex items-center gap-0.5 px-1 py-0.2 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                        <Eye className="h-2.5 w-2.5" />
                        <span>Vision</span>
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-0.5 px-1 py-0.2 rounded text-[10px] font-medium bg-muted text-muted-foreground border border-border/40">
                        <span>Text</span>
                      </span>
                    )}
                  </span>
                )}
                {a.fallback_model_id && (
                  <span className="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-[11px] font-medium border border-border/50">
                    <span className="text-muted-foreground">Fallback:</span>
                    <span className="font-semibold text-foreground">{a.fallback_model_id}</span>
                    {isModelVisionCapable(a.fallback_model_id, a.provider) ? (
                      <span className="inline-flex items-center gap-0.5 px-1 py-0.2 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                        <Eye className="h-2.5 w-2.5" />
                        <span>Vision</span>
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-0.5 px-1 py-0.2 rounded text-[10px] font-medium bg-muted text-muted-foreground border border-border/40">
                        <span>Text</span>
                      </span>
                    )}
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
            </div>

            {/* Alerts if quota limited or unavailable */}
            {state.state === 'quota_limited' && (
              <div className="rounded-md border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-800 dark:text-amber-300">
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
              <div className="flex items-center justify-between gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs">
                <span className="min-w-0 truncate text-destructive">
                  {t('Agent unavailable')} · {t('Inbox will not route new work here.')}
                </span>
                <span className="shrink-0 text-muted-foreground">
                  {t('Check connection')}
                </span>
              </div>
            )}

            {/* 3. Primary Actions Row: Dedicated full-width container with comfortable touch targets */}
            <div className="space-y-2 pt-2 border-t border-border/40">
              <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider block">
                {t('Aksi & Siklus Hidup')}
              </label>
              <div className="flex flex-wrap items-center gap-2 pt-2 border-t border-border/40">
                {canManageAgents && (
                  <button
                    type="button"
                    className="inline-flex min-h-11 items-center justify-center rounded-lg border border-border bg-background px-3.5 py-2 text-xs font-medium text-foreground hover:bg-muted active:scale-[0.98] transition-colors"
                    onClick={() => {
                      setEditingAgent(a)
                      selectAgent(null)
                    }}
                    aria-label={`Edit ${a.name}`}
                  >
                    <Pencil className="mr-1.5 h-3.5 w-3.5 shrink-0" />
                    <span>{t('Edit Config')}</span>
                  </button>
                )}
                {state.state !== 'verified' && (
                  <button
                    type="button"
                    className="inline-flex min-h-11 items-center justify-center rounded-lg border border-border bg-background px-3.5 py-2 text-xs font-medium text-foreground hover:bg-muted active:scale-[0.98] transition-colors disabled:opacity-50"
                    disabled={!!lifecycleBusy || !!removing}
                    onClick={() => void reconnect(a, account || undefined)}
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

            {/* Account Failover & Routing Details */}
            {account && (
              <details className="mt-3 border-t border-border/60 pt-2">
                <summary className="flex min-h-11 cursor-pointer items-center justify-between text-sm font-medium text-foreground">
                  <span>{t('Agent details')}</span>
                  <span className="text-xs text-muted-foreground">{t('Quota, routing and usage')}</span>
                </summary>
                <div className="space-y-2.5 pb-1 mt-2">
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

            {/* Connection details accordion */}
            {(a.last_checked_at || a.verified_at || state.state === 'unknown' || state.state === 'unavailable' || state.state === 'login_required') && (
              <details className="mt-2 text-xs text-muted-foreground border-t border-border/40 pt-2">
                <summary className="inline-flex min-h-11 cursor-pointer items-center text-sm underline underline-offset-2">
                  Connection details
                </summary>
                <div className="space-y-2 pb-2 mt-1">
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
        })()}
      </SlideDrawer>
    </div>
  ) : (
    <div className="space-y-6">
          {/* Subheader: Title, description, and Invite button */}
          <div className="flex items-center justify-between gap-3">
            <div>
              <h2 className="text-lg font-semibold text-foreground">{t('Anggota Tim')}</h2>
              <p className="mt-1 text-sm text-muted-foreground hidden sm:block">
                {t('Daftar staf manusia dan undangan anggota tim kolaborator.')}
              </p>
            </div>
            {isOwner && (
              <button
                type="button"
                onClick={() => setIsInviteModalOpen(true)}
                className={`${primary} shrink-0`}
              >
                <UserPlus className="h-4 w-4" />
                <span>{t('Undang Anggota')}</span>
              </button>
            )}
          </div>

          {/* Search bar */}
          {(members.length > 0 || invitations.length > 0) && (
            <div className="flex flex-wrap items-center gap-2">
              <div className="relative min-w-0 flex-1">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <input
                  className="field-control pl-9 w-full text-sm"
                  aria-label={t('Cari nama atau email staf…')}
                  placeholder={t('Cari nama atau email staf…')}
                  value={memberSearch}
                  onChange={(e) => setMemberSearch(e.target.value)}
                />
              </div>
              {memberSearch.trim() && (
                <button
                  type="button"
                  className="action-secondary text-xs px-2.5 py-1.5"
                  onClick={() => setMemberSearch('')}
                >
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

          {teamError && (
            <div role="alert" className="text-sm text-destructive">
              {teamError}{' '}
              <button type="button" onClick={loadTeamMembers} className="underline">
                {t('Try again')}
              </button>
            </div>
          )}

          {teamLoading ? (
            <p role="status" className="text-sm text-muted-foreground">
              {t('Memuat daftar anggota tim…')}
            </p>
          ) : members.length === 0 && invitations.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border p-8 text-center bg-card">
              <Users className="mx-auto h-8 w-8 text-muted-foreground" />
              <h3 className="mt-3 font-medium text-foreground">{t('Belum Ada Anggota Tim')}</h3>
              <p className="mt-1 text-sm text-muted-foreground">
                {t('Undang staf atau karyawan Anda untuk mulai berkolaborasi di Cuan.')}
              </p>
              {isOwner && (
                <button
                  type="button"
                  onClick={() => setIsInviteModalOpen(true)}
                  className={`${primary} mt-4`}
                >
                  <UserPlus className="h-4 w-4" />
                  <span>{t('Undang Anggota Pertama')}</span>
                </button>
              )}
            </div>
          ) : (
            <div className="space-y-6">
              {/* Active Members Section */}
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
                    <User className="h-3.5 w-3.5" />
                    <span>{t('Staf Aktif')} ({filteredMembers.length})</span>
                  </h3>
                </div>

                <div className="rounded-xl border border-border/40 bg-card/40 overflow-hidden transition-colors">
                  <HairlineList
                    role="list"
                    aria-label={t('Daftar Anggota Tim')}
                  >
                    {filteredMembers.map((member) => {
                    const displayName = member.name || member.username || member.email
                    const initials = displayName.slice(0, 2).toUpperCase()
                    const isSelf = member.id === user?.id
                    const isMemberOwner = member.role === 'owner'

                    return (
                      <div
                        role="listitem"
                        key={member.id}
                        className="p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-3"
                      >
                        <div className="flex items-start gap-3.5 min-w-0">
                          <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-full font-bold text-xs ${
                            isMemberOwner
                              ? 'bg-green-500/15 text-green-700 dark:text-green-400 border border-green-500/30'
                              : 'bg-blue-500/15 text-blue-700 dark:text-blue-400 border border-blue-500/30'
                          }`}>
                            {initials}
                          </div>

                          <div className="min-w-0 space-y-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="font-semibold text-sm text-foreground truncate">
                                {displayName}
                              </span>
                              {isSelf && (
                                <span className="text-[10px] bg-muted text-muted-foreground px-1.5 py-0.5 rounded font-mono">
                                  {t('Anda')}
                                </span>
                              )}
                              <span
                                className={`inline-flex items-center gap-1 px-2 py-0.5 text-[11px] font-semibold rounded-full border ${
                                  isMemberOwner
                                    ? 'bg-green-500/15 text-green-600 dark:text-green-400 border-green-500/30'
                                    : 'bg-blue-500/15 text-blue-600 dark:text-blue-400 border-blue-500/30'
                                }`}
                              >
                                {isMemberOwner ? <Shield className="h-3 w-3" /> : <User className="h-3 w-3" />}
                                <span className="capitalize">{isMemberOwner ? t('Owner') : t('Member')}</span>
                              </span>
                              <span className="inline-flex items-center gap-1 px-1.5 py-0.5 text-[10px] font-medium text-green-600 dark:text-green-400 bg-green-500/10 rounded">
                                <span className="h-1.5 w-1.5 rounded-full bg-green-500" />
                                <span>{t('Aktif')}</span>
                              </span>
                            </div>

                            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                              <span>{member.email}</span>
                              <span aria-hidden="true">·</span>
                              <span>{t('Bergabung')} {formatJoinedDate(member.created_at)}</span>
                            </div>
                          </div>
                        </div>

                        {isOwner && !isSelf && (
                          <div className="shrink-0 flex items-center justify-end">
                            <button
                              type="button"
                              onClick={() => handleRemoveMember(member.id, displayName)}
                              disabled={memberActionBusy === member.id}
                              className="inline-flex min-h-11 items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-destructive hover:bg-destructive/10 rounded-lg transition-colors border border-destructive/20 disabled:opacity-50"
                              title={t('Hapus Akses')}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                              <span>{t('Hapus Akses')}</span>
                            </button>
                          </div>
                        )}
                      </div>
                    )
                  })}
                  </HairlineList>
                </div>
              </div>

              {/* Pending Invitations Section */}
              {filteredInvitations.length > 0 && (
                <div className="space-y-3 pt-2">
                  <div className="flex items-center justify-between">
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
                      <Clock className="h-3.5 w-3.5 text-amber-500" />
                      <span>{t('Undangan Tertunda')} ({filteredInvitations.length})</span>
                    </h3>
                  </div>

                  <div className="rounded-xl border border-border/40 bg-card/40 overflow-hidden transition-colors">
                    <HairlineList
                      role="list"
                      aria-label={t('Daftar Undangan Tertunda')}
                    >
                      {filteredInvitations.map((invite) => {
                      const isInviteOwner = invite.role === 'owner'
                      return (
                        <div
                          role="listitem"
                          key={invite.id}
                          className="p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-3"
                        >
                          <div className="flex items-start gap-3.5 min-w-0">
                            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-amber-500/10 text-amber-600 border border-amber-500/20">
                              <Clock className="h-4 w-4" />
                            </div>

                            <div className="min-w-0 space-y-1">
                              <div className="flex flex-wrap items-center gap-2">
                                <span className="font-semibold text-sm text-foreground truncate">
                                  {invite.email}
                                </span>
                                <span
                                  className={`inline-flex items-center gap-1 px-2 py-0.5 text-[11px] font-semibold rounded-full border ${
                                    isInviteOwner
                                      ? 'bg-green-500/15 text-green-600 dark:text-green-400 border-green-500/30'
                                      : 'bg-blue-500/15 text-blue-600 dark:text-blue-400 border-blue-500/30'
                                  }`}
                                >
                                  {isInviteOwner ? <Shield className="h-3 w-3" /> : <User className="h-3 w-3" />}
                                  <span className="capitalize">{isInviteOwner ? t('Owner') : t('Member')}</span>
                                </span>
                                <span className="inline-flex items-center gap-1 px-2 py-0.5 text-[10px] font-medium text-amber-600 dark:text-amber-400 bg-amber-500/10 rounded-full border border-amber-500/20">
                                  <Clock className="h-2.5 w-2.5" />
                                  <span>{t('Menunggu Aktivasi')}</span>
                                </span>
                              </div>

                              <div className="text-xs text-muted-foreground">
                                <span>{t('Berlaku hingga')} {formatJoinedDate(invite.expires_at)}</span>
                              </div>
                            </div>
                          </div>

                          {isOwner && (
                            <div className="shrink-0 flex items-center justify-end">
                              <button
                                type="button"
                                onClick={() => handleRevokeInvite(invite.id, invite.email)}
                                disabled={memberActionBusy === invite.id}
                                className="inline-flex min-h-11 items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-destructive hover:bg-destructive/10 rounded-lg transition-colors border border-destructive/20 disabled:opacity-50"
                                title={t('Cabut Undangan')}
                              >
                                <UserX className="h-3.5 w-3.5" />
                                <span>{t('Cabut Undangan')}</span>
                              </button>
                            </div>
                          )}
                        </div>
                      )
                    })}
                    </HairlineList>
                  </div>
                </div>
              )}
            </div>
          )}
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
                            <div className="flex items-center justify-between">
                              <label
                                htmlFor="commandcode-model"
                                className="text-sm font-medium"
                              >
                                {t('Model')}
                              </label>
                              <button
                                type="button"
                                onClick={() => void loadModels(provider, true)}
                                disabled={refreshingModels || busy}
                                className="inline-flex items-center gap-1 text-xs text-primary hover:underline disabled:opacity-50"
                                title={t('Refresh model catalog from provider')}
                              >
                                <RefreshCw className={`h-3 w-3 ${refreshingModels ? 'animate-spin' : ''}`} />
                                <span>{refreshingModels ? t('Refreshing...') : t('Refresh Catalog')}</span>
                              </button>
                            </div>

                            {!isCustomCommandcodeModel ? (
                              <select
                                id="commandcode-model"
                                value={commandcodeModel}
                                onChange={(e) => {
                                  if (e.target.value === '__custom_entry__') {
                                    setIsCustomCommandcodeModel(true)
                                  } else {
                                    setCommandcodeModel(e.target.value)
                                  }
                                }}
                                disabled={busy || refreshingModels}
                                className="mt-2 w-full rounded-md border border-input bg-background px-3 py-2.5 text-sm"
                              >
                                {refreshingModels && availableModels.length === 0 ? (
                                  <option value="" disabled>{t('Loading models catalog...')}</option>
                                ) : availableModels.length === 0 ? (
                                  <option value="" disabled>{t('No models found. Click Refresh Catalog to reload.')}</option>
                                ) : (
                                  availableModels.map((m) => {
                                    const isVision = isModelVisionCapable(m.id, provider, m)
                                    const capBadge = isVision ? t('Vision') : t('Text-only')
                                    const defBadge = m.is_default ? ` [${t('Default')}]` : ''
                                    return (
                                      <option key={m.id} value={m.id}>
                                        {m.display_name || m.name || m.id} ({capBadge}){defBadge}
                                      </option>
                                    )
                                  })
                                )}
                                <option value="__custom_entry__">✨ -- {t('Custom Model ID / Manual Entry')} --</option>
                              </select>
                            ) : (
                              <div className="mt-2 space-y-1.5">
                                <div className="flex items-center justify-between">
                                  <span className="text-xs text-muted-foreground">{t('Custom Model ID')}</span>
                                  <button
                                    type="button"
                                    onClick={() => {
                                      setIsCustomCommandcodeModel(false)
                                      if (availableModels.length > 0) {
                                        setCommandcodeModel(availableModels[0].id)
                                      }
                                    }}
                                    className="text-xs text-primary hover:underline"
                                  >
                                    {t('Use catalog model')}
                                  </button>
                                </div>
                                <input
                                  id="custom-commandcode-model"
                                  type="text"
                                  value={commandcodeModel}
                                  onChange={(e) => setCommandcodeModel(e.target.value)}
                                  placeholder="e.g. custom-model-id, claude-3-5-sonnet-20241022"
                                  disabled={busy}
                                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono"
                                />
                              </div>
                            )}

                            {/* Capability Badge and Guidance */}
                            <div className="mt-2 flex flex-wrap items-center gap-2">
                              {isModelVisionCapable(commandcodeModel, provider, availableModels.find((m) => m.id === commandcodeModel)) ? (
                                <span className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-semibold bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                                  <Eye className="h-3 w-3" />
                                  <span>Vision ({t('Supports image and document analysis')})</span>
                                </span>
                              ) : (
                                <span className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium bg-muted text-muted-foreground border border-border/40">
                                  <FileText className="h-3 w-3" />
                                  <span>Text-only ({t('Text processing only (no image analysis)')})</span>
                                </span>
                              )}
                            </div>
                            <p className="mt-1 text-xs text-muted-foreground">
                              {isModelVisionCapable(commandcodeModel, provider, availableModels.find((m) => m.id === commandcodeModel))
                                ? t('✓ Model ini mendukung input multimodal (gambar & dokumen).')
                                : t('ℹ Model teks saja (tidak mendukung input gambar / vision).')}
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
      {editingAgent && (
        <EditAgentModal
          agent={editingAgent}
          open={!!editingAgent}
          onClose={() => setEditingAgent(null)}
          onSaved={(updatedAgent) => {
            setAgents((prev) =>
              prev.map((item) => (item.id === updatedAgent.id ? { ...item, ...updatedAgent } : item))
            )
            setEditingAgent(null)
          }}
        />
      )}
      <InviteMemberModal
        open={isInviteModalOpen}
        onClose={() => setIsInviteModalOpen(false)}
        onInviteCreated={loadTeamMembers}
      />
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
