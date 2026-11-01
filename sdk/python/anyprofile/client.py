# SPDX-License-Identifier: AGPL-3.0-or-later
"""Thin client for AnyProfile's local automation API. The app does the
waiting; this file only speaks JSON."""
from __future__ import annotations

import json
import os
import urllib.request
from typing import Any


class Target(dict):
    """A dict with helpers: Target(text="Lưu"), Target(role="button", name="Save")."""

    def __init__(self, **kw: Any) -> None:
        super().__init__(**{k: v for k, v in kw.items() if v is not None})


class Session:
    def __init__(self, profile: str | None = None, device: str | None = None,
                 base: str | None = None, token: str | None = None) -> None:
        if not profile and not device:
            raise ValueError("give profile or device")
        self.profile, self.device = profile, device
        self.base = (base or os.environ.get("ANYPROFILE_API") or "http://127.0.0.1:51525").rstrip("/")
        self.token = token or os.environ.get("ANYPROFILE_TOKEN", "")

    def act(self, action: str, **args: Any) -> Any:
        body = {"action": action, **args}
        if self.profile:
            body["profile"] = self.profile
        if self.device:
            body["device"] = self.device
        req = urllib.request.Request(
            f"{self.base}/v1/act",
            data=json.dumps(body).encode(),
            headers={"Content-Type": "application/json", "Authorization": f"Bearer {self.token}"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=120) as res:
                data = json.load(res)
        except urllib.error.HTTPError as e:
            err = json.load(e)
            raise RuntimeError(f"{err['error']['code']}: {err['error']['message']}") from None
        return data.get("result")

    def goto(self, url: str) -> Any:
        return self.act("navigate", url=url)

    def find(self, target: Target, limit: int = 10) -> list[dict]:
        return self.act("find", target=dict(target), limit=limit)

    def click(self, target: Target | str) -> Any:
        return self.act("click", **({"ref": target} if isinstance(target, str) else {"target": dict(target)}))

    def tap(self, target: Target | str) -> Any:
        return self.act("tap", **({"ref": target} if isinstance(target, str) else {"target": dict(target)}))

    def type(self, target: Target, text: str, submit: bool = False) -> Any:
        return self.act("type", target=dict(target), text=text, submit=submit)

    def read(self, target: Target | None = None) -> str:
        return self.act("read", **({"target": dict(target)} if target else {}))

    def screenshot(self) -> dict:
        return self.act("screenshot")

    def close(self) -> None:
        """Nothing is leased yet (phase 5); safe to omit."""
