#!/usr/bin/env python3
"""Activate or roll back a versioned native Cuan release (run as root on VPS)."""
import argparse
from contextlib import contextmanager
import hashlib
import fcntl
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import time
import tempfile
import urllib.request
import urllib.parse

BASE = Path('/opt/cuan')
SERVICE = 'cuan'
PORT = 8080
SCHEMA_COUNT = 45


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
                    if target_sha and body.get('commit_sha') != target_sha:
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
    for _ in range(60):
        if healthy(target_sha=target_sha):
            return True
        time.sleep(1)
    return False



def read_service_environment():
    try:
        pid = subprocess.check_output(['systemctl', 'show', SERVICE, '-p', 'MainPID', '--value'], text=True).strip()
        if pid and pid != '0' and (Path('/proc') / pid / 'environ').exists():
            return dict(entry.decode(errors='ignore').split('=', 1) for entry in (Path('/proc') / pid / 'environ').read_bytes().split(b'\0') if b'=' in entry)
    except Exception:
        pass
    env = {}
    for candidate in [BASE / '.env', Path('/opt/cuan/.env')]:
        if candidate.exists():
            try:
                for line in candidate.read_text().splitlines():
                    line = line.strip()
                    if line and not line.startswith('#') and '=' in line:
                        k, v = line.split('=', 1)
                        env[k.strip()] = v.strip().strip('"\'')
                break
            except Exception:
                pass
    return env


def database_environment():
    config = read_service_environment()
    db_url = config.get('DATABASE_URL', '')
    if not db_url:
        return os.environ.copy()
    url = urllib.parse.urlparse(db_url)
    env = os.environ.copy()
    env.update(
        PGHOST=url.hostname or 'localhost',
        PGPORT=str(url.port or 5432),
        PGUSER=urllib.parse.unquote(url.username or ''),
        PGPASSWORD=urllib.parse.unquote(url.password or ''),
        PGDATABASE=url.path.lstrip('/'),
        PGCONNECT_TIMEOUT='5',
    )
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
        try:
            control_group = subprocess.check_output(
                ['systemctl', 'show', SERVICE, '-p', 'ControlGroup', '--value'], text=True).strip()
        except Exception:
            return
    if not control_group or control_group == '/' or control_group == 'none':
        return
    if not control_group.startswith('/') or '..' in Path(control_group).parts:
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

        allow_resume = os.environ.get('CUAN_DEPLOY_ALLOW_RESUME', '').strip().lower() in ('1', 'true', 'yes')
        if not allow_resume:
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


def inbox_admission_lock():
    config = read_service_environment()
    workspace = Path(config.get('CUAN_WORKSPACES_DIR', '/opt/cuan/data/workspaces'))
    if not workspace.is_absolute():
        workspace = BASE / workspace
    path = workspace.parent / 'inbox-admission.lock'
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o660)
    # The first activation may create the file as root before the server does.
    try:
        pid = subprocess.check_output(['systemctl', 'show', SERVICE, '-p', 'MainPID', '--value'], text=True).strip()
        process = Path('/proc') / pid
        if pid and pid != '0' and process.exists():
            identity = process.stat()
            os.fchown(descriptor, identity.st_uid, identity.st_gid)
        return os.fdopen(descriptor, 'a+')
    except Exception:
        os.close(descriptor)
        raise


