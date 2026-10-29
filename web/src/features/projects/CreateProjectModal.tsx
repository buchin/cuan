import { useEffect, useState } from 'react'
import {
  Sparkles,
  FolderGit2,
  GitBranch,
  ShoppingBag,
  Globe,
  Briefcase,
  BookOpen,
  ChevronDown,
  Loader2,
  Check,
} from 'lucide-react'
import { ProjectType } from '../../api/types'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { useLanguage } from '../../context/LanguageContext'
import { GitHubRepositoryPicker } from '../settings/GitHubRepositoryPicker'

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
  const { t } = useLanguage()
  const [mode, setMode] = useState<'new' | 'github'>('new')
  const [name, setName] = useState('')
  const [repoUrl, setRepoUrl] = useState('')
  const [githubToken, setGithubToken] = useState('')
  const [showTokenInput, setShowTokenInput] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (isOpen) {
      setMode('new')
      setName('')
      setRepoUrl('')
      setGithubToken('')
      setShowTokenInput(false)
      setError('')
    }
  }, [isOpen])

  const quickStarters = [
    { label: t('Online Store'), icon: ShoppingBag, suggest: t('My Online Store') },
    { label: t('Landing Page'), icon: Globe, suggest: t('Business Landing Page') },
    { label: t('Company Profile'), icon: Briefcase, suggest: t('Company Profile Website') },
    { label: t('Blog & Content'), icon: BookOpen, suggest: t('Content & Blog Site') },
  ]

  const handleSelectRepo = (url: string) => {
    setRepoUrl(url)
    const clean = url.trim().replace(/\/+$/, '')
    const parts = clean.split('/')
    const repoName = parts[parts.length - 1]
    if (repoName && (!name || name === '')) {
      // Auto-fill project name formatted nicely
      setName(repoName.replace(/[-_]/g, ' '))
    }
  }

  return (
    <ActionDialog
      open={isOpen}
      title={t('New project')}
      onClose={onClose}
      busy={loading}
    >
      <div className="space-y-4">
        {/* Segmented Mode Selector: Mulai Baru vs Hubungkan GitHub */}
        <div className="flex items-center gap-1 p-1 rounded-xl bg-muted/60 text-xs font-medium">
          <button
            type="button"
            id="create-project-mode-new"
            onClick={() => setMode('new')}
            className={`flex-1 min-h-11 flex items-center justify-center gap-1.5 py-2.5 px-3 rounded-lg text-xs font-medium transition-all ${
              mode === 'new'
                ? 'bg-background text-foreground shadow-sm font-semibold'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            <Sparkles className="h-3.5 w-3.5 text-amber-500" />
            <span>{t('Create New')}</span>
          </button>
          <button
            type="button"
            id="create-project-mode-github"
            onClick={() => setMode('github')}
            className={`flex-1 min-h-11 flex items-center justify-center gap-1.5 py-2.5 px-3 rounded-lg text-xs font-medium transition-all ${
              mode === 'github'
                ? 'bg-background text-foreground shadow-sm font-semibold'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            <GitBranch className="h-3.5 w-3.5 text-primary" />
            <span>{t('Connect GitHub')}</span>
          </button>
        </div>

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
                project_type: 'custom',
                repo_url: mode === 'github' ? repoUrl.trim() || undefined : undefined,
                github_token: mode === 'github' ? githubToken.trim() || undefined : undefined,
              })
              setGithubToken('')
              onClose()
            } catch (err) {
              setError(
                err instanceof Error ? err.message : t('Failed to create project'),
              )
            } finally {
              setLoading(false)
            }
          }}
        >
          {/* TAB 1: Mulai Proyek Baru dari Nol */}
          {mode === 'new' && (
            <div className="space-y-3.5">
              <div>
                <label className="text-sm font-medium text-foreground" htmlFor="new-project-name">
                  {t('Project name')}
                </label>
                <input
                  id="new-project-name"
                  className="field-control mt-1.5"
                  required
                  maxLength={200}
                  placeholder={t('e.g. Coffee Shop Website')}
                  value={name}
                  disabled={loading}
                  onChange={(e) => setName(e.target.value)}
                  autoFocus
                />
              </div>

              {/* Quick Starter Inspiration Chips with Active Highlight */}
              <div>
                <span className="text-[11px] font-medium text-muted-foreground">
                  {t('Quick inspiration:')}
                </span>
                <div className="flex flex-wrap gap-1.5 mt-1.5">
                  {quickStarters.map((item) => {
                    const Icon = item.icon
                    const isSelected = name === item.suggest
                    return (
                      <button
                        key={item.label}
                        type="button"
                        onClick={() => setName(item.suggest)}
                        className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] transition-all ${
                          isSelected
                            ? 'border-primary bg-primary/10 text-primary font-medium ring-1 ring-primary/30'
                            : 'border-border/80 bg-muted/30 text-muted-foreground hover:bg-accent hover:text-foreground hover:border-primary/40'
                        }`}
                      >
                        <Icon className={`h-3 w-3 ${isSelected ? 'text-primary' : 'text-primary/80'}`} />
                        <span>{item.label}</span>
                        {isSelected && <Check className="h-2.5 w-2.5 text-primary" />}
                      </button>
                    )
                  })}
                </div>
              </div>

              {/* Micro-benefits Card */}
              <div className="rounded-xl border border-border/60 bg-muted/20 p-3 space-y-1.5 text-xs text-muted-foreground">
                <div className="flex items-center gap-1.5 text-foreground font-medium text-[11px]">
                  <Sparkles className="h-3.5 w-3.5 text-amber-500 shrink-0" />
                  <span>{t('Managed Workspace by Cuan')}</span>
                </div>
                <p className="text-[11px] leading-relaxed">
                  {t(
                    'Cuan will set up an independent workspace on the server. You can immediately start building features via Chat.',
                  )}
                </p>
              </div>
            </div>
          )}

          {/* TAB 2: Hubungkan Repositori GitHub */}
          {mode === 'github' && (
            <div className="space-y-3.5">
              {/* GitHub Picker Integration with Live Search & Cards */}
              <div>
                <GitHubRepositoryPicker
                  disabled={loading}
                  selectedUrl={repoUrl}
                  onSelect={handleSelectRepo}
                />
              </div>

              {/* Selected Repo Card (or Manual URL Fallback) */}
              {repoUrl ? (
                <div className="flex items-center justify-between p-2.5 rounded-xl bg-primary/5 border border-primary/20 text-xs">
                  <div className="flex items-center gap-2 min-w-0">
                    <FolderGit2 className="h-4 w-4 text-primary shrink-0" />
                    <span className="truncate font-medium text-primary">
                      {repoUrl.replace(/^https?:\/\/(?:www\.)?github\.com\//i, '')}
                    </span>
                  </div>
                  <button
                    type="button"
                    onClick={() => setRepoUrl('')}
                    className="text-[11px] text-muted-foreground hover:text-foreground underline shrink-0 ml-2"
                  >
                    {t('Change')}
                  </button>
                </div>
              ) : (
                <div>
                  <label className="text-xs font-medium text-muted-foreground" htmlFor="new-project-repo">
                    {t('Or enter GitHub repository URL manually')}
                  </label>
                  <input
                    id="new-project-repo"
                    type="text"
                    inputMode="url"
                    className="field-control mt-1"
                    placeholder="https://github.com/username/repository"
                    value={repoUrl}
                    disabled={loading}
                    onChange={(e) => handleSelectRepo(e.target.value)}
                  />
                </div>
              )}

              <div>
                <label className="text-sm font-medium text-foreground" htmlFor="new-project-name-gh">
                  {t('Project name')}
                </label>
                <input
                  id="new-project-name-gh"
                  className="field-control mt-1.5"
                  required
                  maxLength={200}
                  placeholder={t('e.g. Shoe Store')}
                  value={name}
                  disabled={loading}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>

              {/* Optional Collapsible Access Token */}
              <div className="pt-0.5">
                {!showTokenInput ? (
                  <button
                    type="button"
                    onClick={() => setShowTokenInput(true)}
                    className="text-xs text-muted-foreground hover:text-foreground flex items-center gap-1 transition-colors"
                  >
                    <ChevronDown className="h-3.5 w-3.5" />
                    <span>{t('+ Enter Personal Access Token (for private repos)')}</span>
                  </button>
                ) : (
                  <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground" htmlFor="new-project-token">
                      {t('Personal Access Token (GitHub PAT)')}
                    </label>
                    <input
                      id="new-project-token"
                      type="password"
                      className="field-control mt-1"
                      autoComplete="off"
                      placeholder="ghp_..."
                      value={githubToken}
                      disabled={loading}
                      onChange={(e) => setGithubToken(e.target.value)}
                    />
                  </div>
                )}
              </div>
            </div>
          )}

          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}

          <div className="flex justify-end gap-2 pt-2 border-t border-border/40">
            <button
              type="button"
              className="action-secondary"
              disabled={loading}
              onClick={onClose}
            >
              {t('Cancel')}
            </button>
            <button
              type="submit"
              id="create-project-submit-btn"
              className="action-primary inline-flex items-center gap-2"
              disabled={loading || !name.trim()}
            >
              {loading && <Loader2 className="h-4 w-4 animate-spin shrink-0" />}
              <span>
                {loading
                  ? mode === 'new'
                    ? t('Creating…')
                    : t('Connecting…')
                  : mode === 'new'
                  ? t('Create project')
                  : t('Connect project')}
              </span>
            </button>
          </div>
        </form>
      </div>
    </ActionDialog>
  )
}
