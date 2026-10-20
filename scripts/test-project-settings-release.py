#!/usr/bin/env python3
"""Targeted compatibility gate for retained project workspaces."""
import importlib.util
from pathlib import Path
import unittest
spec=importlib.util.spec_from_file_location('activation',Path(__file__).with_name('activate-release.py'))
activation=importlib.util.module_from_spec(spec);spec.loader.exec_module(activation)
class WorkspaceReleaseGate(unittest.TestCase):
 def setUp(self):
  activation.database_environment=lambda:{}
 def test_compatible_release_does_not_require_database_query(self):
  activation.database_scalar=lambda *args: self.fail('Unexpected compatibility query')
  activation.assert_project_workspace_support({'features':{'project_workspace_revisions':1}})
 def test_old_release_allowed_before_connection_changes(self):
  values=iter(['t','0']);activation.database_scalar=lambda *args: next(values)
  activation.assert_project_workspace_support({})
 def test_old_release_blocked_when_task_workspaces_are_retained(self):
  values=iter(['t','1']);activation.database_scalar=lambda *args: next(values)
  with self.assertRaisesRegex(RuntimeError,'cannot resume tasks'):
   activation.assert_project_workspace_support({})
if __name__=='__main__':unittest.main()
