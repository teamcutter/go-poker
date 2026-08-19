import { Component } from 'react'
import type { ErrorInfo, ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error('App crashed', error, info)
  }

  render(): ReactNode {
    if (this.state.error) {
      return (
        <main className="center" style={{ justifyContent: 'center', textAlign: 'center' }}>
          <div className="login-mark">⚠</div>
          <h1 style={{ fontSize: 21, marginBottom: 6 }}>Something broke</h1>
          <p className="muted" style={{ fontSize: 14, maxWidth: 280, marginTop: 0 }}>
            {this.state.error.message}
          </p>
          <button className="btn btn-secondary mt-12" onClick={() => this.setState({ error: null })}>
            Try again
          </button>
        </main>
      )
    }
    return this.props.children
  }
}