@contextmanager
def drain_inbox_admission(no_restart=False, timeout=660, wait_active=True):
    if no_restart:
        yield
        return
    env = database_environment()
    deadline = time.monotonic() + timeout
    with inbox_admission_lock() as admission:
        while True:
            try:
                fcntl.flock(admission, fcntl.LOCK_EX | fcntl.LOCK_NB)
                break
            except BlockingIOError:
                if time.monotonic() >= deadline:
                    raise RuntimeError('Inbox admission did not drain; release was not switched')
                time.sleep(1)
        # Queued requests are durable and must not prevent activation. This gate
        # is mandatory even when resumable task work is explicitly permitted.
        try:
            while wait_active:
                if database_scalar("SELECT to_regclass('public.inbox_requests') IS NOT NULL", env) != 't':
                    break
                active = int(database_scalar("SELECT count(*) FROM inbox_requests WHERE state IN ('running','cancelling')", env))
                if active == 0:
                    break
                if time.monotonic() >= deadline:
                    raise RuntimeError('Active Inbox conversations did not finish; release was not switched')
                print(f'==> Waiting for {active} active Inbox conversation(s); new messages remain queued.', flush=True)
                time.sleep(1)
            yield
        finally:
            fcntl.flock(admission, fcntl.LOCK_UN)


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
    if allow_divergent or not old_dir.exists():
        return  # Explicit operator rollback, never an automatic fallback.
    if old_dir.name == 'legacy-before-s25' and not (old_dir / 'release.json').exists():
        return  # First native migration has no earlier versioned release.
    try:
        old_data = json.loads((old_dir / 'release.json').read_text())
        old_sha = old_data.get('sha') or old_data.get('commit')
        target_data = json.loads((target_dir / 'release.json').read_text())
        target_sha = target_data.get('sha') or target_data.get('commit')
    except (OSError, ValueError, KeyError) as error:
        raise RuntimeError('Release ancestry metadata is unavailable; activation was not changed') from error
    if not all(re.fullmatch('[0-9a-f]{40}', value) for value in (old_sha, target_sha)):
        raise RuntimeError('Release ancestry identity is invalid; activation was not changed')
    if old_sha == target_sha:
        return
    environment = {'PATH': os.defpath, 'GIT_CONFIG_GLOBAL': '/dev/null',
                   'GIT_CONFIG_NOSYSTEM': '1', 'GIT_TERMINAL_PROMPT': '0',
                   'GIT_NO_REPLACE_OBJECTS': '1', 'GIT_NO_LAZY_FETCH': '1'}

    def prove(git_dir):
        # Work only with local objects, without hooks, replace refs, grafts,
        # repository-owned configuration or credential-dependent lazy fetching.
        grafts = git_dir / 'info/grafts'
        if grafts.exists() and grafts.stat().st_size:
            raise RuntimeError('Release ancestry cannot use Git grafts')
        for sha in (old_sha, target_sha):
            result = subprocess.run(['git', '--git-dir', str(git_dir), 'cat-file', '-e', sha + '^{commit}'],
                                    env=environment, capture_output=True, timeout=10)
            if result.returncode:
                raise RuntimeError('Release ancestry objects are unavailable; reconcile source history before activation')
        result = subprocess.run(['git', '--git-dir', str(git_dir), 'merge-base', '--is-ancestor', old_sha, target_sha],
                                env=environment, capture_output=True, timeout=10)
        if result.returncode:
            raise RuntimeError('Regression guard: candidate does not provably contain the active release; activation was not changed')

    pack = target_dir / 'source-commits.pack'
    try:
        if pack.is_file() and not pack.is_symlink():
            if not 0 < pack.stat().st_size <= 64 * 1024 * 1024:
                raise RuntimeError('Release ancestry pack size is invalid')
            # The payload manifest validates this file before this gate. Commit
            # objects carry their real parent hashes; a list of claimed SHAs
            # cannot establish ancestry. No trees/blobs or source checkout needed.
            with tempfile.TemporaryDirectory(prefix='.release-ancestry-', dir=base) as temporary:
                graph = Path(temporary)
                subprocess.run(['git', 'init', '--bare', '--quiet', str(graph)], env=environment,
                               capture_output=True, timeout=10, check=True)
                with pack.open('rb') as source:
                    subprocess.run(['git', '--git-dir', str(graph), 'index-pack', '--stdin'],
                                   stdin=source, env=environment, capture_output=True, timeout=30, check=True)
                prove(graph)
        else:
            # Older retained payloads may use the registered native cache, but
            # missing objects are a blocker, never permission to skip the gate.
            graph = base / 'data/projects/default/.git'
            if not graph.is_dir():
                raise RuntimeError('Release source history is unavailable; activation was not changed')
            prove(graph)
    except (OSError, subprocess.SubprocessError) as error:
        raise RuntimeError('Release ancestry could not be verified; activation was not changed') from error


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
    print('Private pre-release database backup created.')


