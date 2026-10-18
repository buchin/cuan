#!/usr/bin/env python3
"""Install official Antigravity CLI with manifest SHA-512 verification."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import tarfile
import tempfile
import urllib.parse
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--prefix', type=Path, default=Path.home() / '.local')
    args = parser.parse_args()
    arch = {'x86_64': 'amd64', 'aarch64': 'arm64'}.get(platform.machine())
    if platform.system() != 'Linux' or not arch:
        parser.error('Supported platform: Linux amd64 or arm64')
    variant = '_musl' if Path('/etc/alpine-release').exists() else ''
    endpoint = ('https://antigravity-cli-auto-updater-974169037036.us-central1.run.app'
                f'/manifests/linux_{arch}{variant}.json')
    with urllib.request.urlopen(endpoint, timeout=30) as response:
        manifest = json.load(response)
    url = manifest['url']
    digest = manifest['sha512'].lower()
    if urllib.parse.urlparse(url).scheme != 'https' or len(bytes.fromhex(digest)) != 64:
        raise ValueError('Invalid release manifest')
    target_dir = args.prefix / 'bin'
    target_dir.mkdir(parents=True, exist_ok=True)
    receipt = target_dir / '.agy-release.json'
    target = target_dir / 'agy'
    if target.is_file() and receipt.is_file():
        previous = json.loads(receipt.read_text())
        if (previous.get('sha512') == digest and
                hashlib.sha512(target.read_bytes()).hexdigest() == previous.get('installed_sha512')):
            print('Antigravity already installed:', manifest['version'])
            return
    with tempfile.TemporaryDirectory(dir=target_dir) as stage:
        payload = Path(stage) / 'payload'
        checksum = hashlib.sha512()
        size = 0
        with urllib.request.urlopen(url, timeout=120) as response, payload.open('wb') as output:
            while chunk := response.read(1024 * 1024):
                size += len(chunk)
                if size > 512 * 1024 * 1024:
                    raise ValueError('Release exceeds download limit')
                checksum.update(chunk)
                output.write(chunk)
        if checksum.hexdigest() != digest:
            raise ValueError('Release SHA-512 mismatch')
        binary = payload
        if '.tar.gz' in urllib.parse.urlparse(url).path.lower():
            binary = Path(stage) / 'agy'
            with tarfile.open(payload, 'r:gz') as archive:
                candidates = [entry for entry in archive.getmembers()
                              if entry.isfile() and Path(entry.name).name in ('agy', 'antigravity')]
                if len(candidates) != 1:
                    raise ValueError('Release must contain one agy executable')
                with archive.extractfile(candidates[0]) as source, binary.open('wb') as output:
                    shutil.copyfileobj(source, output)
        installed_digest = hashlib.sha512(binary.read_bytes()).hexdigest()
        binary.chmod(0o755)
        os.replace(binary, target)
        receipt.write_text(json.dumps({**manifest, 'installed_sha512': installed_digest}) + '\n')
    print('Installed verified official Antigravity:', manifest['version'])


if __name__ == '__main__':
    main()
