#!/usr/bin/env python3
"""Install/check Cuan's native Linux Inbox sandbox without disabling host policy."""
import argparse
import json
import os
from pathlib import Path
import pwd
import shutil
import subprocess
import tempfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--user", required=True, help="Linux user running the Cuan service")
parser.add_argument("--check", action="store_true", help="Check only; do not install or modify policy")
args = parser.parse_args()
account = pwd.getpwnam(args.user)
home = Path(account.pw_dir)
binary = Path("/opt/cuan/runtime/bwrap")

def as_user(command):
    if os.geteuid() == account.pw_uid:
        return command
    return ["sudo", "-u", args.user, "--", *command]

def check():
    candidate = str(binary) if binary.exists() else shutil.which("bwrap")
    if not candidate:
        raise RuntimeError("Bubblewrap missing; rerun this script as root without --check")
    command = [candidate, "--die-with-parent", "--new-session", "--unshare-pid",
               "--unshare-ipc", "--unshare-uts", "--cap-drop", "ALL", "--clearenv"]
    for path in ["/usr", "/bin", "/lib", "/lib64"]:
        if Path(path).exists():
            command += ["--ro-bind", path, path]
    command += ["--proc", "/proc", "--remount-ro", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--", "/bin/sh", "-c",
                f"test ! -e /opt/cuan/.env && test ! -e /proc/{os.getpid()}/environ && /proc/self/exe -c 'echo SANDBOX_OK'"]
    result = subprocess.run(as_user(command), capture_output=True, text=True, timeout=15)
    if result.returncode or result.stdout.strip() != "SANDBOX_OK":
        raise RuntimeError("Sandbox blocked by host/container policy. Check user namespaces and AppArmor; never use privileged containers or disable the sandbox as a workaround.")
    print(json.dumps({"sandboxReady": True, "providerInvocations": 0}))

try:
    if not args.check:
        if os.geteuid() != 0:
            raise RuntimeError("Installation requires root; --check can run as the service user")
        runtime = binary.parent
        runtime.mkdir(parents=True, exist_ok=True, mode=0o755)
        if runtime.stat().st_uid != 0 or runtime.stat().st_mode & 0o022:
            raise RuntimeError("Runtime directory must be owned by root and not group/world writable")
        with tempfile.TemporaryDirectory(prefix="cuan-bwrap-") as temporary:
            root = Path(temporary)
            subprocess.run(["apt-get", "download", "bubblewrap"], cwd=root, check=True,
                           stdout=subprocess.DEVNULL)
            packages = list(root.glob("bubblewrap*.deb"))
            if len(packages) != 1:
                raise RuntimeError("Expected one distribution Bubblewrap package")
            subprocess.run(["dpkg-deb", "-x", str(packages[0]), str(root/"package")], check=True)
            shutil.copyfile(root/"package/usr/bin/bwrap", binary)
            binary.chmod(0o755)
        restriction = Path("/proc/sys/kernel/apparmor_restrict_unprivileged_userns")
        if restriction.exists() and restriction.read_text().strip() == "1":
            profile = Path("/etc/apparmor.d/cuan-inbox-bwrap")
            content = """abi <abi/4.0>,
include <tunables/global>
profile cuan-inbox-bwrap /opt/cuan/runtime/bwrap flags=(unconfined) {
  userns,
}
"""
            if profile.exists() and profile.read_text() != content:
                raise RuntimeError("Existing AppArmor profile differs; review it before replacement")
            profile.write_text(content)
            profile.chmod(0o644)
            subprocess.run(["apparmor_parser", "-r", str(profile)], check=True)
        bindir = home/".local/bin"
        bindir.mkdir(parents=True, exist_ok=True)
        os.chown(bindir, account.pw_uid, account.pw_gid)
        link = bindir/"bwrap"
        if link.is_symlink():
            link.unlink()
        if link.exists():
            raise RuntimeError("Existing service-user bwrap is not a symlink; review it first")
        link.symlink_to(binary)
        os.lchown(link, account.pw_uid, account.pw_gid)
    check()
except (OSError, RuntimeError, subprocess.SubprocessError) as error:
    parser.exit(1, f"Chat sandbox setup failed: {error}\n")