def apply_pending_migrations(target_dir, env):
    cuan_migrate = target_dir / 'bin' / 'cuan-migrate'
    if cuan_migrate.exists():
        try:
            config = read_service_environment()
            db_url = config.get('DATABASE_URL', '')
            if db_url:
                print('==> Executing cuan-migrate against database...')
                res = subprocess.run([str(cuan_migrate), '--db-url', db_url, '--direction', 'up'], capture_output=True, text=True, timeout=30)
                if res.returncode == 0:
                    print(res.stdout.strip())
                    return
                print(f"Notice: cuan-migrate failed: {res.stderr}; falling back to SQL migration files.")
        except Exception as e:
            print(f"Notice: cuan-migrate execution skipped ({e}); falling back to SQL migration files.")

    mig_dir = target_dir / 'migrations'
    if not mig_dir.exists():
        return
    for up_file in sorted(mig_dir.glob('*.up.sql')):
        version = up_file.name.split('_')[0]
        try:
            applied = database_scalar(f"SELECT count(*) FROM cuan_schema_migrations WHERE version = '{version}'", env)
            if int(applied) == 0:
                print(f"==> Applying migration {up_file.name} to database...")
                result = subprocess.run(['psql', '-XAt', '-v', 'ON_ERROR_STOP=1', '-f', str(up_file)], env=env, capture_output=True, text=True)
                if result.returncode != 0:
                    raise RuntimeError(f"Migration {up_file.name} failed: {result.stderr}")
                mig_name = up_file.name[len(version)+1:-len('.up.sql')]
                database_scalar(f"INSERT INTO cuan_schema_migrations (version, name, applied_at) VALUES ('{version}', '{mig_name}', NOW())", env)
                print(f"✓ Migration {up_file.name} applied successfully.")
        except RuntimeError:
            raise
        except Exception as e:
            raise RuntimeError(f"Could not apply migration {up_file.name}: {e}")


def drain_file_path():
    return Path(os.environ.get('CUAN_DRAIN_FILE', '/tmp/cuan-drain'))


def signal_worker_drain():
    try:
        drain_file = drain_file_path()
        drain_file.touch(mode=0o666, exist_ok=True)
        try:
            os.chmod(drain_file, 0o666)
        except Exception:
            pass
        cmd = ['systemctl', 'kill', '--kill-whom=main', '--signal=USR1', SERVICE+'-runner.service']
        if os.getuid() != 0:
            cmd = ['sudo', '-n'] + cmd
        subprocess.run(cmd, check=False)
        print(f'==> Signaled graceful drain to the task-runner supervisor only ({drain_file} & SIGUSR1).')
    except Exception as e:
        print(f'==> Notice: Could not signal worker drain: {e}')


def cleanup_worker_drain():
    try:
        drain_file_path().unlink(missing_ok=True)
        print('==> Cleaned up drain flag.')
    except Exception:
        pass


def active_workers_dir():
    return Path(os.environ.get('CUAN_WORKERS_DIR', '/tmp/cuan-workers'))


def has_active_detached_workers():
    workers_dir = active_workers_dir()
    if not workers_dir.is_dir():
        return False
    for json_file in workers_dir.glob('att_*.json'):
        try:
            data = json.loads(json_file.read_text())
            pid = data.get('pid')
            if pid and isinstance(pid, int):
                os.kill(pid, 0)
                return True
        except (OSError, ValueError):
            continue
    return False


