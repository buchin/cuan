# Cuan repository guidance

Cuan is for nondeveloper users. Keep ordinary onboarding and work management in
the application UI; agents handle technical defaults and execution details.

## Mandatory Design & UX Definition of Done (DoD)
All UI, frontend, styling, and UX changes MUST strictly adhere to the
[Design Guidelines & Quality Gate](docs/design/design_guidelines.md#7-standar-evaluasi-visual--kualitas-pengalaman-pengguna-usability--low-visual-noise-quality-gate).
Passing automated/unit tests alone is NOT sufficient. The core goal is achieving
**Zero Cognitive Fatigue** matching Bear Notes & Things 3 standards:
1. **Zero Visual Noise:** No screaming neon colors, no 1px white wireframe boxes, and no solid white `#ffffff` date/status pills.
2. **Breathing Room:** Clean whitespace, avoiding cramped DevOps/SaaS dashboard layouts.
3. **Natural Typography Hierarchy:** Effortless readability through font weight and muted tones, not nested box cards.
4. **Predictable Flow & Usability:** Clean communication flow without interrupting workflows. Secondary technical details belong in `SlideDrawer`.
5. **Mandatory Visual Inspection:** Agents must render and evaluate real screenshots (mobile 390px and desktop) before marking tasks complete.

For protected application screens, start with `scripts/verify-ui.sh "${CUAN_TASK_ARTIFACTS_DIR:?}/ui"`
and its real-component fixtures in `web/scripts/verify-inbox-work-panel.mjs`.
Use the inspected source checkout and isolated fixture data; do not borrow an
Owner login. Check which components and behaviors the fixture actually covers.
Extend the relevant fixture for missing task-specific behavior rather than
recreating application HTML. A read-only verifier can use a temporary fixture
outside the checkout; keep the inspected source unchanged.
Before capturing, assert that the requested screen, control and state are
visible, then inspect the resulting images. A successful navigation or HTTP200
to a landing/login page is not evidence of protected Inbox behavior. Report
missing behavior or access as unverified, and distinguish fixture observations
from production observations. See [protected UI verification](docs/skills/cuan-agent/SKILL.md#protected-ui-verification-without-owner-access).

## Root-Cause Problem Solving & Anti-Tambal Sulam
All changes and bug fixes MUST address the root cause, never tambal sulam:
1. **Root Cause Analysis (RCA):** Uncover and fix the core structural, contract, or invariant breakdown behind errors, not just the line throwing an exception.
2. **Anti-Tambal Sulam:** Strictly forbidden to use superficial regex hacks on structured documents, suppress/swallow errors, bypass verification checks, or hardcode fragile special-cases.
3. **Authentic Verification:** Verify against actual runtime behavior and authentic test cases, ensuring user intent is truly satisfied.

## Verification & Test Efficiency
- **Smart Test Selection:** When changes touch only frontend files (`web/`), skip heavy backend Go test suites. Run `./scripts/test.sh --fast` or frontend validation directly.
- **Fixture Authenticity:** Verification fixtures must reflect actual usage scenarios: both customer conversations (with reply mode tabs) AND direct bot/agent conversations (without mode tabs). Ensure no artificial dead zones or visual gaps are created when tabs are hidden.

## Integrations
Before adding or changing an external integration, follow
[the integration design principles](docs/product/integration-principles.md).
Prefer a maintained official provider CLI where appropriate, with a simple Cuan
UI for connection, status, retry and cancellation. Use official APIs/SDKs where
better suited; a CLI is not mandatory for every capability. Never reuse
ambient developer credentials as Cuan user credentials.

## Cuan Platform Operations & Agent Tooling
Whenever inspecting or operating on Cuan (projects, tasks, inbox rooms, or talking with agents):
- Use the official Cuan CLI `cuan` on PATH or MCP tools (`cuan-inspect` / `cuan mcp`).
- NEVER write ad-hoc Python scripts or unauthenticated curl commands to interact with Cuan.
- Subcommands available on PATH:
  - `cuan chat new [--title "..."] [--agent <id>]`
  - `cuan chat send <room-id> "<message>" [--wait]`
  - `cuan chat list [--limit 20]`
  - `cuan chat messages <room-id>`
  - `cuan task list [--status doing]`
  - `cuan task get <task-id>`
  - `cuan task update <task-id> --status done [--complete-note "..."]`
  - `cuan mcp` (stdio MCP server)
  - `cuan config show` / `cuan config set <key> <val>`
- See [cuan-agent SKILL](docs/skills/cuan-agent/SKILL.md) for complete reference.


