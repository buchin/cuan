# Native GitHub Delivery Workflow

## Studio Approval Workflow

Before any work begins, the task must be reviewed and approved through the Studio
approval workflow. A human reviews the proposed scope, acceptance criteria, and
constraints. Only after explicit approval is the task authorized for execution.
This ensures that every change is intentional and sanctioned.

## Isolated Task Branch

Each approved task is executed on its own isolated branch. This branch is
automatically created from the repository's main branch and is scoped to a single
task. No other branches are created, switched to, or merged during execution. This
isolation prevents unrelated changes from interfering with the task deliverable.

## Verification

Before committing, the implementation is verified against the acceptance criteria:

- The expected files exist at the correct paths.
- `git diff --check` confirms there are no whitespace errors.
- Only the scoped files are staged; no unrelated files are modified or deleted.
- The commit is authored with the repository's configured human identity.

## Task Push Workflow

Once the commit passes verification, the isolated task branch is pushed to the
remote (origin) using the Cuan task push command. This command pushes only the
bound task branch without merging, deploying, or force-pushing. After a successful
push, the branch and its commit are visible on the remote GitHub repository,
completing the native GitHub delivery cycle.