def restart_worker_service():
    """Auto-restart cuan-runner service when idle, preserving active in-flight worker attempts."""
    try:
        cleanup_worker_drain()
        if has_active_detached_workers():
            print('==> Notice: Active detached worker detected in cuan-workers; deferring cuan-runner restart to avoid interrupting in-flight tasks.')
            return
        cmd = ['systemctl', 'restart', 'cuan-runner']
        if os.getuid() != 0:
            cmd = ['sudo', '-n'] + cmd
        res = subprocess.run(cmd, check=False, capture_output=True, text=True)
        if res.returncode == 0:
            print('==> Successfully restarted cuan-runner.service on new release.')
        else:
            err_msg = res.stderr.strip() if res.stderr else 'unit not found or inactive'
            print(f'==> Notice: cuan-runner restart returned code {res.returncode}: {err_msg}')
    except Exception as e:
        print(f'==> Notice: Could not restart cuan-runner: {e}')


def supports_chat_worker(manifest):
    return (manifest or {}).get('features', {}).get('inbox_worker') == 1


def authorize_service_releases(base, service, current_sha, target_sha, unit_root=Path('/etc/systemd/system')):
    """Keep legacy workspace helpers from starting an unreviewed release.

    Call only after native payload/ancestry gates. This is service lifecycle
    protection, not a privilege boundary against arbitrary root execution.
    """
    if not re.fullmatch('[0-9a-f]{40}', target_sha or ''):
        raise RuntimeError('Native service authorization requires exact release identities')
    if current_sha and not re.fullmatch('[0-9a-f]{40}', current_sha):
        current_sha = None
    if not base.is_absolute() or base.resolve() != base or not re.fullmatch(r'/[a-zA-Z0-9_./-]+', str(base)):
        raise RuntimeError('Invalid native policy base')
    if not re.fullmatch('[a-zA-Z0-9_-]+', service):
        raise RuntimeError('Invalid native service')
    root = base / 'maintenance/activation-policy'
    if root.is_symlink():
        raise RuntimeError('Native activation policy must not be linked')
    root.mkdir(parents=True, exist_ok=True)
    root.chmod(0o755)
    source = Path(__file__).with_name('verify-authorized-release.py')
    if source.is_symlink() or not source.is_file():
        raise RuntimeError('Native service authorization verifier is unavailable')
    allowed = [target_sha]
    if current_sha:
        allowed.insert(0, current_sha)
    files = {
        root / 'verify-authorized-release.py': source.read_bytes(),
        root / 'authorized.json': (json.dumps({'version': 1, 'allowed': list(dict.fromkeys(allowed))}) + '\n').encode(),
    }
    for destination, content in files.items():
        descriptor, temporary = tempfile.mkstemp(prefix='.policy-', dir=root)
        try:
            with os.fdopen(descriptor, 'wb') as handle:
                handle.write(content)
            os.chmod(temporary, 0o644)
            os.replace(temporary, destination)
        finally:
            if os.path.exists(temporary):
                os.unlink(temporary)
    for name, argument in [(service, ''), (service+'-runner', ''), (service+'-chat@', ' --release %i')]:
        directory = unit_root / (name + '.service.d')
        directory.mkdir(parents=True, exist_ok=True)
        (directory / '90-native-activation-policy.conf').write_text(
            '[Service]\nExecStartPre=/usr/bin/python3 ' + str(root / 'verify-authorized-release.py') +
            ' --base ' + str(base) + argument + '\n')


def configure_api_release(chat_worker=False):
    unit_dir = Path('/etc/systemd/system/' + SERVICE + '.service.d')
    unit_dir.mkdir(exist_ok=True)
    (BASE / 'release.env').write_text(
        'PATH=/home/ubuntu/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n'
        f'CUAN_STATIC_DIR={BASE}/current/web/dist\nCUAN_AUTO_MIGRATE=false\n')
    flag = ' --external-chat-worker' if chat_worker else ''
    (unit_dir / 'release.conf').write_text(
        f'[Service]\nEnvironmentFile={BASE}/release.env\nWorkingDirectory={BASE}/current\nExecStart=\n'
        f'ExecStart={BASE}/current/bin/cuan-server{flag}\n'
        'Environment=PATH=/home/ubuntu/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n'
        f'Environment=CUAN_STATIC_DIR={BASE}/current/web/dist\nEnvironment=CUAN_AUTO_MIGRATE=false\n'
        'KillMode=process\n')


