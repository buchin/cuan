import { Component, ErrorInfo, ReactNode } from 'react'

interface Props { children: ReactNode }
interface State { error: Error | null }

export class ErrorBoundary extends Component<Props, State> {
  public state: State = { error: null }

  public static getDerivedStateFromError(error: Error): State { return { error } }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Uncaught error in React tree:', error, errorInfo)
  }

  public render() {
    if (!this.state.error) return this.props.children
    let indonesian = false
    try { indonesian = localStorage.getItem('cuan-language') === 'id' } catch { /* Use the default language. */ }
    return <main role="alert" className="flex min-h-dvh items-center justify-center bg-background p-6 text-center text-foreground">
      <div className="max-w-sm space-y-4">
        <h1 className="text-base font-semibold">{indonesian ? 'Halaman belum bisa dibuka' : 'Could not open this page'}</h1>
        <button type="button" onClick={() => window.location.reload()} className="action-secondary min-h-11 px-4 text-sm">{indonesian ? 'Muat ulang halaman' : 'Reload page'}</button>
      </div>
    </main>
  }
}
