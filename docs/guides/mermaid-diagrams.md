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
themes. `CUAN_PLAYWRIGHT_MODULE` and `CUAN_CHROMIUM_EXECUTABLE` can select an
installed browser runtime.

When browser libraries are unavailable in the build environment, set
`CUAN_DIAGRAM_BUILD_ONLY=1` to generate the static fixture. Serve that generated
directory on localhost in the browser environment. Its `fixture-path.txt` gives
the page path. Run `mermaid-browser-checks.mjs FIXTURE_URL SCREENSHOT_DIRECTORY`
there using the same Playwright module setting; it needs no application data,
provider credentials or live customer conversation. Pull and inspect the actual
screenshots before considering visual verification complete.
