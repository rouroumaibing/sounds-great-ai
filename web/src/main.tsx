import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { ErrorBoundary } from './components/common/ErrorBoundary'
import { tryAutoReload } from './services/update'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </StrictMode>,
)

// This tab is running an older build than the service worker that just took
// over (vite-plugin-pwa autoUpdate swaps in the new precache with
// skipWaiting); reload once to pick it up. Tabs loaded without a controller
// (first visit) skip this — their controllerchange would be the initial
// activation, not an update.
if ('serviceWorker' in navigator && navigator.serviceWorker.controller) {
  let reloaded = false
  navigator.serviceWorker.addEventListener('controllerchange', () => {
    if (!reloaded) {
      reloaded = true
      tryAutoReload()
    }
  })
}
