# gh — GitHub CLI Taskfile

## What is this Taskfile?

A production-ready, cross-platform Taskfile for installing, verifying, upgrading, uninstalling,
configuring, and operating the [GitHub CLI (`gh`)](https://cli.github.com) on macOS, Windows, and Linux.

Include it in your root `Taskfile.yml` or use it standalone.

## Usage

### Standalone

```sh
task -t taskfiles/gh/Taskfile.yml install
task -t taskfiles/gh/Taskfile.yml auth:login
task -t taskfiles/gh/Taskfile.yml pr:list
```

### Included (recommended)

```yaml
# Taskfile.yml
includes:
  tools: ./taskfiles/gh/Taskfile.yml
```

Then run:

```sh
task tools:install
task tools:auth:login
task tools:pr:list
```

## Public Tasks

| Task                     | Description                                  | Key variables                                    |
| ------------------------ | -------------------------------------------- | ------------------------------------------------ |
| `install`                | Auto-detect OS and install gh                | `VERSION`                                        |
| `install:undo`           | Auto-detect OS and remove gh                 | —                                                |
| `upgrade`                | Auto-detect OS and upgrade gh                | —                                                |
| `version`                | Show the installed gh version                | —                                                |
| `doctor`                 | Run gh self-diagnostic check                 | —                                                |
| `which`                  | Show path to the gh binary                   | —                                                |
| `verify`                 | Verify gh installation and auth status       | —                                                |
| `help`                   | Show gh built-in help                        | —                                                |
| `auth:login`             | Authenticate with GitHub interactively       | —                                                |
| `auth:login:web`         | Authenticate using the web browser           | —                                                |
| `auth:login:ssh`         | Authenticate using SSH protocol              | —                                                |
| `auth:status`            | Show current authentication status           | —                                                |
| `auth:refresh`           | Refresh the authentication token             | —                                                |
| `auth:logout`            | Log out and remove stored credentials        | —                                                |
| `auth:setup-git`         | Configure git to use gh as credential helper | —                                                |
| `repo:view`              | View a repository                            | `OWNER`, `REPO`                                  |
| `repo:create`            | Create a new repository                      | `REPO`, `VISIBILITY`, `DESCRIPTION`              |
| `repo:clone`             | Clone a repository                           | `OWNER`, `REPO`, `CLONE_DIR`                     |
| `repo:fork`              | Fork a repository                            | `OWNER`, `REPO`                                  |
| `repo:list`              | List repositories                            | `OWNER`                                          |
| `repo:sync`              | Sync the current branch with upstream        | `BRANCH`                                         |
| `repo:archive`           | Archive a repository                         | `OWNER`, `REPO`                                  |
| `repo:delete:danger`     | Permanently delete a repository              | `OWNER`, `REPO`                                  |
| `pr:list`                | List open pull requests                      | —                                                |
| `pr:status`              | Show PR status for current branch            | —                                                |
| `pr:view`                | View a pull request                          | `PR_NUMBER`                                      |
| `pr:create`              | Create a pull request                        | `PR_TITLE`, `PR_BODY`, `BASE`, `HEAD`            |
| `pr:checkout`            | Check out a PR branch                        | `PR_NUMBER`                                      |
| `pr:diff`                | Show diff for a pull request                 | `PR_NUMBER`                                      |
| `pr:review`              | Add a review to a pull request               | `PR_NUMBER`                                      |
| `pr:merge`               | Merge a pull request                         | `PR_NUMBER`, `MERGE_METHOD`                      |
| `pr:close`               | Close a pull request                         | `PR_NUMBER`                                      |
| `pr:ready`               | Mark a draft PR as ready for review          | `PR_NUMBER`                                      |
| `pr:comment`             | Add a comment to a pull request              | `PR_NUMBER`, `PR_BODY`                           |
| `issue:list`             | List open issues                             | —                                                |
| `issue:view`             | View an issue                                | `ISSUE_NUMBER`                                   |
| `issue:create`           | Create an issue                              | `ISSUE_TITLE`, `ISSUE_BODY`                      |
| `issue:comment`          | Comment on an issue                          | `ISSUE_NUMBER`, `ISSUE_BODY`                     |
| `issue:close`            | Close an issue                               | `ISSUE_NUMBER`                                   |
| `issue:reopen`           | Reopen a closed issue                        | `ISSUE_NUMBER`                                   |
| `issue:assign`           | Assign a user to an issue                    | `ISSUE_NUMBER`, `ASSIGNEE`                       |
| `issue:label`            | Add a label to an issue                      | `ISSUE_NUMBER`, `LABEL`                          |
| `workflow:list`          | List GitHub Actions workflows                | —                                                |
| `workflow:view`          | View a workflow                              | `WORKFLOW`                                       |
| `workflow:run`           | Trigger a workflow dispatch                  | `WORKFLOW`, `BRANCH`                             |
| `workflow:watch`         | Watch a workflow run in real time            | `WORKFLOW`                                       |
| `run:list`               | List recent workflow runs                    | —                                                |
| `run:view`               | View a workflow run                          | `RUN_ID`                                         |
| `run:logs`               | View logs for a workflow run                 | `RUN_ID`                                         |
| `run:rerun`              | Re-run a workflow run                        | `RUN_ID`                                         |
| `run:cancel`             | Cancel a running workflow run                | `RUN_ID`                                         |
| `release:list`           | List releases                                | `OWNER`, `REPO`                                  |
| `release:view`           | View a release                               | `TAG`, `OWNER`, `REPO`                           |
| `release:create`         | Create a release                             | `TAG`, `TITLE`, `NOTES`, `OWNER`, `REPO`         |
| `release:upload`         | Upload an asset to a release                 | `TAG`, `ASSET`, `OWNER`, `REPO`                  |
| `release:download`       | Download release assets                      | `TAG`, `OWNER`, `REPO`, `DOWNLOAD_DIR`           |
| `release:download:all`   | Download assets from all releases            | `OWNER`, `REPO`, `DOWNLOAD_DIR`                  |
| `release:delete:danger`  | Permanently delete a release                 | `TAG`, `OWNER`, `REPO`                           |
| `secret:list`            | List Actions secrets                         | `ENVIRONMENT`                                    |
| `secret:set`             | Create or update a secret                    | `SECRET_NAME`, `SECRET_VALUE`, `ENVIRONMENT`     |
| `secret:delete:danger`   | Permanently delete a secret                  | `SECRET_NAME`, `ENVIRONMENT`                     |
| `variable:list`          | List Actions variables                       | `ENVIRONMENT`                                    |
| `variable:set`           | Create or update a variable                  | `VARIABLE_NAME`, `VARIABLE_VALUE`, `ENVIRONMENT` |
| `variable:delete:danger` | Permanently delete a variable                | `VARIABLE_NAME`, `ENVIRONMENT`                   |
| `gist:list`              | List gists                                   | —                                                |
| `gist:view`              | View a gist                                  | `GIST_ID`                                        |
| `gist:create`            | Create a gist from a file                    | `FILE`, `DESCRIPTION`                            |
| `gist:delete:danger`     | Permanently delete a gist                    | `GIST_ID`                                        |
| `api:get`                | Make a GET request to the GitHub API         | `ENDPOINT`                                       |
| `api:post`               | Make a POST request to the GitHub API        | `ENDPOINT`, `DATA`                               |
| `api:patch`              | Make a PATCH request to the GitHub API       | `ENDPOINT`, `DATA`                               |
| `api:delete:danger`      | Make a DELETE request to the GitHub API      | `ENDPOINT`                                       |
| `extension:list`         | List installed gh extensions                 | —                                                |
| `extension:install`      | Install a gh extension                       | `EXTENSION`                                      |
| `extension:upgrade`      | Upgrade a gh extension                       | `EXTENSION`                                      |
| `extension:remove`       | Remove a gh extension                        | `EXTENSION`                                      |
| `config:list`            | List all gh config values                    | —                                                |
| `config:get`             | Get a gh config value                        | `CONFIG_KEY`                                     |
| `config:set`             | Set a gh config value                        | `CONFIG_KEY`, `CONFIG_VALUE`                     |
| `ssh-key:list`           | List SSH keys on the GitHub account          | —                                                |
| `ssh-key:add`            | Add an SSH key to the GitHub account         | `SSH_KEY_FILE`, `SSH_KEY_TITLE`                  |
| `ssh-key:delete:danger`  | Permanently delete an SSH key                | `SSH_KEY_ID`                                     |
| `alias:list`             | List gh command aliases                      | —                                                |
| `alias:set`              | Create or update a gh alias                  | `ALIAS_NAME`, `ALIAS_COMMAND`                    |
| `alias:delete`           | Delete a gh alias                            | `ALIAS_NAME`                                     |
| `project:list`           | List GitHub Projects                         | `OWNER`                                          |
| `project:view`           | View a GitHub Project                        | `PROJECT_NUMBER`, `OWNER`                        |
| `project:create`         | Create a GitHub Project                      | `PROJECT_TITLE`, `OWNER`                         |
| `open`                   | Open the current repository in the browser   | —                                                |
| `browse`                 | Browse the repository on GitHub              | —                                                |
| `search:repos`           | Search GitHub repositories                   | `QUERY`                                          |
| `search:issues`          | Search GitHub issues                         | `QUERY`                                          |
| `search:prs`             | Search GitHub pull requests                  | `QUERY`                                          |

## Variables

| Variable         | Default   | Description                                                       |
| ---------------- | --------- | ----------------------------------------------------------------- |
| `PAT_TOKEN`      | _(empty)_ | Personal Access Token — exported as `GH_TOKEN` for all operations |
| `VISIBILITY`     | `public`  | Repository visibility: `public`, `private`, `internal`            |
| `BASE`           | `main`    | Base branch for pull requests                                     |
| `MERGE_METHOD`   | `merge`   | PR merge strategy: `merge`, `squash`, `rebase`                    |
| `CLONE_DIR`      | `.`       | Local directory for `repo:clone`                                  |
| `DOWNLOAD_DIR`   | `.`       | Local directory for `release:download` and `release:download:all` |
| `DATA`           | `{}`      | JSON body for API POST/PATCH requests                             |
| `VERSION`        | _(empty)_ | Pin a specific gh release for `install`; empty installs latest. Exact availability depends on the detected package manager/repository. |
| `OWNER`          | _(empty)_ | GitHub user or organisation name                                  |
| `REPO`           | _(empty)_ | Repository name                                                   |
| `DESCRIPTION`    | _(empty)_ | Repository description                                            |
| `PR_NUMBER`      | _(empty)_ | Pull request number                                               |
| `PR_TITLE`       | _(empty)_ | Pull request title                                                |
| `PR_BODY`        | _(empty)_ | Pull request body / comment text                                  |
| `HEAD`           | _(empty)_ | Head branch for pull requests                                     |
| `ISSUE_NUMBER`   | _(empty)_ | Issue number                                                      |
| `ISSUE_TITLE`    | _(empty)_ | Issue title                                                       |
| `ISSUE_BODY`     | _(empty)_ | Issue body                                                        |
| `ASSIGNEE`       | _(empty)_ | GitHub username to assign                                         |
| `LABEL`          | _(empty)_ | Label name to apply                                               |
| `WORKFLOW`       | _(empty)_ | Workflow file name or ID                                          |
| `RUN_ID`         | _(empty)_ | Workflow run ID                                                   |
| `BRANCH`         | _(empty)_ | Branch name                                                       |
| `TAG`            | _(empty)_ | Release tag (e.g. `v1.0.0`)                                       |
| `TITLE`          | _(empty)_ | Release title                                                     |
| `NOTES`          | _(empty)_ | Release notes                                                     |
| `ASSET`          | _(empty)_ | Path to release asset file                                        |
| `SECRET_NAME`    | _(empty)_ | Actions secret name                                               |
| `SECRET_VALUE`   | _(empty)_ | Actions secret value (piped via stdin)                            |
| `VARIABLE_NAME`  | _(empty)_ | Actions variable name                                             |
| `VARIABLE_VALUE` | _(empty)_ | Actions variable value                                            |
| `ENVIRONMENT`    | _(empty)_ | GitHub environment name                                           |
| `GIST_ID`        | _(empty)_ | Gist ID                                                           |
| `FILE`           | _(empty)_ | File path for gist creation                                       |
| `ENDPOINT`       | _(empty)_ | GitHub REST API endpoint (e.g. `/repos/owner/repo`)               |
| `EXTENSION`      | _(empty)_ | gh extension identifier (e.g. `github/gh-copilot`)                |
| `CONFIG_KEY`     | _(empty)_ | gh configuration key (e.g. `editor`)                              |
| `CONFIG_VALUE`   | _(empty)_ | gh configuration value                                            |
| `SSH_KEY_FILE`   | _(empty)_ | Path to SSH public key file                                       |
| `SSH_KEY_ID`     | _(empty)_ | SSH key ID for deletion                                           |
| `SSH_KEY_TITLE`  | _(empty)_ | Display title for SSH key                                         |
| `ALIAS_NAME`     | _(empty)_ | gh alias name                                                     |
| `ALIAS_COMMAND`  | _(empty)_ | gh command the alias expands to                                   |
| `PROJECT_NUMBER` | _(empty)_ | GitHub Project number                                             |
| `PROJECT_TITLE`  | _(empty)_ | Title for a new GitHub Project                                    |
| `QUERY`          | _(empty)_ | Search query string                                               |

## Examples

```sh
# Install gh automatically on the current OS
task -t taskfiles/gh/Taskfile.yml install

# Install using a specific package manager

# Authenticate
task -t taskfiles/gh/Taskfile.yml auth:login
task -t taskfiles/gh/Taskfile.yml auth:login:web
task -t taskfiles/gh/Taskfile.yml auth:status

# Verify everything is working
task -t taskfiles/gh/Taskfile.yml verify

# Work with repositories
task -t taskfiles/gh/Taskfile.yml repo:view OWNER=github REPO=cli
task -t taskfiles/gh/Taskfile.yml repo:create REPO=my-project VISIBILITY=private DESCRIPTION="My project"
task -t taskfiles/gh/Taskfile.yml repo:clone OWNER=github REPO=cli CLONE_DIR=~/src/gh

# Pull requests
task -t taskfiles/gh/Taskfile.yml pr:list
task -t taskfiles/gh/Taskfile.yml pr:create PR_TITLE="Add feature" BASE=main
task -t taskfiles/gh/Taskfile.yml pr:merge PR_NUMBER=42 MERGE_METHOD=squash

# Issues
task -t taskfiles/gh/Taskfile.yml issue:list
task -t taskfiles/gh/Taskfile.yml issue:create ISSUE_TITLE="Bug report" ISSUE_BODY="Steps to reproduce..."
task -t taskfiles/gh/Taskfile.yml issue:close ISSUE_NUMBER=10

# GitHub Actions
task -t taskfiles/gh/Taskfile.yml workflow:run WORKFLOW=ci.yml BRANCH=main
task -t taskfiles/gh/Taskfile.yml run:logs RUN_ID=1234567890

# Releases
task -t taskfiles/gh/Taskfile.yml release:create TAG=v1.0.0 TITLE="v1.0.0" NOTES="Initial release"
task -t taskfiles/gh/Taskfile.yml release:upload TAG=v1.0.0 ASSET=./dist/app.tar.gz

# Secrets (value is piped via stdin — never exposed in process list)
task -t taskfiles/gh/Taskfile.yml secret:set SECRET_NAME=MY_TOKEN SECRET_VALUE=abc123
task -t taskfiles/gh/Taskfile.yml secret:list

# GitHub API
task -t taskfiles/gh/Taskfile.yml api:get ENDPOINT=/repos/github/cli/releases/latest
task -t taskfiles/gh/Taskfile.yml api:post ENDPOINT=/repos/myorg/myrepo/issues DATA='{"title":"Bug","body":"Details"}'

# Search
task -t taskfiles/gh/Taskfile.yml search:repos QUERY="cli tool language:go stars:>100"
task -t taskfiles/gh/Taskfile.yml search:issues QUERY="is:open label:bug"

# Upgrade
task -t taskfiles/gh/Taskfile.yml upgrade

# Uninstall (requires confirmation prompt)
```
