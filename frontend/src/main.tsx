import React, { useEffect, useReducer } from 'react'
import ReactDOM from 'react-dom/client'
import './styles/globals.css'
import 'goey-toast/styles.css'
import App from './App'
import { ErrorBoundary } from './components/ErrorBoundary'
import { applyTheme, getThemeMode } from './lib/theme'
import { subscribeLanguage } from './i18n'
import { animateLanguageText } from './lib/languageText'
import { restoreWindowState, watchWindowState } from './lib/windowState'
import { restoreZoom } from './lib/zoom'

applyTheme(getThemeMode())

function Root() {
  const [, rerender] = useReducer((value) => value + 1, 0)
  useEffect(() => {
    void restoreWindowState()
    void restoreZoom()
    const stopWatchingWindow = watchWindowState()
    return () => { stopWatchingWindow() }
  }, [])
  useEffect(() => {
    const off = subscribeLanguage(() => {
      rerender()
      requestAnimationFrame(() => requestAnimationFrame(animateLanguageText))
    })
    return () => { off() }
  }, [])
  return <ErrorBoundary><App /></ErrorBoundary>
}

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>,
)
