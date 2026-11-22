import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

/**
 * ErrorBoundary menangkap error render di subtree React agar aplikasi tidak
 * menampilkan layar kosong. Detail error hanya dicatat ke console.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Kesalahan render tidak tertangani:', error, info.componentStack)
  }

  private handleReset = () => {
    this.setState({ error: null })
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex min-h-screen flex-col items-center justify-center gap-4 px-4 text-center">
          <p className="text-5xl font-bold text-destructive">500</p>
          <h1 className="text-xl font-semibold text-foreground">Terjadi kesalahan</h1>
          <p className="max-w-sm text-sm text-muted-foreground">
            Maaf, ada gangguan teknis. Coba muat ulang halaman; bila masih bermasalah, hubungi
            kami.
          </p>
          <div className="flex flex-wrap justify-center gap-2">
            <button
              type="button"
              onClick={() => window.location.reload()}
              className="tap-target inline-flex items-center rounded-lg bg-primary px-4 text-sm font-medium text-background hover:bg-primary/90"
            >
              Muat ulang
            </button>
            <button
              type="button"
              onClick={this.handleReset}
              className="tap-target inline-flex items-center rounded-lg px-4 text-sm font-medium text-foreground ring-1 ring-border hover:bg-muted"
            >
              Coba lagi
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
