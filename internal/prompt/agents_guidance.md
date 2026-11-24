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

## Root-Cause Problem Solving & Anti-Tambal Sulam
All changes and bug fixes MUST address the root cause, never tambal sulam:
1. **Root Cause Analysis (RCA):** Uncover and fix the core structural, contract, or invariant breakdown behind errors, not just the line throwing an exception.
2. **Anti-Tambal Sulam:** Strictly forbidden to use superficial regex hacks on structured documents, suppress/swallow errors, bypass verification checks, or hardcode fragile special-cases.
3. **Authentic Verification:** Verify against actual runtime behavior and authentic test cases, ensuring user intent is truly satisfied.

## Integrations
Before adding or changing an external integration, follow
[the integration design principles](docs/product/integration-principles.md).
Prefer a maintained official provider CLI where appropriate, with a simple Cuan
UI for connection, status, retry and cancellation. Use official APIs/SDKs where
better suited; a CLI is not mandatory for every capability. Never reuse
ambient developer credentials as Cuan user credentials.

## Autonomous Deployment Lane & Agent Scope Decoupling
Deployment is fully decoupled from developer and verifier agent execution:
1. **Zero Manual Deployments by Agents:** Agents in chat, tasks, or terminals MUST NEVER execute `./scripts/deploy.sh` or perform manual VPS SSH/rsync releases.
2. **Backlog to Done Lifecycle:** An agent's Definition of Done ends at: implementing the solution, verifying tests and fixtures locally (`./scripts/test.sh --fast` or component fixtures), and committing & pushing to the approved Git branch.
3. **Automated Pipeline Execution:** Once pushed, Cuan's Webhook Engine and dedicated FIFO Deployment Lane automatically pick up the commit, check `auto_deploy`, and execute the project's configured Deployment Script in isolation.
   - Sequential stages: 1. Source Checkout, 2. Compile & Build, 3. Database Migration, 4. Atomic Switch, 5. Health Verification.
4. **Conversational Deployment Setup in Chat:** When onboarding or discussing a project that lacks auto-deploy configuration, agents proactively propose release setup in conversational Indonesian:
   - Ask if user wants to auto-deploy on Cuan or if they already use another hosting (e.g. Laravel Cloud, Vercel, or shared hosting/cPanel WordPress).
   - If user accepts: generate the build script and configure `auto_deploy: true` via `PUT /api/v1/projects/{id}/deployment-config`.
   - If user declines / uses other hosting: respect external architecture, set `auto_deploy: false`, and offer relevant integrations (e.g., WordPress REST API for shared hosting, or focusing on task dev for Laravel Cloud).
