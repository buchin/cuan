#!/usr/bin/env python3
"""Comprehensive unit tests for scripts/migrate-host.py."""

import importlib.util
from pathlib import Path
import unittest
from unittest.mock import MagicMock, patch

# Dynamically import scripts/migrate-host.py
spec = importlib.util.spec_from_file_location("migrate_host", Path(__file__).with_name("migrate-host.py"))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class TestMigrateHostUtilities(unittest.TestCase):
    def test_parse_database_url_standard(self):
        url = "postgres://cuan_user:cuan_pass@127.0.0.1:5432/cuan_prod?sslmode=disable"
        info = m.parse_database_url(url)
        self.assertEqual(info["user"], "cuan_user")
        self.assertEqual(info["password"], "cuan_pass")
        self.assertEqual(info["host"], "127.0.0.1")
        self.assertEqual(info["port"], 5432)
        self.assertEqual(info["dbname"], "cuan_prod")

    def test_parse_database_url_encoded_and_defaults(self):
        url = "postgresql://admin:p%40ss%23word@db-server/my_cuan"
        info = m.parse_database_url(url)
        self.assertEqual(info["user"], "admin")
        self.assertEqual(info["password"], "p@ss#word")
        self.assertEqual(info["host"], "db-server")
        self.assertEqual(info["port"], 5432)
        self.assertEqual(info["dbname"], "my_cuan")

    def test_parse_database_url_empty(self):
        info = m.parse_database_url("")
        self.assertEqual(info["user"], "cuan")
        self.assertEqual(info["dbname"], "cuan")
        self.assertEqual(info["port"], 5432)

    def test_calculate_memory_limits_high_ram(self):
        # 8GB VPS (7623 MB)
        limits = m.calculate_memory_limits(7623)
        self.assertEqual(limits["MemoryMax"], "3811M")
        self.assertEqual(limits["MemoryHigh"], "3239M")
        self.assertEqual(limits["TasksMax"], "8192")

    def test_calculate_memory_limits_mid_ram(self):
        # 4GB VPS (3659 MB)
        limits = m.calculate_memory_limits(3659)
        self.assertEqual(limits["MemoryMax"], "1829M")
        self.assertEqual(limits["MemoryHigh"], "1554M")

    def test_calculate_memory_limits_low_ram(self):
        # 2GB VPS (2000 MB)
        limits = m.calculate_memory_limits(2000)
        self.assertEqual(limits["MemoryMax"], "1000M")
        self.assertEqual(limits["MemoryHigh"], "850M")

        # 1GB VPS (1000 MB)
        limits_low = m.calculate_memory_limits(1000)
        self.assertEqual(limits_low["MemoryMax"], "700M")
        self.assertEqual(limits_low["MemoryHigh"], "550M")

    def test_ssh_runner_cmd_with_password(self):
        runner = m.SSHRunner(host="10.0.0.1", user="ubuntu", password="my_password", port=2222)
        base = runner._base_cmd()
        self.assertIn("sshpass", base)
        self.assertIn("-p", base)
        self.assertIn("my_password", base)
        self.assertIn("2222", base)
        self.assertIn("ubuntu@10.0.0.1", base)

    def test_ssh_runner_cmd_with_key(self):
        runner = m.SSHRunner(host="10.0.0.2", user="deploy", key_path="/path/to/key.pem")
        base = runner._base_cmd()
        self.assertNotIn("sshpass", base)
        self.assertIn("-i", base)
        self.assertIn("/path/to/key.pem", base)
        self.assertIn("deploy@10.0.0.2", base)


class TestCloudflareDNS(unittest.TestCase):
    def test_find_zone_for_domain(self):
        cf = m.CloudflareDNS("fake-token")
        mock_zones = [
            {"id": "zone-cuan", "name": "cuan.work"},
            {"id": "zone-dojo", "name": "dojo.cc"},
            {"id": "zone-other", "name": "example.com"},
        ]
        with patch.object(cf, "list_zones", return_value=mock_zones):
            z1 = cf.find_zone_for_domain("app.cuan.work")
            self.assertEqual(z1["id"], "zone-cuan")

            z2 = cf.find_zone_for_domain("cuan-app.dojo.cc")
            self.assertEqual(z2["id"], "zone-dojo")

            z3 = cf.find_zone_for_domain("cuan.work")
            self.assertEqual(z3["id"], "zone-cuan")

    def test_find_zone_not_found(self):
        cf = m.CloudflareDNS("fake-token")
        with patch.object(cf, "list_zones", return_value=[{"id": "1", "name": "dojo.cc"}]):
            with self.assertRaises(RuntimeError):
                cf.find_zone_for_domain("unknown.example.org")


class TestHostMigratorExecution(unittest.TestCase):
    def setUp(self):
        self.src_mock = MagicMock(spec=m.SSHRunner)
        self.src_mock.host = "43.173.33.11"
        self.src_mock.user = "ubuntu"
        self.src_mock.test_connection.return_value = "ubuntu"
        self.src_mock.run.return_value = ("", "", 0)

        self.dst_mock = MagicMock(spec=m.SSHRunner)
        self.dst_mock.host = "43.173.40.62"
        self.dst_mock.user = "ubuntu"
        self.dst_mock.test_connection.return_value = "ubuntu"
        self.dst_mock.get_specs.return_value = {"ram_mb": 7623, "cpus": 4, "free_disk": "45G"}
        self.dst_mock.run.return_value = ("", "", 0)

    def test_dry_run_executes_no_mutations(self):
        opts = m.parse_args([
            "--from-host", "43.173.33.11",
            "--to-host", "43.173.40.62",
            "--dry-run",
        ])
        migrator = m.HostMigrator(self.src_mock, self.dst_mock, opts)
        migrator.run_all()

        # Preflight should be called
        self.src_mock.test_connection.assert_called_once()
        self.dst_mock.test_connection.assert_called_once()
        self.dst_mock.get_specs.assert_called_once()

        # No mutating methods should have run
        self.src_mock.pipe_to_remote.assert_not_called()

    def test_database_streaming_stage(self):
        opts = m.parse_args([
            "--from-host", "43.173.33.11",
            "--to-host", "43.173.40.62",
            "--skip-packages",
            "--skip-data",
            "--skip-runtime",
            "--skip-systemd",
            "--skip-nginx",
            "--skip-deploy",
        ])
        migrator = m.HostMigrator(self.src_mock, self.dst_mock, opts)
        migrator.db_info = {"user": "cuan", "password": "pwd", "dbname": "cuan", "host": "localhost", "port": 5432}

        # Mock table count and task count queries
        self.dst_mock.run.side_effect = [
            ("", "", 0),  # CREATE ROLE
            ("0", "", 0),  # Check database -> not exists
            ("", "", 0),  # CREATE DATABASE
            ("", "", 0),  # CREATE EXTENSION
            ("49", "", 0),  # SELECT count(*) FROM information_schema.tables
            ("153", "", 0),  # SELECT count(*) FROM tasks
        ]

        migrator.step_database()

        # Check pipe_to_remote called with pg_dump and psql
        self.src_mock.pipe_to_remote.assert_called_once()
        src_cmd, dst_runner, dst_cmd = self.src_mock.pipe_to_remote.call_args[0][:3]
        self.assertIn("pg_dump -d cuan", src_cmd)
        self.assertEqual(dst_runner, self.dst_mock)
        self.assertIn("psql -d cuan", dst_cmd)


if __name__ == "__main__":
    unittest.main()