def chat_worker_unit(base, service, user, group=''):
    if not re.fullmatch(r'[a-zA-Z_][a-zA-Z0-9_-]*', user):
        raise RuntimeError('Chat worker requires an explicit service user')
    if group and not re.fullmatch(r'[a-zA-Z_][a-zA-Z0-9_-]*', group):
        raise RuntimeError('Chat worker service group is invalid')
    return ('# Managed by Cuan release activation: Inbox worker v1\n'
        '[Unit]\nDescription=Cuan chat worker %i\nAfter=network-online.target\nWants=network-online.target\n'
        'StartLimitIntervalSec=0\n\n[Service]\nType=simple\nUser='+user+'\n'+('Group='+group+'\n' if group else '')+
        f'WorkingDirectory={base}/releases/%i\nEnvironmentFile={base}/.env\nEnvironmentFile={base}/release.env\n'
        f'ExecStart={base}/releases/%i/bin/cuan-server --chat-worker\n'
        f'RuntimeDirectory={service}-chat-%i\nEnvironment=CUAN_CHAT_WORKER_READY_FILE=/run/{service}-chat-%i/ready.json\n'
        'UMask=0077\nRestart=on-failure\nRestartSec=2\nKillMode=mixed\nTimeoutStopSec=430\n\n'
        '[Install]\nWantedBy=multi-user.target\n')


def start_chat_worker(manifest, timeout=45):
    sha = manifest.get('sha', '')
    if not re.fullmatch(r'[0-9a-f]{40}', sha):
        raise RuntimeError('Chat worker requires a verified release identity')
    if not (BASE / '.env').is_file():
        raise RuntimeError('Chat worker requires the retained native environment file')
    props = subprocess.check_output(['systemctl','show',SERVICE,'-p','User','-p','Group'],text=True)
    identity = dict(line.split('=',1) for line in props.splitlines() if '=' in line)
    template = Path('/etc/systemd/system/'+SERVICE+'-chat@.service')
    text = chat_worker_unit(BASE, SERVICE, identity.get('User',''), identity.get('Group',''))
    if template.is_symlink() or (template.exists() and not template.read_text().startswith('# Managed by Cuan release activation: Inbox worker v1\n')):
        raise RuntimeError('An unrelated chat worker service exists; no unit was overwritten')
    template.write_text(text)
    subprocess.run(['systemctl','daemon-reload'],check=True)
    instance = SERVICE+'-chat@'+sha+'.service'
    try:
        subprocess.run(['systemctl','enable','--now',instance],check=True)
        deadline = time.monotonic()+timeout
        ready_file = Path('/run')/(SERVICE+'-chat-'+sha)/'ready.json'
        while time.monotonic()<deadline:
            try:
                ready = json.loads(ready_file.read_text())
                pid = subprocess.check_output(['systemctl','show',instance,'-p','MainPID','--value'],text=True).strip()
                if ready.get('release')==sha and str(ready.get('pid'))==pid and Path('/proc',pid,'exe').resolve()==BASE/'releases'/sha/'bin/cuan-server':
                    return instance
            except (OSError, ValueError):
                pass
            time.sleep(.2)
        raise RuntimeError('Chat worker did not become ready; activation needs rollback')
    except Exception:
        subprocess.run(['systemctl','disable','--now',instance],check=False)
        raise


