## Diagnosis and correction

Read-only inspection of the latest production request found a terminal search
attempt (`run_command` wrapping grep). The native plan-mode runtime denied it;
the process exited0 but returned denied_actions. Cuan correctly rejected the
partial preamble. The initial quota failure and allowed account failover were
separate from this terminal permission rejection. See incident.json.

Release b85d444 makes technical inspection explicit in the coordinator contract,
with native read/search tools and grep_search instead of terminal grep. On a
well-formed denied result with a successful process exit, one analysis-only
continuation reuses the original context, account and model. The failed preamble
and raw command are not copied into the continuation. The application rejects
all application actions and topic changes from its replacement response and
still rejects native execution tools. The original uncertainty remains in the
retry-safety flag; this does not authorize an execution replay.

Targeted tests passed from a clean release checkout. Cases include read-only
recovery, repeated denial, attempted application action, topic change, native
execution, malformed stream and failed process. Existing schema, execution
rejection, trace and outcome tests also passed. Production and dedicated staging
activated b85d444 with private backups, unchanged schema25/configuration and
readiness passing. A new isolated staging conversation tests actual native
repository analysis; it does not replay the owner's failed production message.

## Actual native repository analysis

The isolated staging conversation completed after automatic recovery from an
empty reply and permitted routing around model restrictions/quota. The successful
route used a native read-file tool and returned the exact module path and Go
version from go.mod. A separate read-only server-side file check confirmed both
values and the file hash. No task was created and no production message was sent.
See native-analysis.json. This native run did not hit the denied-tool recovery
branch; its bounded behavior is covered by the deterministic tests above.
