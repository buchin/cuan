import { IconButton } from '../ui/IconButton'
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
    <IconButton
      id={id}
      type="button"
      className="md:w-auto md:gap-2 md:px-3 md:bg-muted/40 text-sm font-medium"
      label={t(label)}
      title={t(label)}
      onClick={onClick}
    >
      <Icon className="h-4 w-4 md:hidden" aria-hidden="true" />
      <Icon className="hidden md:block" aria-hidden="true" />
      <span className="hidden md:inline">{t(label)}</span>
    </IconButton>
  )
}
