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

#### Search mode

- `Enter` or `Esc` - close search mode

#### New entry mode

- `Enter` and `Shift + enter` - navigate the form and submit
- `Ctrl+G` - generate random 24 character password
- `Esc` - quit new entry mode (resets the form)

## Todo for v1.0

- [x] Search
- [x] Create new entry
- [ ] Edit new entry
- [ ] Delete entry
  - [ ] Confirm component
- [ ] Login redesign, both UI and UX
  - [ ] File picker for paths
- [ ] Key hints
  - [ ] Modal with key hints
  - [ ] Configurable keybinds
- [ ] Proper error handling
- [ ] Tests
- [ ] Responsive TUI
