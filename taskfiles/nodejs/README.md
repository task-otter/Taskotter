# Node.js Taskfile

## What is this module?

Installs Node.js via the Nix user profile (`nixpkgs#nodejs` by default).
Package managers (`npm`, `yarn`, `pnpm`) and JS tool Taskfiles depend on
`nodejs:install` before running Node-backed commands.

## Usage

```sh
task nix:install:profile NIX_INSTALLABLE=nixpkgs#nodejs
```

Or include this module and depend on `nodejs:install`.

## Public Tasks

| Task | Description |
|---|---|
| `install` | Install Node.js via the Nix profile |
| `version` | Show the active Node.js version |

Dependents auto-install Node.js via `nodejs:install`.

## Variables

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `NODEJS_NIX_INSTALLABLE` | `nixpkgs#nodejs` | Flake installable for `nix:install:profile` |

## Notes

Pin Node.js by overriding `NODEJS_NIX_INSTALLABLE`. Native Windows auto-install
requires WSL2 (same as other Nix profile modules).
