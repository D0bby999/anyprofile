// SPDX-License-Identifier: AGPL-3.0-or-later
import { Fragment, useEffect, useRef, useState } from 'react'
import { Api, copyText, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Icon } from './icons'
import { focusFirstItem, menuArrows } from '../lib/menu-keys'
import { siteById, type Site } from '../lib/sites'
import { useCookieActions } from './use-cookie-actions'

type Props = {
  row: service.ProfileRow
  sites: Site[]
  onEdit: () => void
  onDuplicate: () => void
  onTrash: () => void
  onMessage: (msg: string) => void
  onError: (msg: string) => void
}

/** The "⋯" menu on each row: edit, duplicate, account copies and trash. */
export function RowMenu({ row, sites, onEdit, onDuplicate, onTrash, onMessage, onError }: Props) {
  const [open, setOpen] = useState(false)
  const cookies = useCookieActions(row, sites)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  const act = (fn: () => void | Promise<void>) => {
    setOpen(false)
    void fn()
  }

  // One set of copy items per platform with an account; named by platform
  // when the profile has more than one.
  const accounts = (row.accounts ?? []).filter((a) => row.sites.includes(a.site) && (a.login || a.hasPassword || a.hasTotp))
  const named = (label: string, site: string) => accounts.length > 1 ? `${label} · ${siteById(sites, site).name}` : label

  const copyPassword = async (site: string) => {
    try {
      await Api.CopyAccountPassword(row.id, site)
      onMessage(t.account.copiedPassword)
    } catch (e) {
      onError(errorText(e))
    }
  }

  const copyCode = async (site: string) => {
    try {
      const c = await Api.TOTPCode(row.id, site)
      await copyText(c.code)
      onMessage(t.menu.copiedCode(c.secondsLeft))
    } catch (e) {
      onError(errorText(e))
    }
  }

  const importBookmarks = async () => {
    try {
      const n = await Api.ImportBookmarks(row.id, t.bookmarks.folder)
      if (n) onMessage(t.bookmarks.imported(n))
    } catch (e) {
      onError(errorText(e))
    }
  }

  const copyLogin = async (login: string) => {
    try {
      await copyText(login)
      onMessage(t.menu.copiedLogin)
    } catch (e) {
      onError(errorText(e))
    }
  }

  return (
    <div className="rowmenu-wrap" ref={ref}>
      <button type="button" className="iconbtn sm" aria-haspopup="menu" aria-expanded={open} aria-label={t.menu.more}
        onClick={() => setOpen(!open)}>
        <Icon name="more" />
      </button>
      {open && (
        <div role="menu" className="rowmenu" ref={focusFirstItem} onKeyDown={(e) => { menuArrows(e); if (e.key === 'Escape') { e.stopPropagation(); setOpen(false) } }}>
          <button role="menuitem" onClick={() => act(onEdit)}>{t.menu.edit}</button>
          <button role="menuitem" onClick={() => act(onDuplicate)}>{t.menu.duplicate}</button>
          <button role="menuitem" onClick={() => act(() => Api.SetPinnedMany([row.id], !row.pinned).catch((e) => onError(errorText(e))))}>
            {row.pinned ? t.menu.unpin : t.menu.pin}
          </button>
          {accounts.map((a) => (
            <Fragment key={a.site}>
              {a.login && <button role="menuitem" onClick={() => act(() => copyLogin(a.login))}>{named(t.menu.copyLogin, a.site)}</button>}
              {a.hasPassword && <button role="menuitem" onClick={() => act(() => copyPassword(a.site))}>{named(t.menu.copyPassword, a.site)}</button>}
              {a.hasTotp && <button role="menuitem" onClick={() => act(() => copyCode(a.site))}>{named(t.menu.copyCode, a.site)}</button>}
            </Fragment>
          ))}
          <hr />
          <button role="menuitem" disabled={cookies.importing} onClick={() => act(cookies.importCookies)}>{t.cookies.import}</button>
          <button role="menuitem" disabled={!cookies.canExport} title={cookies.canExport ? undefined : t.cookies.openFirst}
            onClick={() => act(cookies.exportCookies)}>{t.cookies.export}</button>
          <button role="menuitem" disabled={row.state !== 'stopped'} title={row.state !== 'stopped' ? t.bookmarks.closeFirst : undefined}
            onClick={() => act(importBookmarks)}>{t.bookmarks.import}</button>
          <hr />
          <button role="menuitem" disabled={row.state !== 'stopped'} onClick={() => act(onTrash)}>{t.menu.trash}</button>
        </div>
      )}
    </div>
  )
}
