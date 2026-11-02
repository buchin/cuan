import { useLanguage } from '../../context/LanguageContext'
import { FolderPlus, Plus, SquarePen, UserPlus } from 'lucide-react'

type CreateActionProps = {
  label: string
  onClick: () => void
  kind?: string
  id?: string
}

export function CreateAction({ label, onClick, kind, id }: CreateActionProps) {
  const { t } = useLanguage()

  const Icon =
    kind === 'inbox'
      ? SquarePen
      : kind === 'contacts'
        ? UserPlus
        : kind === 'projects' && label === 'New project'
          ? FolderPlus
          : Plus
  return (
    <button
      id={id}
      type="button"
      className="inline-flex h-8 w-8 min-h-8 min-w-8 md:h-8 md:w-auto md:px-3 items-center justify-center shrink-0 rounded-none text-muted-foreground hover:text-foreground hover:bg-muted md:bg-primary md:text-primary-foreground md:hover:bg-primary/90 text-xs font-semibold transition-colors p-0"
      aria-label={t(label)}
      title={t(label)}
      onClick={onClick}
    >
      <Icon className="h-4 w-4 md:hidden" aria-hidden="true" />
      <Plus className="hidden h-3.5 w-3.5 md:block" aria-hidden="true" />
      <span className="hidden md:inline">{t(label)}</span>
    </button>
  )
}
