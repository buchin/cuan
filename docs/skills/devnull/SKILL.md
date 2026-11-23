---
name: devnull
description: Publishes work updates, daily ship logs, and release announcements to the dev-null.sh developer social network via CLI.
license: Apache-2.0
metadata:
  author: Cuan Team
  site: https://dev-null.sh
---

# DevNull Integration Skill

This skill enables Cuan agents to broadcast work updates, release summaries, and daily ship logs to the **dev-null.sh** developer timeline.

## When to Use
- When the user asks to post or publish an update to dev-null.sh.
- When generating and publishing a Daily Ship Log / Work Summary.
- When celebrating a successfully deployed milestone or feature release.

## Execution
Run the helper script or native CLI command:
```bash
# Plain text post:
scripts/post-to-devnull.sh "Shipped Cuan: <summary of what was built>! https://app.cuan.work"

# With an image:
scripts/post-to-devnull.sh --image screenshots/<image-file>.png "Shipped Cuan: <summary>! https://app.cuan.work"
```

## Best Practices
- Keep posts concise, punchy, and positive.
- Highlight tangible user value (e.g. 1-click terminal, background scheduler) over internal refactors.
- Include the project link (`https://github.com/buchin/cuan` or `https://app.cuan.work`).
