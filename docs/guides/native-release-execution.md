# Native release execution

The release executor runs the repository's maintained release entry point against
an exact clean commit and expected branch. It uses no language model and does not
merge source, accept tasks or infer production health from a zero exit code.
Inbox delivery admission and durable stage orchestration remain separate work;
this command is currently an explicit maintainer operation.

## Repository contract

Track `.cuan/release.json` alongside its executable entry point:

```json
{
  "version": 1,
  "command": ["scripts/deploy.sh"],
  "timeout_seconds": 1200
}
```

The descriptor is limited to16KiB,16 literal command arguments and a timeout of
1–1800seconds. Unknown fields/versions and trailing JSON are rejected. The command
must be a tracked regular executable inside the repository. Symlink traversal,
absolute paths and parent escapes are rejected. Arguments are passed directly;
shell substitutions and environment interpolation are not performed by Cuan.
The maintained script still owns its release, migration, health and rollback
gates. This executor does not replace or disable them.

Planning and execution verify the commit, branch and clean source independently.
The plan cannot be changed by callers after loading. Execution holds the existing
repository lease and checks that tracked source remains unchanged afterward.
Pending work is never reset, cleaned or committed to make a release pass.

## Maintainer CLI

```sh
cuan release inspect --path . --commit FULL_SHA --branch dev
cuan release run --path . --commit FULL_SHA --branch dev --logs
```

`inspect` does not run the script. `run` explicitly uses the maintainer process
environment, like invoking that script directly. Do not use this operator path
as a Cuan user's credentials or as an Inbox delivery grant. Bound task sessions
cannot run this maintainer command. The native Go executor instead requires an
explicit environment slice; nil is rejected and never inherits host credentials.
A future Inbox delivery stage must resolve the saved owner/project environment
and original authorization before invoking it.

Results report source, start/end time, real exit code and unchanged-source status.
Process startup failure uses an unknown exit code, not zero. Raw child output is
absent from JSON. `--logs` prints only the bounded diagnostic tail with known
credential patterns and sensitive operator environment values redacted. It is a
diagnostic, not an interactive terminal or a promise of complete historical logs.

Cancellation kills the owned process tree. An inherited pipe cannot keep the
executor waiting indefinitely after the script exits, and remaining descendants
in its process group are cleaned up. Output retention is bounded to12,000bytes.
No failed/uncertain execution is retried automatically. Cancellation or failure
does not prove that external effects never happened: inspect current release state
before another attempt.

Only a trusted task/attempt/source-bound deployment observation can establish
publication, and an independent review must still check the original criteria.
A successful operator invocation does not write those receipts or mark Inbox work
Done by itself. See [deployment evidence](task-deployment-evidence.md).
