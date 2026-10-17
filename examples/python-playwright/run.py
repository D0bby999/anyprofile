# SPDX-License-Identifier: 0BSD
"""AnyProfile + Playwright (Python) starter. Put your steps in task(); the rest
opens the profile through the local API, connects to its browser and closes it.

    ANYPROFILE_TOKEN=… python run.py "<profile id, exact name or #serial>" [url]
"""
import json
import os
import sys
import urllib.error
import urllib.request

from playwright.sync_api import sync_playwright

PORT = os.environ.get("ANYPROFILE_PORT", "51525")
TOKEN = os.environ.get("ANYPROFILE_TOKEN", "")
API = f"http://127.0.0.1:{PORT}/v1"


def call(method, path, body=None):
    req = urllib.request.Request(API + path, method=method, data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req) as r:
            return json.load(r)
    except urllib.error.HTTPError as e:
        err = json.load(e).get("error", {})
        raise RuntimeError(f"{method} {path}: {err.get('code')}: {err.get('message')}") from None


def find_profile(ref):
    """A profile by id, exact name (any case) or #serial, as AnyProfile finds them."""
    serial = ref.lstrip("#")
    hits = [p for p in call("GET", "/profiles")["profiles"]
            if p["id"] == ref or str(p["serial"]) == serial or p["name"].lower() == ref.lower()]
    if len(hits) != 1:
        raise RuntimeError(f'{len(hits)} profiles match "{ref}"; use the id' if hits else f'no profile "{ref}"')
    return hits[0]


def task(page, url):
    """Your steps. Wait for elements or load states, never for a fixed time."""
    page.goto(url, wait_until="domcontentloaded")
    return {"title": page.title(), "url": page.url}


def main():
    if not TOKEN or len(sys.argv) < 2:
        sys.exit('usage: ANYPROFILE_TOKEN=… python run.py "<profile>" [url]')
    url = sys.argv[2] if len(sys.argv) > 2 else "https://www.youtube.com/"
    profile = find_profile(sys.argv[1])
    was_open = profile["state"] == "running"
    # A profile open without DevTools is refused (api_running_without_devtools): close it first.
    opened = call("POST", f"/profiles/{profile['id']}/start", {"devtools": True})
    try:
        with sync_playwright() as pw:
            browser = pw.chromium.connect_over_cdp(opened["webSocketDebuggerUrl"])
            context = browser.contexts[0]  # the profile's own context, with its cookies
            page = context.pages[0] if context.pages else context.new_page()
            # No set_viewport_size or emulation: a changed viewport no longer matches the fingerprint.
            print(json.dumps(task(page, url), ensure_ascii=False))
    finally:
        # AnyProfile closes the browser itself, so the profile's session is saved properly.
        if not was_open:
            call("POST", f"/profiles/{profile['id']}/stop")


if __name__ == "__main__":
    try:
        main()
    except RuntimeError as e:
        sys.exit(str(e))
