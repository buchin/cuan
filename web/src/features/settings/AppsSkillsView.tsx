import { ListTabs, ListTab } from '../../components/layout/ListControls'
import { IconButton } from '../../components/ui/IconButton'
import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import {
  Boxes,
  Plug,
  Sparkles,
  Check,
  X,
  AlertCircle,
  RefreshCw,
  Bot,
  Receipt,
  Table,
  Unplug,
  ArrowRight,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import {
  appsSkillsApi,
  type App,
  type Skill,
} from '../../api/appsSkills'
import { GroupedSection } from '../../components/layout/GroupedSection'
import { StatusDot } from '../../components/layout/StatusDot'

export interface AppsSkillsViewProps {
  showHeading?: boolean
  initialTab?: 'apps' | 'skills'
}

export function AppsSkillsView({ initialTab = 'apps', showHeading = true }: AppsSkillsViewProps) {
  const { t } = useLanguage()

  const [activeTab, setActiveTab] = useState<'apps' | 'skills'>(initialTab)
  const [apps, setApps] = useState<App[]>([])
  const [skills, setSkills] = useState<Skill[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [actionFeedback, setActionFeedback] = useState<{
    type: 'success' | 'error'
    message: string
  } | null>(null)

  // App Connect Modal State
  const [connectingAppId, setConnectingAppId] = useState<string | null>(null)
  const [accountInput, setAccountInput] = useState('')
  const [connecting, setConnecting] = useState(false)

  // App Disconnect Confirmation State
  const [disconnectingAppId, setDisconnectingAppId] = useState<string | null>(null)
  const [disconnecting, setDisconnecting] = useState(false)

  // Skills filter
  const [skillCategoryFilter, setSkillCategoryFilter] = useState<string>('all')

  const loadData = async (isRefresh = false) => {
    if (isRefresh) setRefreshing(true)
    else setLoading(true)
    setError(null)

    try {
      const [appsRes, skillsRes] = await Promise.all([
        appsSkillsApi.listApps(),
        appsSkillsApi.listSkills(),
      ])
      setApps(appsRes.apps || [])
      setSkills(skillsRes.skills || [])
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('Failed to load apps and skills catalog')
      setError(msg)
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const handleOpenConnect = (app: App) => {
    setConnectingAppId(app.id)
    setAccountInput(app.account || 'bisnis@cuan.id')
  }

  const handleConfirmConnect = async () => {
    if (!connectingAppId) return
    setConnecting(true)
    setActionFeedback(null)

    try {
      const res = await appsSkillsApi.connectApp(connectingAppId, accountInput.trim())
      setApps((prev) =>
        prev.map((a) => (a.id === connectingAppId ? res.app : a))
      )
      setActionFeedback({
        type: 'success',
        message: t('App connected successfully! 1-click integration is now active.'),
      })
      setConnectingAppId(null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('Failed to connect app')
      setActionFeedback({ type: 'error', message: msg })
    } finally {
      setConnecting(false)
    }
  }

  const handleConfirmDisconnect = async () => {
    if (!disconnectingAppId) return
    setDisconnecting(true)
    setActionFeedback(null)

    try {
      const res = await appsSkillsApi.disconnectApp(disconnectingAppId)
      setApps((prev) =>
        prev.map((a) => (a.id === disconnectingAppId ? res.app : a))
      )
      setActionFeedback({
        type: 'success',
        message: t('App disconnected successfully.'),
      })
      setDisconnectingAppId(null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('Failed to disconnect app')
      setActionFeedback({ type: 'error', message: msg })
    } finally {
      setDisconnecting(false)
    }
  }

  const filteredSkills =
    skillCategoryFilter === 'all'
      ? skills
      : skills.filter((s) => s.category?.toLowerCase() === skillCategoryFilter.toLowerCase())

  return (
    <div className="space-y-6" data-testid="apps-skills-view">
      {/* Header & Description */}
      <div className={showHeading ? "flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-2 border-b border-border/50" : "hidden"}>
        {showHeading && <div>
          <h1 className="text-xl font-bold tracking-tight text-foreground flex items-center gap-2">
            <Boxes className="h-5 w-5 text-primary" />
            <span>{t('Apps & Skills')}</span>
          </h1>
          <p className="mt-1 text-xs sm:text-sm text-muted-foreground">
            {t(
              'Connect 1-click external business apps and discover modular AI intelligence capabilities.'
            )}
          </p>
        </div>}

        <IconButton label={t('Refresh catalog')} disabled={loading || refreshing} onClick={() => loadData(true)}><RefreshCw className={refreshing ? 'animate-spin' : ''} /></IconButton>
      </div>
      <ListTabs aria-label={t('Apps & Skills')}>
        <ListTab selected={activeTab === 'apps'} count={apps.length} onClick={() => setActiveTab('apps')} data-testid="subtab-apps">{t('Connected Apps')}</ListTab>
        <ListTab selected={activeTab === 'skills'} count={skills.length} onClick={() => setActiveTab('skills')} data-testid="subtab-skills">{t('Skills Catalog')}</ListTab>
      </ListTabs>

      {/* Action Notification Banner */}
      {actionFeedback && (
        <div
          role="status"
          className={`p-3 rounded-lg border text-xs flex items-center justify-between gap-2 animate-in fade-in ${
            actionFeedback.type === 'success'
              ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-700 dark:text-emerald-300'
              : 'bg-destructive/10 border-destructive/30 text-destructive'
          }`}
        >
          <div className="flex items-center gap-2">
            {actionFeedback.type === 'success' ? (
              <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400 shrink-0" />
            ) : (
              <AlertCircle className="h-4 w-4 text-destructive shrink-0" />
            )}
            <span>{actionFeedback.message}</span>
          </div>
          <button
            type="button"
            onClick={() => setActionFeedback(null)}
            className="p-1 rounded hover:bg-black/5 dark:hover:bg-white/5"
            aria-label={t('Dismiss')}
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* Global Error Banner */}
      {error && (
        <div className="p-3.5 rounded-lg border border-destructive/30 bg-destructive/10 text-destructive text-xs flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
          <button
            type="button"
            onClick={() => loadData()}
            className="font-semibold underline hover:no-underline"
          >
            {t('Retry')}
          </button>
        </div>
      )}

      {/* Loading Skeleton */}
      {loading && !refreshing && (
        <div className="py-12 flex flex-col items-center justify-center gap-3 text-muted-foreground">
          <RefreshCw className="h-6 w-6 animate-spin text-primary" />
          <p className="text-xs">{t('Loading catalog...')}</p>
        </div>
      )}

      {/* ==================================================================== */}
      {/* TAB 1: CONNECTED APPS (Apple-style Inset Grouped Rows) */}
      {/* ==================================================================== */}
      {!loading && activeTab === 'apps' && (
        <div className="space-y-4">
          {apps.length === 0 ? (
            <div className="p-8 text-center rounded-xl border border-border bg-card text-muted-foreground text-xs space-y-2">
              <Plug className="h-8 w-8 mx-auto opacity-40" />
              <p className="font-semibold">{t('No integrated apps found')}</p>
              <p>{t('Apps registered in the system will automatically appear here.')}</p>
            </div>
          ) : (
            <GroupedSection
              title={t('Connected Apps')}
              description={t('1-click integrations with your business services.')}
            >
              {apps.map((app) => {
                const isConnected = app.status === 'connected'
                const isError = app.status === 'error'

                return (
                  <div
                    key={app.id}
                    className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 px-3.5 py-3.5 sm:px-4 sm:py-4 transition-colors"
                    data-testid={`app-card-${app.id}`}
                  >
                    {/* Left: Icon + Info */}
                    <div className="flex items-start gap-3 min-w-0 flex-1">
                      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary mt-0.5">
                        {app.icon === 'sheet' || app.id.includes('sheet') ? (
                          <Table className="h-4 w-4" />
                        ) : (
                          <Plug className="h-4 w-4" />
                        )}
                      </div>

                      <div className="min-w-0 flex-1 space-y-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-sm font-semibold text-foreground">
                            {app.name}
                          </span>
                          <span className="text-[10px] uppercase tracking-wider font-medium text-muted-foreground">
                            {app.category}
                          </span>
                          <span className="inline-flex items-center gap-1.5 text-xs">
                            <StatusDot
                              status={isConnected ? 'healthy' : isError ? 'exception' : 'neutral'}
                              size="xs"
                            />
                            <span
                              className={`text-[11px] font-medium ${
                                isConnected
                                  ? 'text-emerald-600 dark:text-emerald-400'
                                  : isError
                                  ? 'text-destructive'
                                  : 'text-muted-foreground'
                              }`}
                              data-testid={`app-status-${app.id}`}
                            >
                              {isConnected
                                ? t('Connected')
                                : isError
                                ? t('Connection Error')
                                : t('Disconnected')}
                            </span>
                          </span>
                        </div>

                        <p className="text-xs text-muted-foreground leading-relaxed">
                          {app.description}
                        </p>

                        {/* Connected Account Info - flat inline, 0 nested cards */}
                        {isConnected && app.account && (
                          <div className="text-xs text-muted-foreground flex items-center gap-1.5 pt-0.5">
                            <span className="text-[11px] text-muted-foreground/80">
                              {t('Connected Account')}:
                            </span>
                            <span className="font-mono text-foreground font-medium text-xs">
                              {app.account}
                            </span>
                          </div>
                        )}

                        {/* Capabilities - flat minimal text / tags */}
                        {app.capabilities && app.capabilities.length > 0 && (
                          <div className="flex flex-wrap items-center gap-1.5 pt-0.5">
                            <span className="text-[10px] uppercase tracking-wider text-muted-foreground/80">
                              {t('Capabilities')}:
                            </span>
                            {app.capabilities.map((cap) => (
                              <span
                                key={cap}
                                className="px-1.5 py-0.5 rounded text-[10px] font-mono bg-muted text-muted-foreground"
                              >
                                {cap}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>

                    {/* Right: Action button */}
                    <div className="flex shrink-0 items-center sm:self-center pl-12 sm:pl-0">
                      {isConnected ? (
                        <button
                          type="button"
                          onClick={() => setDisconnectingAppId(app.id)}
                          className="min-h-11 sm:min-h-8 inline-flex items-center justify-center gap-1.5 rounded-lg border border-border/60 bg-background px-3 py-1.5 text-xs font-medium text-destructive hover:bg-destructive/10 transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring active:scale-[0.99] cursor-pointer"
                          data-testid={`disconnect-btn-${app.id}`}
                        >
                          <Unplug className="h-3.5 w-3.5" />
                          <span>{t('Disconnect App')}</span>
                        </button>
                      ) : (
                        <button
                          type="button"
                          onClick={() => handleOpenConnect(app)}
                          className="min-h-11 sm:min-h-8 inline-flex items-center justify-center gap-1.5 rounded-lg bg-primary px-3.5 py-1.5 text-xs font-semibold text-primary-foreground hover:bg-primary/90 transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring active:scale-[0.99] shadow-2xs cursor-pointer"
                          data-testid={`connect-btn-${app.id}`}
                        >
                          <Plug className="h-3.5 w-3.5" />
                          <span>{t('1-Click Connect')}</span>
                        </button>
                      )}
                    </div>
                  </div>
                )
              })}
            </GroupedSection>
          )}
        </div>
      )}

      {/* ==================================================================== */}
      {/* TAB 2: SKILLS CATALOG (Apple-style Inset Grouped Rows) */}
      {/* ==================================================================== */}
      {!loading && activeTab === 'skills' && (
        <div className="space-y-4">
          {/* Category Filter Chips */}
          <div className="flex flex-wrap items-center gap-1.5 pb-2">
            {[
              { id: 'all', label: t('All Skills') },
              { id: 'vision', label: t('Vision & OCR') },
              { id: 'finance', label: t('Finance & Accounting') },
              { id: 'productivity', label: t('Productivity') },
              { id: 'automation', label: t('Automation') },
            ].map((cat) => (
              <button
                key={cat.id}
                type="button"
                onClick={() => setSkillCategoryFilter(cat.id)}
                className={`inline-flex min-h-11 sm:min-h-8 items-center rounded-lg px-3 py-1 text-xs font-medium border transition-colors cursor-pointer ${
                  skillCategoryFilter === cat.id
                    ? 'border-primary bg-primary/10 text-primary font-semibold'
                    : 'border-border/60 bg-background text-muted-foreground hover:bg-muted hover:text-foreground'
                }`}
              >
                {cat.label}
              </button>
            ))}
          </div>

          {filteredSkills.length === 0 ? (
            <div className="p-8 text-center rounded-xl border border-border bg-card text-muted-foreground text-xs space-y-2">
              <Sparkles className="h-8 w-8 mx-auto opacity-40" />
              <p className="font-semibold">{t('No skills found in this category')}</p>
            </div>
          ) : (
            <GroupedSection
              title={t('Skills Catalog')}
              description={t('Modular intelligence capabilities available to assign to agents.')}
            >
              {filteredSkills.map((skill) => {
                const isVision =
                  skill.category === 'vision' ||
                  skill.id.includes('vision') ||
                  skill.id.includes('receipt')
                const isSheet =
                  skill.category === 'productivity' || skill.id.includes('sheet')

                return (
                  <div
                    key={skill.id}
                    className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 px-3.5 py-3.5 sm:px-4 sm:py-4 transition-colors"
                    data-testid={`skill-card-${skill.id}`}
                  >
                    {/* Left: Icon + Info */}
                    <div className="flex items-start gap-3 min-w-0 flex-1">
                      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary mt-0.5">
                        {isVision ? (
                          <Receipt className="h-4 w-4" />
                        ) : isSheet ? (
                          <Table className="h-4 w-4" />
                        ) : (
                          <Boxes className="h-4 w-4" />
                        )}
                      </div>

                      <div className="min-w-0 flex-1 space-y-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-sm font-semibold text-foreground">
                            {skill.name}
                          </span>
                          <span className="text-[10px] uppercase tracking-wider font-medium text-muted-foreground">
                            {skill.category}
                          </span>
                          <span className="inline-flex items-center gap-1.5 text-xs">
                            <StatusDot status="healthy" size="xs" />
                            <span className="text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
                              {t('Active')}
                            </span>
                          </span>
                        </div>

                        <p className="text-xs text-muted-foreground leading-relaxed">
                          {skill.description}
                        </p>

                        {/* Bound Agents Badge List */}
                        <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground pt-0.5">
                          <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1">
                            <Bot className="h-3 w-3 text-primary" />
                            <span>{t('Bound Agents')}:</span>
                          </span>

                          {skill.bound_agents && skill.bound_agents.length > 0 ? (
                            <div className="flex flex-wrap gap-1">
                              {skill.bound_agents.map((agId) => (
                                <span
                                  key={agId}
                                  className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[11px] font-medium bg-muted text-foreground"
                                >
                                  <span>{agId.replace('agent_', '@')}</span>
                                </span>
                              ))}
                            </div>
                          ) : (
                            <span className="text-[11px] text-muted-foreground italic">
                              {t('No agents bound yet')}
                            </span>
                          )}
                        </div>

                        {/* Tools list */}
                        {skill.tools && skill.tools.length > 0 && (
                          <div className="flex flex-wrap items-center gap-1 pt-0.5">
                            <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                              Tools:
                            </span>
                            {skill.tools.map((tl) => (
                              <span
                                key={tl}
                                className="px-1.5 py-0.5 rounded text-[10px] font-mono bg-muted/60 text-muted-foreground"
                              >
                                {tl}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>

                    {/* Right: Configure link */}
                    <div className="flex shrink-0 items-center sm:self-center pl-12 sm:pl-0">
                      <Link
                        to="/settings/agents"
                        className="min-h-11 sm:min-h-8 inline-flex items-center justify-center gap-1.5 rounded-lg border border-border/60 bg-background px-3 py-1.5 text-xs font-semibold text-foreground hover:bg-muted transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
                      >
                        <Bot className="h-3.5 w-3.5 text-primary" />
                        <span>{t('Configure in Agents & Team')}</span>
                        <ArrowRight className="h-3.5 w-3.5 ml-0.5 opacity-60" />
                      </Link>
                    </div>
                  </div>
                )
              })}
            </GroupedSection>
          )}
        </div>
      )}

      {/* ==================================================================== */}
      {/* 1-CLICK CONNECT MODAL */}
      {/* ==================================================================== */}
      {connectingAppId && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs animate-in fade-in"
        >
          <div className="w-full max-w-md rounded-xl border border-border bg-card p-5 shadow-xl space-y-4">
            <div className="flex items-center justify-between pb-2 border-b border-border/60">
              <div className="flex items-center gap-2">
                <Plug className="h-4 w-4 text-primary" />
                <h3 className="text-sm font-bold text-foreground">
                  {t('1-Click Connect Account')}
                </h3>
              </div>
              <button
                type="button"
                onClick={() => setConnectingAppId(null)}
                disabled={connecting}
                className="p-1 rounded text-muted-foreground hover:text-foreground"
                aria-label={t('Close')}
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="space-y-3">
              <p className="text-xs text-muted-foreground leading-relaxed">
                {t(
                  'Enter the account email or identifier you wish to connect for this integration. Zero ambient credentials will be leaked.'
                )}
              </p>

              <div className="space-y-1.5">
                <label
                  htmlFor="connect-account-input"
                  className="block text-xs font-semibold text-foreground"
                >
                  {t('Account Identifier / Email')}
                </label>
                <input
                  id="connect-account-input"
                  type="text"
                  value={accountInput}
                  onChange={(e) => setAccountInput(e.target.value)}
                  placeholder="bisnis@cuan.id"
                  disabled={connecting}
                  className="w-full min-h-11 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary disabled:opacity-50"
                />
              </div>

              {/* Quick preset account chips */}
              <div className="flex flex-wrap gap-1.5 pt-1">
                {['bisnis@cuan.id', 'owner@workspace.com', 'admin@cuan.id'].map((acc) => (
                  <button
                    key={acc}
                    type="button"
                    onClick={() => setAccountInput(acc)}
                    disabled={connecting}
                    className="px-2 py-1 text-[11px] rounded bg-muted hover:bg-muted/80 text-foreground border border-border/50 transition-colors"
                  >
                    {acc}
                  </button>
                ))}
              </div>
            </div>

            <div className="flex flex-col-reverse sm:flex-row items-center justify-end gap-2 pt-2 border-t border-border/60">
              <button
                type="button"
                onClick={() => setConnectingAppId(null)}
                disabled={connecting}
                className="w-full sm:w-auto inline-flex min-h-11 items-center justify-center rounded-lg border border-border bg-background px-4 py-2 text-xs font-medium text-foreground hover:bg-muted transition-colors disabled:opacity-50"
              >
                {t('Cancel')}
              </button>
              <button
                type="button"
                onClick={handleConfirmConnect}
                disabled={connecting || !accountInput.trim()}
                className="w-full sm:w-auto inline-flex min-h-11 items-center justify-center gap-1.5 rounded-lg bg-primary px-5 py-2 text-xs font-semibold text-primary-foreground hover:bg-primary/90 transition-colors shadow-xs disabled:opacity-50"
              >
                {connecting ? (
                  <>
                    <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                    <span>{t('Connecting...')}</span>
                  </>
                ) : (
                  <>
                    <Check className="h-3.5 w-3.5" />
                    <span>{t('Connect Now')}</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ==================================================================== */}
      {/* 1-CLICK DISCONNECT CONFIRMATION MODAL */}
      {/* ==================================================================== */}
      {disconnectingAppId && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs animate-in fade-in"
        >
          <div className="w-full max-w-md rounded-xl border border-border bg-card p-5 shadow-xl space-y-4">
            <div className="flex items-center gap-2 text-destructive">
              <AlertCircle className="h-5 w-5" />
              <h3 className="text-sm font-bold text-foreground">
                {t('Disconnect Application?')}
              </h3>
            </div>

            <p className="text-xs text-muted-foreground leading-relaxed">
              {t(
                'Are you sure you want to disconnect this app? Agents will no longer be able to sync data with this service until reconnected.'
              )}
            </p>

            <div className="flex flex-col-reverse sm:flex-row items-center justify-end gap-2 pt-2 border-t border-border/60">
              <button
                type="button"
                onClick={() => setDisconnectingAppId(null)}
                disabled={disconnecting}
                className="w-full sm:w-auto inline-flex min-h-11 items-center justify-center rounded-lg border border-border bg-background px-4 py-2 text-xs font-medium text-foreground hover:bg-muted transition-colors disabled:opacity-50"
              >
                {t('Cancel')}
              </button>
              <button
                type="button"
                onClick={handleConfirmDisconnect}
                disabled={disconnecting}
                className="w-full sm:w-auto inline-flex min-h-11 items-center justify-center gap-1.5 rounded-lg bg-destructive px-5 py-2 text-xs font-semibold text-destructive-foreground hover:bg-destructive/90 transition-colors shadow-xs disabled:opacity-50"
              >
                {disconnecting ? (
                  <>
                    <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                    <span>{t('Disconnecting...')}</span>
                  </>
                ) : (
                  <>
                    <Unplug className="h-3.5 w-3.5" />
                    <span>{t('Yes, Disconnect')}</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
