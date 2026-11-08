# Account outcome review recovery

Production release: `ff5203c701bc476825518dcc02906106ffbf10c3`.

The customer account operation in room
`f299b6de-3470-4fd4-8b9f-30eba98402f8`, task
`msg_aafa00af-d8c1-4d88-8c90-dd6296b5d5cd`, reached `done` through normal
coordinator acceptance after independent read-only observation. The maintainer
did not replay account activation, manually retry the task or override acceptance.

The legacy task inherited a code workflow and incidental local source changes.
Unrelated source prerequisites and publication checks obscured its account outcome.
Subsequent valid PM verification continuations were not recognized because the
independence gate accepted only a dedicated saved-evidence marker. Recovery could
also run before review and schedule one unnecessary observation.

The fix recognizes the exact saved coordinator verification grant for the
immediately preceding terminal attempt, matching request, original instructions,
read-only revision and native unchanged-repository evidence. Original owner
approval/cancellation checks remain mandatory. Already recognized observations
now proceed directly to review. Historical incidental source is preserved and
is not approved for publication by completion of the account operation.

The final native execution `att_473d685049a117c9cb55227995f833ae` succeeded.
Its final report records fresh account/entitlement observations and a live access
API response with HTTP 200 and `has_access=true`. Coordinator request
`c7afa6d9-b82a-4fe2-a407-99d9ac042d8e` saved decision `pass`, requested outcome
`completed`, remaining work `none`, and a customer reply draft. A subsequent
official CLI read confirmed task `done` and the final room result message.
These observations establish this account case; they do not establish that every
other task or source-publication workflow is resolved.

Targeted server tests cover native queue/claim/completion for legacy prerequisite
failure, legacy publication failure, ordinary account completion and a manager
verification continuation. Negative grants cover self-report, foreign request,
changed instruction hash, unwrapped feedback, canceled predecessor, foreign room,
future assessment and intervening predecessor. Source-review and unpublished-code
acceptance regressions also passed. Both releases passed exact-commit production
health verification with 37 migrations. No UI changes were made in this follow-up.