def drain_old_chat_workers(keep=None):
    result = subprocess.run(['systemctl','list-units',SERVICE+'-chat@*.service','--all','--no-legend','--plain'],capture_output=True,text=True,check=True)
    pattern = re.compile(re.escape(SERVICE)+r'-chat@[0-9a-f]{40}\.service')
    for line in result.stdout.splitlines():
        parts = line.split()
        if not parts or not pattern.fullmatch(parts[0]) or parts[0]==keep:
            continue
        subprocess.run(['systemctl','disable',parts[0]],check=True,capture_output=True)
        # SIGTERM goes only to the supervisor. Existing providers finish while
        # the ready new generation admits other conversations immediately.
        subprocess.run(['systemctl','stop','--no-block',parts[0]],check=True)


def record_task_deployment(env, release_sha):
    cap_file = os.environ.get('CUAN_TASK_CAPABILITY_FILE')
    task_id = os.environ.get('CUAN_DEPLOY_EXCLUDE_TASK', '').strip()
    attempt_id = os.environ.get('CUAN_DEPLOY_EXCLUDE_ATTEMPT', '').strip()
    if cap_file and Path(cap_file).is_file():
        cap_data = json.loads(Path(cap_file).read_text())
        task_id = task_id or str(cap_data.get('task_id', '')).strip()
        attempt_id = attempt_id or str(cap_data.get('attempt_id', '')).strip()
    if not task_id and not attempt_id:
        return None
    # A deployment is an external fact, not a task completion report. A missing
    # attempt binding cannot be repaired by updating whichever task is running.
    if not task_id or not attempt_id:
        return None
    if not re.fullmatch(r'[a-zA-Z0-9_-]{1,128}', task_id) or not re.fullmatch(r'[a-zA-Z0-9_-]{1,128}', attempt_id):
        raise ValueError('Invalid task deployment binding')
    if not re.fullmatch(r'[0-9a-f]{40}', release_sha or ''):
        raise ValueError('Invalid verified release SHA')
    receipt_id = 'act_deploy_' + hashlib.sha256((task_id+'\0'+attempt_id+'\0'+release_sha).encode()).hexdigest()[:32]
    payload = json.dumps(dict(version=1, task_id=task_id, attempt_id=attempt_id,
                              commit_sha=release_sha, release_sha=release_sha,
                              environment='production', health='healthy'), separators=(',', ':'))
    query = f"""
    WITH receipt AS (
      INSERT INTO task_activities(id,task_id,attempt_id,activity_type,author,message,created_at)
      SELECT '{receipt_id}',t.id,a.id,'deployment_verified','cuan-deploy','{payload}',NOW()
      FROM tasks t JOIN task_attempts a ON a.task_id=t.id
      WHERE t.id='{task_id}' AND a.id='{attempt_id}'
      ON CONFLICT(id) DO NOTHING RETURNING id
    ) SELECT EXISTS(SELECT id FROM receipt UNION ALL SELECT id FROM task_activities WHERE id='{receipt_id}');
    """
    if database_scalar(query, env) != 't':
        raise RuntimeError('Verified deployment could not be bound to its task attempt')
    print('==> Recorded verified deployment evidence; task acceptance remains separate.')
    return receipt_id


def verify_and_record_task_deployment(env, release_sha, no_restart=False):
    check = healthy if no_restart else restart_and_check
    if not check(release_sha):
        return False
    # Static-only activation cannot establish which server binary is running.
    if release_sha and not no_restart:
        try:
            record_task_deployment(env, release_sha)
        except Exception:
            # A receipt write cannot turn a healthy release into a rollback or
            # strand the worker drain. Missing evidence never completes a task.
            print('==> Deployment evidence was not saved; no task was accepted.')
    return True


