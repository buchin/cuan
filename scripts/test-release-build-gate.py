#!/usr/bin/env python3
"""Regression tests for parallel build gate and child process cleanup in scripts/deploy.sh."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[1]
DEPLOY_SCRIPT = ROOT / "scripts" / "deploy.sh"


class ReleaseBuildGateTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory(prefix="cuan-build-gate-")
        self.addCleanup(self.temp_dir.cleanup)
        self.base = Path(self.temp_dir.name)

        # Isolated repository fixture
        self.repo = self.base / "repo"
        self.repo.mkdir()

        # Isolated home and release destination
        self.home = self.base / "home"
        self.home.mkdir()

        # Tracking directory for started child process PIDs
        self.track_dir = self.base / "track"
        self.track_dir.mkdir()

        # Tools directory containing the stub `go` compiler
        self.tools = self.base / "tools"
        self.tools.mkdir()

        self._init_fixture_repository()
        self._create_stub_go()

    def _init_fixture_repository(self):
        git_env = {
            **os.environ,
            "GIT_AUTHOR_NAME": "Cuan Release Test",
            "GIT_AUTHOR_EMAIL": "test@cuan.local",
            "GIT_COMMITTER_NAME": "Cuan Release Test",
            "GIT_COMMITTER_EMAIL": "test@cuan.local",
        }
        subprocess.run(["git", "init", "--quiet", "-b", "dev", str(self.repo)], env=git_env, check=True)
        subprocess.run(["git", "-C", str(self.repo), "remote", "add", "origin", "https://github.com/buchin/cuan.git"], check=True)

        # Create minimal directory tree expected by deploy.sh
        (self.repo / "scripts").mkdir()
        (self.repo / "cmd" / "server").mkdir(parents=True)
        (self.repo / "cmd" / "migrate").mkdir(parents=True)
        (self.repo / "cmd" / "runner").mkdir(parents=True)
        (self.repo / "web" / "dist").mkdir(parents=True)
        (self.repo / "migrations").mkdir()

        (self.repo / "README.md").write_text("# Cuan Fixture\n")
        (self.repo / "web" / "dist" / "index.html").write_text("<!doctype html><html><body>Cuan</body></html>\n")
        (self.repo / "migrations" / "000001_init.sql").write_text("-- init\n")

        shutil.copy(DEPLOY_SCRIPT, self.repo / "scripts" / "deploy.sh")
        shutil.copy(ROOT / "scripts" / "release-repository.py", self.repo / "scripts" / "release-repository.py")

        subprocess.run(["git", "-C", str(self.repo), "add", "."], env=git_env, check=True)
        subprocess.run(["git", "-C", str(self.repo), "commit", "--quiet", "-m", "Initial fixture"], env=git_env, check=True)

    def _create_stub_go(self):
        # Stub go executable that reads control files to determine exit codes and sleep durations,
        # logs started PIDs, and creates dummy binaries on successful builds.
        stub = self.tools / "go"
        stub.write_text(r"""#!/usr/bin/env bash
set -u

target="unknown"
out=""

args=("$@")
i=0
while [[ $i -lt ${#args[@]} ]]; do
  arg="${args[$i]}"
  case "$arg" in
    -o)
      i=$((i + 1))
      out="${args[$i]}"
      ;;
    *cmd/server*|*cuan-server*)
      target="server"
      ;;
    *cmd/migrate*|*cuan-migrate*)
      target="migrate"
      ;;
    *cmd/runner*|*cuan-runner*)
      target="runner"
      ;;
  esac
  i=$((i + 1))
done

if [[ -n "${BUILD_TRACK_DIR:-}" ]]; then
  echo "$$ $target" >> "${BUILD_TRACK_DIR}/started_pids.txt"
fi

cfg_exit="${STUB_DIR:-.}/${target}.exit"
cfg_sleep="${STUB_DIR:-.}/${target}.sleep"

exit_code=0
if [[ -f "$cfg_exit" ]]; then
  exit_code="$(cat "$cfg_exit")"
fi

