# TypeScript

## What is this Taskfile?

This Taskfile wraps common TypeScript workflows behind consistent, cross-platform
task commands. It covers installing TypeScript tooling, running `.ts` files with
`tsx`, static type-checking with `tsc`, compiling builds, inspecting compiler
configuration, and cleaning generated output.

`tsserver` is included for editor awareness only. It ships with the `typescript`
package and is managed by editors such as VS Code, Neovim, and other TypeScript
integrations.

## Variants

Every runtime + package-manager combination ships as its own leaf Taskfile under
`taskfiles/typescript/`. They all expose the identical public interface documented
below; only the underlying runtime and package manager differ. Include the tool
family once in your root Taskfile:

```yaml
includes:
  typescript: taskfiles/typescript/Taskfile.yml
```

Then run the leaf that matches your project through its namespace (replace
`{TASK}` with any public task):

```bash
task typescript:bun:{TASK}                 # Bun runtime + Bun as package manager
task typescript:node:fnm:npm:{TASK}        # Node via fnm, npm as package manager
task typescript:node:nvm:pnpm:{TASK}       # Node via nvm, pnpm as package manager
```

Available leaves: `bun`, `node/{fnm,nvm}/{npm,pnpm,yarn}`.

## Public Tasks

| Task                 | Variables                                     | Description                                                              |
| -------------------- | --------------------------------------------- | ------------------------------------------------------------------------ |
| `version`            | —                                             | Show resolved `tsc`, `tsx`, and `tsserver` information.                  |
| `tsserver:info`      | —                                             | Show where `tsserver` resolves from and how editors use it.              |
| `install`            | Optional `TYPESCRIPT_VERSION`                      | Install `typescript`, `tsx`, and `@types/node` using lockfile detection. Pass `TYPESCRIPT_VERSION=x.y.z` to pin the `typescript` package. |
| `install:undo`       | — | Remove the locally installed TypeScript dev dependencies.                |
| `upgrade`             | — | Reinstall `typescript`, `tsx`, and `@types/node` at their latest versions. |
| `run`                | Optional `TYPESCRIPT_FILE`, `TYPESCRIPT_TSX_FLAGS`, `CLI_ARGS`      | Execute one TypeScript file once with `tsx`.                             |
| `dev`                | Optional `TYPESCRIPT_FILE`, `TYPESCRIPT_TSX_FLAGS`                  | Run one TypeScript file in `tsx watch` mode.                             |
| `typecheck`          | Optional `TYPESCRIPT_TSCONFIG`, `TYPESCRIPT_TSC_FLAGS`              | Run `tsc --noEmit` for the full project.                                 |
| `typecheck:watch`    | Optional `TYPESCRIPT_TSCONFIG`, `TYPESCRIPT_TSC_FLAGS`              | Run `tsc --noEmit --watch`.                                              |
| `typecheck:files`    | Required `FILES`; optional `TYPESCRIPT_TSC_FLAGS`        | Type-check explicit files without loading `tsconfig.json`.               |
| `build`              | Optional `TYPESCRIPT_TSCONFIG`, `TYPESCRIPT_TSC_FLAGS`              | Compile the project with `tsc --noEmitOnError`.                          |
| `build:watch`        | Optional `TYPESCRIPT_TSCONFIG`, `TYPESCRIPT_TSC_FLAGS`              | Compile in watch mode with `tsc --noEmitOnError --watch`.                |
| `build:clean`        | Optional `TYPESCRIPT_OUT_DIR`, `TYPESCRIPT_TSCONFIG`, `TYPESCRIPT_TSC_FLAGS`   | Remove the output directory and run a fresh compile.                     |
| `emit:dts`           | Optional `TYPESCRIPT_OUT_DIR`, `TYPESCRIPT_TSCONFIG`, `TYPESCRIPT_TSC_FLAGS`   | Emit declaration files only.                                             |
| `config:show`        | Optional `TYPESCRIPT_TSCONFIG`                           | Print the fully resolved TypeScript config.                              |
| `config:init`        | —                                             | Generate a starter `tsconfig.json` with `tsc --init`.                    |
| `config:files`       | Optional `TYPESCRIPT_TSCONFIG`                           | List every file included in the compilation.                             |
| `config:diagnostics` | Optional `TYPESCRIPT_TSCONFIG`                           | Print compiler performance diagnostics.                                  |
| `config:trace`       | Optional `TYPESCRIPT_TRACE_DIR`, `TYPESCRIPT_TSCONFIG`, `TYPESCRIPT_TSC_FLAGS` | Emit a TypeScript performance trace.                                     |
| `start`              | Optional `TYPESCRIPT_OUTFILE`                            | Run compiled JavaScript with Node.js.                                    |
| `ci`                 | Optional `TYPESCRIPT_TSCONFIG`, `TYPESCRIPT_TSC_FLAGS`              | Run the same strict no-emit type-check used by CI.                       |
| `clean`              | Optional `TYPESCRIPT_OUT_DIR`                            | Remove the compiled output directory.                                    |
| `clean:all`          | Optional `TYPESCRIPT_OUT_DIR`                            | Remove output, incremental build cache, and trace output.                |

## Examples

```bash
task typescript:node:fnm:npm:install
task typescript:node:fnm:npm:install TYPESCRIPT_VERSION=5.6.3
task typescript:node:fnm:npm:version
task typescript:node:fnm:npm:config:init

task typescript:node:fnm:npm:run TYPESCRIPT_FILE=scripts/seed.ts
task typescript:node:fnm:npm:dev TYPESCRIPT_FILE=src/server.ts TYPESCRIPT_TSX_FLAGS="--env-file .env"

task typescript:node:fnm:npm:typecheck
task typescript:node:fnm:npm:typecheck TYPESCRIPT_TSCONFIG=tsconfig.strict.json
task typescript:node:fnm:npm:typecheck:files FILES="src/index.ts src/api.ts" TYPESCRIPT_TSC_FLAGS="--strict"

task typescript:node:fnm:npm:build
task typescript:node:fnm:npm:build:clean TYPESCRIPT_OUT_DIR=build
task typescript:node:fnm:npm:emit:dts TYPESCRIPT_OUT_DIR=types

task typescript:node:fnm:npm:config:show
task typescript:node:fnm:npm:config:files
task typescript:node:fnm:npm:config:diagnostics
task typescript:node:fnm:npm:config:trace TYPESCRIPT_TRACE_DIR=.traces/tsc

task typescript:node:fnm:npm:start TYPESCRIPT_OUTFILE=dist/server.js
task typescript:node:fnm:npm:clean --yes
task typescript:node:fnm:npm:clean:all --yes
```

## Notes

`tsx` is fast because it strips types and executes through esbuild; it does not
catch type errors. Use `typecheck`, `build`, or `ci` before committing.

`typecheck:files` intentionally bypasses `tsconfig.json`, because TypeScript
ignores project configuration when explicit files are passed on the command
line. Use it only for controlled quick checks.
