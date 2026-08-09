import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ErrorBoundary } from './ErrorBoundary';

describe('ErrorBoundary', () => {
  it('renders children when no error', () => {
    render(
      <ErrorBoundary>
        <div>content</div>
      </ErrorBoundary>
    );
    expect(screen.getByText('content')).toBeInTheDocument();
  });

  it('renders fallback on error', () => {
    function ThrowComponent(): never {
      throw new Error('test error');
    }

    render(
      <ErrorBoundary>
        <ThrowComponent />
      </ErrorBoundary>
    );
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(screen.getByText('test error')).toBeInTheDocument();
  });

  it('renders custom fallback when provided', () => {
    function ThrowComponent(): never {
      throw new Error('custom');
    }

    render(
      <ErrorBoundary fallback={<div>custom fallback</div>}>
        <ThrowComponent />
      </ErrorBoundary>
    );
    expect(screen.getByText('custom fallback')).toBeInTheDocument();
  });
});
