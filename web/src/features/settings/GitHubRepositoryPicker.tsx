import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  FolderGit2,
  GitBranch,
  Search,
  Check,
  ExternalLink,
  Loader2,
  RefreshCw,
  AlertCircle,
} from 'lucide-react'
import { apiFetch } from '../../api/client'
import { useLanguage } from '../../context/LanguageContext'

type Repository = {
  full_name: string
  default_branch: string
  permissions: { push: boolean }
}

interface GitHubAccount {
  login?: string
  avatar_url?: string
}

interface GitHubConnectionResponse {
  connected: boolean
  account?: GitHubAccount
}

export function GitHubRepositoryPicker({
  disabled = false,
  selectedUrl = '',
  onSelect,
}: {
  disabled?: boolean
  selectedUrl?: string
  onSelect: (url: string) => void
}) {
  const { t } = useLanguage()
  const [checkingStatus, setCheckingStatus] = useState(true)
  const [isConnected, setIsConnected] = useState<boolean | null>(null)
  const [account, setAccount] = useState<GitHubAccount | null>(null)
  const [repos, setRepos] = useState<Repository[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [page, setPage] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    async function checkConnection() {
      setCheckingStatus(true)
      try {
        const res = await apiFetch<GitHubConnectionResponse>('/api/v1/integrations/github')
        if (!active) return
        setIsConnected(res.connected)
        if (res.connected) {
          setAccount(res.account || null)
          // Automatically load page 1 of repositories
          void loadRepositories(1, true)
        }
      } catch {
        if (!active) return
        setIsConnected(false)
      } finally {
        if (active) setCheckingStatus(false)
      }
    }
    void checkConnection()
    return () => {
      active = false
    }
  }, [])

  async function loadRepositories(nextPage: number, reset = false) {
    setBusy(true)
    setError('')
    try {
      const result = await apiFetch<{
        repositories: Repository[]
        has_more: boolean
      }>('/api/v1/integrations/github/repositories?page=' + nextPage)
      setRepos((prev) =>
        reset || nextPage === 1
          ? result.repositories
          : [
              ...prev,
              ...result.repositories.filter(
                (item) => !prev.some((p) => p.full_name === item.full_name),
              ),
            ],
      )
      setPage(nextPage)
      setHasMore(result.has_more)
    } catch {
      setError(t('Could not load repositories. Connect GitHub or try again.'))
    } finally {
      setBusy(false)
    }
  }

  const filteredRepos = repos.filter((r) =>
    r.full_name.toLowerCase().includes(searchQuery.trim().toLowerCase()),
  )

  if (checkingStatus) {
    return (
      <div className="flex items-center gap-2 p-3 rounded-xl border border-border/60 bg-muted/20 text-xs text-muted-foreground">
        <Loader2 className="h-3.5 w-3.5 animate-spin text-primary" />
        <span>{t('Checking GitHub connection…')}</span>
      </div>
    )
  }

  if (!isConnected) {
    return (
      <div className="rounded-xl border border-dashed border-border/80 bg-muted/20 p-3.5 space-y-2.5">
        <div className="flex items-start gap-2.5">
          <GitBranch className="h-4 w-4 text-primary shrink-0 mt-0.5" />
          <div className="space-y-1">
            <p className="text-xs font-semibold text-foreground">
              {t('Connect your GitHub account')}
            </p>
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              {t(
                'Connect GitHub in Settings to browse and select repositories directly, or paste a repository URL manually below.',
              )}
            </p>
          </div>
        </div>
        <div className="pt-0.5">
          <Link
            to="/settings/integrations/github"
            className="action-secondary inline-flex items-center gap-1.5 text-xs py-1 px-2.5 rounded-lg"
          >
            <ExternalLink className="h-3 w-3" />
            <span>{t('Connect GitHub in Settings')}</span>
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {/* Header with Connection Status */}
      <div className="flex items-center justify-between text-xs">
        <div className="flex items-center gap-1.5 text-foreground font-medium">
          <span className="flex h-2 w-2 rounded-full bg-emerald-500 ring-2 ring-emerald-500/20" />
          <span>
            {account?.login
              ? `${t('Connected as')} @${account.login}`
              : t('GitHub connected')}
          </span>
        </div>
        <Link
          to="/settings/integrations/github"
          className="text-[11px] text-muted-foreground hover:text-foreground transition-colors inline-flex items-center gap-1"
        >
          <span>{t('Manage')}</span>
          <ExternalLink className="h-3 w-3" />
        </Link>
      </div>

      {/* Interactive Search Box */}
      <div className="relative">
        <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
        <input
          id="github-repository-search"
          type="search"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder={t('Search your GitHub repositories…')}
          className="field-control pl-8 text-xs h-9"
          disabled={disabled || busy}
        />
        {busy && (
          <Loader2 className="absolute right-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 animate-spin text-muted-foreground" />
        )}
      </div>

      {/* Repository List Box */}
      <div
        id="github-repository-picker-list"
        className="rounded-xl border border-border/80 bg-background divide-y divide-border/60 max-h-48 overflow-y-auto shadow-inner"
      >
        {filteredRepos.length > 0 ? (
          filteredRepos.map((repo) => {
            const repoUrl = `https://github.com/${repo.full_name}`
            const cleanSelected = (selectedUrl || '').trim().replace(/\.git$/, '').toLowerCase()
            const isSelected =
              cleanSelected === repoUrl.toLowerCase() ||
              cleanSelected.endsWith('/' + repo.full_name.toLowerCase())

            return (
              <button
                key={repo.full_name}
                type="button"
                disabled={disabled}
                onClick={() => onSelect(repoUrl)}
                className={`w-full text-left p-2.5 flex items-center justify-between gap-2 text-xs transition-colors hover:bg-accent/50 ${
                  isSelected
                    ? 'bg-primary/10 text-primary font-medium'
                    : 'text-foreground'
                }`}
              >
                <div className="flex items-center gap-2 min-w-0">
                  <FolderGit2 className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                  <span className="truncate">{repo.full_name}</span>
                  {repo.permissions?.push === false && (
                    <span className="text-[10px] text-muted-foreground bg-muted px-1.5 py-0.5 rounded shrink-0">
                      {t('read-only')}
                    </span>
                  )}
                </div>
                {isSelected && (
                  <Check className="h-3.5 w-3.5 text-primary shrink-0" />
                )}
              </button>
            )
          })
        ) : (
          <div className="p-3 text-center text-xs text-muted-foreground">
            {searchQuery
              ? t('No repositories match your search.')
              : t('No repositories found.')}
          </div>
        )}
      </div>

      {/* Load More Button or Error */}
      {hasMore && (
        <div className="pt-0.5">
          <button
            type="button"
            onClick={() => void loadRepositories(page + 1)}
            disabled={disabled || busy}
            className="text-xs text-muted-foreground hover:text-foreground inline-flex items-center gap-1 transition-colors"
          >
            <RefreshCw className={`h-3 w-3 ${busy ? 'animate-spin' : ''}`} />
            <span>{t('Load more repositories…')}</span>
          </button>
        </div>
      )}

      {error && (
        <p role="alert" className="text-xs text-destructive flex items-center gap-1 pt-1">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          <span>{error}</span>
          <button
            type="button"
            disabled={disabled || busy}
            className="underline ml-1"
            onClick={() => void loadRepositories(page || 1)}
          >
            {t('Try again')}
          </button>
        </p>
      )}
    </div>
  )
}
