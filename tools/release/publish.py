"""Publish verified platform archives only to the public product repository."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import subprocess
import tempfile

from prepare import ROOT, clean_revision, digest, run

REPO = 'moreveal/mimic-overview'
CHECKS = {'runtimecheck:v8', 'runtimecheck:quickjs', 'runtimecheck:goja',
          'examples:puppeteer', 'examples:playwright', 'examples:concurrency'}


def verified_archive(output, version, host, source_revision, public_revision):
    receipt = json.loads((output / f'{host}-amd64.receipt.json').read_text())
    if (receipt['version'] != version or receipt['platform'] != f'{host}-amd64'
            or receipt['sourceRevision'] != source_revision
            or receipt['overviewRevision'] != public_revision
            or receipt['verified'] is not True or set(receipt['checks']) != CHECKS):
        raise RuntimeError(f'Stale or unverified {host} receipt')
    filename = receipt['archive']
    suffix = '.zip' if host == 'windows' else '.tar.gz'
    if filename != f'mimic-{version}-{host}-amd64{suffix}':
        raise RuntimeError('Unexpected archive name')
    archive = output / filename
    if digest(archive) != receipt['sha256'] or archive.stat().st_size != receipt['size']:
        raise RuntimeError(f'Archive changed: {filename}')
    return archive, receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True)
    parser.add_argument('--overview', type=Path, required=True)
    args = parser.parse_args()
    if not re.fullmatch(r'v\d+\.\d+\.\d+-beta\.\d+', args.version):
        parser.error('Expected vMAJOR.MINOR.PATCH-beta.NUMBER')
    overview = args.overview.resolve()
    source_revision = clean_revision(ROOT)
    public_revision = clean_revision(overview)
    remote_revision = run('gh', 'api', f'repos/{REPO}/commits/main', '--jq', '.sha', capture=True).strip()
    if public_revision != remote_revision:
        raise RuntimeError('Push the public overview commit before publishing')
    output = ROOT / '.build/releases' / args.version
    assets, public_assets = [], []
    for host in ('windows', 'linux'):
        archive, receipt = verified_archive(output, args.version, host, source_revision, public_revision)
        assets.append(archive)
        public_assets.append({key: receipt[key] for key in
                              ('platform', 'archive', 'sha256', 'size', 'binarySha256', 'checks')})
    manifest = output / 'release-manifest.json'
    manifest.write_text(json.dumps({
        'version': args.version, 'overviewRevision': public_revision,
        'license': 'PolyForm-Shield-1.0.0', 'exampleLicense': 'MIT',
        'requirements': {'windows-amd64': 'Windows x64; validated on Windows 11',
                         'linux-amd64': 'glibc 2.39+, libgcc_s, installed fonts; Ubuntu 24.04 / WSL2'},
        'clients': {'playwright-core': '1.63.0', 'puppeteer-core': '25.10.0'},
        'artifacts': public_assets,
    }, indent=2) + '\n', encoding='utf-8')
    assets.append(manifest)
    sums = output / 'SHA256SUMS'
    sums.write_text(''.join(f'{digest(path)}  {path.name}\n' for path in assets), encoding='utf-8')
    assets.append(sums)
    body = (overview / 'RELEASE_NOTES.md').read_text(encoding='utf-8')
    if args.version not in body.splitlines()[0]:
        raise RuntimeError('Release notes do not match the release version')
    body = re.sub(r'\]\((?!https?://)([^)]+)\)',
                  lambda m: f'](https://github.com/{REPO}/blob/{public_revision}/{m[1]})', body)
    notes = output / 'release-body.md'
    notes.write_text(body, encoding='utf-8')
    existing = subprocess.run(['gh', 'release', 'view', args.version, '--repo', REPO,
                               '--json', 'isDraft,targetCommitish'], capture_output=True, text=True)
    if existing.returncode == 0:
        current = json.loads(existing.stdout)
        if not current['isDraft'] or current['targetCommitish'] != public_revision:
            raise RuntimeError('Refusing to overwrite an existing release or unrelated draft')
    else:
        # Verify repository access independently; do not hide an authentication failure.
        run('gh', 'repo', 'view', REPO, '--json', 'name', capture=True)
        run('gh', 'release', 'create', args.version, '--repo', REPO, '--draft', '--prerelease',
            '--target', public_revision, '--title', f'Mimic {args.version} — Public Beta',
            '--notes-file', str(notes))
    run('gh', 'release', 'upload', args.version, '--repo', REPO, '--clobber', *map(str, assets))
    with tempfile.TemporaryDirectory(prefix='mimic-release-download-') as temp:
        run('gh', 'release', 'download', args.version, '--repo', REPO, '--dir', temp)
        for path in assets:
            if digest(Path(temp) / path.name) != digest(path):
                raise RuntimeError(f'GitHub download verification failed: {path.name}')
    run('gh', 'release', 'edit', args.version, '--repo', REPO,
        '--draft=false', '--prerelease', '--notes-file', str(notes))
    print(f'Published and download-verified: https://github.com/{REPO}/releases/tag/{args.version}')


if __name__ == '__main__':
    main()