sleep_dur=0.01
if [[ -f "$cfg_sleep" ]]; then
  sleep_dur="$(cat "$cfg_sleep")"
fi

if (( $(echo "$sleep_dur > 0" | bc -l 2>/dev/null || [ "$sleep_dur" != "0" ]) )); then
  sleep "$sleep_dur"
fi

if [[ "$exit_code" -eq 0 && -n "$out" ]]; then
  mkdir -p "$(dirname "$out")"
  echo "dummy-$target" > "$out"
fi

exit "$exit_code"
""")
        stub.chmod(0o755)

    def _get_tracked_pids(self):
        pids_file = self.track_dir / "started_pids.txt"
        if not pids_file.exists():
            return []
        pids = []
        for line in pids_file.read_text().splitlines():
            line = line.strip()
            if line:
                pid, target = line.split(None, 1)
                pids.append((int(pid), target))
        return pids

    def _assert_no_lingering_children(self, timeout=2.0):
        # Verify that all child processes that were started have stopped
        deadline = time.time() + timeout
        pids = self._get_tracked_pids()
        for pid, target in pids:
            while time.time() < deadline:
                try:
                    os.kill(pid, 0)
                    time.sleep(0.02)
                except OSError:
                    break
            else:
                self.fail(f"Child process {pid} ({target}) is still running after timeout!")

    def _run_deploy(self, server_cfg, migrate_cfg, runner_cfg, extra_args=None):
        cfg_dir = self.base / "cfg"
        if cfg_dir.exists():
            shutil.rmtree(cfg_dir)
        cfg_dir.mkdir()

        (cfg_dir / "server.exit").write_text(str(server_cfg.get("exit", 0)))
        (cfg_dir / "server.sleep").write_text(str(server_cfg.get("sleep", 0.01)))

        (cfg_dir / "migrate.exit").write_text(str(migrate_cfg.get("exit", 0)))
        (cfg_dir / "migrate.sleep").write_text(str(migrate_cfg.get("sleep", 0.01)))

        (cfg_dir / "runner.exit").write_text(str(runner_cfg.get("exit", 0)))
        (cfg_dir / "runner.sleep").write_text(str(runner_cfg.get("sleep", 0.01)))

        # Clean previous started PIDs log
        pids_file = self.track_dir / "started_pids.txt"
        if pids_file.exists():
            pids_file.unlink()

        env = {
            **os.environ,
            "PATH": f"{self.tools}:{os.environ['PATH']}",
            "HOME": str(self.home),
            "BUILD_TRACK_DIR": str(self.track_dir),
            "STUB_DIR": str(cfg_dir),
            "CUAN_DEPLOY_SKIP_CHECKS": "1",
            "CUAN_DEPLOY_SKIP_DEPLOY": "1",
        }
        # Clear capability variables so child script does not inherit task capability
        for cap in ("CUAN_TASK_CAPABILITY_JSON", "CUAN_TASK_CAPABILITY_FILE", "CUAN_TASK_EXECUTABLE", "CUAN_TASK_ARTIFACTS_DIR"):
            env.pop(cap, None)

        cmd = ["bash", "scripts/deploy.sh", "--branch", "dev"]
        if extra_args:
            cmd.extend(extra_args)

        return subprocess.run(cmd, cwd=str(self.repo), env=env, capture_output=True, text=True)

    def _get_release_dir(self):
        head_commit = subprocess.check_output(["git", "-C", str(self.repo), "rev-parse", "HEAD"], text=True).strip()
        return self.home / ".local" / "share" / "cuan-releases" / head_commit

    def test_server_failure_rejects_release_and_terminates_children(self):
        result = self._run_deploy(
            server_cfg={"exit": 7, "sleep": 0.02},
            migrate_cfg={"exit": 0, "sleep": 2.0},
            runner_cfg={"exit": 0, "sleep": 2.0},
        )
        self.assertNotEqual(result.returncode, 0, "Deploy must fail when cuan-server build fails")
        self.assertEqual(result.returncode, 7, "Deploy exit status should match failing build process")
        self.assertIn("cuan-server", result.stderr, "Error message must identify failed component")
        self.assertNotIn("✓ Backend binaries compiled successfully.", result.stdout)

        # Artifact publication must not have run
        release_dir = self._get_release_dir()
        self.assertFalse((release_dir / "release.json").exists(), "Artifact publication stage must not run on build failure")
        self.assertFalse((release_dir / "source-commits.pack").exists())

        # Children must be terminated
        self._assert_no_lingering_children()

    def test_migrate_failure_rejects_release_and_terminates_children(self):
        result = self._run_deploy(
            server_cfg={"exit": 0, "sleep": 2.0},
            migrate_cfg={"exit": 3, "sleep": 0.02},
            runner_cfg={"exit": 0, "sleep": 2.0},
        )
        self.assertNotEqual(result.returncode, 0, "Deploy must fail when cuan-migrate build fails")
        self.assertEqual(result.returncode, 3, "Deploy exit status should match failing build process")
        self.assertIn("cuan-migrate", result.stderr, "Error message must identify failed component")
        self.assertNotIn("✓ Backend binaries compiled successfully.", result.stdout)

        # Artifact publication must not have run
        release_dir = self._get_release_dir()
        self.assertFalse((release_dir / "release.json").exists(), "Artifact publication stage must not run on build failure")
        self.assertFalse((release_dir / "source-commits.pack").exists())

        # Children must be terminated
        self._assert_no_lingering_children()

    def test_runner_failure_rejects_release_and_terminates_children(self):
        result = self._run_deploy(
            server_cfg={"exit": 0, "sleep": 2.0},
            migrate_cfg={"exit": 0, "sleep": 2.0},
            runner_cfg={"exit": 5, "sleep": 0.02},
        )
        self.assertNotEqual(result.returncode, 0, "Deploy must fail when cuan-runner build fails")
        self.assertEqual(result.returncode, 5, "Deploy exit status should match failing build process")
        self.assertIn("cuan-runner", result.stderr, "Error message must identify failed component")
        self.assertNotIn("✓ Backend binaries compiled successfully.", result.stdout)

        # Artifact publication must not have run
        release_dir = self._get_release_dir()
        self.assertFalse((release_dir / "release.json").exists(), "Artifact publication stage must not run on build failure")
        self.assertFalse((release_dir / "source-commits.pack").exists())

        # Children must be terminated
        self._assert_no_lingering_children()

    def test_all_builds_succeed_advances_to_artifact_publication(self):
        result = self._run_deploy(
            server_cfg={"exit": 0, "sleep": 0.02},
            migrate_cfg={"exit": 0, "sleep": 0.03},
            runner_cfg={"exit": 0, "sleep": 0.01},
        )
        self.assertEqual(result.returncode, 0, f"Deploy must succeed when all builds succeed: {result.stderr}")
        self.assertIn("✓ Backend binaries compiled successfully.", result.stdout)
        self.assertIn("✓ Release artifacts packaged successfully", result.stdout)

        # Artifact publication must have succeeded
        release_dir = self._get_release_dir()
        self.assertTrue((release_dir / "release.json").exists(), "release.json must be created on success")
        self.assertTrue((release_dir / "source-commits.pack").exists(), "source-commits.pack must be created on success")
        self.assertTrue((release_dir / "bin" / "cuan-server").exists())
        self.assertTrue((release_dir / "bin" / "cuan-migrate").exists())
        self.assertTrue((release_dir / "bin" / "cuan-runner").exists())

        # Children must be reaped
        self._assert_no_lingering_children()

    def test_child_process_cleanup_when_peer_fails_quickly(self):
        start_time = time.time()
        result = self._run_deploy(
            server_cfg={"exit": 9, "sleep": 0.01},
            migrate_cfg={"exit": 0, "sleep": 10.0},
            runner_cfg={"exit": 0, "sleep": 10.0},
        )
        elapsed = time.time() - start_time
        self.assertNotEqual(result.returncode, 0)
        self.assertLess(elapsed, 2.0, f"Release abort must terminate long-running children quickly, took {elapsed:.2f}s")
        self._assert_no_lingering_children()


if __name__ == "__main__":
    unittest.main()
