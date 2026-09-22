#!/usr/bin/env python3
"""Install the reviewed sync workflow into the owner's existing package repos.
Uses the current gh login; no credentials are copied to GitHub Actions secrets.
"""
import base64
import json
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[1]
def api(endpoint, data=None, method=None):
    args = ['gh', 'api', endpoint]
    if method:
        args += ['--method', method]
    if data is not None:
        args += ['--input', '-']
    r = subprocess.run(args, input=json.dumps(data) if data is not None else None, text=True, capture_output=True, check=True)
    return json.loads(r.stdout)
for repo, kind in [('homebrew-tap', 'brew'), ('scoop-bucket', 'scoop')]:
    prefix = f'repos/k0ngk0ng/{repo}/git/'
    base = api(prefix + 'ref/heads/main')['object']['sha']
    tree = api(prefix + 'commits/' + base)['tree']['sha']
    entries = []
    for local, remote in [('packaging/sync-talk.py', 'scripts/sync-talk.py'), (f'packaging/sync-talk-{kind}.yml', '.github/workflows/sync-talk.yml')]:
        data = (root / local).read_bytes()
        blob = api(prefix + 'blobs', dict(content=base64.b64encode(data).decode(), encoding='base64'))['sha']
        entries.append(dict(path=remote, mode='100644', type='blob', sha=blob))
    new_tree = api(prefix + 'trees', dict(base_tree=tree, tree=entries))['sha']
    commit = api(prefix + 'commits', dict(message='Add verified talk release synchronization', tree=new_tree, parents=[base]))['sha']
    api(prefix + 'refs/heads/main', dict(sha=commit, force=False), 'PATCH')
    print(f'{repo}: {commit}')
