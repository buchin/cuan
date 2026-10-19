## Chat delegation verification

Removed the draft-only requirement for execution requests. Added internal coordinator/development contracts, structured delegation through the existing task runner, scoped authorization provenance and project checks, source/request idempotency, progress delivery, durable coordinator result review and bounded concrete corrections.

Targeted deterministic tests exercise actual chat acceptance → provider action → tracked task → real PTY → final report → coordinator result returned to chat. They cover replay without new attempts, internal context visibility, unauthorized system/foreign approval sources, malformed/unknown protocol fields, concrete corrections of the same task and a three-attempt ceiling. No real provider is called.

Existing targeted inbox/draft/project/context, task runner/workspace and explicit Done tests passed. The product guide documents role boundaries, usage and current limitations: the model interprets semantic consent; provenance validation alone does not prove it, there is no new sprint scheduler, and live provider adherence is not fixture-tested.

Deployment and production verification are recorded below after release.
