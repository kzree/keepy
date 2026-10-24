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

Before implementing, look for similar code and existing helpers that can be
reused or extended. Keep code clean and readable: avoid duplication, unnecessary
abstractions, and extra files or helpers that add more noise than clarity.

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
- Root UI test fakes and shared command-driving helpers belong in
  `internal/ui/test_helpers_test.go`.
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

The root-owned BottomBar renders application-wide errors and global save/reload
notices. Screens receive the remaining content space and must not append global
notices themselves. Keep draft-specific errors and review guidance in the form.

Keymap help reuses application binding constants and the effective table/form
navigation maps. `?` opens help only outside text inputs; `F1` works everywhere.
Help preserves underlying screen models and continues processing background
completions. Help input must not trigger actions in the underlying view.

Responsive layout changes preserve filters, selected entry identity, draft values,
and form focus. Resize existing form models rather than rebuilding them. Keep the
table's row schema stable when hiding columns, and include cell padding and pane
borders in width/height budgets.

Login updates saved credentials without resetting existing preferences. A config
save error is reported separately and must not undo successful authentication or
prevent using the opened vault.

Login browsing must preserve entered values on selection and cancellation, and
file selection must never submit authentication automatically. Include a key file
in the authentication request only when explicitly enabled. Scope directory-read
completions to their picker session/request so stale results cannot replace a
newer listing or reopen a cancelled picker.

Startup config-load failures require explicit reload before login; never overwrite
an unreadable or malformed config with fallback credentials. Clipboard startup
failure disables copying without blocking vault use. Check clipboard readiness
before retrieving a password for copying, and preserve independent notices as
individual startup/config/copy errors recover.

Run vault authentication, mutations, and reauthentication through `tea.Cmd`.
Commands capture immutable inputs and return entry snapshots; only `Update`
changes UI state. Root accepts one vault operation at a time and validates its
full identity before applying completion. Do not read the vault from `Update`
while a worker may hold its lock. Quit key requests wait for accepted work to
finish rather than abandoning a potentially committed save.

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
- Use a commit body only when necessary to explain important context that the
  subject cannot capture. Prefer a subject-only commit. Separate a needed body
  from the subject with a blank line. Document breaking changes with a
  `BREAKING CHANGE:` footer when applicable.
- Run the repository hooks normally. Do not bypass them. If a hook fails, fix the
  issue and retry the commit.
- After committing, verify status and report the commit hash and message.
