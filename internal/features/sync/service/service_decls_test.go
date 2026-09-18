// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"errors"
	"os"
)

type (
	failingOps struct{}

	dirSnapshot struct {
		root string
	}

	tempEntry struct {
		entry os.DirEntry
		root  string
	}

	fakeDirEntry struct {
		name string
		dir  bool
	}

	stubSnapshot struct {
		root string
	}
)

const (
	lockRelPath = "taskfiles/.taskotter-lock.yml"
	metaRelPath = "taskfiles/.taskotter/metadata.yml"

	srcModuleName = "src"
	emptyTaskYAML = "version: \"3\"\ntasks: {}\n"
	goDestPath    = "taskfiles/go"
	targetWantFmt = "target = %q, want %q"

	wantErrText      = "expected error"
	unexpectFmt      = "unexpected error: %v"
	stagingName      = "staging-root"
	fileNameTxt      = "file.txt"
	payloadText      = "payload"
	badYAMLText      = "{{bad"
	byteX            = "x"
	pathA            = "a"
	pathB            = "b"
	outName          = "o"
	srcDir           = "/src"
	srcFileA         = "/src/a"
	wsRoot           = "/ws"
	wsFileX          = "/ws/x"
	rootDir          = "/root"
	rootMetaPath     = "/root/go/metadata.yml"
	goTaskfileRel    = "taskfiles/go/Taskfile.yml"
	goOldTxtRel      = "taskfiles/go/old.txt"
	oldTargetFolder  = "old-taskfiles"
	oldTargetFileRel = "old-taskfiles/go/a.txt"
	otherMetaRel     = "other/.taskotter/metadata.yml"
	missingTxt       = "missing.txt"
	gitDirName       = ".git"
	errWantFmt       = "err = %v, want %v"
	errSkipDirFmt    = "err = %v, want SkipDir"
	listsFmt         = "lists = %+v"
	relScanFmt       = "rel=%q scan=%v"
	errBareFmt       = "err = %v"
	removeEmptyCtx   = "remove empty"
	eslintNodePNPM   = "eslint/node/pnpm"
	eslintBun        = "eslint/bun"
	taskCI           = "ci"
	pathMissing      = "missing"

	unknownFileChange = 99

	subDirName  = "sub"
	goSubOldRel = "taskfiles/go/sub/old.txt"
	gotFmt      = "got = %v"
	modXName    = "mod-x"
)

var errStub = errors.New("stub failure")
