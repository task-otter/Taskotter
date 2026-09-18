// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

const (
	stubModeEnv = "TASKOTTER_GIT_STUB_MODE"
	stubOK      = "ok"
	stubAbbrev  = "abbrev"
	stubNoRefs  = "norefs"
	stubBadRefs = "badrefs"
	stubShowOK  = "showok"
	stubShowBad = "showbad"
	stubScript  = `#!/bin/sh
mode="$TASKOTTER_GIT_STUB_MODE"
args="$*"
case "$mode" in
ok) exit 0 ;;
abbrev)
  case "$args" in
  *symbolic-ref*) exit 1 ;;
  *--abbrev-ref*) echo "origin/main"; exit 0 ;;
  *) exit 1 ;;
  esac ;;
badrefs)
  case "$args" in
  *symbolic-ref*|*--abbrev-ref*) exit 1 ;;
  *rev-parse\ refs/remotes/origin/HEAD*) echo "0123456789abcdef0123456789abcdef01234567"; exit 0 ;;
  *for-each-ref*) exit 1 ;;
  *) exit 1 ;;
  esac ;;
norefs)
  case "$args" in
  *symbolic-ref*|*--abbrev-ref*) exit 1 ;;
  *rev-parse\ refs/remotes/origin/HEAD*) echo "0123456789abcdef0123456789abcdef01234567"; exit 0 ;;
  *for-each-ref*) echo "origin/HEAD"; exit 0 ;;
  *) exit 1 ;;
  esac ;;
showok)
  case "$args" in
  *"remote show origin"*) echo "* remote origin"; echo "  HEAD branch: main"; exit 0 ;;
  *) exit 1 ;;
  esac ;;
showbad)
  case "$args" in
  *"remote show origin"*) echo "* remote origin"; exit 0 ;;
  *) exit 1 ;;
  esac ;;
*) exit 1 ;;
esac
`
)
