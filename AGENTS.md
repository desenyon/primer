# Primer

Primer is a universal local-development environment launcher and diagnostic tool.

> Clone a repository. Run `primer`. Start coding.

Treat the terminal interface as part of the product. Build toward the user's full
Primer specification; the current implementation boundary is recorded in
`docs/STATUS.md`. Do not describe a narrow milestone as the finished product.

## Product loop

UNDERSTAND → CHECK → REPAIR → START → OBSERVE

Primer understands a repository as a local system. It should eventually identify
components, runtimes, package managers, environment requirements, commands,
services, infrastructure, ports, generated artifacts and migrations; build a
reviewable repair plan; launch services in dependency order; and remain useful as
a dashboard throughout the development session.

Build vertically. The first milestone is a real Node / Next.js repository:
detection → command preview → supervised launch → URL and logs → clean stop.
Then add environment detection, PostgreSQL, Redis, Compose, Prisma, doctor and
repair planning. Do not create empty screens or meaningless CLI subcommands.

The eventual MVP includes Node and Python, npm/pnpm/yarn/bun, Compose, env
templates, PostgreSQL, Redis and Prisma; deterministic checks; reviewed package
installation, Compose startup and development migrations; process supervision,
restart, stop and logs; and scan, plan, dashboard, diagnostics and palette views.

## Determinism and evidence

- Deterministic before intelligent. The core works offline without an LLM.
- Detection is read-only and never executes repository code.
- Every inference needs inspectable file evidence and an explicit confidence
  when it is inferred rather than established by a structured declaration.
- Structured declarations outrank README prose. Never trust README commands.
- Keep detectors small and composable. No giant parser.
- A repository can be a monorepo and contain multiple processes.
- Use dependency graphs for ordering; do not start dependent services blindly.
- Invalidate cached facts when relevant files change. Never let stale inference
  override repository truth.
- Do not recursively inspect `.git`, `node_modules`, vendor or generated trees.
- Respect `.gitignore` and explicit ignore settings when recursive scanning is
  introduced. Bound file reads and honor cancellation.

## Architecture and implementation

Go, Bubble Tea, Lip Gloss, Bubbles only where useful. Cobra and SQLite only when
they materially simplify real behavior. Prefer direct libraries and the standard
library. Ship one binary. Build macOS/Linux correctly before Windows support.

CLI/TUI → Application → Repository Model → Detectors → Resolvers → Actions →
Runtime Supervisor

The TUI consumes typed state. No detector, planner or OS process policy in Update.
Favor small packages, explicit interfaces, deterministic functions and readable
Go. Avoid global state, reflection-heavy abstractions, god objects and a utils
package. Keep dependencies and scope small.

Follow think-before-coding, simplicity-first, surgical-change and goal-driven
execution guidelines. State assumptions and observable acceptance criteria;
verify the complete path. Touch only what the task requires.

Use codebase-memory-mcp for code discovery, indexing first when necessary. Fall
back to `rg` when graph tools are unavailable or insufficient, or for non-code
files and string literals. Use Context7 to check current library documentation.

## Execution and repair

Repositories are untrusted input, including manifests, scripts, Makefiles,
Dockerfiles, READMEs and Primer configuration. Inspection and execution are
strictly separate. Show what will run before executing meaningful-risk commands.

Every repair action has an ID, description, risk (safe/review/destructive),
reversibility and preview. Group safe operations. Destructive operations always
require explicit confirmation. No deletion in `clean` without size preview and
confirmation. No database mutation without a described action.

Never casually edit application source, overwrite `.env`, delete databases,
reset Git, delete uncommitted files, force upgrades, globally install arbitrary
software, kill unrelated processes or mutate production services.

Never upload repository content without opt-in or transmit environment variables.
Never print secrets by default or write them to `.primer/`. Show environment names
and configured/missing/empty/invalid/optional states. Keep local development as the
boundary; do not build a cloud secret manager.

Supervise owned processes and their descendants. Handle SIGINT, SIGTERM, terminal
close and launch failure correctly. Record commands, cwd, PID, start/exit times,
exit status, restart count, output and ports. Never fake health or metrics.

Machine-local metadata belongs in `.primer/`; persistent team overrides belong
in optional `primer.yaml`. Do not generate a manifest merely because inference
is incomplete. SQLite, global state, snapshots and team infrastructure wait
until needed by working local behavior.

## Terminal identity

Calm, competent, terse, precise, fast, tactile, deliberate. No emoji,
motivational copy, chatty assistant voice or generic troubleshooting dumps.

The visual direction is an electric-blue halftone interpretation of the Creation
of Adam. Extract composition, contrast and grain; do not plaster hands, religious
language or Renaissance artwork throughout the product.

Palette:

- Primer Blue `#2D24E8`
- Primer Blue 2 `#392FFF`
- Paper `#F4F2EC`
- Ink `#111111`

Use semantic theme tokens. Occasional whole blue regions, near-white detail,
dramatic negative space, editorial composition and restrained print texture.
Alternate spare milestone screens with dense operational views.

Hierarchy uses whitespace, alignment, weight, sparse capitals, background fields
and inversion. Borders mean focus, inspection, overlays or confirmation. No card
soup, rainbow/purple gradients, hacker green, faux HUDs, excessive icons, block
spam, excessive borders or perpetual decorative motion.

Consistent vocabulary:

- `●` running / ready
- `◐` attention / partial
- `○` stopped / unavailable
- `×` failure
- `→` action
- `✓` completed

Color must never carry meaning alone. Support truecolor, 256-color, ANSI16,
NO_COLOR and dumb/CI plain output. Respect TERM and COLORTERM. Eventually provide
motion, texture and color settings. Never add a startup delay for branding.

Subtle texture (`· . : ░ ▒ ▓`) belongs in scanning, transitions or occasional
hero/empty states. Keep operational content clean and readable. Long-running
work never blocks input. Target responsive rendering, not decorative 60 FPS work.

Work at 80×24, 120×35 and wide sizes. Cap content width deliberately. For every
significant screen inspect hierarchy, selected state, no-color, errors and empty
states. A working screen is not automatically visually complete.

Keyboard: arrows/j/k navigate, enter inspect/approve, esc back, / search, : or
ctrl+k palette, l logs, r restart, s stop/start, o open, d doctor, ? help, q quit.
Show only useful shortcuts. No mouse requirement. CLI help and useful plain/JSON
interfaces must coexist with the TUI. Never leak interactive assumptions into CI.

Logs preserve source, stream, timestamps and original data; presentation is a
view. Build toward filter, search, pause, follow, copy, save, error navigation and
service visibility. Never emit hostile terminal controls from repository output.

## Verification and completion

Use fixture repositories for model/detector/planner behavior and real subprocess
tests for lifecycle. Separate model, detection, planning, process and UI tests.
Selected golden states are acceptable; do not rely heavily on screenshots.

The eventual polished demo is Next.js + pnpm + PostgreSQL + Redis + Prisma with
a wrong Node version, Redis stopped, one missing env variable, pending migrations
and port 3000 occupied. Primer identifies the failures, previews repairs, applies
approved changes, launches the system and enters the dashboard.

Do not claim this demo or the full MVP complete without fresh end-to-end evidence.
Maintain exact limitations and the next vertical milestone in `docs/STATUS.md`.
