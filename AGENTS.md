# Repository Guidelines

## Keep These Guidelines Current

Keep `AGENTS.md` up to date when new repository conventions are established or
existing conventions change. Record durable agreements here, and remove stale
guidance. Distinguish agreed conventions from proposals or unfinished roadmap
work.

## Project Structure

Keepy is a KeePass database TUI written in Go using Bubble Tea, Bubbles, Huh, and
Lip Gloss.

- `cmd/app/`: application entry point.
- `internal/keepy/`: application startup.
- `internal/service/`: vault operations, configuration, and password generation.
- `internal/ui/`: root model, screens, components, and message handling.
- `internal/control/`: key bindings.
- `internal/textprovider/`: localization.
- `internal/style/`: shared visual styles.
- `internal/util/`: utilities and shared test utilities.

Follow the surrounding naming and structure. Keep business logic in services and
UI state transitions in the relevant Bubble Tea models.

## Scope and Roadmap Work

- Implement the requested task and its verification. Do not silently expand a
  task into the rest of the roadmap.
- Break larger tasks into smaller actions with clear goals and completion
  criteria.
- Clearly distinguish documented target guarantees from guarantees the code
  actually enforces. A save contract is a development specification, not a
  runtime component.
- When a separate subagent review is requested, obtain it after implementation,
  fix substantial findings, and repeat review as needed until none remain.
  Optional suggestions do not automatically expand the task scope.

## Development Documentation

- Put local reviews, implementation plans, and development contracts in
  `dev-docs/`, which is ignored by Git.
- Do not commit those generated development Markdown files unless explicitly
  requested.
- Keep committed code understandable without access to ignored development
  documents; put necessary API guarantees and lock requirements in code comments.
- `README.md` and this `AGENTS.md` are repository documentation, not local
  development notes, and may be tracked normally.

## Go and Test Conventions

- Format changed Go files with `gofmt`.
- Prefer focused regression tests that verify behavior and failure recovery over
  tests that merely increase coverage or mirror the implementation.
- Extract repeated test setup and plumbing into small, clearly named helpers.
  Keep scenario-specific inputs, actions, and expectations visible in the test.
- Vault-specific helpers belong in
  `internal/service/vault_test_helpers_test.go`. Shared helpers belong in
  `internal/util/testutil/`.
- Helpers that accept `*testing.T` must call `t.Helper()` so failures point to
  the calling test.
- Concurrent test workers should return errors. Run fatal assertions on the test
  goroutine after the workers finish; do not call `require`/`FailNow` in workers.
- Use temporary directories for filesystem tests. When changing the test home,
  use `testutil.TempHome`, which sets both `HOME` and `USERPROFILE` through
  `t.Setenv` for Unix and Windows compatibility and automatic restoration.

## Vault and Persistence Conventions

- Share a `*Vault`; do not copy a Vault after first use because it contains a
  mutex.
- Public vault methods acquire the appropriate lock. Mutations, saving, and
  authentication use an exclusive lock; reads use a shared lock.
- Lock-held helpers must not reacquire the mutex. Document their lock ownership
  requirements. For example, a CRUD method holding the lock calls `lockAndSave`,
  not the public `LockAndSave` method.
- Preserve absolute paths and resolve supported home-relative paths correctly.
  Saves and reauthentication use the resolved paths stored by authentication,
  including resolved symlink targets.
- A file baseline must fingerprint the same bytes decoded during authentication
  and capture permissions from the opened file.
- Treat successful atomic replacement as the transactional commit boundary.
  Distinguish uncommitted failures, conflicts, and committed saves with uncertain
  durability. Do not label legacy failures as transactional outcomes until their
  guarantees are implemented.
- Never automatically retry a mutation whose replacement has already committed.

## Verification

UI save handling must distinguish uncommitted failures/conflicts from committed
durability errors. Retain failed drafts for explicit retry; never automatically
repeat a mutation. Conflict reloads preserve drafts for review and do not submit
them. Guard queued mutation, edit-opening, and confirmation messages with request
identities so stale commands cannot affect a newer operation. Reserve viewport
space for save/recovery notices.

Run checks appropriate to the change:

```sh
go test ./...
go vet ./...
```

For locking or concurrency changes, also run `go test -race ./...` or the relevant
package under the race detector. Before committing, check `git diff --check`.
Documentation-only changes do not require Go tests.

The test workflow runs tests, race tests, and vet on native Linux, macOS, and
Windows runners, using the Go version in `go.mod`. Keep the platform checks
alongside the existing Linux coverage job. Cross-compilation does not substitute
for native runtime verification of persistence behavior.

## Commits

- Commit only when the user requests it; do not push unless requested.
- Inspect status, the diff, and recent commit messages before committing. Stage
  only intended files, and leave unrelated user changes untouched.
- Follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/)
  and the repository's existing style: `type: concise lowercase description`.
  Use an optional scope or breaking-change marker when appropriate. Examples:
  - `feat: add entry editing`
  - `fix: disable q keybind when input is active`
  - `refactor: pin vault paths and capture database baseline`
  - `test: improve test coverage for services`
  - `ci: create coverage report on test run`
- Add a commit body when useful to explain motivation, important design choices,
  or behavior changes that the subject cannot capture. Separate it from the
  subject with a blank line. Document breaking changes with a `BREAKING CHANGE:`
  footer when applicable.
- Run the repository hooks normally. Do not bypass them. If a hook fails, fix the
  issue and retry the commit.
- After committing, verify status and report the commit hash and message.
