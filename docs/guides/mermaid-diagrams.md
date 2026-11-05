# Diagrams in conversations and Markdown previews

Fenced `mermaid` and `flowchart` blocks render as SVG previews in Inbox and the
shared Markdown viewer. Small diagrams keep their natural text size; previews
fit the available width and scroll within a bounded surface for large graphs.
The header offers source, copy and expand. Expanded diagrams use the shared
SlideDrawer, with zoom, mouse drag, native touch scrolling, Escape and focus
restoration. Source remains selectable when copying is unavailable.

Invalid diagrams and diagrams outside the rendering budget retain readable
source without breaking the message. Labels support text and light emphasis.
HTML attributes, executable callbacks, image-node metadata and URL-based diagram
styles are excluded. Images belong in ordinary attachments rather than remote
resources embedded in diagrams. SVG previews are displayed in an image context;
node click callbacks are not executed.

## Runtime and build

`MermaidDiagram` is shared by ChatMarkdown and MarkdownViewer. Theme changes
regenerate the preview. Rendering is serialized because Mermaid's configuration
is global; a stale or unmounted component cannot overwrite a newer result.
Source size and edge count bound browser work. The source is never posted to a
CDN or external rendering service.

Mermaid 12.3.0 is pinned in npm's manifest and integrity-checked lockfile. The
Vite plugin `mermaid-assets.ts` emits the official prebuilt ESM distribution and
license under a versioned local asset directory. It does not rebundle Mermaid's
entire diagram dependency tree, which exceeded the build resource budget.
Only the small preview component enters Cuan's ordinary application chunks;
Mermaid and the required diagram modules download on demand. No engine request
is made for a conversation without diagrams. Other chart types retain the
upstream engine's lazy loaders.
The TypeScript boundary imports only the provider's configuration type and
describes the two runtime methods used here. This avoids loading every layout
engine's internal declaration graph into application type-checking; it does not
change the lazy runtime module or relax configuration checks.

Versioned `.mjs` files also use the existing retained-release asset resolver.
Tabs opened before a Mermaid update can load their original modules while that
release is retained. Unrecognized files, traversal and symlinked paths do not
qualify for fallback. Missing modules return 404 rather than SPA HTML.

## Verification

```sh
npm test --prefix web
npm run build --prefix web
go test ./internal/server -run '^TestRetainedAssetsKeepOldTabsWorkingWithoutHTMLFallback$' -count=1
node web/scripts/check-mermaid.mjs /absolute/path/to/screenshots
```

The browser check uses actual React components, providers and application CSS.
It verifies lazy loading, SVG labels, source/copy, denied clipboard recovery,
zoom and mouse pan, Escape/focus restoration, sequence diagrams in the Markdown
preview, invalid/oversized source, and untrusted HTML/image/style input without
external resource requests. It captures 390px and 1440px screenshots in both
themes. The native runner supplies `CUAN_BROWSER_MODULE` for its installed
Playwright runtime. `CUAN_PLAYWRIGHT_MODULE` can override that module, and
`CUAN_CHROMIUM_EXECUTABLE` can select a browser executable.

When browser libraries are unavailable in the build environment, set
`CUAN_DIAGRAM_BUILD_ONLY=1` to generate the static fixture. Serve that generated
directory on localhost in the browser environment. Its `fixture-path.txt` gives
the page path. Run `mermaid-browser-checks.mjs FIXTURE_URL SCREENSHOT_DIRECTORY`
there using the same Playwright module setting; it needs no application data,
provider credentials or live customer conversation. Pull and inspect the actual
screenshots before considering visual verification complete.

## Runner-owned capture evidence

`scripts/verify-ui.sh PRIVATE_OUTPUT_DIRECTORY` is the registered visual entry
point. It runs the real React checks above and exports four PNGs plus
`visual-evidence.json`, naming the covered components. The output directory must
already exist and contain no files with those names. This recipe covers Mermaid
in ChatMarkdown and MarkdownViewer; it does not verify the entire Inbox UI.
It compiles the fixture before launching Chromium, keeping compiler and browser
memory peaks separate. Compiler parallelism and Node heap are bounded; a failed
build records its exit status or signal instead of becoming a visual pass.

For vision-required live inspections, a capable runner binds the recipe to the
exact source commit before issuing the model capability. It runs the recipe
before the model, with a private home and only the configured browser runtime
variables, within the existing attempt deadline. Capture hashes, dimensions and
component coverage are delivered through authenticated runner transport. An
uncertain upload response is reconciled against the same saved receipt rather
than replaying the upload. A task instruction prohibiting screenshots prevents
automatic capture.

The coordinator checks the exact attempt's bound recipe and delivered image
bytes before allowing acceptance. Agent attachments remain ordinary evidence;
they do not acquire runner capture provenance. Native provenance establishes
which registered recipe produced the files, not that every task criterion is
satisfied: a vision-capable reviewer must still inspect them and identify any
missing coverage. Capture failure preserves diagnostics and cannot grant a new
attempt, extend its deadline, or accept the task.
