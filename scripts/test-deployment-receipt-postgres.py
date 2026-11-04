"""Exercise native deployment receipts on a dedicated migrated launch-test database."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
from unittest.mock import patch
from urllib.parse import parse_qs, urlparse

spec = importlib.util.spec_from_file_location('activation', Path(__file__).with_name('activate-release.py'))
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)
u = urlparse(os.environ.get('CUAN_LAUNCH_TEST_DATABASE_URL', ''))
if not u.path.startswith('/cuan_launch_queue_') or u.hostname not in (None, '', 'localhost'):
    raise SystemExit('A dedicated local cuan_launch_queue_* PostgreSQL fixture is required')
env = dict(os.environ, PGHOST=parse_qs(u.query).get('host', [u.hostname or 'localhost'])[0],
           PGUSER=u.username or 'cuan_fixture', PGDATABASE=u.path[1:], LD_LIBRARY_PATH='')

def sql(query):
    return a.database_scalar(query, env)

sql("""INSERT INTO tasks(id,project_id,title,instructions,status) VALUES
('task-native-receipt','launch-project','Receipt fixture','Approved work','doing'),
('task-other-receipt','launch-project','Other fixture','Other approved work','queued');
INSERT INTO task_attempts(id,task_id,runner_id,agent_name,status) VALUES
('att_native_receipt','task-native-receipt','fixture','agent','running');
INSERT INTO task_launches(id,task_id,project_id,request_id,attempt_id,owner_id,room_id,source_request_id,agent_id,instructions_hash,state,available_at,expires_at,created_at,updated_at)
VALUES('launch_native_receipt','task-native-receipt','launch-project','fixture','att_native_receipt','owner','91000000-0000-4000-8000-000000000001','fixture','agent','fixture','claimed',NOW(),NOW()+INTERVAL '1 hour',NOW(),NOW());""")

def states():
    return [sql("SELECT row_to_json(t)::text FROM " + table + " t WHERE id='" + key + "'")
            for table, key in [('tasks','task-native-receipt'), ('task_attempts','att_native_receipt'), ('task_launches','launch_native_receipt')]]

before = states()
settings = dict(CUAN_TASK_CAPABILITY_FILE='', CUAN_DEPLOY_EXCLUDE_TASK='task-native-receipt', CUAN_DEPLOY_EXCLUDE_ATTEMPT='att_native_receipt')
with patch.dict(os.environ, settings):
    first = a.record_task_deployment(env, 'a'*40)
    assert first == a.record_task_deployment(env, 'a'*40), 'same deployment must be idempotent'
assert states() == before, 'a deployment receipt must not complete attempts, accept tasks or stop launches'
assert sql("SELECT count(*) FROM task_activities WHERE activity_type='deployment_verified' AND task_id='task-native-receipt'") == '1'
receipt = json.loads(sql("SELECT message FROM task_activities WHERE id='"+first+"'"))
assert receipt == dict(version=1, task_id='task-native-receipt', attempt_id='att_native_receipt', commit_sha='a'*40, release_sha='a'*40, environment='production', health='healthy')
with patch.dict(os.environ, dict(settings, CUAN_DEPLOY_EXCLUDE_TASK='task-other-receipt')):
    try:
        a.record_task_deployment(env, 'a'*40)
    except RuntimeError:
        pass
    else:
        raise AssertionError('an attempt belonging to another task was accepted')
assert sql("SELECT count(*) FROM task_activities WHERE activity_type='deployment_verified'") == '1'
assert states() == before
print('PostgreSQL native receipt: exact binding, idempotency and unchanged task/attempt/launch state verified.')
