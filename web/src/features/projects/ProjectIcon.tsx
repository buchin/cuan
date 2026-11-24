import { Folder, FolderGit2 } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'

export function ProjectIcon({ repoUrl }: { repoUrl?: string }) {
  const { t } = useLanguage()
  const hasRepo = Boolean(repoUrl && repoUrl.trim())
  const label = hasRepo ? 'Repository' : 'Project without a repository'
  return (
    <span role="img" aria-label={t(label)} title={t(label)} className="relative flex h-8 w-8 shrink-0 items-center justify-center text-muted-foreground">
      {hasRepo ? (
        <FolderGit2 aria-hidden="true" className="h-4.5 w-4.5 text-muted-foreground" />
      ) : (
        <Folder aria-hidden="true" className="h-4 w-4 text-muted-foreground" />
      )}
    </span>
  )
}
