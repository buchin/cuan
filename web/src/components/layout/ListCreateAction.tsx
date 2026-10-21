import { CreateAction } from './CreateAction'

// Desktop list actions live with their list, above its search and scroll area.
// The app header owns the same action below the secondary-sidebar breakpoint.
export function ListCreateAction({ label, onClick, kind }: {
  label: string
  onClick?: () => void
  kind: string
}) {
  if (!onClick) return null
  return <div className="hidden pb-3 min-[1200px]:flex [&>button]:w-full [&>button]:justify-start">
    <CreateAction label={label} onClick={onClick} kind={kind} />
  </div>
}
