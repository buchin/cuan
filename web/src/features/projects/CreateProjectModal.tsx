import { useEffect, useState } from 'react'
import { ProjectType } from '../../api/types'
import { ActionDialog } from '../../components/dialogs/ActionDialog'

interface CreateProjectModalProps {
  isOpen: boolean
  onClose: () => void
  onSubmit: (data: {
    name: string
    project_type: ProjectType
    repo_url?: string
    github_token?: string
  }) => Promise<void>
}
export function CreateProjectModal({
  isOpen,
  onClose,
  onSubmit,
}: CreateProjectModalProps) {
  const [name, setName] = useState(''),
    [projectType, setProjectType] = useState<ProjectType>('custom'),
    [repoUrl, setRepoUrl] = useState(''),
    [githubToken, setGithubToken] = useState(''),
    [loading, setLoading] = useState(false),
    [error, setError] = useState('')
  useEffect(() => {
    if (isOpen) {
      setName('')
      setProjectType('custom')
      setRepoUrl('')
      setGithubToken('')
      setError('')
    }
  }, [isOpen])
  return (
    <ActionDialog
      open={isOpen}
      title="New project"
      onClose={onClose}
      busy={loading}
    >
      <form
        className="space-y-4"
        onSubmit={async (e) => {
          e.preventDefault()
          if (loading || !name.trim()) return
          setLoading(true)
          setError('')
          try {
            await onSubmit({
              name: name.trim(),
              project_type: projectType,
              repo_url: repoUrl.trim() || undefined,
              github_token: githubToken.trim() || undefined,
            })
            setGithubToken('')
            onClose()
          } catch (e) {
            setError(
              e instanceof Error ? e.message : 'Failed to create project',
            )
          } finally {
            setLoading(false)
          }
        }}
      >
        <div>
          <label className="text-sm font-medium" htmlFor="new-project-name">
            Project name
          </label>
          <input
            id="new-project-name"
            className="field-control mt-1"
            required
            maxLength={200}
            value={name}
            disabled={loading}
            onChange={(e) => setName(e.target.value)}
          />
        </div>
        <div>
          <label className="text-sm font-medium" htmlFor="new-project-type">
            Project category
          </label>
          <select
            id="new-project-type"
            className="field-control mt-1"
            value={projectType}
            disabled={loading}
            onChange={(e) => setProjectType(e.target.value as ProjectType)}
          >
            {Object.entries({
              custom: 'Custom / General Software',
              saas: 'SaaS Application',
              blog: 'Content Blog / WordPress',
              youtube: 'YouTube Creator Production',
              olshop: 'Online Shop / Digital Products',
              affiliate: 'Affiliate Marketing',
              blank: 'Blank Workspace',
            }).map(([id, label]) => (
              <option key={id} value={id}>
                {label}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="text-sm font-medium" htmlFor="new-project-repo">
            Repository URL (optional)
          </label>
          <input
            id="new-project-repo"
            type="text"
            inputMode="url"
            className="field-control mt-1"
            placeholder="https://github.com/username/repository"
            value={repoUrl}
            disabled={loading}
            onChange={(e) => setRepoUrl(e.target.value)}
          />
        </div>
        <div>
          <label className="text-sm font-medium" htmlFor="new-project-token">
            Access token (optional for private repos)
          </label>
          <input
            id="new-project-token"
            type="password"
            className="field-control mt-1"
            autoComplete="off"
            value={githubToken}
            disabled={loading}
            onChange={(e) => setGithubToken(e.target.value)}
          />
        </div>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <div className="flex justify-end gap-2">
          <button
            type="button"
            className="action-secondary"
            disabled={loading}
            onClick={onClose}
          >
            Cancel
          </button>
          <button className="action-primary" disabled={loading || !name.trim()}>
            {loading ? 'Creating…' : 'Create project'}
          </button>
        </div>
      </form>
    </ActionDialog>
  )
}
