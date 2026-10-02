# Primer implementation checkpoint

The supplied product direction is preserved as operating constraints in
`AGENTS.md`. This is the first vertical milestone, not the full MVP.

## Implemented

- Read-only, bounded Node detection: project location, Next.js dependency,
  package manager declarations/lockfiles, dev scripts, numeric Node pins,
  environment templates and common Next.js port declarations.
- Structured model with evidence, heuristic confidence and manifest fingerprint.
- Doctor: installed runtime/manager versions, conflicting declarations,
  missing dependencies, missing/empty template variables and local bind checks.
- Command preview including predev/dev/postdev. Long previews wrap and scroll;
  approval waits until the end is visible. Execution rechecks the manifest and
  local prerequisites after review.
- macOS/Linux process groups, stdout/stderr capture, restart, cancellation,
  graceful stop followed by bounded escalation, and descendant cleanup when
  the launcher exits. Raw output remains in bounded private session memory;
  views strip terminal controls and redact known private environment values.
- Immediate scan, blue-field review, dashboard, diagnostics/evidence, logs with
  filtering/scroll/pause/follow, help and session command palette.
- Plain/JSON doctor and env; headless start with explicit `--yes`; useful default
  inspection in CI/non-TTY mode. No implicit repository execution in CI.
- Canonical GitHub module path, embedded release version, shell installer,
  checksummed macOS/Linux ARM64/AMD64 archives, editorial README artwork,
  native-platform CI and tag-triggered alpha release publishing.

## Verification — 2026-10-01

- Full ordinary Go tests passed on macOS, including an actual loopback collision,
  process-tree termination and escalation, final stderr capture, cancellation,
  secret redaction, changed-script rejection and inspection without execution.
- `go test -race ./...` passed for all packages, including local checks.
- `go vet ./...` passed.
- TUI views were rendered and inspected at 80×24, 120×35 and 180×45, with selected
  diagnostic, error and no-color empty states. Tests also bound output at 40×16.
- A live PTY session exercised scan → diagnostics → quit and terminal restoration.
- Linux/amd64 cross-build passed; native Linux runtime behavior remains untested.
- Real Next.js 16.3.8 / pnpm 11.4.0 smoke passed in an isolated temporary copy:
  doctor returned no issues, explicit headless approval launched the server,
  HTTP returned 200 with fixture content, and ready output was streamed. SIGINT
  produced exit 0, the owned process group disappeared, and the port was closed.
  Results are recorded in `next-smoke-proof.json`. The fixture's temporary dev
  script declared its chosen test port; Primer did not choose an alternate port.

The images in `visual-review/` are static renders of real View methods with
illustrative model data, rather than screenshots of a live multi-service system.
The wide render deliberately caps content width and leaves negative space.

## Current boundaries

Inspection targets the nearest Node package; a root dev command can delegate to
a monorepo, but there is no per-component service model or dependency graph yet.
The runtime owns one dev launch group. There is no detached-session persistence,
CLI status/stop, CPU/memory telemetry, readiness protocol or Git dashboard state.
The displayed URL is expected, not verified health.

Missing dependencies block launch. No install, runtime activation, automatic
alternate port, Compose, database, Prisma, environment-file copy, cleanup or
snapshot action is implemented. Doctor checks that node_modules is populated;
it does not verify dependency integrity or lockfile synchronization.

Environment templates establish names, not required/optional semantics. Empty or
absent names are advisory. Values are not evaluated as shell expressions. Known
secrets are concealed; arbitrary unknown credentials printed by applications
cannot be identified reliably. Nothing is uploaded or persisted.

Windows launch explicitly reports unsupported process supervision. macOS has
native execution evidence; Linux requires native runtime validation in addition
to compilation. Fatal crashes/SIGKILL recovery and machine-local state remain
future work. No release-readiness claim is made.

## Next vertical milestone

Add a reviewed dependency-install action and explicit typed repair plan, then
Compose service evidence and ordered PostgreSQL/Redis startup. Continue with
Prisma generation/migration checks, development migration previews, and the
broken Next.js/PostgreSQL/Redis/Prisma demo from the product specification.

## Distribution checkpoint

The release is explicitly marked `v0.1.0-alpha.1`. Installation places a single
binary at `~/.local/bin/primer` without sudo or shell-profile edits. The installer
requires a valid, unique checksum and matching embedded version before replacing
the destination. Automated tests cover successful installation, corrupted or
ambiguous checksums, unsupported platforms and mismatched binary versions.

The prepared CI workflow tests native macOS and Linux with the race detector.
Tag publishing waits for both platforms to pass. The remote workflow has not run
yet because public publication awaits explicit approval. Release archives include the MIT license. Git history is
created with one file per commit, as requested.

The codebase-memory MCP architecture/index requests did not return usable data
in this session. Index the actual implementation before future graph discovery.

The locally built macOS ARM64 release was installed on PATH. Bare `primer`
completed review → launch → dashboard → quit against the real Next.js fixture.
HTTP returned 200, ready logs appeared, quit returned 0, and the owned process
group and listening port disappeared. Results are in `distribution-proof.json`.
The GitHub download/install path remains unverified until publication.
