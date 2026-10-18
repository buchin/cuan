import React, { useState } from 'react'
import { ProjectType } from '../../api/types'
import { X, FolderGit2, AlertCircle } from 'lucide-react'

interface CreateProjectModalProps {
  isOpen: boolean
  onClose: () => void
  onSubmit: (data: { name: string; project_type: ProjectType; repo_url?: string; github_token?: string }) => Promise<void>
}

export function CreateProjectModal({
  isOpen,
  onClose,
  onSubmit,
}: CreateProjectModalProps) {
  const [name, setName] = useState('')
  const [projectType, setProjectType] = useState<ProjectType>('custom')
  const [repoUrl, setRepoUrl] = useState('')
  const [githubToken, setGithubToken] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      setError('Project name is required')
      return
    }

    setLoading(true)
    setError(null)
    try {
      await onSubmit({
        name: name.trim(),
        project_type: projectType,
        repo_url: repoUrl.trim() || undefined,
        github_token: githubToken.trim() || undefined,
      })
      onClose()
      setName('')
      setRepoUrl('')
      setGithubToken('')
    } catch (err: any) {
      setError(err.message || 'Failed to create project')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 font-mono select-none">
      <div className="w-full max-w-md bg-card border border-border shadow-2xl p-6 rounded-lg text-card-foreground">
        <div className="flex items-center justify-between pb-4 border-b border-border mb-4">
          <div className="flex items-center gap-2">
            <FolderGit2 className="h-5 w-5 text-primary" />
            <h2 className="text-sm font-bold text-foreground">
              Create New Project
            </h2>
          </div>
          <button
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {error && (
          <div className="mb-4 p-2.5 bg-destructive/10 border border-destructive/30 text-destructive text-xs flex items-center gap-2 rounded">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1">
              Project Name *
            </label>
            <input
              type="text"
              required
              placeholder="e.g. Toko Sepatu, Tech Blog, Mobile App"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1">
              Project Category
            </label>
            <select
              value={projectType}
              onChange={(e) => setProjectType(e.target.value as ProjectType)}
              className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
            >
              <option value="custom">Custom / General Software</option>
              <option value="saas">SaaS Application</option>
              <option value="blog">Content Blog / WordPress</option>
              <option value="youtube">YouTube Creator Production</option>
              <option value="olshop">Online Shop / Digital Products</option>
              <option value="affiliate">Affiliate Marketing</option>
              <option value="blank">Blank Workspace</option>
            </select>
          </div>

          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1">
              GitHub Repository URL (Optional)
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
              Personal Access Token (Optional for private repos)
            </label>
            <input
              type="password"
              placeholder="ghp_xxxxxxxxxxxx"
              value={githubToken}
              onChange={(e) => setGithubToken(e.target.value)}
              className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
            />
          </div>

          <div className="pt-2 flex items-center justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-4 py-2 bg-primary text-primary-foreground font-semibold text-xs hover:bg-primary/90 transition-colors disabled:opacity-50 rounded"
            >
              {loading ? 'Creating...' : 'Create Project'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
