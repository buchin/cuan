# Inbox stage journal

Inbox response processing saves every new sanitized stage in an indexed,
append-only PostgreSQL journal. A request and sequence identify each event.
The request row remains a bounded projection for polling, dispatch and recovery.
Its first marker, safety evidence, latest five capacity waits and recent stages
remain ordered; recovery never treats a discarded stage as permission to retry.

Journal insertion, projection update and finalization are atomic under the
request row lock. Concurrent callbacks receive distinct increasing sequences.
Repeated finalization of a terminal request creates no duplicate final event.
If stage persistence fails, execution is stopped and the existing uncertain-outcome
path requires reconciliation; diagnostic failures do not silently authorize replay.
No raw provider error, tool payload or credential is written to application logs.

Read a page without sending a message or starting another attempt:

```sh
cuan chat trace ROOM_ID REQUEST_ID --after-seq 0 --limit 100
```

Use `next_seq` as the next cursor while `has_more` is true. Owner authentication,
conversation resolution, exact request ownership and historical sensitive-file
redaction also apply to the read-only `/requests/REQUEST_ID/steps` endpoint.
Visitors have no access. Pages are limited to200 events. The live message response
includes `step_count` and `steps_truncated`; it is not the complete diagnostic log.
The journal is deleted with its request/conversation through the existing cascade.

Migration36 retains all existing recorded steps. Histories that reached the old
100-stage cap also receive an explicit uncertainty marker: earlier discarded
activity cannot be recreated. These requests require reconciliation before retry.
Do not infer an old execution's result from this migration marker. New binaries
keep the request projection bounded while complete events remain in the journal.
Deploy the additive migration before switching workers. Keep the journal when
rolling back application code; dropping it destroys the new complete trace.
