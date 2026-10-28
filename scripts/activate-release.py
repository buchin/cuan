#!/usr/bin/env python3
"""Activate or roll back a versioned native Cuan release (run as root on VPS)."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import time
import urllib.request
import urllib.parse

BASE = Path('/opt/cuan')
SERVICE = 'cuan'
PORT = 8080
SCHEMA_COUNT = 27


def switch(link, target):
    temporary = link.with_name(link.name + '.next')
    temporary.unlink(missing_ok=True)
    temporary.symlink_to(target)
    os.replace(temporary, link)


def healthy(target_sha=None):
    try:
        for endpoint in ('healthz', 'readyz'):
            with urllib.request.urlopen(f'http://127.0.0.1:{PORT}/' + endpoint, timeout=3) as response:
                body = json.load(response)
                if endpoint == 'healthz':
                    if body.get('status') not in (None, 'healthy'):
                        return False
                    if target_sha and body.get('commit_sha') and body.get('commit_sha') != target_sha:
                        return False
                elif endpoint == 'readyz' and (body.get('migrations_count') != SCHEMA_COUNT or body.get('status') != 'ready' or body.get('database') != 'connected'):
                    return False
        with urllib.request.urlopen(f'http://127.0.0.1:{PORT}/tasks', timeout=3) as response:
            if response.read() != (BASE / 'current/web/dist/index.html').read_bytes():
                return False
        return True
    except Exception:
        return False


def restart_and_check(target_sha=None):
    try:
        subprocess.run(['systemctl', 'restart', SERVICE], check=True)
    except subprocess.CalledProcessError:
        return False
    for _ in range(20):
        if healthy(target_sha=target_sha):
            return True
        time.sleep(1)
    return False



def database_environment():
    pid=subprocess.check_output(['systemctl','show',SERVICE,'-p','MainPID','--value'],text=True).strip()
    config=dict(entry.decode().split('=',1) for entry in Path('/proc/'+pid+'/environ').read_bytes().split(b'\0') if b'=' in entry)
    url=urllib.parse.urlparse(config['DATABASE_URL'])
    env=os.environ.copy()
    env.update(PGHOST=url.hostname or 'localhost',PGPORT=str(url.port or 5432),PGUSER=urllib.parse.unquote(url.username or ''),PGPASSWORD=urllib.parse.unquote(url.password or ''),PGDATABASE=url.path.lstrip('/'),PGCONNECT_TIMEOUT='5')
    return env


def database_scalar(query, env):
    result=subprocess.run(['psql','-XAt','-v','ON_ERROR_STOP=1','-c',query],env=env,capture_output=True,text=True,timeout=10)
    if result.returncode:
        raise RuntimeError('Could not verify deployment work state; release was not switched')
    return result.stdout.strip()


def assert_no_github_login(control_group=None, proc_root=Path('/proc'), cgroup_root=Path('/sys/fs/cgroup')):
    # Device login is held in a running gh process, not a task database row.
    # A release must not cut off the owner's pending browser authorization.
    if control_group is None:
        control_group = subprocess.check_output(
            ['systemctl', 'show', SERVICE, '-p', 'ControlGroup', '--value'], text=True).strip()
    if not control_group.startswith('/') or control_group == '/' or '..' in Path(control_group).parts:
        raise RuntimeError('Could not verify pending GitHub sign-in; release was not switched')
    try:
        pids = (cgroup_root / control_group.lstrip('/') / 'cgroup.procs').read_text().split()
        for pid in pids:
            if not pid.isdigit():
                raise RuntimeError('Could not verify pending GitHub sign-in; release was not switched')
            try:
                args = (proc_root / pid / 'cmdline').read_bytes().split(b'\0')
            except FileNotFoundError:
                continue  # This exact process already ended.
            if len(args) >= 3 and Path(os.fsdecode(args[0])).name == 'gh' and args[1] == b'auth' and args[2] in (b'login', b'token'):
                raise RuntimeError('GitHub sign-in is active; finish or cancel it in Cuan before changing release')
    except OSError as error:
        raise RuntimeError('Could not verify pending GitHub sign-in; release was not switched') from error


def assert_idle_work(no_restart=False):
    # A pre-switch gate, not a lock on HTTP admission. Queued requests that arrive
    # after this check retain their normal durable restart/recovery semantics.
    if not no_restart:
        env=database_environment()
        exclude_attempt = os.environ.get('CUAN_DEPLOY_EXCLUDE_ATTEMPT', '').strip()
        exclude_task = os.environ.get('CUAN_DEPLOY_EXCLUDE_TASK', '').strip()
        cap_file = os.environ.get('CUAN_TASK_CAPABILITY_FILE')
        if cap_file and Path(cap_file).is_file():
            try:
                cap_data = json.loads(Path(cap_file).read_text())
                if not exclude_attempt:
                    exclude_attempt = str(cap_data.get('attempt_id', '')).strip()
                if not exclude_task:
                    exclude_task = str(cap_data.get('task_id', '')).strip()
            except Exception:
                pass

        for table, states in [('inbox_requests',"'running','queued','cancelling'"),('task_attempts',"'running','queued'"),('task_launches',"'queued','claimed'")]:
            if database_scalar("SELECT to_regclass('public."+table+"') IS NOT NULL",env) != 't':
                continue
            column='state' if table!='task_attempts' else 'status'
            query = 'SELECT count(*) FROM '+table+' WHERE '+column+' IN ('+states+')'
            if table == 'task_attempts' and exclude_attempt:
                excluded_attempts = [a.strip() for a in exclude_attempt.split(',') if a.strip()]
                if excluded_attempts:
                    quoted = ", ".join("'" + a.replace("'", "''") + "'" for a in excluded_attempts)
                    query += " AND id NOT IN (" + quoted + ")"
            if table == 'task_launches' and exclude_task:
                excluded_tasks = [t.strip() for t in exclude_task.split(',') if t.strip()]
                if excluded_tasks:
                    quoted = ", ".join("'" + t.replace("'", "''") + "'" for t in excluded_tasks)
                    query += " AND task_id NOT IN (" + quoted + ")"
            if int(database_scalar(query,env)):
                raise RuntimeError('Active or waiting work exists; finish or cancel it before changing release')

    assert_no_github_login()


def assert_project_workspace_support(manifest, retained_environment=None):
    if manifest.get('features', {}).get('project_workspace_revisions') == 1:
        return
    env = retained_environment if retained_environment is not None else database_environment()
    if database_scalar("SELECT to_regclass('public.projects') IS NOT NULL", env) != 't':
        return
    changed = database_scalar("SELECT count(*) FROM projects WHERE COALESCE(source_metadata::jsonb->'cuan_workspace_versions','{}'::jsonb) <> '{}'::jsonb", env)
    if int(changed):
        raise RuntimeError('This release cannot resume tasks with retained project workspaces. Activate a workspace-aware release; no data was changed.')


def assert_task_worktree_support(manifest, retained_environment=None):
    if manifest.get('features', {}).get('task_execution_worktrees') == 1:
        return
    env = retained_environment if retained_environment is not None else database_environment()
    if database_scalar("SELECT to_regclass('public.tasks') IS NOT NULL", env) != 't':
        return
    count = database_scalar("SELECT count(*) FROM tasks WHERE instructions LIKE '%\"isolated_repository\":true%'", env)
    if int(count):
        raise RuntimeError('This release cannot resume isolated task worktrees. Activate a worktree-aware release; no data was changed.')


def assert_no_accidental_downgrade(old_dir, target_dir, base, allow_divergent=False):
    if allow_divergent or not old_dir.exists() or not target_dir.exists():
        return
    old_manifest_path = old_dir / 'release.json'
    target_manifest_path = target_dir / 'release.json'
    if not old_manifest_path.exists() or not target_manifest_path.exists():
        return
    try:
        old_sha = json.loads(old_manifest_path.read_text()).get('sha', '')
        target_sha = json.loads(target_manifest_path.read_text()).get('sha', '')
        if old_sha and target_sha and old_sha != target_sha:
            git_dir = base / 'data/projects/default/.git'
            if git_dir.exists():
                has_old = subprocess.run(
                    ['git', '--git-dir', str(git_dir), 'cat-file', '-e', old_sha + '^{commit}'],
                    capture_output=True
                ).returncode == 0
                has_target = subprocess.run(
                    ['git', '--git-dir', str(git_dir), 'cat-file', '-e', target_sha + '^{commit}'],
                    capture_output=True
                ).returncode == 0
                if has_old and has_target:
                    res = subprocess.run(
                        ['git', '--git-dir', str(git_dir), 'merge-base', '--is-ancestor', old_sha, target_sha],
                        capture_output=True
                    )
                    if res.returncode != 0:
                        raise RuntimeError(
                            f'Regression guard: target release {target_sha[:10]} does not contain active release {old_sha[:10]}. '
                            'Activation aborted to prevent overwriting merged production state. Use --allow-divergent to override.'
                        )
    except RuntimeError:
        raise
    except Exception:
        pass


def backup_database(release):
    backups=BASE/'backups'
    backups.mkdir(exist_ok=True,mode=0o700)
    target=backups/(release+'.dump')
    if target.exists():
        if target.stat().st_mode & 0o077 or not target.stat().st_size:
            raise RuntimeError('Existing backup is empty or not private')
        subprocess.run(['pg_restore', '--list', str(target)], stdout=subprocess.DEVNULL, check=True)
        return
    env=database_environment()
    staged=target.with_suffix('.pending')
    descriptor=os.open(staged,os.O_WRONLY|os.O_CREAT|os.O_TRUNC,0o600)
    with os.fdopen(descriptor,'wb') as output:
        subprocess.run(['pg_dump','--format=custom','--no-owner','--no-privileges'],env=env,stdout=output,check=True)
    subprocess.run(['pg_restore', '--list', str(staged)], stdout=subprocess.DEVNULL, check=True)
    os.replace(staged,target)
    print('Private pre-release database backup created; no migrations will run.')


def main():
    global BASE, SERVICE, PORT, SCHEMA_COUNT
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--release')
    group.add_argument('--rollback', action='store_true')
    parser.add_argument('--base', type=Path, default=BASE, help='Native installation directory (isolated drill uses its own base)')
    parser.add_argument('--service', default=SERVICE, help='Existing systemd service')
    parser.add_argument('--port', type=int, default=PORT, help='Loopback health port')
    parser.add_argument('--schema-count', type=int, choices=(19, 20, 21, 22, 23, 24, 25, 26, 27), default=SCHEMA_COUNT, help='Exact applied DB count, including on application rollback')
    parser.add_argument('--allow-divergent', action='store_true', help='Allow activating a release that does not contain active release commits')
    parser.add_argument('--no-restart', action='store_true', help='Skip service restart for zero-downtime static asset releases')
    args = parser.parse_args()
    BASE, SERVICE, PORT, SCHEMA_COUNT = args.base.resolve(), args.service, args.port, args.schema_count
    if not re.fullmatch(r'[a-zA-Z0-9_-]+', SERVICE) or not 1 <= PORT <= 65535:
        parser.error('Invalid service or port')
    if os.geteuid() != 0:
        parser.error('Run with sudo on the deployment host')
    with (BASE / '.release.lock').open('w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.release:
            if not re.fullmatch(r'[0-9a-f]{40}',args.release): parser.error('Invalid release SHA')

        current = BASE / 'current'
        previous = BASE / 'previous'
        if not current.exists():
            baseline = BASE / 'releases/legacy-before-s25'
            baseline.mkdir(parents=True, exist_ok=True)
            for name in ('bin', 'web', 'migrations', 'scripts'):
                if (BASE / name).exists() and not (baseline / name).exists():
                    shutil.copytree(BASE / name, baseline / name)
            if (BASE / 'README.md').exists():
                shutil.copy2(BASE / 'README.md', baseline / 'README.md')
            switch(current, baseline)
        old = current.resolve()
        if args.rollback:
            if not previous.exists():
                parser.error('No previous release is retained')
            target = previous.resolve()
        else:
            if not re.fullmatch(r'[0-9a-f]{40}', args.release):
                parser.error('Release must be a complete Git SHA')
            target = BASE / 'releases' / args.release
        manifest = json.loads((target / 'release.json').read_text())
        if manifest.get('sha') != target.name:
            parser.error('Release manifest mismatch')
        import hashlib
        checks = manifest.get('sha256', {})
        required = {'bin/cuan-server', 'web/dist/index.html'}
        actual = {str(p.relative_to(target)) for p in target.rglob('*') if p.is_file() and not p.is_symlink() and p.name != 'release.json' and not {'data', 'sponsors'}.intersection(p.relative_to(target).parts)}
        if not required.issubset(checks) or set(checks) != actual:
            parser.error('Release manifest file set mismatch')
        for name, expected in manifest['sha256'].items():
            path = (target / name).resolve()
            if not path.is_relative_to(target.resolve()) or hashlib.sha256(path.read_bytes()).hexdigest() != expected:
                parser.error('Release payload checksum mismatch')
        if not (target / 'bin/cuan-server').is_file() or not (target / 'web/dist/index.html').is_file():
            parser.error('Incomplete release')
        if args.no_restart and old.exists():
            old_binary = old / 'bin/cuan-server'
            target_binary = target / 'bin/cuan-server'
            if old_binary.exists() and target_binary.exists():
                if hashlib.sha256(old_binary.read_bytes()).hexdigest() != hashlib.sha256(target_binary.read_bytes()).hexdigest():
                    raise RuntimeError('Cannot use --no-restart when bin/cuan-server binary has changed; service restart is required')
        assert_idle_work(args.no_restart)
        assert_project_workspace_support(manifest)
        assert_task_worktree_support(manifest)
        if args.release and not args.rollback:
            assert_no_accidental_downgrade(old, target, BASE, args.allow_divergent)
        if args.release:
            backup_database(args.release)
        for release in (old, target):
            for name in ('data', 'sponsors'):
                if not (release / name).exists():
                    (release / name).symlink_to(BASE / name)
        unit_dir = Path('/etc/systemd/system/' + SERVICE + '.service.d')
        unit_dir.mkdir(exist_ok=True)
        # systemd EnvironmentFile overrides Environment, even in drop-ins.
        # Load these non-secret release settings after the retained private .env.
        (BASE / 'release.env').write_text(
            'PATH=/home/ubuntu/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n'
            f'CUAN_STATIC_DIR={BASE}/current/web/dist\nCUAN_AUTO_MIGRATE=false\n')
        (unit_dir / 'release.conf').write_text(
            f'[Service]\nEnvironmentFile={BASE}/release.env\nWorkingDirectory={BASE}/current\nExecStart=\n'
            f'ExecStart={BASE}/current/bin/cuan-server\n'
            'Environment=PATH=/home/ubuntu/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n'
            f'Environment=CUAN_STATIC_DIR={BASE}/current/web/dist\n'
            'Environment=CUAN_AUTO_MIGRATE=false\n')
        subprocess.run(['systemctl', 'daemon-reload'], check=True)
        retained_environment = database_environment()
        assert_idle_work(args.no_restart)  # Recheck after backup and unit preparation.
        switch(current, target)
        target_sha = (manifest.get('sha') if manifest else None) if not args.no_restart else None
        check_healthy = (lambda: healthy(target_sha)) if args.no_restart else (lambda: restart_and_check(target_sha))
        if not check_healthy():
            old_manifest = json.loads((old / 'release.json').read_text())
            assert_project_workspace_support(old_manifest, retained_environment)
            assert_task_worktree_support(old_manifest, retained_environment)
            switch(current, old)
            old_sha = old_manifest.get('sha')
            old_healthy = (lambda: healthy(old_sha)) if args.no_restart else (lambda: restart_and_check(old_sha))
            recovered = old_healthy()
            raise SystemExit('Release health failed; rollback ' + ('healthy' if recovered else 'requires intervention'))
        switch(previous, old)
        print(json.dumps({'release': target.name, 'previous': old.name, 'health': 'healthy', 'migrations': SCHEMA_COUNT}))


if __name__ == '__main__':
    main()
