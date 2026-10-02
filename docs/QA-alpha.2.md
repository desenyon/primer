# Alpha.2 verification

This record distinguishes detector fixtures, illustrative UI renders, real local
execution and published release verification.

## Local checks

Focused detector, CLI and TUI tests pass. They cover cross-language model/evidence,
Python manager/script declarations, read-only inspection, unsafe symlink rejection,
review gating, known-secret redaction, and 80×24 / 120×35 / 180×45 / 40×16 bounds.
`go vet ./...`, shell syntax checks and whitespace checks pass.

The full `go test -race ./...` suite passed on macOS, including actual socket
collision and process-group lifecycle tests. Four macOS/Linux ARM64/AMD64 archives
built successfully. Fresh native CI and installed-release checks follow publication.

## Real execution on macOS ARM64

Five servers served HTTP 200: FastAPI/uv, requirements-based Python, Go,
Vite/TypeScript and generic Procfile. Each stopped with exit 0, closed its port,
and left no owned process group. Seven Go/Rust/TypeScript/Makefile build/test/check
commands also exited 0. [Machine-readable smoke results](alpha.2-smoke-proof.json).

Bare `primer` was tested in 80×24 truecolor PTYs against Python and Vite/TypeScript:
environment view, command browser, separate launch review, live readiness output,
logs, quit and cleanup all passed. [PTY results](alpha.2-pty-proof.json).

## Visual review

Actual View-method output was rendered and inspected at 80×24, 120×35 and 180×45.
The blue launch field, selected command row, aligned process columns, diagnostics,
logs, environment state and empty/no-color behavior were reviewed. Illustration
uses dummy project data; it does not demonstrate a running multi-service stack.

- [Launch review](visual-review/plan-80x24.png)
- [Dashboard](visual-review/dashboard-120x35.png)
- [Wide dashboard](visual-review/dashboard-180x45.png)
- [Commands](visual-review/commands-80x24.png)
- [Environment](visual-review/environment-120x35.png)
- [Diagnostics](visual-review/diagnostics-80x24.png)

## Scope

FastAPI, requirements-based Python, Go, Vite/TypeScript and Procfile launch checks
use isolated copies of published fixtures. Framework versions and dependency
installation belong to those copies. Django/Flask/Poetry have deterministic
pattern support; they are not being claimed as independently smoke-tested stacks.
Native Linux lifecycle coverage comes from CI, rather than a macOS cross-build.
See [STATUS](STATUS.md) for the remaining limitations.

The conventional FastAPI inference path also passed HTTP 200, reload-process cleanup
and exit 0 without a Procfile override. [Inference proof](alpha.2-inference-proof.json).
The existing real Next.js fixture passed the same launch/shutdown regression checks.
[Next.js regression proof](alpha.2-next-proof.json).
