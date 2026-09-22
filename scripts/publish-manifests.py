#!/usr/bin/env python3
"""Publish generated definitions using a narrowly scoped GitHub token."""
import argparse
import base64
import json
from pathlib import Path
import subprocess
p = argparse.ArgumentParser()
p.add_argument('--version', required=True)
a = p.parse_args()
root = Path(__file__).resolve().parents[1]
for repo, remote, local in [('homebrew-tap', 'Formula/talk.rb', 'talk.rb'), ('scoop-bucket', 'bucket/talk.json', 'talk.json')]:
    endpoint = f'repos/k0ngk0ng/{repo}/contents/{remote}'
    old = subprocess.run(['gh', 'api', endpoint], text=True, capture_output=True)
    body = dict(message=f'talk: update to {a.version}', content=base64.b64encode((root / 'dist' / local).read_bytes()).decode())
    if old.returncode == 0:
        body['sha'] = json.loads(old.stdout)['sha']
    elif '(HTTP 404)' not in old.stderr:
        raise SystemExit(old.stderr)
    subprocess.run(['gh', 'api', '--method', 'PUT', endpoint, '--input', '-'], input=json.dumps(body), text=True, check=True, stdout=subprocess.DEVNULL)
    print(f'Published {repo}/{remote}')
