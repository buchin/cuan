#!/usr/bin/env python3
"""Emit a credential-free GitHub source identity for release build metadata."""
import re
import subprocess
from urllib.parse import urlsplit


def source_identity(remote):
    remote = remote.strip()
    if remote.startswith("git@github.com:"):
        path = remote[len("git@github.com:"):]
    else:
        parsed = urlsplit(remote)
        if parsed.scheme not in ("https", "ssh") or parsed.hostname != "github.com" or parsed.query or parsed.fragment:
            raise ValueError("Release source repository is not a supported GitHub remote")
        path = parsed.path.lstrip("/")
    path = path.rstrip("/")
    if path.endswith(".git"):
        path = path[:-4]
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*", path):
        raise ValueError("Release source repository identity is invalid")
    return path.lower()


if __name__ == "__main__":
    try:
        remote = subprocess.check_output(["git", "remote", "get-url", "origin"], text=True, stderr=subprocess.DEVNULL)
        print(source_identity(remote))
    except (ValueError, subprocess.SubprocessError):
        raise SystemExit("Release source repository could not be identified safely")
