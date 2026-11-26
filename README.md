# lazymanager

A TUI package manager inspired by lazygit and lazydocker. It brings together system package managers (Linux, *BSD, and Homebrew on macOS) and, when running inside a repository, the project's package manager as well.

## Installation

```sh
go install github.com/shenri1/lazymanager
```

Or, build from source:

```sh
go build -o lazymanager .
```

## Usage

```sh
lazymanager            # Inspects the current directory
lazymanager -C ~/proj  # Inspects another directory
lazymanager --version
```

## Keybindings

| Key | Action |
| --- | --- |
| `tab` / `shift+tab` | Switches the focused panel |
| `1` `2` `3` `4` / `h` `l` / `←` `→` | Focuses Managers / Packages / Updates / Details |
| `j` `k` / `↑` `↓` | Moves the selection |
| `pgup` / `pgdown` | Moves one page in lists / scrolls Details |
| `g` / `G` | Jumps to the beginning / end |
| mouse | Click to focus and select; scroll wheel moves the list or scrolls Details |
| `/` | Filters packages (case-insensitive; `esc` clears the filter) |
| `s` | Searches the remote repository |
| `i` | Installs one or more packages (space-separated, e.g. `htop btop`) |
| `d` | Removes the selected package (confirmation required) |
| `u` | Upgrades the selected package or update (confirmation required) |
| `U` | Upgrades all packages (confirmation required) |
| `A` | Upgrades everything: all system managers with pending updates |
| `r` | Reloads packages and updates for the current manager |
| `enter` | Loads package or update details |
| `?` | Shows help |
| `q` / `ctrl+c` | Exits |

Commands that modify the system run inside the TUI, with live output displayed in the **Details** panel. When `sudo` requires a password, a masked input modal prompts for it. The password is sent only to `sudo -S -v` through stdin (never as a command-line argument). Subsequent commands run with `sudo -n`.

This relies on the sudo credential cache (`timestamp_timeout` greater than zero, which is the default).

## Updates Panel

The four panels are arranged as follows: **Managers**, **Packages**, and **Updates** in the left column, and **Details** in the right column.

Updates are loaded in the background whenever you switch managers and cached by manager name, making it instantaneous to return to a previously visited manager.

The panel title displays `Updates (N)`. When a manager does not support listing updates, the panel displays `updates not supported for <name>`. When there are no pending updates, a small green "all good" indicator is shown.

Starting at 30 pending updates (`manyUpdates`), the title displays a yellow `⚠` warning, and the status bar shows an alert such as:

```text
dnf: 120 updates pending — press U to upgrade all
```

In the Updates panel:
- `enter` displays package information in Details.
- `u` upgrades the selected item.
- `U` upgrades everything.

All upgrade operations require confirmation.

After any command that modifies the system, packages and updates are automatically reloaded.

The following managers support update listing: `apt`, `dnf`, `pacman`, `brew`, `flatpak`, `go`, `npm`, `pip`, `uv`, and `poetry`.

Other managers remain available for all their supported operations but display a message indicating that update listing is unsupported.

## Supported Package Managers

**System:** `apt`, `dnf`, `pacman`, `zypper`, `apk`, `xbps`, `pkg` (FreeBSD), `brew`, `flatpak`.

**Project:** `go` (`go.mod`), `npm` (`package.json`), `pnpm` (`pnpm-lock.yaml`), `yarn` (`yarn.lock`), `bun` (`bun.lockb` / `bun.lock`), `cargo` (`Cargo.toml`), `uv` (`uv.lock`), `poetry` (`poetry.lock`), `pip` (`requirements.txt`), `bundle` (`Gemfile`), `composer` (`composer.json`).

Only package managers whose binaries are available in `PATH` are detected. Project-level managers also require their corresponding detection files to be present in the inspected directory.

When a `pnpm`, `yarn`, or `bun` lockfile is detected, `npm` is not offered.

## Project Structure

```text
.
├── main.go
├── internal
│   ├── manager
│   │   ├── manager.go
│   │   ├── table.go
│   │   ├── parse.go
│   │   └── manager_test.go
│   └── ui
│       ├── model.go
│       ├── update.go
│       ├── mouse.go
│       ├── view.go
│       ├── run.go
│       └── ui_test.go
├── go.mod
└── go.sum
```
