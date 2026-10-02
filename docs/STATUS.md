# Primer implementation checkpoint — alpha.2

Primer now runs a useful cross-language slice of the product loop. It remains an
alpha, rather than the full universal-environment MVP in `AGENTS.md`.

## Implemented

- Bounded, read-only detection of Node/TypeScript, Python, Go and Rust projects.
  Nearest project markers are found without recursively scanning dependency trees.
- Node: npm/pnpm/yarn/bun declarations and lockfiles, runtime pins, Next.js/Vite/
  Astro/Nuxt dependency identity, declared scripts and `start` fallback.
- Python: real TOML parsing, pyproject/requirements, uv/Poetry/pip evidence, Python
  requirements/pins, project scripts and conventional FastAPI/Django/Flask/main.py
  launch inference. uv runs disable syncing and automatic Python downloads.
- Go/Rust: conventional binary entrypoints and declared toolchain build/test commands.
- Generic repositories: Procfile processes and simple Makefile targets. An explicit
  dev target outranks a web process when both are present. No README execution.
- Structured commands, evidence, confidence, bounded source fingerprints and
  changed-evidence rejection after review. Intermediate symlink directories are
  rejected during source inspection.
- Installed runtime/manager checks, numeric version constraints, conflicting
  lockfiles, Node dependency presence, Python environment presence, environment
  names/states and declared/inferred local-port checks.
- `primer commands`, `primer run NAME`, `primer why`, doctor/env JSON and plain
  output. Unknown command names fail closed; non-interactive execution needs
  explicit `--yes`. Libraries can run build/test without a dev declaration.
- Refined blue review field, aligned session table, project-command browser,
  environment view, repository-command palette, diagnostics/evidence, logs with
  filter/scroll/pause/follow and keyboard help. Selection leads to another review.
- macOS/Linux process groups, concurrent output capture, restart, cancellation,
  graceful stop/escalation and descendant cleanup. One selected launch group.
- Known-value secret redaction in script and argv previews, reports and log views;
  hostile terminal controls are stripped. Raw logs stay in bounded session memory.
- Checksummed macOS/Linux ARM64/AMD64 archives, versioned installer, native CI and
  tag-triggered prereleases. Each changed file is committed individually.

## Verification

Fresh alpha.2 results are recorded in [QA-alpha.2.md](QA-alpha.2.md). Historical
alpha.1 Next.js and published-install proofs remain available, with their original
versions and provenance. Screenshots in `visual-review/` are real View-method
renders with illustrative model data, not live multi-service screenshots.

## Current boundaries

One nearest project and one selected command group; a declared command can itself
start children, but Primer has no per-component dependency graph. No detached
state, CLI status/stop, CPU/memory telemetry, readiness protocol or Git dashboard.
URLs are expected addresses, rather than assertions of healthy services.

Python inference uses a few conventional files and explicit app assignments, not
AST/import analysis. A Procfile supports unfamiliar layouts. Poetry can use its
own environment. Other Python projects require `.venv`; doctor checks its presence
and the system interpreter constraint, not installed modules or the virtualenv's
interpreter version. Numeric Python constraints are supported; unsupported syntax
fails closed. Node dependency presence does not prove integrity or lockfile sync.

Makefile discovery covers simple literal targets; recipes/includes/expansions
execute only after review. Included files are not expanded or fingerprinted.
Procfile commands run through a shell and are shown in full before approval.
Go/Cargo commands can fetch dependencies as part of their reviewed execution.
Version inspection refuses repository-local tool executables.

Environment templates establish names, not required/optional semantics. Missing
or empty entries are advisory. No shell evaluation of env files, env-file copying,
package installation, runtime activation, alternate-port repair, Compose/database/
Prisma action, cleanup or snapshot is implemented. `why` explains current checks;
it does not persist failures. Arbitrary unknown credentials in output cannot be
reliably identified. Primer uploads and persists nothing during normal use.

Windows process supervision is explicitly unsupported. Fatal crashes/SIGKILL
recovery and machine-local state remain future work. Cross-builds do not establish
native execution on every architecture.

## Next vertical milestone

Reviewed dependency installation and typed repair actions, followed by Compose
service evidence and ordered PostgreSQL/Redis startup, then Prisma checks and the
broken-system demo. Keep launch inference conservative and every action reviewable.
