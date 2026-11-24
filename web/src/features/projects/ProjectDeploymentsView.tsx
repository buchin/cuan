import { useEffect, useState, useMemo } from 'react'
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
  ChevronDown,
  Copy,
  Check,
  RefreshCw,
  AlertCircle,
  Terminal,
  Ban
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { SlideDrawer } from '../../components/layout/SlideDrawer'
import {
  projectsApi,
  ProjectDeploymentsResponse
} from '../../api/projects'
import type { Project } from '../../api/types'

interface ProjectDeploymentsViewProps {
  project: Project
  onRetry?: () => void
}

export function ProjectDeploymentsView({ project }: ProjectDeploymentsViewProps) {
  const { t } = useLanguage()
  const [data, setData] = useState<ProjectDeploymentsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [triggering, setTriggering] = useState(false)
  const [cancelingId, setCancelingId] = useState<string | null>(null)
  const [selectedDepId, setSelectedDepId] = useState<string | null>(null)
  const [copiedSha, setCopiedSha] = useState(false)
  const [expandedLogs, setExpandedLogs] = useState<Record<string, boolean>>({})

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
    } catch (e: any) {
      setError(e?.message || t('Gagal memuat riwayat deployment proyek.'))
    } finally {
      if (!silent) setLoading(false)
    }
  }

  useEffect(() => {
    void loadDeployments()
  }, [project.id])

  // Auto-refresh when any deployment is currently building
  const isAnyBuilding = useMemo(() => {
    return data?.deployments.some((d) => d.status === 'building')
  }, [data])

  useEffect(() => {
    if (!isAnyBuilding) return
    const timer = setInterval(() => {
      void loadDeployments(true)
    }, 2500)
    return () => clearInterval(timer)
  }, [isAnyBuilding, project.id])

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

  const toggleLog = (stepId: string) => {
    setExpandedLogs((prev) => ({
      ...prev,
      [stepId]: !prev[stepId]
    }))
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
      case 'canceled':
      default:
        return (
          <div className="flex items-center justify-center rounded-full bg-muted p-0.5 text-muted-foreground">
            <Slash className={dim} />
          </div>
        )
    }
  }

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

        {/* Action Buttons: Deploy & Visit */}
        <div className="flex items-center gap-2.5">
          <button
            type="button"
            disabled={triggering}
            onClick={handleTriggerDeploy}
            id="deployments-trigger-btn"
            className="inline-flex min-h-9 items-center justify-center gap-2 rounded-lg bg-sky-600 px-3.5 py-1.5 text-xs font-medium text-white shadow-xs hover:bg-sky-500 active:bg-sky-700 disabled:opacity-50 transition-colors cursor-pointer"
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
                <div className="flex shrink-0 items-center gap-3 text-xs text-muted-foreground">
                  <div className="hidden md:flex items-center gap-1.5">
                    <span>
                      {dep.status === 'canceled'
                        ? t('Canceled deploy from')
                        : dep.status === 'building'
                        ? t('Deploying from')
                        : t('Deployed from')}
                    </span>
                    <span className="inline-flex items-center rounded bg-muted/80 px-1.5 py-0.5 font-mono text-[11px] text-foreground border border-border/40">
                      {dep.branch}
                    </span>
                    <span>{dep.relative_time}</span>
                    {dep.trigger_author && (
                      <span>by {dep.trigger_author}</span>
                    )}
                  </div>

                  {dep.duration && (
                    <span className="inline-flex items-center rounded-full bg-muted/60 px-2 py-0.5 text-[11px] font-medium text-muted-foreground">
                      {dep.duration}
                    </span>
                  )}

                  <ChevronRight className="h-4 w-4 text-muted-foreground/40 group-hover:text-muted-foreground transition-colors" />
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

                {selectedDeployment.status === 'building' && (
                  <button
                    type="button"
                    disabled={cancelingId === selectedDeployment.id}
                    onClick={() => handleCancelDeploy(selectedDeployment.id)}
                    className="inline-flex items-center gap-1.5 rounded-lg border border-destructive/30 bg-destructive/10 px-2.5 py-1 text-xs font-medium text-destructive hover:bg-destructive/20 transition-colors cursor-pointer"
                  >
                    <Ban className="h-3.5 w-3.5" />
                    <span>{t('Cancel deploy')}</span>
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

            {/* Vertical Timeline with Continuous Vertical Connecting Line */}
            <div className="space-y-3">
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-1">
                {t('Deployment Timeline')}
              </h3>

              {/* The Timeline Container with Continuous Vertical Line */}
              <div className="relative pl-6">
                {/* Continuous Vertical Line Connector */}
                <div
                  data-timeline-connector="true"
                  className="absolute left-[15px] top-3 bottom-3 w-0.5 bg-border/60 -translate-x-1/2"
                  aria-hidden="true"
                />

                {selectedDeployment.steps && selectedDeployment.steps.length > 0 ? (
                  selectedDeployment.steps.map((step) => {
                    const isExpanded = !!expandedLogs[step.id] || step.status === 'failed' || step.status === 'building'

                    let stepIcon = (
                      <div className="h-2 w-2 rounded-full bg-muted-foreground/40" />
                    )
                    if (step.status === 'success') {
                      stepIcon = <Check className="h-3 w-3 text-emerald-600 dark:text-emerald-400 stroke-[3]" />
                    } else if (step.status === 'building') {
                      stepIcon = <Loader2 className="h-3 w-3 text-sky-600 dark:text-sky-400 animate-spin" />
                    } else if (step.status === 'failed') {
                      stepIcon = <XCircle className="h-3.5 w-3.5 text-rose-600 dark:text-rose-400" />
                    }

                    return (
                      <div key={step.id} className="relative flex items-start gap-3.5 pb-6 last:pb-0">
                        {/* Step Circle Node on the vertical line */}
                        <div
                          className={`relative z-10 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-card border-2 shadow-2xs ${
                            step.status === 'success'
                              ? 'border-emerald-500/60 bg-emerald-500/10'
                              : step.status === 'building'
                              ? 'border-sky-500/60 bg-sky-500/10'
                              : step.status === 'failed'
                              ? 'border-rose-500/60 bg-rose-500/10'
                              : 'border-border/60 bg-muted/60'
                          }`}
                        >
                          {stepIcon}
                        </div>

                        {/* Step Details & Logs */}
                        <div className="min-w-0 flex-1 pt-0.5">
                          <div className="flex flex-wrap items-center justify-between gap-2">
                            <span className="text-sm font-medium text-foreground">
                              {step.name}
                            </span>
                            {step.duration && (
                              <span className="font-mono text-[11px] text-muted-foreground bg-muted/60 px-1.5 py-0.5 rounded">
                                {step.duration}
                              </span>
                            )}
                          </div>

                          {step.summary && (
                            <p className="text-xs text-muted-foreground mt-0.5">
                              {step.summary}
                            </p>
                          )}

                          {/* Expandable Step Logs */}
                          {step.log && (
                            <div className="mt-2">
                              <button
                                type="button"
                                onClick={() => toggleLog(step.id)}
                                className="inline-flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                              >
                                <Terminal className="h-3 w-3" />
                                <span>{isExpanded ? t('Sembunyikan log') : t('Lihat log stdout/stderr')}</span>
                                {isExpanded ? (
                                  <ChevronDown className="h-3 w-3" />
                                ) : (
                                  <ChevronRight className="h-3 w-3" />
                                )}
                              </button>

                              {isExpanded && (
                                <pre className="mt-2 rounded-lg bg-neutral-950 p-3 font-mono text-[11px] leading-relaxed text-neutral-300 overflow-x-auto whitespace-pre-wrap select-text border border-neutral-800 shadow-inner">
                                  {step.log}
                                </pre>
                              )}
                            </div>
                          )}
                        </div>
                      </div>
                    )
                  })
                ) : (
                  <p className="text-xs text-muted-foreground py-2">
                    {t('Tidak ada detail langkah yang tercatat.')}
                  </p>
                )}
              </div>
            </div>
          </div>
        )}
      </SlideDrawer>
    </div>
  )
}
