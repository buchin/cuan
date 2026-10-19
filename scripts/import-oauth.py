#!/usr/bin/env python3
"""One-time private migration of the same Cuan owner's CLI credential over SSH stdin."""
import os
from pathlib import Path
import shutil
import sys
import tempfile


def main():
    state = Path.home() / '.local/share/cuan-s25'
    state.mkdir(parents=True, exist_ok=True, mode=0o700)
    receipt = state / 'oauth-import-used'
    descriptor = os.open(receipt, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    os.close(descriptor)
    try:
        raw = sys.stdin.buffer.read(1024 * 1024 + 1)
        if not 16 <= len(raw) <= 1024 * 1024:
            raise ValueError('Invalid credential payload size')
        directory = Path.home() / '.gemini/antigravity-cli'
        directory.mkdir(parents=True, exist_ok=True, mode=0o700)
        target = directory / 'antigravity-oauth-token'
        backup = directory / 'antigravity-oauth-token.before-s25'
        if target.exists() and not backup.exists():
            shutil.copy2(target, backup)
            backup.chmod(0o600)
        fd, name = tempfile.mkstemp(dir=directory)
        try:
            with os.fdopen(fd, 'wb') as output:
                output.write(raw)
                output.flush()
                os.fsync(output.fileno())
            os.replace(name, target)
        finally:
            Path(name).unlink(missing_ok=True)
        print('Owner credential imported privately; one-time receipt recorded.')
    except Exception:
        receipt.unlink(missing_ok=True)
        raise


if __name__ == '__main__':
    main()
