import React from 'react'
import ReactDOM from 'react-dom/client'
import './styles/globals.css'
import 'goey-toast/styles.css'
import App from './App'
import { ErrorBoundary } from './components/ErrorBoundary'
import { applyTheme, getThemeMode } from './lib/theme'

applyTheme(getThemeMode())

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <ErrorBoundary><App /></ErrorBoundary>
  </React.StrictMode>,
)
