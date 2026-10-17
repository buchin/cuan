import { useState, useEffect } from 'react'
import { Button } from '../../ui/button'
import { Badge } from '../../ui/badge'
import {
  Globe,
  ExternalLink,
  RotateCcw,
  AlertCircle,
  KeyRound,
  Rocket
} from 'lucide-react'
import { WordPressDeploymentDTO } from '../types'

export interface WordPressControlsProps {
  projectId: string
  projectName?: string
  projectSlug?: string
  baseDomain?: string
  onOpenDeployerWizard: (initialStep?: 1 | 2 | 3 | 4) => void
  onOpenDashboard?: () => void
  onToast?: (type: 'success' | 'error', message: string) => void
  className?: string
}

export function WordPressControls({
  projectId,
  projectName: _projectName = 'WordPress Project',
  projectSlug = '',
  baseDomain = 'cuan.dojo.cc',
  onOpenDeployerWizard,
  onOpenDashboard,
  onToast,
  className = '',
}: WordPressControlsProps) {
  const [deployment, setDeployment] = useState<WordPressDeploymentDTO | null>(null)
  const [restarting, setRestarting] = useState(false)

  // Fetch status
  const fetchStatus = async () => {
    if (!projectId || projectId === 'default') return
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress`)
      if (res.ok) {
        const data: WordPressDeploymentDTO = await res.json()
        setDeployment(data)
      } else if (res.status === 404) {
        setDeployment(null)
      }
    } catch {
      // Ignore network errors in polling
    }
  }

  useEffect(() => {
    fetchStatus()
    const timer = setInterval(fetchStatus, 10000)
    return () => clearInterval(timer)
  }, [projectId])

  // Restart Handler
  const handleRestart = async () => {
    if (!deployment) return
    setRestarting(true)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/restart`, {
        method: 'POST',
      })
      if (res.ok) {
        const data = await res.json()
        if (data.deployment) {
          setDeployment(data.deployment)
        }
        onToast?.('success', '⚡ Service WordPress container berhasil di-restart.')
        fetchStatus()
      } else {
        const err = await res.json().catch(() => ({}))
        onToast?.('error', err.error || 'Gagal me-restart WordPress.')
      }
    } catch (err: any) {
      onToast?.('error', err.message || 'Gagal me-restart WordPress.')
    } finally {
      setRestarting(false)
    }
  }

  // Derive status
  const isRunning = deployment?.status === 'running'
  const isHealthy = deployment?.health_status === 'healthy'
  const isDegraded = isRunning && deployment?.health_status !== 'healthy'
  const isStopped = deployment?.status === 'stopped'

  // URL calculation
  const fallbackDomain = projectSlug ? `${projectSlug}.${baseDomain}` : ''
  const siteUrl = deployment?.domain
    ? `https://${deployment.domain}`
    : fallbackDomain
    ? `https://${fallbackDomain}`
    : deployment?.port
    ? `http://127.0.0.1:${deployment.port}`
    : ''
  const adminUrl = siteUrl ? `${siteUrl}/wp-login.php` : ''

  return (
    <div className={`flex flex-wrap items-center gap-2 text-xs ${className}`}>
      {/* WordPress Status Badge */}
      {deployment ? (
        <div className="flex items-center gap-1.5 bg-slate-100 dark:bg-slate-800/80 px-2.5 py-1 rounded-md border border-slate-200 dark:border-slate-700/60 shadow-2xs">
          <Globe className="h-3.5 w-3.5 text-blue-500" />
          <span className="font-medium text-slate-700 dark:text-slate-300">WordPress:</span>
          {isHealthy ? (
            <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 flex items-center gap-1 text-[11px] px-1.5 py-0 font-medium">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
              Healthy
            </Badge>
          ) : isDegraded ? (
            <Badge className="bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20 flex items-center gap-1 text-[11px] px-1.5 py-0 font-medium">
              <AlertCircle className="h-2.5 w-2.5" />
              Degraded
            </Badge>
          ) : isStopped ? (
            <Badge className="bg-slate-500/10 text-slate-600 dark:text-slate-400 border border-slate-500/20 text-[11px] px-1.5 py-0 font-medium">
              Stopped
            </Badge>
          ) : (
            <Badge className="bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20 text-[11px] px-1.5 py-0 font-medium">
              {deployment.status}
            </Badge>
          )}

          {/* Quick Action Links */}
          {siteUrl && (
            <a
              href={siteUrl}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-0.5 text-blue-600 dark:text-blue-400 hover:underline font-medium ml-1"
              title="Visit Live WordPress Site"
            >
              <span>Site</span>
              <ExternalLink className="h-3 w-3" />
            </a>
          )}

          {adminUrl && (
            <a
              href={adminUrl}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-0.5 text-indigo-600 dark:text-indigo-400 hover:underline font-medium ml-1"
              title="Open WP-Admin Login"
            >
              <KeyRound className="h-3 w-3" />
              <span>Admin</span>
            </a>
          )}

          {/* Quick Restart Button */}
          {isRunning && (
            <button
              onClick={handleRestart}
              disabled={restarting}
              className="p-1 hover:bg-slate-200 dark:hover:bg-slate-700 rounded text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 transition-colors ml-0.5"
              title="Restart WordPress container"
            >
              <RotateCcw className={`h-3 w-3 ${restarting ? 'animate-spin text-blue-500' : ''}`} />
            </button>
          )}
        </div>
      ) : (
        <div className="flex items-center gap-1.5 bg-slate-100 dark:bg-slate-800/80 px-2 py-1 rounded-md border border-slate-200 dark:border-slate-700/60 text-slate-500 text-[11px]">
          <Globe className="h-3 w-3 text-slate-400" />
          <span>WP: Not Deployed</span>
        </div>
      )}

      {/* 1-Click Launchpad Wizard Trigger */}
      <Button
        variant="outline"
        size="sm"
        onClick={() => onOpenDeployerWizard(1)}
        className="h-7 text-xs gap-1.5 border-blue-200 dark:border-blue-900/50 hover:bg-blue-50 dark:hover:bg-blue-950/40 text-blue-700 dark:text-blue-300 font-medium shadow-2xs"
      >
        <Rocket className="h-3.5 w-3.5 text-blue-500" />
        <span>{deployment ? 'Manage WP' : '1-Click WordPress'}</span>
      </Button>

      {/* Studio View Button if deployed */}
      {deployment && onOpenDashboard && (
        <Button
          variant="ghost"
          size="sm"
          onClick={onOpenDashboard}
          className="h-7 text-xs gap-1 text-slate-600 dark:text-slate-300 hover:text-blue-600 dark:hover:text-blue-400"
        >
          <Globe className="h-3.5 w-3.5 text-blue-500" />
          <span>Webmaster Studio</span>
        </Button>
      )}
    </div>
  )
}

export const WordPressQuickControls = WordPressControls
