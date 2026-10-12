// SPDX-License-Identifier: AGPL-3.0-or-later
import React from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource-variable/inter'
import './styles/tokens.css'
import './styles/base.css'
import './styles/shell.css'
import App from './App'
import { Api } from './api'
import { setLanguage } from './i18n'

const root = createRoot(document.getElementById('root')!)
Api.Language().then(setLanguage).catch(() => setLanguage('en')).finally(() => root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
))
