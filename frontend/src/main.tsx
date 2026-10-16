// SPDX-License-Identifier: AGPL-3.0-or-later
import React from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource-variable/inter'
import './styles/tokens.css'
import './styles/base.css'
import './styles/shell.css'
import App from './App'
import { ToastProvider } from './components/toast'
import { ConfirmProvider } from './components/confirm'
import { Api } from './api'
import { setLanguage } from './i18n'
import { isWindows } from './lib/platform'

// Windows draws its own title bar above the page; no room for traffic lights.
if (isWindows) document.documentElement.classList.add('win')

const root = createRoot(document.getElementById('root')!)
Api.Language().then(setLanguage).catch(() => setLanguage('en')).finally(() => root.render(
  <React.StrictMode>
    <ToastProvider>
      <ConfirmProvider>
        <App />
      </ConfirmProvider>
    </ToastProvider>
  </React.StrictMode>,
))
