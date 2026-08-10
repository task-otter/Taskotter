# FNM Taskfile Public Tasks

## What is fnm?

fnm (Fast Node Manager) is a cross-platform Node.js version manager written in Rust. It lets you install and switch between multiple Node.js versions on the same machine, and is compatible with `.nvmrc` and `.node-version` files used by nvm.

Unlike nvm, fnm is a single binary that works natively on Linux, macOS, and Windows — there is no separate implementation per platform. It is significantly faster than nvm due to being compiled ahead-of-time.

Key differences from nvm:

- **Single binary** — no shell function to source; fnm is invoked directly.
- **Shell integration via `eval`** — run `eval "$(fnm env --use-on-cd)"` to activate the selected Node.js version in your current shell session.
- **Cross-platform** — one tool for Linux, macOS, and Windows.

---

This document describes the public tasks exposed by the FNM Taskfile.

The Taskfile provides one cross-platform interface for managing fnm and Node.js versions.

Linux and macOS use the official fnm install script, installing fnm to `$HOME/.local/share/fnm` via `--install-dir`. Shell activation is configured separately by the `shell:setup` task.

Windows uses winget.

---

## Auto-install behaviour

Tasks that operate on Node.js versions automatically install fnm first if it is not already present — you do not need to run `task install` manually before using those tasks.

Tasks that only read state (`version`, `ls`, `node:version`) do **not** auto-install fnm. They fail with a clear error message if fnm is missing, so you know exactly what is needed.

Installs are **idempotent**: each internal install task has a `status` check that exits early when the target is already installed, so running any task multiple times is safe and only does work when something is actually missing.

| Task                             | Behavior when fnm is not installed                                      |
| -------------------------------- | ----------------------------------------------------------------------- |
| `version`, `ls`, `node:version`  | Fails with a clear error — run `task install` first                     |
| `node:install`, `node:uninstall` | Auto-installs fnm, then runs the Node.js operation                      |
| `node:use`                       | Auto-installs fnm → installs the Node.js version if missing → activates |

---

## Public Tasks

| Task             | Variables          | Description                                                                                                                                                                                                                                                    |
| ---------------- | ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `install`        | —                  | Install fnm for the current operating system and configure shell activation.                                                                                                                                                                                   |
| `install:undo`   | —                  | Remove fnm from the current operating system.                                                                                                                                                                                                                  |
| `version`        | —                  | Show the installed fnm version. Fails clearly if fnm is not installed.                                                                                                                                                                                         |
| `node:install`   | Optional `VERSION` | Install a Node.js version. If `VERSION` is omitted, install latest LTS. Accepts full versions (`18.0.0`), major versions (`20`), and aliases (`--lts`, `lts-latest`). Auto-installs fnm if missing.                                                            |
| `node:uninstall` | Required `VERSION` | Uninstall a Node.js version managed by fnm. Accepts the same version formats as `node:install`. Auto-installs fnm if missing.                                                                                                                                  |
| `node:use`       | Optional `VERSION` | Install (if needed) and activate a Node.js version. Resolves the version alias to the concrete installed version and sets it as the fnm default for new shells. If `VERSION` is omitted, use latest LTS. Auto-installs fnm and the Node.js version if missing. |
| `ls`             | —                  | List Node.js versions installed through fnm. Fails clearly if fnm is not installed.                                                                                                                                                                            |
| `node:version`   | —                  | Show the active Node.js and npm versions. Fails clearly if fnm is not installed.                                                                                                                                                                               |
| `shell:setup`    | —                  | Configure fnm activation in the current user's shell profile. Detects the shell, appends only missing lines (never duplicates). Supports bash, zsh, and fish on Unix and PowerShell on Windows.                                                                |

---

## Install fnm

Install fnm for the current platform.

```bash
task install
```

On Linux and macOS, fnm is installed to `$HOME/.local/share/fnm` using the `--install-dir` flag with `--skip-shell`, so the Taskfile controls shell configuration exclusively via the `shell:setup` task. The `install` task calls `shell:setup` automatically after installing, giving you a ready-to-use setup from a single command.

All Node.js tasks (`node:install`, `node:uninstall`, `node:use`) call `install` automatically, so this is only needed if you want to install fnm without doing anything else yet.

---

## Shell activation

The `install` task automatically calls `shell:setup`, which detects your current shell and writes the fnm PATH export and `eval "$(fnm env --use-on-cd --shell SHELL)"` activation line to the appropriate profile file (`~/.bashrc`, `~/.zshrc`, `~/.config/fish/config.fish`, etc.).

The setup is smart — it checks each profile file individually and only appends the lines that are missing. Running `shell:setup` multiple times is safe and never duplicates entries.

You can also run `shell:setup` independently at any time:

```bash
task shell:setup
```

After setup, restart your shell (or `source` the profile) for the activation to take effect. Once active, fnm automatically selects the right Node.js version when you `cd` into a directory with a `.node-version` or `.nvmrc` file.

To activate fnm in your **current** shell session without restarting, run:

```bash
eval "$(fnm env --use-on-cd)"
```

---

## Switching Node.js versions

`task node:use` installs the requested version (if missing), resolves the alias to the concrete version using `fnm current`, and sets that resolved version as the fnm default for all new shells.

```bash
task node:use                    # latest LTS
task node:use VERSION=20         # latest Node.js 20.x
task node:use VERSION=22.0.0     # exact version
task node:use VERSION=--lts      # explicit LTS alias
```

The `VERSION` variable accepts any format that `fnm install` understands: full semver (`18.0.0`), major-only (`20`), and string aliases (`--lts`, `lts-latest`).

---

## Security Notes

**Unix install script**: The `install` task fetches and pipes the fnm install script directly into `bash` (`curl | bash`). This is the method recommended by the fnm project. It relies on HTTPS transport for integrity — no additional checksum verification is performed. Review the script at `FNM_INSTALL_URL` before running in security-sensitive environments.

**Windows installer**: The `install` task uses `winget install Schniz.fnm`. winget verifies package signatures from the Microsoft Store source. No manual checksum step is required.
