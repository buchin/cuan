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
