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
        : kind === 'projects'
          ? FolderPlus
          : Plus
  return (
    <button
      id={id}
      type="button"
      className="action-primary h-11 w-11 shrink-0 p-0 md:w-auto md:px-3"
      aria-label={t(label)}
      title={t(label)}
      onClick={onClick}
    >
      <Icon className="h-5 w-5 md:hidden" aria-hidden="true" />
      <Plus className="hidden h-4 w-4 md:block" aria-hidden="true" />
      <span className="hidden md:inline">{t(label)}</span>
    </button>
  )
}
