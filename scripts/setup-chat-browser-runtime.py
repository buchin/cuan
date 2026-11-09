#!/usr/bin/env python3
"""Install/check Cuan's pinned, optional anonymous chat browser (no AI login)."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import platform
import pwd
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request

VERSION = '0.2.6'
ARCHIVES = {
    'x86_64': 'f1188c1a3b1ef70d56d594beebbb34b693ff89c280fa073820ad019ed555a02c',
    'aarch64': '5bd8a40e4ff9b35ae985e695f2445181b6d40a115c447f3bd2013fb7b77aeb69',
}
MAX_ARCHIVE = 64 * 1024 * 1024
MAX_BINARY = 64 * 1024 * 1024


def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()


def download(url, destination):
    # Installation uses official public assets, never ambient proxies or keys.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(url, timeout=30) as response, destination.open('wb') as out:
        count = 0
        while True:
            chunk = response.read(1024 * 1024)
            if not chunk:
                break
            count += len(chunk)
            if count > MAX_ARCHIVE:
                raise RuntimeError('Browser archive exceeded its size limit')
            out.write(chunk)


def executable(path):
    path = Path(path).resolve(strict=True)
    if not path.is_file() or not os.access(path, os.X_OK):
        raise RuntimeError('Browser runtime executable is unavailable')
    return path


def setup(home, arch, sandbox, loader=None, libraries=None, archive=None, check=False):
    if arch not in ARCHIVES:
        raise RuntimeError('Pinned chat browser supports Linux x86_64 and aarch64')
    sandbox = executable(sandbox)
    if bool(loader) != bool(libraries):
        raise RuntimeError('Select both a Cuan-managed loader and its libraries')
    loader = executable(loader) if loader else None
    libraries = Path(libraries).resolve(strict=True) if libraries else None
    if libraries and not libraries.is_dir():
        raise RuntimeError('Browser compatibility libraries are unavailable')
    base = home / '.local/share/cuan-chat-browser'
    root = base / VERSION
    manifest = base / 'runtime.json'
    binary = root / 'obscura'
    if base.is_symlink() or root.is_symlink():
        raise RuntimeError('Browser runtime directories must not be symlinks')
    if check:
        saved = json.loads(manifest.read_text())
        if saved.get('version') != VERSION or saved.get('archive_sha256') != ARCHIVES[arch] or saved.get('binary') != str(binary) or binary.is_symlink() or digest(binary) != saved.get('binary_sha256'):
            raise RuntimeError('Installed browser integrity check failed')
        # Checking does not silently change the selected sandbox or libraries.
        if saved.get('sandbox') != str(sandbox) or saved.get('loader', '') != str(loader or '') or saved.get('library_dir', '') != str(libraries or ''):
            raise RuntimeError('Selected browser runtime differs from its manifest')
    else:
        base.mkdir(parents=True, exist_ok=True, mode=0o700)
        root.mkdir(exist_ok=True, mode=0o700)
        fd = os.open(base / 'install.lock', os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, 'w') as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            if binary.exists() or binary.is_symlink():
                # Never overwrite unrelated/modified work to make setup look ready.
                return setup(home, arch, sandbox, loader, libraries, check=True)
            if shutil.disk_usage(base).free < 256 * 1024 * 1024:
                raise RuntimeError('Chat browser setup needs 256 MiB free headroom')
            with tempfile.TemporaryDirectory(prefix='install-', dir=base) as temp:
                temp = Path(temp)
                source = Path(archive) if archive else temp / 'archive.tar.gz'
                if not archive:
                    download(f'https://github.com/h4ckf0r0day/obscura/releases/download/v{VERSION}/obscura-{arch}-linux-no-render.tar.gz', source)
                if source.stat().st_size > MAX_ARCHIVE or digest(source) != ARCHIVES[arch]:
                    raise RuntimeError('Official browser archive checksum mismatch')
                candidate = temp / 'obscura'
                with tarfile.open(source, 'r:gz') as bundle:
                    members = [member for member in bundle.getmembers() if member.name == 'obscura']
                    if len(members) != 1 or not members[0].isfile() or members[0].size > MAX_BINARY:
                        raise RuntimeError('Invalid browser executable in archive')
                    # No extractall: archive paths, links and the unused worker
                    # cannot write anywhere in the service account.
                    with bundle.extractfile(members[0]) as inp, candidate.open('xb') as out:
                        shutil.copyfileobj(inp, out, length=1024 * 1024)
                candidate.chmod(0o700)
                os.replace(candidate, binary)
            saved = {'version': VERSION, 'archive_sha256': ARCHIVES[arch], 'binary_sha256': digest(binary), 'binary': str(binary), 'sandbox': str(sandbox), 'lock_file': str(base / 'browser.lock'), 'loader': str(loader or ''), 'library_dir': str(libraries or '')}
            # The stable shared inode must exist before provider sandboxes bind it.
            fd = os.open(base / 'browser.lock', os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
            os.close(fd)
            with tempfile.NamedTemporaryFile(mode='w', dir=base, delete=False) as out:
                json.dump(saved, out)
                temporary = Path(out.name)
            try:
                os.replace(temporary, manifest)
            finally:
                temporary.unlink(missing_ok=True)
    # Version verification is not a claim of browser/MCP/sandbox readiness.
    command = ([str(loader), '--library-path', str(libraries)] if loader else []) + [str(binary), '--version']
    with tempfile.TemporaryDirectory(prefix='cuan-browser-version-') as temp:
        result = subprocess.run(command, capture_output=True, text=True, timeout=5,
                                env={'HOME': temp, 'PATH': '/usr/bin:/bin'})
    if result.returncode or result.stdout.strip() != 'obscura ' + VERSION:
        raise RuntimeError('Browser cannot start; Linux release requires glibc or an explicit Cuan-managed compatibility runtime')
    return {'installed': True, 'version_verified': True, 'version': VERSION, 'manifest': str(manifest)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--user', required=True, help='Non-root Cuan service user')
    parser.add_argument('--check', action='store_true', help='Verify existing files/version; no installation or download')
    parser.add_argument('--archive', type=Path, help='Optional existing official archive; pinned checksum still required')
    parser.add_argument('--sandbox', type=Path, help='Application-managed Bubblewrap executable')
    parser.add_argument('--loader', type=Path, help='Explicit Cuan-managed glibc loader on musl hosts')
    parser.add_argument('--library-dir', type=Path, help='Libraries for the explicit loader')
    args = parser.parse_args()
    try:
        account = pwd.getpwnam(args.user)
        if sys.platform != 'linux' or account.pw_uid == 0:
            raise RuntimeError('Choose a non-root native Linux Cuan service account')
        if args.check and args.archive:
            raise RuntimeError('--check does not import an archive')
        if os.geteuid() == 0:
            # Drop privileges before touching the service user's files.
            completed = subprocess.run(['sudo', '-n', '-H', '-u', args.user, '--', sys.executable, str(Path(__file__).resolve()), *sys.argv[1:]], check=False)
            raise SystemExit(completed.returncode)
        if os.geteuid() != account.pw_uid:
            raise RuntimeError('Run as the selected service user or root')
        # Reuse the application's saved compatibility selection on ordinary
        # --check/reinstall; never discover an ambient provider/Fural loader.
        existing = Path(account.pw_dir) / '.local/share/cuan-chat-browser/runtime.json'
        if not args.loader and not args.library_dir and existing.exists():
            saved = json.loads(existing.read_text())
            args.loader = saved.get('loader') or None
            args.library_dir = saved.get('library_dir') or None
        selected = args.sandbox or (Path('/opt/cuan/runtime/bwrap') if Path('/opt/cuan/runtime/bwrap').exists() else shutil.which('bwrap', path='/usr/bin:/bin'))
        if not selected:
            raise RuntimeError('Cuan chat sandbox is missing; prepare the native runtime first')
        print(json.dumps(setup(Path(account.pw_dir), platform.machine().lower(), selected, args.loader, args.library_dir, args.archive, args.check)))
    except (OSError, KeyError, ValueError, RuntimeError, tarfile.TarError, subprocess.SubprocessError) as error:
        detail = str(error) if isinstance(error, RuntimeError) else 'Setup failed; inspect the private installation log'
        parser.exit(1, 'Chat browser setup failed: ' + detail + '\n')


if __name__ == '__main__':
    main()
