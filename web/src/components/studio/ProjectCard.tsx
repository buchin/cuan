import { useState, useEffect } from 'react'
import { Card, CardHeader } from '../ui/card'
import { Button } from '../ui/button'
import { Badge } from '../ui/badge'
import {
  Globe,
  ExternalLink,
  Play,
  Square,
  RotateCcw,
  Terminal,
  FileCode,
  Cpu,
  Layers,
  FolderGit2,
  UploadCloud,
  Check,
  Copy,
  Server,
  RefreshCw,
  CheckCircle2,
  Clock,
  MessageSquare,
  Kanban
} from 'lucide-react'
import { DeploymentItem, ServerItem, CustomDomainItem } from './types'

export interface ProjectHealthBadgeInfo {
  status: 'healthy' | 'degraded' | 'unhealthy' | 'down' | string
  response_time_ms?: number
  uptime_percent?: number
}

interface ProjectCardProps {
  deployment: DeploymentItem
  baseDomain?: string
  servers?: ServerItem[]
  customDomains?: CustomDomainItem[]
  healthInfo?: ProjectHealthBadgeInfo
  onOpenDrawer: (dep: DeploymentItem) => void
  onOpenCustomDomains?: (dep: DeploymentItem) => void
  onOpenDiscussions?: (dep: DeploymentItem) => void
  onOpenTasks?: (dep: DeploymentItem) => void
  onQuickStart: (dep: DeploymentItem) => void
  onQuickStop: (dep: DeploymentItem) => void
  onQuickRestart: (dep: DeploymentItem) => void
  isActionLoading?: boolean
  copiedId?: string | null
  onCopyUrl?: (id: string, url: string) => void
}

