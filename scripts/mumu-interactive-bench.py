#!/usr/bin/env python3
"""On macOS: create, measure and remove ONE disposable MuMu device. Requires adb."""
import json
import os
from pathlib import Path
import subprocess
import time

SCROLL = (Path(__file__).resolve().parents[1] / 'internal/mobile/avo/testdata/settings-scroll.sh').read_text()
CLI = '/Applications/MuMuPlayer Pro.app/Contents/MacOS/mumu-cli'


def mumu(*args):
    reply = json.loads(subprocess.check_output([CLI, *args], timeout=40))
    if reply['errcode']:
        raise RuntimeError(reply['message'])
    return reply.get('return')


def devices():
    return mumu('info', 'all')['results']


def measure(depth, count):
    existing = {d['bundle_path'] for d in devices()}
    start = time.monotonic()
    device = mumu('create', '--type', 'phone', '--setting', '{"vmCpuCount":3,"vmMemorySize":4096,"resolutionWidthHeight":"780x1688"}')
    path = device['bundle_path']
    if path in existing:
        raise RuntimeError('MuMu did not return a new bundle; refusing to operate')

    def own():
        return next(d for d in devices() if d['bundle_path'] == path)

    def action(cmd):
        mumu(cmd, str(own()['index']))

    def close():
        if own()['state'] != 'stopped':
            action('close')
        for _ in range(60):
            if own()['state'] == 'stopped':
                return
            time.sleep(.5)
        raise RuntimeError(f'Test device did not stop: {path}')

    try:
        for cycle in ((1,) if count else (1, 2)):
            if cycle == 2:
                start = time.monotonic()
            action('open')
            while time.monotonic() - start < 120:
                port = own().get('adb_port')
                if port:
                    serial = f'127.0.0.1:{port}'
                    def shell(cmd, timeout=12):
                        return subprocess.check_output(['adb', '-s', serial, 'shell', cmd], timeout=timeout, stderr=subprocess.DEVNULL).decode().strip()
                    try:
                        subprocess.run(['adb', 'connect', serial], timeout=4, capture_output=True, check=True)
                        if shell('getprop sys.boot_completed') == '1':
                            break
                    except (subprocess.TimeoutExpired, subprocess.CalledProcessError):
                        pass
                time.sleep(.25)
            else:
                raise RuntimeError('ADB did not confirm boot')
            if 'Status: ok' not in shell('am start -W -a android.settings.SETTINGS'):
                raise RuntimeError('Settings did not open')
            shell('input keyevent KEYCODE_HOME')
            while 'launcher' not in shell('dumpsys activity activities | grep -im1 resumedactivity').lower():
                if time.monotonic() - start > 150:
                    raise RuntimeError('HOME not confirmed')
                time.sleep(.25)
            print(f'cycle={cycle} interactive_ms={round((time.monotonic()-start)*1000)}', flush=True)
            if count:
                # Match AnyMobile's default logical layout for the scrolling workload.
                shell('wm density 320')
                print(shell('wm size; wm density; getconf _NPROCESSORS_ONLN; grep MemTotal /proc/meminfo'), flush=True)
                frames = shell(SCROLL)
                print(f'instances={depth} settings_scroll={frames}', flush=True)
                if depth < count:
                    measure(depth + 1, count)
                elif os.environ.get('MOBILE_BENCH_SUSTAINED') == '1':
                    print('sustained_scroll=' + shell('set -- 20\n' + SCROLL, timeout=150), flush=True)
            close()
    finally:
        close()
        action('delete')


if __name__ == '__main__':
    count = int(os.environ.get('MUMU_BENCH_INSTANCES', '0'))
    if count not in (0, 2, 3, 4):
        raise ValueError('MUMU_BENCH_INSTANCES must be 2–4 or unset')
    measure(1, count)
