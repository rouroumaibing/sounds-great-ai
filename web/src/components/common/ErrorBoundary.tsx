import { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';
import { isStaleChunkError, tryAutoReload } from '../../services/update';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error('ErrorBoundary caught:', error, errorInfo);
    // A lazy-route chunk that vanished after a redeploy only recovers with a
    // fresh page load: React caches the rejected import, so the in-place
    // "Try again" button cannot re-fetch it.
    if (isStaleChunkError(error)) tryAutoReload();
  }

  render(): ReactNode {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }
      return (
        <div style={{ padding: '2rem', textAlign: 'center' }}>
          <h2>Something went wrong</h2>
          <p style={{ color: '#666', marginTop: '0.5rem' }}>
            {this.state.error?.message ?? 'Unknown error'}
          </p>
          <button
            onClick={() => this.setState({ hasError: false, error: null })}
            style={{
              marginTop: '1rem',
              padding: '0.5rem 1rem',
              cursor: 'pointer',
              border: '1px solid #ccc',
              borderRadius: '4px',
              background: 'transparent',
            }}
          >
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
