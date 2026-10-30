#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later

"""Local macOS candidate smoke/infobar gates using ONLY disposable mock-keychain profiles.

Not a real Keychain, fingerprint, notarization, or distribution acceptance test.
Never pass these diagnostic switches to production profiles or ground-truth capture.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import signal
import subprocess
import tempfile
import time
from urllib.parse import quote

CANARY = "AP_MACOS_GATE_CANARY"
MISSING_KEYS = re.compile(r"(?:API keys?[^\n]*missing|missing[^\n]*API keys?)", re.I)
GOOGLE_ENV = ("GOOGLE_API_KEY", "GOOGLE_DEFAULT_CLIENT_ID", "GOOGLE_DEFAULT_CLIENT_SECRET")
AX_SCRIPT = '''tell application "System Events"
set processMatches to every process whose unix id is PID_NUMBER
if (count of processMatches) is 0 then return "WINDOWS=0"
set targetProcess to item 1 of processMatches
set windowCount to count of windows of targetProcess
if windowCount is 0 then return "WINDOWS=0"
set collectedItems to {"WINDOWS=" & windowCount, "TITLE=" & (name of window 1 of targetProcess as text)}
set windowElements to entire contents of window 1 of targetProcess
set end of collectedItems to "ELEMENTS=" & (count of windowElements)
repeat with el in windowElements
set elementText to ""
try
set elementText to role of el as text
end try
try
set elementText to elementText & " | " & (name of el as text)
end try
try
set elementText to elementText & " | " & (description of el as text)
end try
try
set elementText to elementText & " | " & (value of el as text)
end try
set end of collectedItems to elementText
end repeat
set AppleScript's text item delimiters to linefeed
return collectedItems as text
end tell'''


def clean_env() -> dict[str, str]:
    return {key: value for key, value in os.environ.items() if key not in GOOGLE_ENV}


def digest(path: Path) -> str:
    hasher = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            hasher.update(block)
    return hasher.hexdigest()


def identify(app: Path) -> dict:
    app = app.resolve(strict=True)
    binary = app / "Contents/MacOS/Chromium"
    framework_dir = app / "Contents/Frameworks/Chromium Framework.framework"
    framework = framework_dir / "Chromium Framework"
    if not binary.is_file() or not framework.is_file():
        raise ValueError(f"not a Chromium bundle: {app}")
    return {"app": str(app), "binary": str(binary), "core_sha256": {
        "binary": digest(binary), "framework": digest(framework)}}


def command(binary: str, profile: str, headless: bool) -> list[str]:
    page = "data:text/html," + quote(f"<title>{CANARY}</title><body>{CANARY}</body>")
    args = [binary, f"--user-data-dir={profile}", "--no-first-run",
            "--no-default-browser-check", "--use-mock-keychain"]
    if headless:
        args += ["--headless=new", "--dump-dom"]
    else:
        args += ["--window-position=80,80", "--window-size=900,700"]
    return args + [page]


def stop_owned(process: subprocess.Popen) -> dict:
    """Only terminate the process group created by this invocation, never pkill."""
    result = {"forced": False}
    # The browser may exit before a helper in its group. Kill only our private
    # session/group even in that case, so no test helper survives root exit.
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    try:
        process.wait(timeout=10)
    except subprocess.TimeoutExpired:
        result["forced"] = True
        os.killpg(process.pid, signal.SIGKILL)
        process.wait(timeout=5)
    result["exit"] = process.returncode
    return result


def evaluate_headless(exit_code: int, stdout: str, stderr: str, timed_out: bool) -> str:
    return "PASS" if (not timed_out and exit_code == 0 and CANARY in stdout
                      and "watchdog expired" not in stderr.lower()) else "FAIL"


def evaluate_ax(text: str) -> dict:
    def number(key: str) -> int:
        match = re.search(rf"^{key}=(\d+)$", text, re.M)
        return int(match[1]) if match else 0
    ready = (number("WINDOWS") > 0 and number("ELEMENTS") > 0
             and re.search(rf"^TITLE=.*{CANARY}", text, re.M) is not None)
    return {"ready": ready, "windows": number("WINDOWS"),
            "elements": number("ELEMENTS"), "missing_keys_warning": bool(MISSING_KEYS.search(text))}


def evaluate_infobar(control: dict, candidate: dict) -> str:
    if not control.get("ready") or not candidate.get("ready"):
        return "BLOCKED"
    # An empty/partial tree must never prove absence. The positive control must
    # actually expose the exact warning under the same launch/AX conditions.
    if not control.get("missing_keys_warning"):
        return "BLOCKED"
    return "FAIL" if candidate.get("missing_keys_warning") else "PASS"


def headless_case(artifact: dict, label: str, out: Path, timeout: float) -> dict:
    with tempfile.TemporaryDirectory(prefix="ap-headless-gate-") as profile:
        started = time.monotonic()
        process = subprocess.Popen(command(artifact["binary"], profile, True),
                                   env=clean_env(), stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, start_new_session=True)
        timed_out = False
        cleanup = None
        try:
            stdout, stderr = process.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            timed_out = True
            cleanup = stop_owned(process)
            stdout, stderr = process.communicate(timeout=5)
        finally:
            if cleanup is None:
                cleanup = stop_owned(process)
        stdout, stderr = stdout.decode(errors="replace"), stderr.decode(errors="replace")
        (out / f"{label}-headless.stdout").write_text(stdout)
        (out / f"{label}-headless.stderr").write_text(stderr)
        return {"status": evaluate_headless(process.returncode, stdout, stderr, timed_out),
                "exit": process.returncode, "timeout": timed_out,
                "seconds": round(time.monotonic() - started, 3), "cleanup": cleanup}


def native_case(artifact: dict, label: str, out: Path, timeout: float) -> dict:
    with tempfile.TemporaryDirectory(prefix="ap-native-gate-") as profile:
        with (out / f"{label}-native.stderr").open("w") as stderr:
            process = subprocess.Popen(command(artifact["binary"], profile, False),
                                       env=clean_env(), stdout=subprocess.DEVNULL,
                                       stderr=stderr, start_new_session=True)
            text, error = "", ""
            observed = {"ready": False}
            ax_exit = None
            ready_since = None
            deadline = time.monotonic() + timeout
            try:
                while time.monotonic() < deadline and process.poll() is None:
                    try:
                        result = subprocess.run(
                            ["osascript", "-e", AX_SCRIPT.replace("PID_NUMBER", str(process.pid))],
                            capture_output=True, text=True,
                            timeout=max(0.1, min(8, deadline - time.monotonic())))
                    except subprocess.TimeoutExpired:
                        error = "Accessibility query timed out"
                        break
                    text, error, ax_exit = result.stdout, result.stderr, result.returncode
                    observed = evaluate_ax(text)
                    if ax_exit:
                        observed["ready"] = False
                        break
                    if observed["ready"]:
                        if ready_since is None:
                            ready_since = time.monotonic()
                        # Do not infer absence from the first, possibly partial,
                        # startup tree. Control must actually expose the warning.
                        positive = label != "control" or observed["missing_keys_warning"]
                        if positive and time.monotonic() - ready_since >= 1:
                            break
                    else:
                        ready_since = None
                    time.sleep(0.25)
                exited_before_cleanup = process.poll() is not None
                if time.monotonic() >= deadline or exited_before_cleanup:
                    observed["ready"] = False
            finally:
                cleanup = stop_owned(process)
            (out / f"{label}-native-ax.txt").write_text(text)
            (out / f"{label}-native-ax.error").write_text(error)
            if error or ax_exit != 0 or cleanup["forced"]:
                observed["ready"] = False
            return {**observed, "error": error, "ax_exit": ax_exit, "cleanup": cleanup}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate-app", required=True, type=Path)
    parser.add_argument("--control-app", type=Path, help="pre-0159 positive-control bundle; required for infobar")
    parser.add_argument("--mode", choices=("headless", "infobar", "all"), default="all")
    parser.add_argument("--output-dir", required=True, type=Path, help="new evidence directory; refuses overwrite")
    parser.add_argument("--timeout", default=20.0, type=float)
    args = parser.parse_args()
    if platform.system() != "Darwin":
        parser.error("native macOS host required")
    if not 1 <= args.timeout <= 120:
        parser.error("timeout must be between 1 and 120 seconds")
    if args.mode != "headless" and args.control_app is None:
        parser.error("infobar requires a pre-0159 positive control; absence alone is not PASS")
    artifacts = {"candidate": identify(args.candidate_app)}
    if args.control_app:
        artifacts["control"] = identify(args.control_app)
    args.output_dir.mkdir(parents=True, exist_ok=False)
    report = {"status": "NOT_RUN", "keychain": "mock-test-only",
              "not_proven": ["real Keychain", "ground-truth fingerprint", "signed release",
                             "notarization", "full candidate acceptance"],
              "artifacts": artifacts, "headless": {}, "native": {}}
    try:
        if args.mode in ("headless", "all"):
            for label, artifact in artifacts.items():
                report["headless"][label] = headless_case(artifact, label, args.output_dir, args.timeout)
        if args.mode in ("infobar", "all"):
            for label in ("control", "candidate"):
                report["native"][label] = native_case(artifacts[label], label, args.output_dir, args.timeout)
            report["infobar_status"] = evaluate_infobar(report["native"]["control"],
                                                       report["native"]["candidate"])
        statuses = [case["status"] for case in report["headless"].values()]
        if "infobar_status" in report:
            statuses.append(report["infobar_status"])
        for label, artifact in artifacts.items():
            if identify(Path(artifact["app"]))["core_sha256"] != artifact["core_sha256"]:
                raise RuntimeError(f"{label} core changed during the run; invalidate evidence")
        report["status"] = "PASS" if statuses and all(s == "PASS" for s in statuses) else "FAIL"
        if "BLOCKED" in statuses and "FAIL" not in statuses:
            report["status"] = "BLOCKED"
    except Exception as error:
        report["status"] = "FAIL"
        report["error"] = f"{type(error).__name__}: {error}"
    (args.output_dir / "result.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({"status": report["status"], "report": str(args.output_dir / "result.json"),
                      "keychain": report["keychain"]}))
    return 0 if report["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
