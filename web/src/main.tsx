import { installExampleExtension } from './extensions/example'
import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { NotificationsProvider } from './context/NotificationsContext'
import { PWAProvider } from './context/PWAContext'
import { LanguageProvider } from './context/LanguageContext'
import { ThemeProvider } from './context/ThemeContext'
import { AuthProvider } from './context/AuthContext'
import { ErrorBoundary } from './components/ui/ErrorBoundary'
import './index.css'

if (import.meta.env.VITE_CUAN_EXAMPLE_EXTENSIONS === '1')
  installExampleExtension()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <LanguageProvider>
        <ThemeProvider>
          <AuthProvider>
            <BrowserRouter>
              <NotificationsProvider>
                <PWAProvider>
                  <App />
                </PWAProvider>
              </NotificationsProvider>
            </BrowserRouter>
          </AuthProvider>
        </ThemeProvider>
      </LanguageProvider>
    </ErrorBoundary>
  </React.StrictMode>,
)
