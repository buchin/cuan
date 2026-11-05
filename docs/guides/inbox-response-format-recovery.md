# Recovering a completed conversation response

When the provider completes but its coordinator JSON is invalid, Cuan performs
at most one format-repair stage. It supplies the original completed response as
an explicitly untrusted JSON string along with the original conversation scope.
The repair retains findings, uncertainty, task identity and authorization;
it does not restart the user's investigation from scratch.

The runtime denies file reads, searches, MCP and execution tools during this
stage. Only the provider's native finish/format-response tool is allowed. The
workspace is not added as an inspection directory. The existing response and
application action validators still apply before any task or other application
action can be accepted. A forbidden tool, interrupted provider, exhausted bound
or invalid result cannot claim that work was completed. The candidate never
appears in public activity steps. Oversized candidates beyond the existing
40,000-byte transport limit do not trigger another provider turn.

This does not reduce the chosen model, original request deadline or verification
criteria. First-turn implementation claims still need source evidence. A follow-up
that only prioritizes previously discussed proposals should give a concrete
recommendation and tradeoff, without repeating the catalogue or inspecting
unrelated health metrics. Prior proposals remain proposals; fresh implementation
claims require fresh evidence where needed.

Verification:
`go test ./internal/server -run 'TestInboxFormatRepair|TestInboxCLI|TestDecodeInboxAgyResult|TestRecoverCoordinatorReply'`.
Fixtures verify reuse of the candidate, one bounded repair, absent workspace
inspection arguments, rejection of attempted rereads and no candidate leakage in
activity traces. They demonstrate the removed repeated investigation path, not
a measured production latency guarantee.

## Bounded response formatting

Formatting an already completed answer has a 40-second native print timeout
and a 45-second outer process limit. It retains
an earlier parent deadline and cancellation; it cannot consume the remaining
multi-minute investigation budget or renew the original request. This is a limit
on response formatting, not a promise that every investigation finishes in 45
seconds. The existing single-repair, no-tools and no-action-replay gates remain.

The formatter receives only its schema, fixed repair rules and the rejected
candidate as untrusted data. It does not reread project guidance, reattach the
full conversation or brand context, or inherit project/attachment mounts and
working directory. Analysis-only repair retains its action-null schema. The
normal response/action validators and original authorization checks still apply.

Execution also honors the earliest of the durable claimed-request deadline and
the configured local lifetime. Storage currently claims a six-minute request;
the former ten-minute local default must not let it keep running past that saved
deadline. Expired requests are not renewed and no recovery budget is reset.

A provider `agent_response DONE` event marks one model stage, not a valid final
runtime result or process exit. If the formatter becomes silent after that
event, its process group is stopped at the formatting deadline. The request
reports that formatting timed out, instead of waiting for the full turn deadline.
It does not publish an unvalidated answer or start a task from rejected output.

Rejected coordinator responses now record a bounded diagnostic category (invalid
JSON, missing/extra envelope fields, choice metadata, or action/contract issue).
These categories contain no reply content, file paths, candidate field values or
customer data. They diagnose future contract failures without logging candidates.

Regression coverage: `TestInboxFormatRepairHasIndependentBoundedDeadline`,
`TestInboxFormatRepairStopsSilentProviderWithoutReplay` and
`TestInboxResponseDiagnosticNeverIncludesCandidateData`.

## Correlating a response failure

Provider stages emit two bounded structured journal records (start/end), keyed
by the durable `request_id`, room, agent, provider, running release and phase.
Analysis, format repair, direct recovery and task review have distinct phases.
A parent phase closes before a recovery phase starts, so their durations are not
double-counted. Deadline, cancellation, ordinary failure and completion are
separate outcomes; `failure_key` groups equivalent provider/stage failures
independently of a specific request or release.

Records also carry the internal routed `account_id`, `requested_model`, and
numeric `prompt_bytes`, `context_bytes` and (for formatting) `candidate_bytes`.
Byte counts measure UTF-8 input size, not billed tokens. `requested_model=auto`
means the CLI/provider chooses the model; it does not identify which model the
provider ultimately used. These private operational fields do not expose the
backing accounts in ordinary chat. Formatter context size is zero because the
original conversation/project context is excluded from that stage.

The original request's durable steps retain the native session identifiers and
response-contract category. Start from `cuan chat messages ROOM_ID --json`, find
the exact request, then correlate its ID with the structured stage records.
Do not infer that a model-stage completion is a final answer, that an unknown
stop was OOM, or that a deployment interrupted a request without timestamped
process evidence. Background stage logging never includes prompt/response text,
account profile paths or environment contents.

This adopts Fural's correlation, phase timing, failure grouping and explicit
stop-source practices without adding a telemetry service or changing the chat
layout. `TestInboxResponseStageCarriesCorrelationWithoutPrivateContext` verifies
identity/release linkage, bounded fields, grouping and exactly one terminal record.
# Native runtime diagnostics

Server logs include one `inbox runtime summary` per completed native process,
correlated by `request_id`, stage and release. `tool_calls` and `tool_failures`
continue counting after the activity timeline reaches its 64-event display
limit. They describe observed native events, not proof of successful execution.

`tool_detail_events`, `tool_input_events`, `tool_output_events` and
`tool_error_events` count presence of recognized fields in native `tool_info`
events. They do not retain the values. Zero means no recognized field was
observed; it does not prove the tool produced no output. These counters help
verify the actual runtime contract before adding an expandable detail UI.
No file contents, paths, arguments, error bodies or reasoning are archived by
this summary. The summary's `reason` describes stream validation; final response
validation and provider-stage outcome remain separate records.

