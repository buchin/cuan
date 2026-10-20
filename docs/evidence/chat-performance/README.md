## Chat and shared-page performance

Reproduce using `npm --prefix web run build`, then `CUAN_PERF_LABEL=after node docs/evidence/chat-performance/benchmark.cjs`. Requires the sandbox's existing Playwright and Chromium paths; all API data is synthetic. No inference, messages, or tasks are sent to production.

The fixture contains 200 messages, 100 completed requests, and 32 recorded steps per request. Measurements use Chromium CDP, not Lighthouse or field Core Web Vitals. Before/after runs use the same machine and fixture; individual timing and heap readings vary.

| Measurement | Before | After |
| --- | ---: | ---: |
| DOM nodes | 36,189 | 2,325 |
| Mounted messages | 200 | 50 |
| Collapsed timeline items | 3,100 | 0 |
| JavaScript during 3.3s idle | 743 ms | 31 ms |
| Synthetic page ready | 726 ms | 455 ms |
| Synthetic history payload | 403,789 bytes | 403,789 bytes |

Changes:
- Closed response Detail panels do not mount timeline or technical JSON. Opening shows the same recorded events; closing removes them. No request is rerun.
- Inbox initially renders the newest 50 messages. **Load older messages** adds 50 at a time and preserves the viewport anchor. This is client rendering, not backend pagination.
- Request-to-message lookup is indexed once per snapshot, rather than scanning all requests for every message. Markdown rendering is memoized by content.
- Connection-status expiry timers live inside Inbox components and schedule at expiry boundaries. The application no longer ticks every second on every page.
- Contact polling pauses in hidden tabs, refreshes on return, avoids overlapping scheduled polls, and retains equal contact/agent arrays. Existing task/project and notification polling remains page/visibility scoped.
- Unchanged SSE snapshots send comment heartbeats instead of resending full history. Changed snapshots and initial/reconnect recovery retain the existing contract.

Verification: frontend TypeScript/Vite build; fake-provider `TestInboxLifecycleContextFIFOStreamAndTask` including unchanged snapshot heartbeat; browser assertions for lazy Detail, all 200 messages reachable, scroll anchor, and horizontal overflow on Contacts, Projects, Notifications, Settings. These four pages recorded 0ms idle JavaScript in their short 1.2s samples, which does not measure their longer polling intervals. A broader legacy browser script exceeded a 20s terminal limit and was stopped; it is not counted as passing.

Limits: server still loads the full selected conversation and its recorded steps. The initial payload and memory for raw history still grow with history length. Loading every older page also grows the DOM. Server-side cursor pagination and on-demand trace retrieval remain future work for exceptionally large conversations. The backend still queries/serializes on stream ticks; heartbeat deduplication saves transfer and browser updates, not that database work. Build retains pre-existing CSS/minified chunk warnings.
