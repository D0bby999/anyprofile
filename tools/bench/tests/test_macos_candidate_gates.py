# SPDX-License-Identifier: AGPL-3.0-or-later

"""Pure fixture tests: no browser, no Keychain, no Accessibility permissions needed."""
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location(
    "macos_candidate_gates", Path(__file__).resolve().parents[1] / "macos-candidate-gates.py")
gates = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(gates)


def tree(warning=False):
    return (f"WINDOWS=1\nTITLE={gates.CANARY} - Chromium\nELEMENTS=35\n"
            "AXButton | Reload\n" + ("AXStaticText | Google API keys are missing.\n" if warning else ""))


class HeadlessTests(unittest.TestCase):
    def test_marker_and_zero_exit_pass(self):
        self.assertEqual(gates.evaluate_headless(0, gates.CANARY, "", False), "PASS")

    def test_marker_does_not_hide_bad_exit(self):
        self.assertEqual(gates.evaluate_headless(2, gates.CANARY, "", False), "FAIL")

    def test_zero_exit_without_marker_fails(self):
        self.assertEqual(gates.evaluate_headless(0, "", "", False), "FAIL")

    def test_watchdog_or_timeout_fails(self):
        self.assertEqual(gates.evaluate_headless(0, gates.CANARY, "Teardown watchdog expired", False), "FAIL")
        self.assertEqual(gates.evaluate_headless(0, gates.CANARY, "", True), "FAIL")


class AccessibilityTests(unittest.TestCase):
    def test_zero_windows_is_not_ready(self):
        self.assertFalse(gates.evaluate_ax("WINDOWS=0\n")["ready"])

    def test_other_window_or_empty_tree_is_not_ready(self):
        self.assertFalse(gates.evaluate_ax("WINDOWS=1\nTITLE=Other app\nELEMENTS=4")["ready"])
        self.assertFalse(gates.evaluate_ax(f"WINDOWS=1\nTITLE={gates.CANARY}\nELEMENTS=0")["ready"])

    def test_control_required_for_absence_proof(self):
        clean = gates.evaluate_ax(tree())
        self.assertEqual(gates.evaluate_infobar(clean, clean), "BLOCKED")
        self.assertEqual(gates.evaluate_infobar({"ready": False}, clean), "BLOCKED")

    def test_exact_positive_control_and_clean_candidate_pass(self):
        self.assertEqual(gates.evaluate_infobar(gates.evaluate_ax(tree(True)),
                                               gates.evaluate_ax(tree())), "PASS")

    def test_warning_on_candidate_fails(self):
        observed = gates.evaluate_ax(tree(True))
        self.assertEqual(gates.evaluate_infobar(observed, observed), "FAIL")

    def test_generic_missing_value_is_not_api_warning(self):
        self.assertFalse(gates.evaluate_ax(tree() + "AXGroup | missing value")["missing_keys_warning"])

    def test_reverse_word_order_warning(self):
        self.assertTrue(gates.MISSING_KEYS.search("You are missing API keys"))

    def test_ax_query_materializes_exact_pid_matches(self):
        self.assertIn("every process whose unix id is PID_NUMBER", gates.AX_SCRIPT)
        self.assertIn("item 1 of processMatches", gates.AX_SCRIPT)
        self.assertNotIn("first process whose", gates.AX_SCRIPT)


class SafetyTests(unittest.TestCase):
    def test_test_flags_and_profile_are_explicit(self):
        for headless in (True, False):
            args = gates.command("/fixture/Chromium", "/fixture/owned-profile", headless)
            self.assertIn("--user-data-dir=/fixture/owned-profile", args)
            self.assertIn("--use-mock-keychain", args)
            self.assertEqual("--headless=new" in args, headless)
            self.assertNotIn("--no-sandbox", args)
            self.assertNotIn("--disable-gpu", args)
            self.assertNotIn("--test-type", args)

    def test_google_keys_not_inherited(self):
        with mock.patch.dict(os.environ, {"GOOGLE_API_KEY": "canary", "PATH": "/fixture"}, clear=True):
            self.assertEqual(gates.clean_env(), {"PATH": "/fixture"})

    def test_cleanup_only_own_process_group(self):
        process = mock.Mock(pid=12345, returncode=0)
        with mock.patch.object(gates.os, "killpg") as kill:
            result = gates.stop_owned(process)
        kill.assert_called_once_with(12345, signal.SIGTERM)
        self.assertFalse(result["forced"])

    def test_timeout_cleanup_is_explicit(self):
        process = mock.Mock(pid=12345, returncode=-9)
        process.wait.side_effect = [subprocess.TimeoutExpired("fixture", 10), -9]
        with mock.patch.object(gates.os, "killpg") as kill:
            result = gates.stop_owned(process)
        self.assertEqual(kill.call_args_list, [mock.call(12345, signal.SIGTERM),
                                              mock.call(12345, signal.SIGKILL)])
        self.assertTrue(result["forced"])


class RunnerTests(unittest.TestCase):
    def test_changed_core_invalidates_even_successful_smoke(self):
        original = {"app": "/fixture/Chromium.app", "core_sha256": {"binary": "before"}}
        changed = {"app": "/fixture/Chromium.app", "core_sha256": {"binary": "after"}}
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp) / "run"
            args = ["gate", "--candidate-app", "/fixture/Chromium.app", "--mode", "headless",
                    "--output-dir", str(out)]
            with mock.patch("sys.argv", args), mock.patch.object(gates.platform, "system", return_value="Darwin"), \
                 mock.patch.object(gates, "identify", side_effect=[original, changed]), \
                 mock.patch.object(gates, "headless_case", return_value={"status": "PASS"}), \
                 contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(gates.main(), 1)
            result = json.loads((out / "result.json").read_text())
            self.assertEqual(result["status"], "FAIL")
            self.assertIn("changed during the run", result["error"])

    def test_missing_positive_control_exits_nonzero_and_records_blocked(self):
        artifact = {"app": "/fixture/Chromium.app", "core_sha256": {"binary": "same"}}
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp) / "run"
            args = ["gate", "--candidate-app", artifact["app"], "--control-app", artifact["app"],
                    "--mode", "infobar", "--output-dir", str(out)]
            with mock.patch("sys.argv", args), mock.patch.object(gates.platform, "system", return_value="Darwin"), \
                 mock.patch.object(gates, "identify", return_value=artifact), \
                 mock.patch.object(gates, "native_case", return_value=gates.evaluate_ax(tree())), \
                 contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(gates.main(), 1)
            result = json.loads((out / "result.json").read_text())
            self.assertEqual(result["status"], "BLOCKED")

    def test_existing_evidence_is_never_overwritten(self):
        artifact = {"app": "/fixture/Chromium.app", "core_sha256": {"binary": "same"}}
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp)
            sentinel = out / "result.json"
            sentinel.write_text("preserve previous evidence")
            args = ["gate", "--candidate-app", artifact["app"], "--mode", "headless", "--output-dir", temp]
            with mock.patch("sys.argv", args), mock.patch.object(gates.platform, "system", return_value="Darwin"), \
                 mock.patch.object(gates, "identify", return_value=artifact):
                with self.assertRaises(FileExistsError):
                    gates.main()
            self.assertEqual(sentinel.read_text(), "preserve previous evidence")


if __name__ == "__main__":
    unittest.main()
