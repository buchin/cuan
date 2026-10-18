import { useState } from 'react'
import { Project } from '../../api/types'
import { Settings, GitBranch, Globe, Server, AlertTriangle, Save, CheckCircle2 } from 'lucide-react'

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

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!activeProject || !onUpdateProject) return

    setSaving(true)
    try {
      await onUpdateProject(activeProject.id, {
        repo_url: repoUrl,
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
        <h1 className="text-xl font-bold text-[#1a1b26] dark:text-[#c0caf5] flex items-center gap-2">
          <Settings className="h-5 w-5 text-[#7aa2f7]" />
          <span>{activeProject ? `Project Settings: ${activeProject.name}` : 'Global Workplace Settings'}</span>
        </h1>
        <p className="text-xs text-[#565f89] mt-0.5">
          Configure Git repository links, custom domains, and server credentials in one unified place.
        </p>
      </div>

      {savedNotice && (
        <div className="p-3 bg-[#9ece6a]/15 border border-[#9ece6a]/30 text-[#9ece6a] text-xs flex items-center gap-2">
          <CheckCircle2 className="h-4 w-4" />
          <span>Settings saved successfully.</span>
        </div>
      )}

      {activeProject ? (
        <form onSubmit={handleSave} className="space-y-6">
          {/* Git Repository Card */}
          <div className="border border-[#cfd6e6] dark:border-[#24283b] bg-[#e2e7f1] dark:bg-[#16161e] p-5 space-y-4">
            <div className="flex items-center gap-2 border-b border-[#cfd6e6] dark:border-[#24283b] pb-3">
              <GitBranch className="h-4 w-4 text-[#7aa2f7]" />
              <h3 className="text-xs font-bold text-[#1a1b26] dark:text-[#c0caf5] uppercase tracking-wider">
                Git Repository Configuration
              </h3>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="md:col-span-2">
                <label className="block text-xs font-semibold text-[#565f89] mb-1">
                  Repository URL
                </label>
                <input
                  type="text"
                  placeholder="https://github.com/username/repository"
                  value={repoUrl}
                  onChange={(e) => setRepoUrl(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-[#d5dce8] dark:bg-[#1f2335] border border-[#bcc5d8] dark:border-[#292e42] text-[#1a1b26] dark:text-[#c0caf5] focus:outline-none focus:border-[#7aa2f7]"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-[#565f89] mb-1">
                  Default Branch
                </label>
                <input
                  type="text"
                  placeholder="main"
                  value={defaultBranch}
                  onChange={(e) => setDefaultBranch(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-[#d5dce8] dark:bg-[#1f2335] border border-[#bcc5d8] dark:border-[#292e42] text-[#1a1b26] dark:text-[#c0caf5] focus:outline-none focus:border-[#7aa2f7]"
                />
              </div>
            </div>
          </div>

          {/* Custom Domain Card */}
          <div className="border border-[#cfd6e6] dark:border-[#24283b] bg-[#e2e7f1] dark:bg-[#16161e] p-5 space-y-4">
            <div className="flex items-center gap-2 border-b border-[#cfd6e6] dark:border-[#24283b] pb-3">
              <Globe className="h-4 w-4 text-[#7aa2f7]" />
              <h3 className="text-xs font-bold text-[#1a1b26] dark:text-[#c0caf5] uppercase tracking-wider">
                Custom Domain (BYOD)
              </h3>
            </div>

            <div>
              <label className="block text-xs font-semibold text-[#565f89] mb-1">
                Domain Name
              </label>
              <input
                type="text"
                placeholder="example.com or app.example.com"
                value={domain}
                onChange={(e) => setDomain(e.target.value)}
                className="w-full px-3 py-2 text-xs bg-[#d5dce8] dark:bg-[#1f2335] border border-[#bcc5d8] dark:border-[#292e42] text-[#1a1b26] dark:text-[#c0caf5] focus:outline-none focus:border-[#7aa2f7]"
              />
              <p className="text-[11px] text-[#565f89] mt-1">
                Point an A record to your server IP. Caddy automatically provisions Let's Encrypt TLS.
              </p>
            </div>
          </div>

          {/* Server VPS Card */}
          <div className="border border-[#cfd6e6] dark:border-[#24283b] bg-[#e2e7f1] dark:bg-[#16161e] p-5 space-y-4">
            <div className="flex items-center gap-2 border-b border-[#cfd6e6] dark:border-[#24283b] pb-3">
              <Server className="h-4 w-4 text-[#7aa2f7]" />
              <h3 className="text-xs font-bold text-[#1a1b26] dark:text-[#c0caf5] uppercase tracking-wider">
                Deployment Host Server (BYOS)
              </h3>
            </div>

            <div>
              <label className="block text-xs font-semibold text-[#565f89] mb-1">
                Target Server Hostname / IP
              </label>
              <input
                type="text"
                placeholder="192.168.1.100 or vps.example.com"
                value={serverIp}
                onChange={(e) => setServerIp(e.target.value)}
                className="w-full px-3 py-2 text-xs bg-[#d5dce8] dark:bg-[#1f2335] border border-[#bcc5d8] dark:border-[#292e42] text-[#1a1b26] dark:text-[#c0caf5] focus:outline-none focus:border-[#7aa2f7]"
              />
            </div>
          </div>

          <div className="flex items-center justify-end">
            <button
              type="submit"
              disabled={saving}
              className="flex items-center gap-2 px-5 py-2 bg-[#7aa2f7] text-[#16161e] font-semibold text-xs hover:bg-[#89b4fa] transition-colors disabled:opacity-50"
            >
              <Save className="h-4 w-4" />
              <span>{saving ? 'Saving...' : 'Save Settings'}</span>
            </button>
          </div>

          {/* Danger Zone */}
          {onDeleteProject && (
            <div className="border border-[#f7768e]/40 bg-[#f7768e]/5 p-5 space-y-3">
              <div className="flex items-center gap-2">
                <AlertTriangle className="h-4 w-4 text-[#f7768e]" />
                <h4 className="text-xs font-bold text-[#f7768e] uppercase tracking-wider">
                  Danger Zone
                </h4>
              </div>
              <p className="text-xs text-[#565f89]">
                Deleting this project removes its configuration from Cuan. Database migrations and underlying code repositories are preserved.
              </p>
              <button
                type="button"
                onClick={() => {
                  if (confirm(`Are you sure you want to delete project "${activeProject.name}"?`)) {
                    onDeleteProject(activeProject.id)
                  }
                }}
                className="px-4 py-2 bg-[#f7768e] text-[#16161e] font-bold text-xs hover:bg-[#ff8f9e] transition-colors"
              >
                Delete Project
              </button>
            </div>
          )}
        </form>
      ) : (
        <div className="border border-[#cfd6e6] dark:border-[#24283b] bg-[#e2e7f1] dark:bg-[#16161e] p-6 text-center">
          <p className="text-xs text-[#565f89]">
            Select a project from the top switcher to configure its repository, domain, and server settings.
          </p>
        </div>
      )}
    </div>
  )
}
