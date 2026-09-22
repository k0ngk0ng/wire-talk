#!/usr/bin/env python3
"""Exercise real foreground/background process lifecycles with a null audio driver.
The test-only binary is built under .cache and is never used by package.py.
"""
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import tempfile
import time

root = Path(__file__).resolve().parents[1]
env = dict(os.environ)
for var, folder in [('GOCACHE', 'go-build'), ('GOMODCACHE', 'go-mod'), ('GOTMPDIR', 'tmp'), ('TMPDIR', 'tmp')]:
    dest = root / '.cache' / folder
    dest.mkdir(parents=True, exist_ok=True)
    env[var] = str(dest)
exe = root / '.cache' / ('talk-test.exe' if os.name == 'nt' else 'talk-test')
subprocess.run(['go', 'build', '-tags', 'talk_test_audio', '-o', str(exe), './cmd/wirectl-talk'], env=env, cwd=root, check=True)
with tempfile.TemporaryDirectory(dir=root / '.cache') as directory:
    state = Path(directory) / 'state'
    env['WIRE_TALK_HOME'] = str(state)
    def command(*args, check=True):
        return subprocess.run([str(exe), *args], env=env, text=True, capture_output=True, timeout=25, check=check)
    def wait_online():
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            r = command('status', check=False)
            if r.returncode == 0:
                return json.loads(r.stdout)
            time.sleep(.05)
        raise AssertionError('session never became online')
    command('init', '--listen', '127.0.0.1:0')
    foreground = subprocess.Popen([str(exe), 'join'], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, creationflags=subprocess.CREATE_NEW_PROCESS_GROUP if os.name == 'nt' else 0)
    try:
        assert wait_online()['peers'] == []
        assert command('join', check=False).returncode != 0
        assert command('daemon', 'start', check=False).returncode != 0
        command('mute')
        assert json.loads(command('status').stdout)['muted'] is True
        command('unmute')
        assert json.loads(command('status').stdout)['muted'] is False
        watcher = subprocess.Popen([str(exe), 'watch'], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(.15)
        watcher.terminate()
        watcher.wait(timeout=5)
        assert command('status').returncode == 0
        # Console-free Windows CI cannot reliably deliver Ctrl+C. Its control
        # stop path is tested; Unix additionally proves SIGINT lifecycle cleanup.
        if os.name == 'nt':
            command('daemon', 'stop')
        else:
            foreground.send_signal(signal.SIGINT)
        out, err = foreground.communicate(timeout=5)
        assert foreground.returncode == 0, (out, err)
        assert command('status', check=False).returncode != 0
        command('daemon', 'start')
        wait_online()
        assert command('join', check=False).returncode != 0
        command('daemon', 'stop')
        assert command('status', check=False).returncode != 0
        # Start again proves all audio, UDP and process ownership was released.
        command('daemon', 'start')
        command('daemon', 'stop')
        # Native service worker must recover from unavailable resources instead
        # of exhausting a platform service manager's finite restart counter.
        blocker = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        blocker.bind(('127.0.0.1', 0))
        config_path = state / 'config.json'
        config = json.loads(config_path.read_text())
        config['listen'] = '127.0.0.1:' + str(blocker.getsockname()[1])
        config_path.write_text(json.dumps(config))
        worker = subprocess.Popen([str(exe), '__serve'], env=env)
        try:
            time.sleep(.3)
            assert worker.poll() is None
            blocker.close()
            wait_online()
            command('daemon', 'stop')
            worker.wait(timeout=5)
            assert worker.returncode == 0
        finally:
            blocker.close()
            if worker.poll() is None:
                worker.kill()
                worker.wait()
        print('Foreground/background/mute/watch/stop/restart/resource recovery lifecycle passed')
    finally:
        command('daemon', 'stop', check=False)
        if foreground.poll() is None:
            foreground.kill()
            foreground.wait()
