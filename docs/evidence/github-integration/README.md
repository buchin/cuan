# GitHub integration verification

The production frontend build from clean commit fb6d1af passed browser fixtures
at 360, 820 and 1440 pixels in light and dark themes. Tests cover connection error
retention, successful connection hiding the token input, disconnect, repository
pagination and page retry retaining existing rows, project save with an empty
branch resolving to GitHub default trunk, and mobile overflow.

Targeted backend tests cover owner authentication, private encrypted storage,
account discovery, redirects, default branch selection, shared credential rotation,
legacy project credentials and isolated original task workspace retention. Real
Git clone fixtures assert shared access is passed in environment, not arguments.
No production provider calls or live GitHub writes were made.

This evidence does not prove live account permissions, repository creation,
automatic agent push/merge, or OAuth. Those remain separate work.

Production release `8641f5c` passed read-only browser checks at 360/820/1440 pixels, exact asset matching, actual rollback/reactivation and retention checks. Schema remains 25 with automatic migration disabled; 37 application tables, 904 profile files, six item states and eleven read flags were retained. No live GitHub connection or mutation was submitted.
