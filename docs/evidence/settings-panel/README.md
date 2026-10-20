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

## Production

Live release fd20e716e6f0fab907b9657da5df7b6652a71441. Read-only production checks
confirm identical desktop project sidebar position/width between Tasks/Settings,
form in right panel, correct Tasks destination, unchanged-save disabled,
collapsed Advanced and no New task. Responsive light/dark checks pass.
All application writes were blocked after login. Actual rollback/reactivation
passed; schema25 unchanged,37 app tables,904 profile files,6 item entries and11
read flags retained. Tested and released frontend asset hashes match.