def main():
    global BASE, SERVICE, PORT, SCHEMA_COUNT
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--release')
    group.add_argument('--rollback', action='store_true')
    parser.add_argument('--base', type=Path, default=BASE, help='Native installation directory (isolated drill uses its own base)')
    parser.add_argument('--service', default=SERVICE, help='Existing systemd service')
    parser.add_argument('--port', type=int, default=PORT, help='Loopback health port')
    parser.add_argument('--schema-count', type=int, choices=(19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45), default=SCHEMA_COUNT, help='Exact applied DB count, including on application rollback')
    parser.add_argument('--allow-divergent', action='store_true', help='Allow activating a release that does not contain active release commits')
    parser.add_argument('--no-restart', action='store_true', help='Skip service restart for zero-downtime static asset releases')
    args = parser.parse_args()
    if args.allow_divergent and (os.environ.get('CUAN_TASK_CAPABILITY_FILE') or os.environ.get('CUAN_TASK_CAPABILITY_JSON')):
        parser.error('Task executions cannot override release ancestry; retain saved work for publication reconciliation')
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
            apply_pending_migrations(target, database_environment())
        for release in (old, target):
            for name in ('data', 'sponsors'):
                if not (release / name).exists():
                    (release / name).symlink_to(BASE / name)
        old_manifest = json.loads((old / 'release.json').read_text()) if (old / 'release.json').exists() else {}
        chat_worker = supports_chat_worker(manifest)
        if args.no_restart and chat_worker != supports_chat_worker(old_manifest):
            raise RuntimeError('Changing Inbox process roles requires a supervised activation, not a static-only switch')
        configure_api_release(chat_worker)
        subprocess.run(['systemctl', 'daemon-reload'], check=True)
        retained_environment = database_environment()
        with drain_inbox_admission(args.no_restart, wait_active=not (chat_worker and supports_chat_worker(old_manifest))):
            assert_idle_work(args.no_restart)  # Recheck after backup and unit preparation.
            cuan_bin = target / 'bin' / 'cuan'
            if not cuan_bin.exists() and not cuan_bin.is_symlink():
                try:
                    cuan_bin.symlink_to('cuan-server')
                except Exception:
                    pass
            # First migration retains its legacy rollback behavior. Service
            # authorization follows all native preconditions and migrations.
            authorize_service_releases(BASE, SERVICE, old.name if re.fullmatch('[0-9a-f]{40}', old.name) else None, target.name)
            subprocess.run(['systemctl', 'daemon-reload'], check=True)
            signal_worker_drain()
            switch(current, target)
            target_sha = (manifest.get('sha') if manifest else None) if not args.no_restart else None
            chat_instance = None
            activation_ok = False
            try:
                if chat_worker and not args.no_restart:
                    chat_instance = start_chat_worker(manifest)
                activation_ok = verify_and_record_task_deployment(retained_environment, target_sha, args.no_restart)
            except Exception:
                print('==> Worker or API activation failed; restoring the prior release.', flush=True)
            if not activation_ok:
                if chat_instance:
                    subprocess.run(['systemctl','disable','--now',chat_instance],check=False)
                cleanup_worker_drain()
                configure_api_release(supports_chat_worker(old_manifest))
                subprocess.run(['systemctl','daemon-reload'],check=True)
                assert_project_workspace_support(old_manifest, retained_environment)
                assert_task_worktree_support(old_manifest, retained_environment)
                switch(current, old)
                old_sha = old_manifest.get('sha')
                old_healthy = (lambda: healthy(old_sha)) if args.no_restart else (lambda: restart_and_check(old_sha))
                recovered = old_healthy()
                if supports_chat_worker(old_manifest) and not args.no_restart:
                    try:
                        start_chat_worker(old_manifest)
                    except Exception as err:
                        print(f'==> Warning: Failed to restart prior chat worker on rollback: {err}', flush=True)
                restart_worker_service()
                raise SystemExit('Release health failed; rollback ' + ('healthy' if recovered else 'requires intervention'))
            if not args.no_restart:
                drain_old_chat_workers(keep=chat_instance)
            cleanup_worker_drain()
            restart_worker_service()
            switch(previous, old)
            print(json.dumps({'release': target.name, 'previous': old.name, 'health': 'healthy', 'migrations': SCHEMA_COUNT}))



if __name__ == '__main__':
    main()
