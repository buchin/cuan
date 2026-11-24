import { useEffect, useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  CheckCircle2,
  XCircle,
  Loader2,
  Slash,
  GitCommit,
  GitBranch,
  Rocket,
  Globe,
  ChevronRight,
  Copy,
  Check,
  RefreshCw,
  AlertCircle,
  Ban,
  Clock,
  Webhook,
  ShieldCheck,
  Sliders
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { SlideDrawer } from '../../components/layout/SlideDrawer'
import { AccordionTimeline, AccordionTimelineItem } from '../../components/ui/AccordionTimeline'
import { StatusDot } from '../../components/ui/StatusDot'
import {
  projectsApi,
  ProjectDeploymentsResponse,
  ProjectDeploymentItem
} from '../../api/projects'
import { inboxApi } from '../../api/inbox'
import type { Project } from '../../api/types'

interface ProjectDeploymentsViewProps {
  project: Project
  onRetry?: () => void
}

export function ProjectDeploymentsView({ project }: ProjectDeploymentsViewProps) {
  const { t, language } = useLanguage()
  const navigate = useNavigate()
  const [data, setData] = useState<ProjectDeploymentsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [triggering, setTriggering] = useState(false)
  const [cancelingId, setCancelingId] = useState<string | null>(null)
  const [selectedDepId, setSelectedDepId] = useState<string | null>(null)
  const [escalatingId, setEscalatingId] = useState<string | null>(null)
  const [isWebhookOpen, setIsWebhookOpen] = useState(false)
  const [autoDeploy, setAutoDeploy] = useState(true)
  const [deployBranch, setDeployBranch] = useState('dev')
  const [deployScript, setDeployScript] = useState('')
  const [savingConfig, setSavingConfig] = useState(false)
  const [savedSuccess, setSavedSuccess] = useState(false)
  const [copiedSha, setCopiedSha] = useState(false)
  const [copiedUrl, setCopiedUrl] = useState(false)
  const [copiedSecret, setCopiedSecret] = useState(false)

  const selectedDeployment = useMemo(() => {
    if (!selectedDepId || !data) return null
    return data.deployments.find((d) => d.id === selectedDepId) || null
  }, [selectedDepId, data])

  const loadDeployments = async (silent = false) => {
    if (!silent) setLoading(true)
    setError('')
    try {
      const res = await projectsApi.getDeployments(project.id, 50)
      setData(res)
      if (res) {
        setAutoDeploy(res.auto_deploy !== false)
        setDeployBranch(res.deploy_branch || res.branch || 'dev')
        setDeployScript(res.deploy_script || '')
      }
    } catch (e: any) {
      setError(e?.message || t('Gagal memuat riwayat deployment proyek.'))
    } finally {
      if (!silent) setLoading(false)
    }
  }

  useEffect(() => {
    void loadDeployments()
  }, [project.id])

  // Auto-refresh when any deployment is currently building or queued in lane
  const isAnyActive = useMemo(() => {
    return data?.deployments.some((d) => d.status === 'building' || d.status === 'queued')
  }, [data])

  useEffect(() => {
    if (!isAnyActive) return
    const timer = setInterval(() => {
      void loadDeployments(true)
    }, 2500)
    return () => clearInterval(timer)
  }, [isAnyActive, project.id])

  const handleTriggerDeploy = async () => {
    if (triggering) return
    setTriggering(true)
    try {
      const res = await projectsApi.triggerDeploy(project.id, {
        branch: data?.branch || project.default_branch || 'dev'
      })
      // Open the drawer for the newly triggered deployment
      if (res?.deployment?.id) {
        setSelectedDepId(res.deployment.id)
      }
      await loadDeployments(true)
    } catch (e: any) {
      alert(e?.message || t('Gagal memicu deployment.'))
    } finally {
      setTriggering(false)
    }
  }

  const handleCancelDeploy = async (depId: string) => {
    if (cancelingId) return
    setCancelingId(depId)
    try {
      await projectsApi.cancelDeploy(project.id, depId)
      await loadDeployments(true)
    } catch (e: any) {
      console.error('Cancel deploy error:', e)
    } finally {
      setCancelingId(null)
    }
  }

  const handleVisit = () => {
    let url = data?.domain
    if (!url) {
      alert(t('Domain proyek belum dikonfigurasi.'))
      return
    }
    if (!url.startsWith('http://') && !url.startsWith('https://')) {
      url = 'https://' + url
    }
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  const handleCopySha = (sha: string) => {
    navigator.clipboard.writeText(sha)
    setCopiedSha(true)
    setTimeout(() => setCopiedSha(false), 2000)
  }

  const handleCopyUrl = () => {
    if (!data?.webhook_url) return
    navigator.clipboard.writeText(data.webhook_url)
    setCopiedUrl(true)
    setTimeout(() => setCopiedUrl(false), 2000)
  }

  const handleCopySecret = () => {
    if (!data?.webhook_secret) return
    navigator.clipboard.writeText(data.webhook_secret)
    setCopiedSecret(true)
    setTimeout(() => setCopiedSecret(false), 2000)
  }

  const handleSaveConfig = async () => {
    if (savingConfig) return
    setSavingConfig(true)
    try {
      await projectsApi.updateDeploymentConfig(project.id, {
        auto_deploy: autoDeploy,
        deploy_branch: deployBranch,
        deploy_script: deployScript,
      })
      setSavedSuccess(true)
      setTimeout(() => setSavedSuccess(false), 2500)
      await loadDeployments(true)
    } catch (e: any) {
      alert(e?.message || t('Gagal menyimpan konfigurasi deployment.'))
    } finally {
      setSavingConfig(false)
    }
  }

  const handleEscalateToAgent = async (dep: ProjectDeploymentItem) => {
    setEscalatingId(dep.id)
    try {
      const failedStep = dep.steps?.find((s) => s.status === 'failed') || dep.steps?.[1]
      const title = `Perbaikan Deployment #${dep.short_sha}`
      const errorSnippet = (failedStep?.log || dep.error_message || failedStep?.summary || '').trim()
      const prompt = `Deployment rilis #${dep.short_sha} untuk commit "${dep.commit_message || 'manual'}" gagal pada tahap "${failedStep?.name || 'Build'}".\n\nRingkasan:\n${failedStep?.summary || 'Build pipeline failed.'}\n\n${errorSnippet ? `Log Diagnostik:\n\`\`\`\n${errorSnippet.slice(-1500)}\n\`\`\`\n\n` : ''}Mohon bantu analisis akar masalahnya (RCA) dan perbaiki agar rilis berikutnya dapat berjalan lancar.`

      const res = await inboxApi.create({
        title,
        message: prompt,
        project_id: project.id,
      })
      if (res?.thread?.id) {
        navigate(`/inbox/${encodeURIComponent(res.thread.id)}`)
      }
    } catch (err: any) {
      console.error('Failed to escalate deployment to agent:', err)
    } finally {
      setEscalatingId(null)
    }
  }

  const formatDeploymentTime = (dep: ProjectDeploymentItem) => {
    const raw = dep.created_at || dep.started_at
    if (!raw) return dep.relative_time || ''
    const n = Date.parse(raw)
    if (!Number.isFinite(n)) return dep.relative_time || ''
    const seconds = Math.max(0, Math.round((Date.now() - n) / 1000))
    if (language === 'id') {
      if (seconds < 60) return 'Baru saja'
      if (seconds < 3600) return `${Math.floor(seconds / 60)}m lalu`
      if (seconds < 86400) return `${Math.floor(seconds / 3600)}j lalu`
      if (seconds < 604800) return `${Math.floor(seconds / 86400)}h lalu`
      return new Date(n).toLocaleDateString('id-ID', { month: 'short', day: 'numeric' })
    }
    if (seconds < 60) return 'Just now'
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
    if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
    if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`
    return new Date(n).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
  }

  const renderStatusIcon = (status: string, size = 'md') => {
    const dim = size === 'sm' ? 'h-4 w-4' : 'h-5 w-5'
    switch (status) {
      case 'success':
        return (
          <div className="flex items-center justify-center rounded-full bg-emerald-500/10 p-0.5 text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className={dim} />
          </div>
        )
      case 'building':
        return (
          <div className="flex items-center justify-center rounded-full bg-sky-500/10 p-0.5 text-sky-600 dark:text-sky-400">
            <Loader2 className={`${dim} animate-spin`} />
          </div>
        )
      case 'failed':
        return (
          <div className="flex items-center justify-center rounded-full bg-rose-500/10 p-0.5 text-rose-600 dark:text-rose-400">
            <XCircle className={dim} />
          </div>
        )
      case 'queued':
        return (
          <div className="flex items-center justify-center rounded-full bg-amber-500/10 p-0.5 text-amber-600 dark:text-amber-400" title={t('Queued in lane')}>
            <Clock className={dim} />
          </div>
        )
      case 'canceled':
      default:
        return (
          <div className="flex items-center justify-center rounded-full bg-muted p-0.5 text-muted-foreground">
            <Slash className={dim} />
          </div>
        )
    }
  }

  const timelineItems: AccordionTimelineItem[] = useMemo(() => {
    if (!selectedDeployment?.steps) return []
    return selectedDeployment.steps.map((step) => {
      let indicator: React.ReactNode = <StatusDot size="md" status="neutral" aria-hidden />
      if (step.status === 'success') {
        indicator = <CheckCircle2 className="h-4 w-4 text-[color:var(--status-success)]" aria-hidden />
      } else if (step.status === 'building') {
        indicator = <StatusDot size="md" status="working" pulse aria-hidden />
      } else if (step.status === 'failed') {
        indicator = <StatusDot size="md" status="danger" aria-hidden />
      } else if (step.status === 'pending') {
        indicator = <StatusDot size="md" status="idle" aria-hidden />
      }

      // Defensive UI sanitization: ensure success steps never display pending or compiling placeholder strings
      let displaySummary = step.summary
      if (step.status === 'success') {
        const isStalePlaceholder = !displaySummary ||
          displaySummary.startsWith('Pending') ||
          displaySummary.startsWith('Compiling') ||
          displaySummary.startsWith('Verifying') ||
          displaySummary.startsWith('Performing') ||
          displaySummary.startsWith('Probing')

        if (isStalePlaceholder) {
          switch (step.id) {
            case 'step-1':
              displaySummary = `Checked out commit ${selectedDeployment.short_sha}`
              break
            case 'step-2':
              displaySummary = t('Application compiled successfully')
              break
            case 'step-3':
              displaySummary = t('Database schema verified and up to date')
              break
            case 'step-4':
              displaySummary = t('Release symlink swapped and process reloaded')
              break
            case 'step-5':
              displaySummary = t('Live service health check passed')
              break
          }
        }
      }

      const hasContent = !!displaySummary || !!step.log

      return {
        id: step.id,
        title: <span className="font-medium text-foreground text-sm">{step.name}</span>,
        meta: step.duration && (
          <span className="font-mono text-[11px] text-muted-foreground bg-muted/60 px-1.5 py-0.5 rounded">
            {step.duration}
          </span>
        ),
        indicator,
        content: hasContent ? (
          <div className="space-y-2 pt-1 text-xs">
            {displaySummary && (
              <p className="text-muted-foreground">{displaySummary}</p>
            )}
            {step.log && (
              <pre className="mt-1.5 rounded-lg bg-neutral-950 p-3 font-mono text-[11px] leading-relaxed text-neutral-300 overflow-x-auto whitespace-pre-wrap select-text border border-neutral-800 shadow-inner">
                {step.log}
              </pre>
            )}
          </div>
        ) : undefined,
      }
    })
  }, [selectedDeployment?.steps, selectedDeployment?.short_sha, t])

  return (
    <div className="flex h-full flex-col bg-background text-foreground">
      {/* Top Header matching Laravel Cloud design */}
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-border/40 px-4 py-3 sm:px-6">
        <div>
          <h2 className="text-base font-semibold tracking-tight text-foreground sm:text-lg">
            {t('Latest deployments')}
          </h2>
          {data && (
            <p className="text-xs text-muted-foreground mt-0.5 flex items-center gap-2">
              <span>{t('Branch')}: <span className="font-mono font-medium text-foreground">{data.branch}</span></span>
              <span>•</span>
              <span>{data.total} {t('deployment(s)')}</span>
            </p>
          )}
        </div>

        {/* Action Buttons: Webhook, Deploy & Visit */}
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setIsWebhookOpen(true)}
            id="deployments-webhook-btn"
            className="inline-flex min-h-9 items-center justify-center gap-1.5 rounded-lg border border-border/60 bg-background px-3 py-1.5 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors cursor-pointer"
            title={t('Pengaturan Deployment, Script & Webhook')}
          >
            <Sliders className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="hidden sm:inline">{t('Pengaturan')}</span>
          </button>

          <button
            type="button"
            disabled={triggering}
            onClick={handleTriggerDeploy}
            id="deployments-trigger-btn"
            className="inline-flex min-h-9 items-center justify-center gap-2 rounded-lg bg-foreground px-3.5 py-1.5 text-xs font-medium text-background shadow-2xs hover:bg-foreground/90 active:bg-foreground/80 disabled:opacity-50 transition-colors cursor-pointer"
          >
            {triggering ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Rocket className="h-3.5 w-3.5" />
            )}
            <span>{t('Deploy')}</span>
          </button>

          <button
            type="button"
            onClick={handleVisit}
            id="deployments-visit-btn"
            className="inline-flex min-h-9 items-center justify-center gap-2 rounded-lg border border-border/60 bg-background px-3.5 py-1.5 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors cursor-pointer"
          >
            <Globe className="h-3.5 w-3.5 text-muted-foreground" />
            <span>{t('Visit')}</span>
          </button>
        </div>
      </div>

      {/* Deployments List Container */}
      <div className="flex-1 overflow-y-auto px-4 py-4 sm:px-6">
        {loading && !data && (
          <div className="flex flex-col items-center justify-center py-16 text-center text-sm text-muted-foreground">
            <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground/60 mb-2" />
            <p>{t('Memuat riwayat deployment...')}</p>
          </div>
        )}

        {error && (
          <div className="rounded-xl border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive flex items-start gap-3 my-4">
            <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
            <div>
              <p className="font-medium">{t('Gagal memuat deployment')}</p>
              <p className="text-xs text-destructive/80 mt-0.5">{error}</p>
              <button
                type="button"
                onClick={() => void loadDeployments(false)}
                className="mt-2 text-xs font-semibold underline hover:no-underline cursor-pointer"
              >
                {t('Coba lagi')}
              </button>
            </div>
          </div>
        )}

        {!loading && !error && data?.deployments.length === 0 && (
          <div className="flex flex-col items-center justify-center py-16 text-center text-sm text-muted-foreground">
            <Rocket className="h-8 w-8 text-muted-foreground/40 mb-3" />
            <p className="font-medium text-foreground">{t('Belum ada deployment')}</p>
            <p className="max-w-xs text-xs text-muted-foreground/80 mt-1">
              {t('Klik tombol Deploy di atas untuk memicu build dan rilis pertama Anda.')}
            </p>
          </div>
        )}

        {!loading && data && data.deployments.length > 0 && (
          <div className="rounded-xl border border-border/40 bg-card/60 shadow-2xs divide-y divide-border/20 overflow-hidden">
            {data.deployments.map((dep) => (
              <div
                key={dep.id}
                data-deployment-row="true"
                onClick={() => setSelectedDepId(dep.id)}
                className="group flex min-h-[56px] items-center justify-between gap-3 px-4 py-3 transition-colors hover:bg-muted/40 active:bg-muted/60 cursor-pointer select-none"
              >
                {/* Left: Status Icon, Short SHA, Commit Message */}
                <div className="flex min-w-0 flex-1 items-center gap-3">
                  {renderStatusIcon(dep.status)}

                  <span className="font-mono text-xs font-medium text-muted-foreground group-hover:text-foreground transition-colors shrink-0">
                    {dep.short_sha}
                  </span>

                  <div className="flex min-w-0 items-center gap-2">
                    <GitCommit className="h-3.5 w-3.5 text-muted-foreground/60 shrink-0" />
                    <span className="truncate text-sm font-normal text-foreground group-hover:text-primary transition-colors">
                      {dep.commit_message}
                    </span>
                  </div>
                </div>

                {/* Right: Deployed from branch relative time by author, duration */}
                <div className="flex shrink-0 items-center gap-2.5 text-xs text-muted-foreground">
                  <div className="hidden lg:flex items-center gap-1.5 text-[11px]">
                    <span className="text-muted-foreground/70">
                      {dep.status === 'canceled'
                        ? t('Canceled deploy from')
                        : dep.status === 'queued'
                        ? t('Queued deploy from')
                        : dep.status === 'building'
                        ? t('Deploying from')
                        : t('Deployed from')}
                    </span>
                    <span className="inline-flex items-center rounded bg-muted/80 px-1.5 py-0.5 font-mono text-[10px] text-foreground border border-border/40">
                      {dep.branch}
                    </span>
                    {dep.trigger_author && (
                      <span className="text-muted-foreground/80">by {dep.trigger_author}</span>
                    )}
                    {dep.trigger_source === 'webhook' && (
                      <span className="inline-flex items-center gap-1 rounded bg-muted/60 px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground border border-border/30">
                        <Webhook className="h-2.5 w-2.5" />
                        webhook
                      </span>
                    )}
                  </div>

                  {/* Deploy Relative Time - ALWAYS visible across all screen sizes */}
                  <span className="text-xs text-muted-foreground whitespace-nowrap font-medium">
                    {formatDeploymentTime(dep)}
                  </span>

                  {/* Duration Pill - secondary detail */}
                  {dep.duration && (
                    <span
                      className="hidden sm:inline-flex items-center rounded-md bg-muted/50 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground/80 border border-border/30"
                      title={t('Durasi eksekusi pipeline')}
                    >
                      {dep.duration}
                    </span>
                  )}

                  {dep.status === 'failed' && (
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation()
                        void handleEscalateToAgent(dep)
                      }}
                      disabled={escalatingId === dep.id}
                      className="hidden sm:inline-flex items-center gap-1 rounded-md border border-border/60 bg-background/80 hover:bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                      title={t('Minta Agent bantu perbaiki kegagalan rilis ini')}
                    >
                      {escalatingId === dep.id && (
                        <Loader2 className="h-3 w-3 animate-spin" />
                      )}
                      <span>{t('Perbaiki')}</span>
                    </button>
                  )}

                  <ChevronRight className="h-4 w-4 text-muted-foreground/40 group-hover:text-muted-foreground transition-colors shrink-0" />
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* SlideDrawer: Deployment Timeline & Log Viewer */}
      <SlideDrawer
        isOpen={!!selectedDeployment}
        onClose={() => setSelectedDepId(null)}
        title={
          <div className="flex items-center gap-2.5">
            {selectedDeployment && renderStatusIcon(selectedDeployment.status, 'sm')}
            <span className="font-semibold text-foreground">
              {t('Deployment')} #{selectedDeployment?.short_sha}
            </span>
          </div>
        }
        description={selectedDeployment?.commit_message}
        size="lg"
      >
        {selectedDeployment && (
          <div className="space-y-6 pb-6 text-sm">
            {/* Escalation Banner if Deployment Failed */}
            {selectedDeployment.status === 'failed' && (
              <div className="rounded-xl border border-destructive/25 bg-destructive/5 p-4 space-y-3">
                <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                  <div className="space-y-1 min-w-0">
                    <div className="flex items-center gap-1.5 text-xs font-semibold text-destructive">
                      <AlertCircle className="h-4 w-4 shrink-0" />
                      <span>{t('Deployment Mengalami Kendala')}</span>
                    </div>
                    <p className="text-xs text-muted-foreground leading-relaxed">
                      {t('Pipeline terhenti pada tahap build/rilis. Eskalasikan log error ini ke Agent untuk dianalisis dan diperbaiki langsung di Inbox.')}
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => void handleEscalateToAgent(selectedDeployment)}
                    disabled={escalatingId === selectedDeployment.id}
                    id="escalate-failed-deploy-btn"
                    className="inline-flex shrink-0 items-center justify-center gap-2 rounded-lg bg-foreground hover:bg-foreground/90 active:bg-foreground/80 px-3.5 py-2 text-xs font-medium text-background shadow-2xs disabled:opacity-50 transition-colors cursor-pointer"
                  >
                    {escalatingId === selectedDeployment.id && (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    )}
                    <span>{t('Perbaiki Bersama Agent')}</span>
                  </button>
                </div>
              </div>
            )}

            {/* Meta Card */}
            <div className="rounded-xl border border-border/40 bg-muted/30 p-4 space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <span className="inline-flex items-center gap-1.5 rounded-md bg-background px-2 py-1 font-mono text-xs border border-border/40 font-medium">
                    <GitBranch className="h-3.5 w-3.5 text-muted-foreground" />
                    {selectedDeployment.branch}
                  </span>

                  <button
                    type="button"
                    onClick={() => handleCopySha(selectedDeployment.commit_sha)}
                    className="inline-flex items-center gap-1.5 rounded-md bg-background px-2 py-1 font-mono text-xs border border-border/40 hover:bg-muted transition-colors cursor-pointer"
                    title={t('Salin SHA commit')}
                  >
                    {copiedSha ? (
                      <Check className="h-3 w-3 text-emerald-600" />
                    ) : (
                      <Copy className="h-3 w-3 text-muted-foreground" />
                    )}
                    <span>{selectedDeployment.short_sha}</span>
                  </button>
                </div>

                {(selectedDeployment.status === 'building' || selectedDeployment.status === 'queued') && (
                  <button
                    type="button"
                    disabled={cancelingId === selectedDeployment.id}
                    onClick={() => handleCancelDeploy(selectedDeployment.id)}
                    className="inline-flex items-center gap-1.5 rounded-lg border border-destructive/30 bg-destructive/10 px-2.5 py-1 text-xs font-medium text-destructive hover:bg-destructive/20 transition-colors cursor-pointer"
                  >
                    <Ban className="h-3.5 w-3.5" />
                    <span>{selectedDeployment.status === 'queued' ? t('Cancel queue') : t('Cancel deploy')}</span>
                  </button>
                )}
              </div>

              <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 text-xs text-muted-foreground pt-1 border-t border-border/20">
                <div>
                  <span className="block text-[11px] text-muted-foreground/80">{t('Status')}</span>
                  <span className="font-medium text-foreground capitalize mt-0.5 block">
                    {selectedDeployment.status}
                  </span>
                </div>
                <div>
                  <span className="block text-[11px] text-muted-foreground/80">{t('Duration')}</span>
                  <span className="font-medium text-foreground mt-0.5 block">
                    {selectedDeployment.duration || 'In progress'}
                  </span>
                </div>
                <div>
                  <span className="block text-[11px] text-muted-foreground/80">{t('Triggered by')}</span>
                  <span className="font-medium text-foreground mt-0.5 block truncate">
                    {selectedDeployment.trigger_author || selectedDeployment.trigger_source}
                  </span>
                </div>
              </div>
            </div>

            {/* Standard Unified AccordionTimeline */}
            <div className="space-y-3">
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-1">
                {t('Deployment Timeline')}
              </h3>

              {timelineItems.length > 0 ? (
                <AccordionTimeline
                  label={t('Deployment Timeline')}
                  items={timelineItems}
                  followLatest
                  showTriggerCaret
                />
              ) : (
                <p className="text-xs text-muted-foreground py-2 px-1">
                  {t('Tidak ada detail langkah yang tercatat.')}
                </p>
              )}
            </div>
          </div>
        )}
      </SlideDrawer>

      {/* SlideDrawer: Webhook & Deployment Settings */}
      <SlideDrawer
        isOpen={isWebhookOpen}
        onClose={() => setIsWebhookOpen(false)}
        title={
          <div className="flex items-center gap-2">
            <Sliders className="h-5 w-5 text-foreground" />
            <span className="font-semibold text-foreground">
              {t('Pengaturan Deployment Proyek')}
            </span>
          </div>
        }
        description={t('Konfigurasi auto-deploy, branch rilis, script deployment per-proyek, dan webhook.')}
        size="md"
      >
        <div className="space-y-6 pb-6 text-sm">
          {/* Explanation Banner */}
          <div className="rounded-xl border border-border/50 bg-muted/30 p-4 space-y-2">
            <div className="flex items-center gap-2 text-foreground font-medium text-xs uppercase tracking-wider">
              <ShieldCheck className="h-4 w-4 text-muted-foreground" />
              <span>{t('Deployment Lane Terisolasi & Antre (FIFO)')}</span>
            </div>
            <p className="text-xs text-muted-foreground leading-relaxed">
              {t('Setiap proyek memiliki antrean build independen. Saat ada push atau rilis baru, pipeline akan mengeksekusi script deployment proyek ini di latar belakang tanpa tabrakan.')}
            </p>
          </div>

          {/* Configuration Form: Auto Deploy, Target Branch, Deployment Script */}
          <div className="space-y-4">
            {/* Auto Deploy Toggle */}
            <div className="rounded-xl border border-border/40 bg-muted/20 p-4 space-y-2">
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-0.5">
                  <label htmlFor="auto-deploy-toggle" className="text-xs font-semibold text-foreground cursor-pointer">
                    {t('Otomatis Deploy saat Push (Auto Deploy)')}
                  </label>
                  <p className="text-[11px] text-muted-foreground leading-relaxed">
                    {t('Saat aktif, push ke branch rilis via webhook akan otomatis memicu antrean build dan deployment tanpa perlu perintah manual.')}
                  </p>
                </div>
                <input
                  id="auto-deploy-toggle"
                  type="checkbox"
                  checked={autoDeploy}
                  onChange={(e) => setAutoDeploy(e.target.checked)}
                  className="mt-0.5 h-4 w-4 rounded border-border text-foreground focus:ring-foreground/20 cursor-pointer"
                />
              </div>
            </div>

            {/* Target Branch */}
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-foreground flex items-center justify-between">
                <span>{t('Branch Rilis (Deployment Branch)')}</span>
                <span className="text-[11px] text-muted-foreground font-mono">Git Ref</span>
              </label>
              <input
                type="text"
                value={deployBranch}
                onChange={(e) => setDeployBranch(e.target.value)}
                placeholder="dev"
                className="w-full rounded-lg border border-border/50 bg-background px-3 py-2 font-mono text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-foreground/20"
              />
              <p className="text-[11px] text-muted-foreground">
                {t('Webhook hanya akan memproses push yang ditujukan ke branch ini.')}
              </p>
            </div>

            {/* Deployment Script */}
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-foreground flex items-center justify-between">
                <span>{t('Script Deployment (Build & Release Hook)')}</span>
                <span className="text-[11px] text-muted-foreground font-mono">bash (opsional)</span>
              </label>
              <div className="flex items-center gap-1.5 flex-wrap pb-0.5">
                <span className="text-[11px] text-muted-foreground mr-1">{t('Preset cepat:')}</span>
                <button
                  type="button"
                  onClick={() => setDeployScript('npm ci && npm run build')}
                  className="rounded-md border border-border/40 bg-muted/40 hover:bg-muted px-2 py-0.5 text-[11px] font-mono text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                >
                  Node.js
                </button>
                <button
                  type="button"
                  onClick={() => setDeployScript('go build -v -o app .')}
                  className="rounded-md border border-border/40 bg-muted/40 hover:bg-muted px-2 py-0.5 text-[11px] font-mono text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                >
                  Golang
                </button>
                <button
                  type="button"
                  onClick={() => setDeployScript('composer install --no-dev && php artisan optimize')}
                  className="rounded-md border border-border/40 bg-muted/40 hover:bg-muted px-2 py-0.5 text-[11px] font-mono text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                >
                  Laravel
                </button>
                <button
                  type="button"
                  onClick={() => setDeployScript('./scripts/deploy.sh')}
                  className="rounded-md border border-border/40 bg-muted/40 hover:bg-muted px-2 py-0.5 text-[11px] font-mono text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                >
                  scripts/deploy.sh
                </button>
                {deployScript && (
                  <button
                    type="button"
                    onClick={() => setDeployScript('')}
                    className="rounded-md border border-transparent px-1.5 py-0.5 text-[11px] text-muted-foreground hover:text-destructive transition-colors cursor-pointer"
                  >
                    {t('Kosongkan (Otomatis)')}
                  </button>
                )}
              </div>
              <textarea
                rows={4}
                value={deployScript}
                onChange={(e) => setDeployScript(e.target.value)}
                placeholder={"# Opsional: Kosongkan untuk deteksi otomatis (Node.js/Go/PHP)\n# Atau tentukan perintah kustom, contoh:\n# npm ci && npm run build"}
                className="w-full rounded-lg border border-border/50 bg-neutral-950 p-3 font-mono text-xs leading-relaxed text-neutral-200 focus:outline-none focus:ring-1 focus:ring-foreground/20 shadow-inner"
              />
              <p className="text-[11px] text-muted-foreground">
                {t('Jika dikosongkan, Cuan akan otomatis mendeteksi stack (Node.js, Go, PHP) atau mencari file scripts/deploy.sh di repositori Anda.')}
              </p>
            </div>

            {/* Save Button */}
            <div className="flex items-center justify-end gap-2.5 pt-1">
              {savedSuccess && (
                <span className="text-xs text-emerald-600 dark:text-emerald-400 flex items-center gap-1 font-medium">
                  <Check className="h-3.5 w-3.5" />
                  {t('Pengaturan tersimpan!')}
                </span>
              )}
              <button
                type="button"
                disabled={savingConfig}
                onClick={handleSaveConfig}
                id="save-deployment-config-btn"
                className="inline-flex min-h-[34px] items-center gap-1.5 rounded-lg bg-foreground px-3.5 py-1.5 text-xs font-medium text-background shadow-2xs hover:bg-foreground/90 disabled:opacity-50 transition-colors cursor-pointer"
              >
                {savingConfig ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Check className="h-3.5 w-3.5" />
                )}
                <span>{t('Simpan Pengaturan')}</span>
              </button>
            </div>
          </div>

          {/* Webhook Configuration Fields */}
          <div className="space-y-4 pt-4 border-t border-border/30">
            <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
              <Webhook className="h-3.5 w-3.5 text-muted-foreground" />
              <span>{t('Integrasi Git Webhook')}</span>
            </h4>

            <div className="space-y-1.5">
              <label className="text-xs font-medium text-foreground flex items-center justify-between">
                <span>{t('Payload URL')}</span>
                <span className="text-[11px] text-muted-foreground font-mono">POST</span>
              </label>
              <div className="flex items-center gap-2">
                <input
                  type="text"
                  readOnly
                  value={data?.webhook_url || ''}
                  className="flex-1 rounded-lg border border-border/50 bg-muted/40 px-3 py-2 font-mono text-xs text-foreground focus:outline-none select-all"
                />
                <button
                  type="button"
                  onClick={handleCopyUrl}
                  className="inline-flex min-h-[34px] items-center gap-1.5 rounded-lg border border-border/60 bg-background px-3 py-1.5 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors shrink-0 cursor-pointer"
                >
                  {copiedUrl ? (
                    <Check className="h-3.5 w-3.5 text-emerald-600" />
                  ) : (
                    <Copy className="h-3.5 w-3.5 text-muted-foreground" />
                  )}
                  <span>{copiedUrl ? t('Tersalin') : t('Salin')}</span>
                </button>
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-medium text-foreground flex items-center justify-between">
                <span>{t('Secret Token')}</span>
                <span className="text-[11px] text-muted-foreground">{t('HMAC-SHA256')}</span>
              </label>
              <div className="flex items-center gap-2">
                <input
                  type="text"
                  readOnly
                  value={data?.webhook_secret || ''}
                  className="flex-1 rounded-lg border border-border/50 bg-muted/40 px-3 py-2 font-mono text-xs text-foreground focus:outline-none select-all"
                />
                <button
                  type="button"
                  onClick={handleCopySecret}
                  className="inline-flex min-h-[34px] items-center gap-1.5 rounded-lg border border-border/60 bg-background px-3 py-1.5 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors shrink-0 cursor-pointer"
                >
                  {copiedSecret ? (
                    <Check className="h-3.5 w-3.5 text-emerald-600" />
                  ) : (
                    <Copy className="h-3.5 w-3.5 text-muted-foreground" />
                  )}
                  <span>{copiedSecret ? t('Tersalin') : t('Salin')}</span>
                </button>
              </div>
            </div>
          </div>

          {/* Setup Guide */}
          <div className="space-y-3 pt-2 border-t border-border/30">
            <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              {t('Langkah Pengaturan di GitHub')}
            </h4>
            <ol className="space-y-2.5 text-xs text-muted-foreground list-decimal list-inside leading-relaxed">
              <li>
                {t('Buka repository GitHub Anda > ')}
                <span className="font-medium text-foreground">Settings</span>
                {t(' > ')}
                <span className="font-medium text-foreground">Webhooks</span>
                {t(' > ')}
                <span className="font-medium text-foreground">Add webhook</span>.
              </li>
              <li>
                {t('Masukkan ')}
                <span className="font-medium text-foreground">Payload URL</span>
                {t(' dengan URL di atas.')}
              </li>
              <li>
                {t('Pilih ')}
                <span className="font-medium text-foreground">Content type</span>
                {t(' sebagai ')}
                <code className="rounded bg-muted/60 px-1 py-0.5 font-mono text-[11px] text-foreground border border-border/30">
                  application/json
                </code>
                .
              </li>
              <li>
                {t('Masukkan ')}
                <span className="font-medium text-foreground">Secret</span>
                {t(' dengan token rahasia di atas.')}
              </li>
              <li>
                {t('Pada ')}
                <span className="font-medium text-foreground">Which events would you like to trigger this webhook?</span>
                {t(', pilih ')}
                <span className="font-medium text-foreground">Just the push event</span>.
              </li>
              <li>
                {t('Pastikan ')}
                <span className="font-medium text-foreground">Active</span>
                {t(' tercentang, lalu klik ')}
                <span className="font-medium text-foreground">Add webhook</span>.
              </li>
            </ol>
          </div>
        </div>
      </SlideDrawer>
    </div>
  )
}
