#!/usr/bin/env python3
"""Install/check the native browser used for task screenshot verification."""
import argparse
import json
import os
from pathlib import Path
import pwd
import shutil
import subprocess
import sys

PLAYWRIGHT_VERSION = '1.64.1'
SMOKE = '''const {chromium}=require(process.argv[1]);
(async()=>{const browser=await chromium.launch({headless:true});
try {const page=await browser.newPage({viewport:{width:390,height:844}});
await page.setContent('<html><body>Browser verification</body></html>');
if((await page.screenshot()).length<100)throw new Error('Empty screenshot');
} finally {await browser.close()}})().catch(()=>process.exit(1));'''


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--user', required=True, help='Non-root runner service account')
    parser.add_argument('--check', action='store_true', help='Launch browser and capture in-memory screenshot; no installation')
    parser.add_argument('--install-deps', action='store_true', help='Install official browser OS dependencies (requires root)')
    args = parser.parse_args()
    if args.check and args.install_deps:
        parser.error('--check cannot install dependencies')
    try:
        account = pwd.getpwnam(args.user)
        if sys.platform != 'linux' or account.pw_uid == 0:
            raise RuntimeError('Choose a non-root native Linux runner account')
        if args.install_deps and os.geteuid() != 0:
            raise RuntimeError('--install-deps requires root')
        if os.geteuid() not in (0, account.pw_uid):
            raise RuntimeError('Run as root or the runner account')
        node, npm = shutil.which('node'), shutil.which('npm')
        if not node or (not args.check and not npm):
            raise RuntimeError('Node.js and npm are required for browser setup')
        home = Path(account.pw_dir)
        root = home/'.local/share/cuan-browser'/PLAYWRIGHT_VERSION
        module = root/'node_modules/playwright-core'
        cli = module/'cli.js'
        command = home/'.local/bin/cuan-browser'

        def as_user(argv):
            return ['sudo', '-H', '-u', args.user, '--', *argv] if os.geteuid() == 0 else argv

        def run(argv, timeout=180, user=True):
            subprocess.run(as_user(argv) if user else argv, check=True, timeout=timeout)

        if not args.check:
            if shutil.disk_usage(home).free < 3 * 1024**3:
                raise RuntimeError('Browser setup needs 3 GiB free to preserve task execution headroom')
            run([npm, 'install', '--prefix', str(root), '--ignore-scripts', '--no-audit', '--no-fund', 'playwright-core@'+PLAYWRIGHT_VERSION])
            if args.install_deps:
                if os.geteuid() != 0:
                    raise RuntimeError('--install-deps requires root')
                run([node, str(cli), 'install-deps', 'chromium'], user=False)
            run([node, str(cli), 'install', 'chromium', '--only-shell'], timeout=240)
        if not cli.is_file():
            raise RuntimeError('Browser runtime is missing; run setup without --check')
        # Launch and actually render; a package or cached browser directory alone
        # does not prove its system libraries or executable are usable.
        run([node, '-e', SMOKE, str(module)], timeout=30)
        if not args.check:
            run(['mkdir', '-p', str(command.parent)])
            run(['ln', '-sfn', str(cli), str(command)])
        if not command.is_symlink() or command.resolve() != cli.resolve():
            raise RuntimeError('Browser CLI link is missing or points at another runtime')
        print(json.dumps({'browserReady': True, 'screenshotsVerified': True, 'version': PLAYWRIGHT_VERSION, 'cli': str(command), 'module': str(module)}))
    except (OSError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        # Do not echo command outputs or a provider environment on failure.
        detail = str(error) if isinstance(error, RuntimeError) else 'Browser setup or rendering failed; inspect the private installation log'
        parser.exit(1, 'Browser runtime setup failed: '+detail+'\n')


if __name__ == '__main__':
    main()
