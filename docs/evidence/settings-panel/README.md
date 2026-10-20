## Project settings in the existing project panel

Project Settings now renders as the right-hand content of ProjectsWorkspace,
retaining the project sidebar and shared project header. Settings no longer
switches to the standalone global settings layout/breadcrumb header. Tasks links
back within the same project. Embedded form removes its duplicate standalone
Back to project link and owns only its content padding.

No task creation button appears in Settings. Existing save/dirty/failure/conflict
reload and Advanced connection behavior remains verified. Browser fixtures cover
light/dark at360/390/820/1440, no overflow, matching desktop sidebar x/width between
Tasks and Settings, settings form to the right, and Tasks navigation. Targeted
cross-page interactions/persistence regression and isolated frontend build pass.
No provider calls or task launches.
