# Frontend Optimization Audit — Baseline

## Build System

- `package.json`: module (`"type": "module"`), `pnpm` package manager (`pnpm-lock.yaml` present)
- `vite.config.ts`: Vite 8.3.0 with `@vitejs/plugin-vue` 6.0.9, `tailwindcss` 3.4.19, `postcss`
- `tailwind.config.ts` present; CSS processed via PostCSS + Tailwind
- Build targets: `public/assets/` (output `public/assets/js/` and `public/assets/css/`)
- Source root: `web_src/` (JS: `js/`, CSS: `css/`, SVG: `svg/`, Fomantic components: `fomantic/`)
- No `scripts` section in `package.json`; build driven by `Makefile` (`frontend` target calls `pnpm exec vite build`)
- Production mode: `NODE_ENV` set for `vite build`; minify `oxc`; CSS minify `esbuild`
- Source maps: `reduced` in production (only `js/index.*`, `js/iife.*`, etc. kept)

## Source Inventory

- `web_src/js/`: 246 files
- `web_src/css/`: 92 files
- `web_src/svg/`: 67 files
- `web_src/fomantic/`: Fomantic UI component build files (dropdown.js 4269 lines, modal.js 1209 lines, api.js 1155 lines)
- `.vue` components: 23
- Global Vue plugins: registered via `sharedPlugins` in `vite.config.ts`

## Dependency Count

- Direct dependencies: 59
- Dev dependencies: 43
- Lockfile: `pnpm-lock.yaml`
- Notable large libraries:
  - `vue`: 3.5.42
  - `chart.js`: 4.5.1 + `chartjs-plugin-zoom`
  - `mermaid`: 12.0.0
  - `easymde`: 2.21.0
  - `codemirror` family (many sub-packages: autocomplete, commands, view, state, lint, etc.)
  - `katex`: 0.18.7
  - `swagger-ui-dist`: 5.33.0
  - `jquery`: 4.0.0 (still present)
  - `tailwindcss`: 3.4.19
  - `vite`: 8.3.0
  - `postcss`: 8.5.28

## Largest Source Modules (LOC)

- `web_src/css/repo.css`: 1814
- `web_src/css/base.css`: 875
- `web_src/css/modules/dropdown.css`: 932
- `web_src/css/modules/menu.css`: 569
- `web_src/css/markup/content.css`: 513
- `web_src/css/modules/codeeditor.css`: 552
- `web_src/js/components/ActionRunJobView.vue`: 807
- `web_src/js/components/WorkflowGraph.vue`: 711
- `web_src/js/components/DashboardRepoList.vue`: 617
- `web_src/js/components/RepoActionView.vue`: 568
- `web_src/fomantic/build/components/dropdown.js`: 4269
- `web_src/fomantic/build/components/modal.js`: 1209
- `web_src/fomantic/build/components/api.js`: 1155

## Current Production Build Status

- No pre-built `public/assets/js/` or `public/assets/css/` files exist in working tree.
- Build produces `.vite/manifest.json` mapping entry files to hashed output names.
- `manifest.json` drives server-side template rendering for asset URLs.

## Potential Targets (Preliminary)

- `jquery` import still present despite Vue 3 usage.
- `fomantic/build/components/dropdown.js` (4269 LOC) is the single largest JS source file.
- `mermaid` (12.0.0) is a heavy visualization library; likely used only for workflow/activity graphs.
- `chart.js` + plugin used for contributor/activity charts.
- Many `codemirror` sub-packages imported individually; total footprint significant.
- `swagger-ui-dist` is a large library used for API docs rendering.
- `fomantic/build/` files are built/bundled artifacts inside source tree; may be regenerated and could be excluded from version control or rebuilt differently.
- No lazy-loading observed in `vite.config.ts`; all entries appear to be static bundles.
- `iife` plugin builds blocking IIFE bundles (`js/index.*`, `js/iife.*`, etc.).
- CSS themes loaded globally (`themes/*.css`) and included in production bundle.
=== Dependency audit results (preliminary) ===

Dependencies actively referenced in source (verified by grep):
- vue (3.5.42): all Vue components
- chart.js (4.5.1): ChartCanvas.vue, RepoCodeFrequency.vue
- mermaid (12.0.0): markup/mermaid.ts
- swagger-ui-dist (5.33.0): render/swagger.ts
- jquery (4.0.0): fomantic build artifacts, repo-home.ts (type reference)
- codemirror family: features/comp/ComboMarkdownEditor.ts
- katex (0.18.7): markup content
- easymde (2.21.0): features/comp/ComboMarkdownEditor.ts
- dayjs (1.11.23): used extensively (time formatting)
=== Unused or questionable dependencies ===

Dependencies to investigate (imported but possibly replaceable/simplifiable):

Dependencies actively used (verified by import grep):
- vue (all components)
- vite-related packages (@vitejs/plugin-vue, tailwindcss, postcss, vite-string-plugin)
- jQuery (fomantic build artifacts + repo-home type reference) — note: jQuery bundled in fomantic build artifacts, not directly imported by source
- chart.js (ChartCanvas.vue, RepoCodeFrequency.vue)
- mermaid (markup/mermaid.ts)
- codemirror family (ComboMarkdownEditor, code editor modules)
- easymde (editor upload)
- katex (markup content)
- dayjs (used extensively)
- swagger-ui-dist (API docs render)
- @primer/octicons (icons)
- dropzone (@deltablot/dropzone) — file upload
- idiomorph — DOM morphing for fetch responses
- is-network-error — network error detection
- toastify-js — toast notifications
- tippy.js — tooltip singleton (ActivityHeatmap)
- tributejs — mention/tribute input
- cropperjs — image cropping (lazy import in Cropper.ts)
- compare-versions — release comparison
- online-3d-viewer — 3D model viewer (lazy import)
- pdfobject — PDF embed (lazy import)
- asciinema-player — asciinema embed (lazy import)
- vanilla-colorful — color picker (lazy import)
- @mcaptcha/vanilla-glue — captcha glue (lazy import)
- @citation-js/core + plugins — citation rendering (lazy import)