export function ProjectCard({
  deployment: dep,
  baseDomain = 'cuan.dojo.cc',
  servers = [],
  customDomains = [],
  healthInfo,
  onOpenDrawer,
  onOpenCustomDomains,
  onOpenDiscussions,
  onOpenTasks,
  onQuickStart,
  onQuickStop,
  onQuickRestart,
  isActionLoading = false,
  copiedId,
  onCopyUrl
}: ProjectCardProps) {
  // Internal health polling if healthInfo is not provided
  const [internalHealth, setInternalHealth] = useState<ProjectHealthBadgeInfo | null>(null)

  useEffect(() => {
    if (healthInfo) return
    if (dep.status !== 'running') {
      setInternalHealth(null)
      return
    }

    let isMounted = true
    const checkHealth = async () => {
      try {
        const res = await fetch(`/api/deployments/${dep.id}/health`)
        if (res.ok) {
          const data = await res.json()
          if (isMounted) {
            setInternalHealth({
              status: data.status,
              response_time_ms: data.response_time_ms,
              uptime_percent: data.uptime_percent,
            })
          }
        }
      } catch {
        // ignore
      }
    }

    checkHealth()
    const interval = setInterval(checkHealth, 15000)
    return () => {
      isMounted = false
      clearInterval(interval)
    }
  }, [dep.id, dep.status, healthInfo])

  const effectiveHealth = healthInfo || internalHealth
  const healthStatus = dep.status === 'stopped'
    ? null
    : (effectiveHealth?.status || (dep.status === 'running' ? 'healthy' : 'down'))
  const healthLatency = effectiveHealth?.response_time_ms

  const liveURL = dep.domain
    ? `http://${dep.domain}`
    : `http://${dep.slug}.${baseDomain}`

  const getRuntimeIcon = (runtime: string) => {
    switch (runtime) {
      case 'php':
        return <FileCode className="h-4 w-4 text-blue-500" />
      case 'nodejs':
        return <Cpu className="h-4 w-4 text-emerald-500" />
      case 'static':
        return <Globe className="h-4 w-4 text-amber-500" />
      default:
        return <Layers className="h-4 w-4 text-slate-400" />
    }
  }

  // Filter custom domains specifically for this deployment
  const depCustomDomains = customDomains.filter((cd) => cd.deployment_id === dep.id)
  const hasCustomDomains = depCustomDomains.length > 0
  const verifiedDomainsCount = depCustomDomains.filter((cd) => cd.dns_status === 'verified').length

  return (
    <Card className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs hover:shadow-md transition-all flex flex-col justify-between overflow-hidden group">
      <CardHeader className="p-5 pb-3 space-y-2.5">
        <div className="flex items-start justify-between gap-2">
          <div className="space-y-0.5 min-w-0">
            <div className="flex items-center gap-2">
              <h3
                onClick={() => onOpenDrawer(dep)}
                className="font-bold text-slate-900 dark:text-white text-base hover:text-emerald-600 dark:hover:text-emerald-400 cursor-pointer truncate"
              >
                {dep.project_name}
              </h3>
            </div>
            <p className="text-xs font-mono text-slate-400">/{dep.slug}</p>
          </div>

          {/* Status Badges: Process Status + Observability Health Status (Healthy, Degraded, Down) */}
          <div className="flex items-center gap-1.5 shrink-0 flex-wrap justify-end">
            {/* Process Status */}
            {dep.status === 'running' ? (
              <Badge variant="success" className="gap-1 py-0.5 px-2 text-[11px] shrink-0">
                <span className="h-1.5 w-1.5 rounded-full bg-white animate-pulse" />
                Running
              </Badge>
            ) : dep.status === 'stopped' ? (
              <Badge variant="secondary" className="gap-1 py-0.5 px-2 text-[11px] shrink-0">
                <span className="h-1.5 w-1.5 rounded-full bg-slate-400" />
                Stopped
              </Badge>
            ) : (
              <Badge variant="outline" className="text-[11px] shrink-0">
                {dep.status}
              </Badge>
            )}

            {/* Health Status Badge (S06.4 DoD requirement) */}
            {healthStatus === 'healthy' ? (
              <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 gap-1 py-0.5 px-2 text-[10px] font-semibold shrink-0">
                <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
                Healthy{healthLatency ? ` (${healthLatency}ms)` : ''}
              </Badge>
            ) : healthStatus === 'degraded' ? (
              <Badge className="bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30 gap-1 py-0.5 px-2 text-[10px] font-semibold shrink-0">
                <span className="h-1.5 w-1.5 rounded-full bg-amber-500" />
                Degraded{healthLatency ? ` (${healthLatency}ms)` : ''}
              </Badge>
            ) : healthStatus === 'down' || healthStatus === 'unhealthy' ? (
              <Badge variant="destructive" className="gap-1 py-0.5 px-2 text-[10px] font-semibold shrink-0">
                <span className="h-1.5 w-1.5 rounded-full bg-white" />
                Down
              </Badge>
            ) : null}
          </div>
        </div>

        {/* Live Subdomain Link Row */}
        <div className="flex items-center justify-between p-2.5 bg-slate-50 dark:bg-slate-800/60 rounded-xl border border-slate-100 dark:border-slate-800 text-xs">
          <a
            href={liveURL}
            target="_blank"
            rel="noreferrer"
            className="font-mono text-[11px] text-emerald-600 dark:text-emerald-400 hover:underline flex items-center gap-1 truncate"
          >
            {liveURL.replace('http://', '')}
            <ExternalLink className="h-3 w-3 shrink-0" />
          </a>
          {onCopyUrl && (
            <button
              onClick={() => onCopyUrl(dep.id, liveURL)}
              className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 p-1"
              title="Salin Subdomain URL"
            >
              {copiedId === dep.id ? (
                <Check className="h-3.5 w-3.5 text-emerald-500" />
              ) : (
                <Copy className="h-3.5 w-3.5" />
              )}
            </button>
          )}
        </div>

        {/* Custom Domains Active Badge Row (S05.4) */}
        {hasCustomDomains ? (
          <div className="flex items-center justify-between p-2 bg-emerald-50/50 dark:bg-emerald-950/20 rounded-lg border border-emerald-200/60 dark:border-emerald-800/50 text-[11px]">
            <div className="flex items-center gap-1.5 truncate">
              <Globe className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" />
              {depCustomDomains.length === 1 ? (
                <span className="font-mono font-semibold text-emerald-700 dark:text-emerald-300 truncate">
                  {depCustomDomains[0].domain}
                </span>
              ) : (
                <span className="font-semibold text-emerald-700 dark:text-emerald-300 truncate">
                  {depCustomDomains.length} Custom Domains ({verifiedDomainsCount} Verified)
                </span>
              )}
            </div>

            <button
              onClick={() => onOpenCustomDomains?.(dep)}
              className="text-[10px] font-semibold text-emerald-600 dark:text-emerald-400 hover:underline ml-2 shrink-0 flex items-center gap-0.5"
            >
              {verifiedDomainsCount === depCustomDomains.length ? (
                <span className="flex items-center gap-1">
                  <CheckCircle2 className="h-3 w-3 text-emerald-500" /> TLS Active
                </span>
              ) : (
                <span className="flex items-center gap-1 text-amber-600 dark:text-amber-400">
                  <Clock className="h-3 w-3" /> Setup DNS
                </span>
              )}
            </button>
          </div>
        ) : null}

        {/* Metadata tags: Stack, Target Server, Port, Source */}
        <div className="flex items-center gap-2 flex-wrap pt-1 text-[11px] text-slate-500 dark:text-slate-400">
          <span className="inline-flex items-center gap-1 font-semibold capitalize bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-md">
            {getRuntimeIcon(dep.runtime)}
            {dep.runtime}
          </span>

          {/* Target Server Badge (Local Sandbox vs Remote VPS) */}
          {dep.server_id ? (
            <span className="inline-flex items-center gap-1 font-semibold text-indigo-600 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-950/60 border border-indigo-200 dark:border-indigo-800/60 px-2 py-0.5 rounded-md">
              <Server className="h-3 w-3" />
              <span>
                {servers.find((s) => s.id === dep.server_id)?.name || 'Remote VPS'}
              </span>
            </span>
          ) : (
            <span className="inline-flex items-center gap-1 font-semibold text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-950/60 border border-emerald-200 dark:border-emerald-800/60 px-2 py-0.5 rounded-md">
              <Cpu className="h-3 w-3" /> Local Sandbox
            </span>
          )}

          {dep.internal_port > 0 && (
            <span className="font-mono bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-md">
              :{dep.internal_port}
            </span>
          )}
          <span className="inline-flex items-center gap-1 bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-md">
            {dep.source_type === 'git' ? (
              <>
                <FolderGit2 className="h-3 w-3 text-blue-500" /> Git
              </>
            ) : (
              <>
                <UploadCloud className="h-3 w-3 text-emerald-500" /> ZIP
              </>
            )}
          </span>
        </div>
      </CardHeader>

      {/* Card Action Controls Footer */}
      <div className="p-4 pt-3 border-t border-slate-100 dark:border-slate-800/80 bg-slate-50/50 dark:bg-slate-900/50 flex items-center justify-between gap-2">
        <div className="flex items-center gap-1.5">
          {dep.status === 'running' ? (
            <Button
              size="sm"
              variant="outline"
              onClick={() => onQuickStop(dep)}
              disabled={isActionLoading}
              className="h-8 text-xs font-semibold border-amber-300 dark:border-amber-800 text-amber-700 dark:text-amber-300 hover:bg-amber-50 dark:hover:bg-amber-950 flex items-center gap-1"
            >
              {isActionLoading ? (
                <RefreshCw className="h-3 w-3 animate-spin" />
              ) : (
                <Square className="h-3 w-3 fill-current" />
              )}
              Stop
            </Button>
          ) : (
            <Button
              size="sm"
              onClick={() => onQuickStart(dep)}
              disabled={isActionLoading}
              className="h-8 text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white flex items-center gap-1"
            >
              {isActionLoading ? (
                <RefreshCw className="h-3 w-3 animate-spin" />
              ) : (
                <Play className="h-3 w-3 fill-current" />
              )}
              Start
            </Button>
          )}

          <Button
            size="sm"
            variant="outline"
            onClick={() => onQuickRestart(dep)}
            disabled={isActionLoading}
            className="h-8 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 px-2"
            title="Restart proses"
          >
            <RotateCcw className="h-3 w-3" />
          </Button>

          {/* Custom Domains Shortcut Button */}
          {onOpenCustomDomains && (
            <Button
              size="sm"
              variant="outline"
              onClick={() => onOpenCustomDomains(dep)}
              className="h-8 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 px-2 flex items-center gap-1"
              title="Kelola Custom Domains"
            >
              <Globe className="h-3.5 w-3.5 text-emerald-500" />
              {hasCustomDomains && (
                <span className="h-4 px-1 rounded-full bg-emerald-100 dark:bg-emerald-950 text-emerald-700 dark:text-emerald-300 text-[10px] font-mono">
                  {depCustomDomains.length}
                </span>
              )}
            </Button>
          )}
          {/* Discussions Shortcut Button */}
          {onOpenDiscussions && (
            <Button
              size="sm"
              variant="outline"
              onClick={() => onOpenDiscussions(dep)}
              className="h-8 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 px-2 flex items-center gap-1"
              title="Buka Ruang Diskusi Project"
            >
              <MessageSquare className="h-3.5 w-3.5 text-emerald-500" />
            </Button>
          )}

          {/* Tasks Board Shortcut Button */}
          {onOpenTasks && (
            <Button
              size="sm"
              variant="outline"
              onClick={() => onOpenTasks(dep)}
              className="h-8 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 px-2 flex items-center gap-1"
              title="Buka Tasks Kanban Board"
            >
              <Kanban className="h-3.5 w-3.5 text-indigo-500" />
            </Button>
          )}
        </div>

        <Button
          size="sm"
          variant="outline"
          onClick={() => onOpenDrawer(dep)}
          className="h-8 text-xs font-semibold flex items-center gap-1 text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <Terminal className="h-3.5 w-3.5 text-emerald-500" />
          <span>Logs & Drawer</span>
        </Button>
      </div>
    </Card>
  )
}
