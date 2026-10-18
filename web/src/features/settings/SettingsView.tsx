import React, { useState } from 'react'
import { Project } from '../../api/types'
import {
  Settings,
  GitBranch,
  Globe,
  Server,
  AlertTriangle,
  Save,
  CheckCircle2,
  Key,
  ShieldCheck,
  Plus,
  Trash2
} from 'lucide-react'

interface SettingsViewProps {
  activeProject?: Project
  onUpdateProject?: (id: string, data: any) => Promise<void>
  onDeleteProject?: (id: string) => Promise<void>
}

export function SettingsView({
  activeProject,
  onUpdateProject,
  onDeleteProject,
}: SettingsViewProps) {
  const [repoUrl, setRepoUrl] = useState(activeProject?.repo_url || '')
  const [defaultBranch, setDefaultBranch] = useState(activeProject?.default_branch || 'main')
  const [domain, setDomain] = useState('')
  const [serverIp, setServerIp] = useState('')
  const [savedNotice, setSavedNotice] = useState(false)
  const [saving, setSaving] = useState(false)

  // Environment Variables state
  const [envVars, setEnvVars] = useState<{ key: string; value: string }[]>([
    { key: 'APP_ENV', value: 'production' },
    { key: 'PORT', value: '8080' },
  ])
  const [newEnvKey, setNewEnvKey] = useState('')
  const [newEnvVal, setNewEnvVal] = useState('')

  const handleAddEnv = () => {
    if (newEnvKey.trim()) {
      setEnvVars([...envVars, { key: newEnvKey.trim().toUpperCase(), value: newEnvVal.trim() }])
      setNewEnvKey('')
      setNewEnvVal('')
    }
  }

  const handleRemoveEnv = (index: number) => {
    setEnvVars(envVars.filter((_, i) => i !== index))
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!activeProject || !onUpdateProject) return

    setSaving(true)
    try {
      await onUpdateProject(activeProject.id, {
        repo_url: repoUrl,
        default_branch: defaultBranch,
      })
      setSavedNotice(true)
      setTimeout(() => setSavedNotice(false), 3000)
    } catch (err) {
      console.error(err)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      <div>
        <h1 className="text-xl font-bold text-foreground flex items-center gap-2">
          <Settings className="h-5 w-5 text-primary" />
          <span>{activeProject ? `Project Settings: ${activeProject.name}` : 'Global Workplace Settings'}</span>
        </h1>
        <p className="text-xs text-muted-foreground mt-0.5">
          Configure Git repository links, custom domains, and server credentials in one unified place.
        </p>
      </div>

      {savedNotice && (
        <div className="p-3 bg-emerald-500/10 border border-emerald-500/30 text-emerald-600 dark:text-emerald-400 text-xs flex items-center gap-2 rounded">
          <CheckCircle2 className="h-4 w-4" />
          <span>Settings saved successfully.</span>
        </div>
      )}

      {activeProject ? (
        <form onSubmit={handleSave} className="space-y-6">
          {/* Git Repository Card */}
          <div className="border border-border bg-card text-card-foreground p-5 space-y-4 rounded-lg">
            <div className="flex items-center gap-2 border-b border-border pb-3">
              <GitBranch className="h-4 w-4 text-primary" />
              <h3 className="text-xs font-bold text-foreground uppercase tracking-wider">
                Git Repository Configuration
              </h3>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="md:col-span-2">
                <label className="block text-xs font-semibold text-muted-foreground mb-1">
                  Repository URL
                </label>
                <input
                  type="text"
                  placeholder="https://github.com/username/repository"
                  value={repoUrl}
                  onChange={(e) => setRepoUrl(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-muted-foreground mb-1">
                  Default Branch
                </label>
                <input
                  type="text"
                  placeholder="main"
                  value={defaultBranch}
                  onChange={(e) => setDefaultBranch(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
                />
              </div>
            </div>
          </div>

          {/* Custom Domain Card */}
          <div className="border border-border bg-card text-card-foreground p-5 space-y-4 rounded-lg">
            <div className="flex items-center gap-2 border-b border-border pb-3">
              <Globe className="h-4 w-4 text-primary" />
              <h3 className="text-xs font-bold text-foreground uppercase tracking-wider">
                Custom Domain (BYOD)
              </h3>
            </div>

            <div>
              <label className="block text-xs font-semibold text-muted-foreground mb-1">
                Domain Name
              </label>
              <input
                type="text"
                placeholder="example.com or app.example.com"
                value={domain}
                onChange={(e) => setDomain(e.target.value)}
                className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
              />
              <p className="text-[11px] text-muted-foreground mt-1">
                Point an A record to your server IP. Caddy automatically provisions Let's Encrypt TLS.
              </p>
            </div>
          </div>

          {/* Server VPS Card */}
          <div className="border border-border bg-card text-card-foreground p-5 space-y-4 rounded-lg">
            <div className="flex items-center gap-2 border-b border-border pb-3">
              <Server className="h-4 w-4 text-primary" />
              <h3 className="text-xs font-bold text-foreground uppercase tracking-wider">
                Deployment Host Server (BYOS)
              </h3>
            </div>

            <div>
              <label className="block text-xs font-semibold text-muted-foreground mb-1">
                Target Server Hostname / IP
              </label>
              <input
                type="text"
                placeholder="192.168.1.100 or vps.example.com"
                value={serverIp}
                onChange={(e) => setServerIp(e.target.value)}
                className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
              />
            </div>
          </div>

          {/* Environment Variables & Secrets Card */}
          <div className="border border-border bg-card text-card-foreground p-5 space-y-4 rounded-lg">
            <div className="flex items-center gap-2 border-b border-border pb-3">
              <Key className="h-4 w-4 text-primary" />
              <h3 className="text-xs font-bold text-foreground uppercase tracking-wider">
                Environment Variables & Secrets
              </h3>
            </div>

            <div className="space-y-2">
              <div className="flex gap-2">
                <input
                  type="text"
                  placeholder="KEY (e.g. DATABASE_URL)"
                  value={newEnvKey}
                  onChange={(e) => setNewEnvKey(e.target.value)}
                  className="w-1/3 px-3 py-1.5 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
                />
                <input
                  type="text"
                  placeholder="VALUE"
                  value={newEnvVal}
                  onChange={(e) => setNewEnvVal(e.target.value)}
                  className="flex-1 px-3 py-1.5 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
                />
                <button
                  type="button"
                  onClick={handleAddEnv}
                  className="px-3 py-1.5 bg-primary text-primary-foreground font-semibold text-xs hover:bg-primary/90 transition-colors flex items-center gap-1 rounded"
                >
                  <Plus className="h-3.5 w-3.5" />
                  <span>Add</span>
                </button>
              </div>

              <div className="space-y-1 pt-2">
                {envVars.map((item, idx) => (
                  <div
                    key={idx}
                    className="flex items-center justify-between p-2 bg-muted/40 border border-border text-xs rounded"
                  >
                    <div className="flex items-center gap-2 font-mono">
                      <span className="text-primary font-bold">{item.key}</span>
                      <span className="text-muted-foreground">=</span>
                      <span className="text-foreground">{item.value}</span>
                    </div>
                    <button
                      type="button"
                      onClick={() => handleRemoveEnv(idx)}
                      className="text-destructive hover:opacity-80 p-1"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div className="flex items-center justify-end">
            <button
              type="submit"
              disabled={saving}
              className="flex items-center gap-2 px-5 py-2 bg-primary text-primary-foreground font-semibold text-xs hover:bg-primary/90 transition-colors disabled:opacity-50 rounded"
            >
              <Save className="h-4 w-4" />
              <span>{saving ? 'Saving...' : 'Save Settings'}</span>
            </button>
          </div>

          {/* Danger Zone */}
          {onDeleteProject && (
            <div className="border border-destructive/40 bg-destructive/5 p-5 space-y-3 rounded-lg">
              <div className="flex items-center gap-2">
                <AlertTriangle className="h-4 w-4 text-destructive" />
                <h4 className="text-xs font-bold text-destructive uppercase tracking-wider">
                  Danger Zone
                </h4>
              </div>
              <p className="text-xs text-muted-foreground">
                Deleting this project removes its configuration from Cuan. Database migrations and underlying code repositories are preserved.
              </p>
              <button
                type="button"
                onClick={() => {
                  if (confirm(`Are you sure you want to delete project "${activeProject.name}"?`)) {
                    onDeleteProject(activeProject.id)
                  }
                }}
                className="px-4 py-2 bg-destructive text-destructive-foreground font-bold text-xs hover:bg-destructive/90 transition-colors rounded"
              >
                Delete Project
              </button>
            </div>
          )}
        </form>
      ) : (
        <div className="space-y-4">
          <div className="border border-border bg-card text-card-foreground p-6 space-y-4 rounded-lg">
            <div className="flex items-center gap-2 border-b border-border pb-3">
              <ShieldCheck className="h-4 w-4 text-emerald-500" />
              <h3 className="text-xs font-bold text-foreground uppercase tracking-wider">
                Workplace Instance Overview
              </h3>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
              <div className="p-3 bg-muted/40 border border-border rounded">
                <div className="text-[10px] text-muted-foreground uppercase font-bold">Platform Version</div>
                <div className="font-mono text-primary mt-1 font-semibold">Cuan v0.3.0 (Direct Messenger Hub)</div>
              </div>
              <div className="p-3 bg-muted/40 border border-border rounded">
                <div className="text-[10px] text-muted-foreground uppercase font-bold">Base Domain</div>
                <div className="font-mono text-emerald-500 mt-1 font-semibold">cuan.dojo.cc</div>
              </div>
            </div>
            <p className="text-xs text-muted-foreground">
              To configure Git repositories, custom domains, or server hosts for a specific project, select that project using the top breadcrumb dropdown.
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
