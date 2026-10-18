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
        <div className="h-screen w-screen flex flex-col items-center justify-center bg-[#1a1b26] text-[#c0caf5] p-6 font-mono text-center select-none">
          <div className="max-w-md bg-[#16161e] border border-[#f7768e]/40 p-6 shadow-2xl space-y-4">
            <div className="h-10 w-10 mx-auto bg-[#f7768e]/10 text-[#f7768e] flex items-center justify-center font-bold text-lg">
              !
            </div>
            <h2 className="text-sm font-bold text-[#f7768e]">
              Aplikasi Mengalami Kendala Tampilan
            </h2>
            <p className="text-xs text-[#a9b1d6] leading-relaxed">
              {this.state.error?.message || 'Terjadi kesalahan runtime yang tidak terduga.'}
            </p>
            <div className="pt-2 flex justify-center gap-3">
              <button
                onClick={() => window.location.reload()}
                className="px-4 py-2 bg-[#7aa2f7] text-[#16161e] font-bold text-xs hover:bg-[#89b4fa] transition-colors"
              >
                Muat Ulang Halaman
              </button>
              <button
                onClick={this.handleReset}
                className="px-4 py-2 bg-[#24283b] text-[#a9b1d6] font-semibold text-xs hover:text-white transition-colors border border-[#414868]"
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
