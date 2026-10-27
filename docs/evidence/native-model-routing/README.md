## Native model routing and quota event investigation

Dedicated staging876358e/schema25 contains catalog-aware model routing and native
error-message lifecycle handling. Production remains79c04c0 because its release
guard detected a genuinely running task; activation was not forced.

Actual observations:

- The quota RPC includes obsolete gemini-2.5-flash while installed `agy models`
  lists Gemini3.8 variants, Sonnet4.6, Opus4.6 and GPT-OSS120B.
- Unsupported IDs are excluded from Auto routing. Supported catalog entries
  lacking quota numbers remain unknown (-1 internally), never assumed exhausted
  or100% available. Measured positive quota ranks higher; cooldowns and existing
  account/model fallback permissions are retained.
- The actual CLI quota error stream includes a non-tool `error_message` step.
  Previously this incorrectly marked the turn unsafe to retry. A no-tool error
  lifecycle is now recognized; tool metadata on it still blocks replay. Native
  error details are not exposed in trace labels.
- The real staging continuation request in fallback.json tried Sonnet, recorded
  its quota failure as safe, then selected supported Gemini3.8. The latter failed
  because the provider rejected this server location. No duplicate task or
  implementation attempt was created.
- Separate tiny no-tool probes: Sonnet quota-limited; GPT-OSS quota-limited;
  Gemini location-restricted; Opus answered a tiny prompt, but the later actual
  continuation received a quota failure. A tiny probe is not proof that a larger
  work request can finish. The original task still has only its initial failed
  preparation attempt; no GitHub delivery is claimed.

Account model fallback was explicitly enabled on the dedicated test account for
this scenario. Production account preferences were not changed. Attempting to
select an explicit model through the existing CLI-agent PATCH did not persist a
model preference: the schema retains its CLI connection link in model_id. No
success is claimed for that attempted test configuration.

Targeted catalog isolation/cache/overflow, verification selection, unknown quota
ranking, routing, concurrency, preflight and safe quota lifecycle tests pass.
Catalog calls use the owned isolated profile and refresh lease, bounded output,
five-second timeout and five-minute cache; no inference runs during catalog reads.

Remaining: await a usable dedicated AI route, resume the original approved task,
verify commit/push and report review, and activate the tested candidate when
production's active work is finished. Do not borrow production/Fural credentials
or stop active work merely to deploy.
