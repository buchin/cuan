import { Component, ErrorInfo, ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
  }

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Uncaught error in React tree:', error, errorInfo)
  }

  private handleReset = () => {
    localStorage.clear()
    window.location.href = '/'
  }

  public render() {
    if (this.state.hasError) {
      return (
        <div className="h-screen w-screen flex flex-col items-center justify-center bg-background text-foreground p-6 font-mono text-center select-none">
          <div className="max-w-md bg-card border border-destructive/40 p-6 shadow-2xl space-y-4 rounded-lg">
            <div className="h-10 w-10 mx-auto bg-destructive/10 text-destructive flex items-center justify-center font-bold text-lg rounded-full">
              !
            </div>
            <h2 className="text-sm font-bold text-destructive">
              Aplikasi Mengalami Kendala Tampilan
            </h2>
            <p className="text-xs text-muted-foreground leading-relaxed">
              {this.state.error?.message || 'Terjadi kesalahan runtime yang tidak terduga.'}
            </p>
            <div className="pt-2 flex justify-center gap-3">
              <button
                onClick={() => window.location.reload()}
                className="px-4 py-2 bg-primary text-primary-foreground font-bold text-xs hover:bg-primary/90 transition-colors rounded"
              >
                Muat Ulang Halaman
              </button>
              <button
                onClick={this.handleReset}
                className="px-4 py-2 bg-secondary text-secondary-foreground font-semibold text-xs hover:bg-secondary/80 transition-colors border border-border rounded"
              >
                Reset Sesi
              </button>
            </div>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
