# Native release workflow: task-list limit

## Scope and first observation

A new useful code task must fix `cuan task list --limit 1 --json` returning 31
rows, preserve filters/order and shared CLI/MCP behavior, add targeted tests, then
reach a verified native release through PM/developer/independent verification.
No pagination redesign, UI changes or customer task mutations. Registered Cuan
project metadata confirms auto_deploy enabled; the preferred model's quota policy
was not changed. The prior no-publication benchmark does not prove this path.

- Room: `8f59eed3-e84e-4595-851b-f15b9c96eb57`
- Request: `4e8eca39-3403-470e-8f37-869956919ac3`
- Original owner message: `d238d12f-3552-42bc-9f98-019673646493`

The official request trace shows a 68.9-second initial provider stage and a
23.3-second format repair. Both responses failed the action contract. The saved
reply nevertheless promised delegation and appended an invalid-plan clarification
note. No task was created. The original request was not blindly resent.
Native stage metadata records ~105KB initial prompt and a candidate-only repair;
no raw rejected action values were preserved, so the exact offending production
field cannot be identified retrospectively.

## Reproduced protocol gaps and correction

The provider's actual `--json-schema` allowed arbitrary fields inside every
application action payload, but the delegation decoder rejects unknown fields
and unsupported types. Candidate-only repair received that same unconstrained
payload schema, without the native contract. Delegation now has a distinct typed
schema branch with exactly the Go payload's fields/types, supported category and
action enums, and creation/proposal/continuation required-field alternatives.
Other action kinds retain their existing behavior. Native authorization, scope,
parser and execution checks remain authoritative; no unknown fields are ignored.
A schema/parser parity test detects field/type drift and unrestricted branches.
The ordinary and repair turns reuse the same schema rather than another protocol.

A separate executable regression proves malformed delegation payloads were
salvaged as successful marked-up replies. The recovered delegation now passes
through the existing parser before being returned; invalid recovery remains a
failure with no task-start promise or action applied. Valid transport recovery
retains original project/authorization. Response diagnostics distinguish payload
fields/types from constraints using fixed labels, without recording private
candidate contents. Coordinator, formatting/recovery and diagnostics tests pass.
These tests reproduce the class of failure; they do not recover the missing raw
production field or demonstrate successful native publication yet.
