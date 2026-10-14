// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import type { Site } from '../lib/sites'
import { useConfirm } from './confirm'
import { useToast } from './toast'

/** Import and export of a profile's cookies, with the toasts and the export warning. */
export function useCookieActions(row: service.ProfileRow, sites: Site[]) {
  const toast = useToast()
  const confirm = useConfirm()
  const [importing, setImporting] = useState(false)
  const canExport = row.state === 'running'

  const importCookies = async () => {
    setImporting(true)
    try {
      const res = await Api.ImportCookies(row.id)
      if (!res || !res.imported && !res.skipped) return // cancelled
      const domains = res.domains ?? []
      toast.success(t.cookies.imported(res.imported, domains.slice(0, 2).join(', '), Math.max(0, domains.length - 2), res.skipped))
      if (res.siteMismatch) {
        const site = row.sites.map((id) => sites.find((s) => s.id === id)?.name ?? id).join(', ')
        toast.info(t.cookies.mismatch(res.siteMismatch, site))
      }
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setImporting(false)
    }
  }

  const exportCookies = async () => {
    if (!canExport || !(await confirm(t.confirm.exportCookies))) return
    try {
      const n = await Api.ExportCookies(row.id)
      if (n > 0) toast.success(t.cookies.exported(n))
    } catch (e) {
      toast.error(errorText(e))
    }
  }

  return { importCookies, exportCookies, importing, canExport }
}