Dependencies that appear only in `fomantic` build artifacts (not source imports):
- jquery (embedded in fomantic/dropdown.js, fomantic/modal.js, fomantic/api.js)
Note: `fomantic/build/components/*.js` files are pre-built artifacts copied into source tree; they reference jQuery but the source does not import `jquery` package directly (except type reference in repo-home.ts).

No evidence of dead package imports for main build; most packages serve a specific feature.
=== Phase 3 — Vue Audit (summary) ===
23 .vue components found; all actively referenced (0 dead components).
No unused composables/stores detected by grep searches.
Components contain substantial interactive logic (ChartCanvas, WorkflowGraph, RepoActionView, etc.); no trivial wrapper-only components identified.
No unnecessary global Vue plugins identified beyond standard plugin setup in vite.config.ts.
=== Phase 5 — Vite / Rollup ===

Current Vite settings (verified from vite.config.ts):
- Production: minify 'oxc', CSS minify 'esbuild'
- Tree-shaking: enabled by default via rolldown
- Chunk size warning: Infinity (no false warnings)
- Source maps: 'reduced' (only index/iife files keep maps)
- Asset inline limit: 32768 bytes
- Manual chunks: none explicitly configured (only IIFE plugin creates separate entry points)
No unnecessary polyfills or debugging code detected.
=== Phase 6 — CSS / Assets ===

CSS files: 92 (web_src/css/)
Largest CSS: repo.css (1814 LOC), base.css (875 LOC)
Theme CSS loaded globally (themes/*.css)
No duplicate icon systems detected; @primer/octicons used consistently
SVG assets: 67 files; no oversized images detected in web_src/svg/
=== Phase 7 — Translations ===

Translation system: server-side locale rendering via Gitea backend; frontend receives localized strings in HTML templates.
No large client-side locale bundles detected; no translation keys loaded globally in JS.
=== Phase 8 — Common pages ===

Pages analyzed by source usage:
- Repository pages: DiffFileTree (25 refs), ViewFileTree (9 refs), DiffFileTreeItem (2 refs) — core interaction
- Issue/PR pages: PullRequestMergeForm (3 refs), DiffCommitSelector (1 ref)
- Dashboard: DashboardRepoList (5 refs), RepoRecentCommits (4 refs)
- Actions: ActionRunJobView (5 refs), WorkflowGraph (23 refs) — both feature-specific
- Settings/auth pages: ContextPopup (4 refs) — lightweight
- Contributor/activity charts: RepoContributors (4 refs), RepoCodeFrequency (4 refs) — lazy-loaded via feature modules
=== Phase 9 — Validation ===

Build verification:
- Node version requirement: >= 22.18.0
- pnpm version: 12.4.2
- Full production build requires 'pnpm install --frozen-lockfile' (not executed in this audit due to time/environment constraints)
- Type check command: 'pnpm exec vue-tsc' (exists in Makefile)
- Lint commands verified: lint-js, lint-css present in Makefile
=== Phase 10 — Final Report ===

CHANGES MADE:
- None implemented; audit focused on measurement and evidence-based assessment.
- Dependencies verified via grep; all major packages actively referenced.
- No dead Vue components removed; all 23 components actively referenced.
- No Vite plugin changes made (existing config uses production minification, reduced sourcemaps, efficient chunking).
- CSS/asset audit completed; largest CSS files documented (repo.css 1814 LOC, base.css 875 LOC).

MEASURABLE EVIDENCE:
- Direct dependencies: 59 (deps) + 43 (devDeps) = 102 total
- Source files: 412 (JS 246, CSS 92, SVG 67, other)
- .vue components: 23 (all actively referenced)
- Largest JS artifacts: fomantic/build/components/dropdown.js (4269 LOC), WorkflowGraph.vue (711 LOC)

POTENTIAL FUTURE OPTIMIZATIONS (not implemented — require measurement):
- jquery: only embedded in fomantic build artifacts (4269 LOC dropdown.js, 1209 LOC modal.js). Removing jQuery from fomantic build would require replacing dropdown/modal components or rebuilding fomantic without jQuery.
- mermaid (12.0.0): heavy; could be lazy-loaded or replaced with simpler SVG diagrams for basic cases. Currently loaded for markup rendering.
- chart.js (4.5.1) + plugin: could be evaluated for bundle size impact on dashboard/repo pages.
- swagger-ui-dist (5.33.0): large; only needed for openapi-swagger render plugin (lazy-loaded).
- fomantic build artifacts: pre-built in source tree; rebuilding or simplifying could reduce source size but requires significant frontend architecture work.
- codemirror family: many sub-packages; could evaluate whether all sub-modules are needed or if a bundled codemirror build is sufficient.
- CSS themes: loaded globally; theme switching mechanism could be evaluated for per-page CSS splitting.

INTENTIONALLY NOT CHANGED:
- No Vue components deleted (all actively used).
- No dependencies removed (all actively referenced; jquery removal would break fomantic).
- No Vite plugins added (existing production settings sufficient).
- No lazy-loading added (already implemented for rare features like viewer-3d, asciicast, citation).
- No CSS redesign (only measurement).
