# Keepy

Keepass database TUI powered by Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea)

## Keymap

Default keymap. Application actions can be rebound in the config; see
[Custom keybindings](#custom-keybindings).

### Global

- `Ctrl+C` or `Q` - quit the application
- `?` - open keymap help outside text inputs
- `F1` - open keymap help everywhere, including forms and search

If a vault operation is in progress, a quit request waits for its result.

### Keymap help

Help is a separate, scrollable view with global bindings and groups for login,
the entry list, search, entry forms, delete confirmation, and recovery. The current
context is marked; conditional shortcuts describe when they apply.

- `Esc`, `?`, or `F1` - close help and return without losing the underlying state
- `Up`/`Down` or `k`/`j` - scroll
- `Page Up`/`Page Down` - scroll by a page
- `Home`/`End` - first/last help page

The BottomBar stays visible and background operations continue while help is
open. Question marks remain normal text inside inputs. `q` quits from help;
it waits for accepted vault work to finish.

### Login view

- `Enter` or `Tab` - next field; `Enter` submits on the last field
- `Shift+Tab` - previous field
- `Ctrl+T` - toggle file path fields
- `F2` - browse for a database
- `F3` - browse for a key file (enables key-file authentication on selection)
- `F4` - enable/disable the optional key file; retain its path in the current form
- `Ctrl+R` - retry configuration loading after a startup configuration error

The selected database and key-file state stay visible above the login form.
Manual paths remain supported with `Ctrl+T`. Disabling a key file uses password-only
authentication; enabling it requires selecting or entering a key-file path.

#### Login file picker

- `Enter` - open a directory or choose a file
- Arrow keys or `j`/`k` - move; Page Up/Down - page through files
- `Left`, `Backspace`, or `h` - parent directory
- `Esc` - cancel browsing without discarding entered values
- `Ctrl+R` - retry a failed directory read
- `?` or `F1` - open help

Both file pickers start in the home folder each time they are opened.
Selecting a file returns to the form; it never submits authentication by itself.

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

- `Enter` or `Tab` - next field; `Enter` submits on the last field
- `Shift+Tab` - previous field
- `Ctrl+G` - generate random 24 character password
- `Esc` - quit entry form (resets the form)

### Save outcomes

Application-wide warnings and global save/reload notices appear in a root-level
bottom bar. Draft-specific errors remain beside the form values.

Authentication, saves, and conflict reloads run in the background. Loading
indicators and resizing remain responsive; further vault actions and form input
wait until the current operation finishes.

- Failed saves keep the form values. Correct the problem, review the draft, and
  submit again to retry; saves are never retried automatically.
- External-change conflicts block mutations until `Ctrl+R` reloads the database.
  The form draft is retained for review against the latest entries. Reloading
  does not repeat the pending mutation or automatically merge changes.
- If a save commits but reports uncertain durability, the form closes and the
  list refreshes from the committed vault. The warning remains visible; do not
  repeat the mutation.

### Responsive layout

The entry table sizes its columns to the available space. On narrower screens,
group and URL columns are hidden first, then username/email; titles remain visible.
Title/username filtering still works when the username column is hidden. Entry
forms use a side pane when there is enough room, or the full content area
otherwise. Narrow forms stack labels above inputs.

Resizing preserves filters, selection, draft values, and the focused field.
The BottomBar and help screen keep their own bounded space.

### Startup recovery

If the configuration cannot be read or parsed, Keepy shows the error in the
bottom bar and waits for you to correct the file or its access permissions.
Press `Ctrl+R` to retry loading it, or `Ctrl+C` to quit. Login is blocked until
loading succeeds, so an existing broken configuration is not overwritten with
fallback credentials.

Clipboard initialization runs separately. If it fails, the vault remains usable
and copying is disabled. Restore clipboard access and restart Keepy to enable
copying; the clipboard library caches initialization failures for the process.
Ordinary clipboard-write failures can be retried explicitly with `C`.

### Clipboard timeout

Copied passwords are cleared from the clipboard after 2 minutes, and when Keepy
quits. If another application has replaced the clipboard contents in the
meantime, they are left alone. Copying again restarts the timer. Configure it in
`~/.config/keepy/config.toml`:

```toml
[preferences]
clipboard_timeout = "30s" # Go duration such as "45s", "2m", "1h30m"
# clipboard_timeout = "off" # keep copied passwords until overwritten
```

An invalid value keeps the 2-minute default and is reported in the bottom bar.
If clearing fails, the bottom bar asks you to clear the clipboard manually.

### Custom keybindings

Override actions in `~/.config/keepy/config.toml` with a `[keybindings]` table.
Each action takes a list of keys; unlisted actions keep their defaults.

```toml
[keybindings]
copy_entry = ["c", "ctrl+y"]
show_help_anywhere = ["f12"]
```

Actions: `quit`, `quit_alt`, `show_help`, `show_help_anywhere`,
`login_toggle_paths`, `login_browse_database`, `login_browse_key_file`,
`login_toggle_key_file`, `copy_entry`, `new_entry`, `edit_entry`,
`delete_entry`, `search`, `clear_filter`, `generate_password`, `reload`,
`confirm`, `cancel`.

Keys use Bubble Tea names: modifiers `ctrl`, `alt`, `shift`, `meta`, `hyper`,
`super` joined with `+` (`ctrl+r`), named keys such as `enter`, `tab`, `space`,
`up`, `pgdown`, `home`, `f1`–`f24`, or a single character. Write shifted
characters directly (`"A"`, `"?"`), not as `shift+a`.

Esc, Enter, and table, picker, help, and text-editing navigation keys are fixed.
Keepy rejects overrides that conflict with them or with another action in the
same view, and keys that would type text in a text input. If any override is
invalid, all overrides are ignored, defaults are used, and the bottom bar lists
the problems; login is not blocked and the file is not rewritten. The help view
always shows the keys in effect: customized bindings are marked with `✱`, keys
that differ from the defaults are highlighted, and the default is shown when the
terminal is wide enough.

## Todo for v1.0

- [x] Search
- [x] Create new entry
- [x] Edit entry
- [x] Delete entry
  - [x] Confirm component
- [x] Login redesign, both UI and UX
  - [x] File picker for paths
- [ ] Key hints
  - [x] Scrollable keymap help (`?` / `F1`)
  - [x] Configurable keybinds
- [x] i18n
- [ ] Proper error handling
- [ ] Tests
- [x] Responsive TUI

## Post release roadmap

- Cloud provider support
  - [ ] Dropbox
  - [ ] OneDrive
