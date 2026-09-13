#!/usr/bin/env python3
"""Offline publishable-tree audit. Report locations, never credential contents."""
import ast
import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LOCAL = {'.build', '.git', '__pycache__', '.cache', '.venv'}
CREDENTIAL = re.compile(r'(?i)(?:bearer\s+[A-Za-z0-9._-]{16,}|(?:sk-|ghp_|github_pat_|AKIA)[A-Za-z0-9_-]{16,}|-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----)')
LOCAL_PATH = re.compile(r'(?i)(?:[A-Z]:[\\/]+(?:Users|GitHub)[\\/]+|/(?:home|Users)/)')
TARGET = re.compile(r'(?i)(?:cloudflare|turnstile|browserscan|voxel\.shop|iroshop|cf_clearance|/cdn-cgi/|/fo/)')
BAD_DIRS = {'.tmp', '.chrome-for-testing', '.source-cache', 'node_modules', 'venv'}
BAD_SUFFIXES = {'.exe', '.dll', '.so', '.a', '.pid', '.log', '.prof', '.pprof', '.pyc', '.pfx', '.p12', '.key'}
issues = []
count = 0
for path in sorted(ROOT.rglob('*')):
    relative = path.relative_to(ROOT)
    if LOCAL.intersection(relative.parts) or not path.is_file():
        continue
    count += 1
    if BAD_DIRS.intersection(relative.parts) or path.suffix in BAD_SUFFIXES or path.name.startswith('.tmp-'):
        issues.append((str(relative), 'unarchived local artifact'))
    if path.name == '.env' or (path.name.startswith('.env.') and path.name != '.env.example'):
        issues.append((str(relative), 'environment file'))
    try:
        text = path.read_text(encoding='utf-8')
    except UnicodeDecodeError:
        if path.suffix not in {'.jpg', '.png'}:
            issues.append((str(relative), 'unexpected binary/non-UTF8 file'))
        continue
    if path.suffix == '.py':
        try:
            ast.parse(text, filename=str(relative))
        except SyntaxError:
            issues.append((str(relative), 'invalid Python syntax'))
    if path.suffix == '.json':
        try:
            # Decode JSON escapes before looking for local paths/credentials.
            text = json.dumps(json.loads(text), ensure_ascii=False).replace('\\\\', '\\')
        except ValueError:
            issues.append((str(relative), 'invalid JSON'))
    if CREDENTIAL.search(text):
        issues.append((str(relative), 'credential pattern'))
    if LOCAL_PATH.search(text):
        issues.append((str(relative), 'machine-specific path'))
    if relative.parts[0] == 'internal' and not path.name.endswith('_test.go') and TARGET.search(text):
        issues.append((str(relative), 'site/vendor runtime condition needs review'))
if issues:
    for path, reason in issues:
        print(f'FAIL {path}: {reason}')
    raise SystemExit(1)
subprocess.run([sys.executable, str(ROOT / 'tools' / 'check_oracle_captures.py')], check=True)
print(f'PASS: {count} publishable files checked; no credential/local-path patterns, debris or runtime site/vendor matches')
print('Heuristic scan only; synthetic cookie/header test data and public SPKI keys are intentional.')
