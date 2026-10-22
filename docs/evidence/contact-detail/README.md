## Contact detail simplification

Identity uses avatar, name, email and a source chip. Added manually is a system source, separate from editable user tags. Updated date, email verification and channel information live in More details.

Notes lead the page. Empty notes show a person-specific prompt; note search appears when notes or a query exist. Tags are compact chips with a collapsed Add tag form. Adding/removing saves immediately; failure keeps the draft with Try again. Existing note permissions and source metadata remain unchanged.

Desktop overflow actions reveal on hover, focus or while open; touch actions stay visible. Selected contact background includes its action area. Search placeholder is shorter while continuing to search notes.

Targeted verification: isolated frontend build, existing cross-page gestures/read/archive regression, tag add/remove autosave, failed save/retry, empty-note search, hidden metadata, source chip, hover/focus and actual touch emulation. Light/dark screenshots at 360/390/820/1440, no overflow. No provider inference or real chat/task submission.

## Production

Live at https://cuan.dojo.cc/contacts, release 7faa35cb950236957f6cb551f91237ab566b04ff. Read-only production checks passed for responsive menus and actual contact detail, with no Save tags button and More details available. All application writes blocked after login; no provider inference.

Actual rollback/reactivation and readiness passed. Schema25 unchanged, auto-migration false. Retention verified: 37 app tables (auth sessions excluded), 896 profile files unchanged, six item preferences unchanged, 11 notification read flags retained.

Existing CSS minification and bundle-size warnings remain.
