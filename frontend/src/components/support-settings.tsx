// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Icon } from './icons'
import { useToast } from './toast'

/** Settings → Help: copies a redacted report for a bug report. */
export function SupportSettings() {
  const [error, setError] = useState('')
  const toast = useToast()
  const copy = async () => {
    setError('')
    try { await Api.CopyDiagnostics(); toast.success(t.support.copied) } catch (e) { setError(errorText(e)) }
  }
  return (
    <div className="field">
      <span className="row">
        <button type="button" className="btn sm" onClick={copy}><Icon name="copy" className="icon sm" />{t.support.copy}</button>
      </span>
      <span className="hint">{t.support.hint}</span>
      {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
    </div>
  )
}
