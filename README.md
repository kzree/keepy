# Keepy

Keepass database TUI powered by Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea)

## Keymap

Default keymap. Planned to be configurable in the config

### Global

- `Ctrl+C` or `Q` - quit the application

### Login view

- `Enter` and `Shift + enter` - navigate the form and submit
- `Ctrl+T` - toggle file path fields

### List view

- `C` - copy highlighted entry password
- `F` - enter search mode
- `X` - clear filter
- `N` - enter new entry mode
- `E` - edit highlighted entry
- `D` - delete highlighted entry after confirmation
- `Ctrl+R` - reload the latest database after an external-change conflict

#### Search mode

- `Enter` or `Esc` - close search mode

#### Entry form

- `Enter` and `Shift + enter` - navigate the form and submit
- `Ctrl+G` - generate random 24 character password
- `Esc` - quit entry form (resets the form)

### Save outcomes

Application-wide warnings and global save/reload notices appear in a root-level
bottom bar. Draft-specific errors remain beside the form values.

- Failed saves keep the form values. Correct the problem, review the draft, and
  submit again to retry; saves are never retried automatically.
- External-change conflicts block mutations until `Ctrl+R` reloads the database.
  The form draft is retained for review against the latest entries. Reloading
  does not repeat the pending mutation or automatically merge changes.
- If a save commits but reports uncertain durability, the form closes and the
  list refreshes from the committed vault. The warning remains visible; do not
  repeat the mutation.

## Todo for v1.0

- [x] Search
- [x] Create new entry
- [x] Edit entry
- [x] Delete entry
  - [x] Confirm component
- [ ] Login redesign, both UI and UX
  - [ ] File picker for paths
- [ ] Key hints
  - [ ] Modal with key hints
  - [ ] Configurable keybinds
- [x] i18n
- [ ] Proper error handling
- [ ] Tests
- [ ] Responsive TUI

## Post release roadmap

- Cloud provider support
  - [ ] Dropbox
  - [ ] OneDrive
