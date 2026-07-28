package main

import (
	"context"
	"errors"
	"testing"

	"github.com/task-otter/Taskotter/internal/app"
	"github.com/task-otter/Taskotter/internal/config"
)

func withMainHooks(
	t *testing.T,
	load func() (*config.Config, error),
	run func(context.Context, *config.Config) (*app.Result, error),
	write func(*config.Config, *app.Result) error,
) {
	t.Helper()

	originalLoad := loadConfig
	originalRun := runApp
	originalWrite := writeActionOutputs
	originalExit := exitProcess

	loadConfig = load
	runApp = run
	writeActionOutputs = write
	exitProcess = func(code int) {
		panic(code)
	}

	t.Cleanup(func() {
		loadConfig = originalLoad
		runApp = originalRun
		writeActionOutputs = originalWrite
		exitProcess = originalExit
	})
}

func TestMainExitsWithRunCode(t *testing.T) {
	withMainHooks(
		t,
		func() (*config.Config, error) {
			return &config.Config{}, nil
		},
		func(context.Context, *config.Config) (*app.Result, error) {
			return &app.Result{}, nil
		},
		func(*config.Config, *app.Result) error {
			return nil
		},
	)

	defer func() {
		got, ok := recover().(int)
		if !ok {
			t.Fatalf("main panic = %#v, want exit code", got)
		}

		if got != 0 {
			t.Fatalf("exit code = %d, want 0", got)
		}
	}()

	main()
}

func TestRunExitCodes(t *testing.T) {
	cases := []struct {
		name     string
		cfg      *config.Config
		loadErr  error
		result   *app.Result
		runErr   error
		writeErr error
		want     int
	}{
		{
			name:    "config error",
			loadErr: errors.New("bad config"),
			want:    1,
		},
		{
			name:   "app error",
			cfg:    &config.Config{},
			runErr: errors.New("sync failed"),
			want:   1,
		},
		{
			name:     "output error",
			cfg:      &config.Config{},
			result:   &app.Result{},
			writeErr: errors.New("output failed"),
			want:     1,
		},
		{
			name:   "changed allowed",
			cfg:    &config.Config{},
			result: &app.Result{Changed: true},
			want:   0,
		},
		{
			name:   "changed fail on changes",
			cfg:    &config.Config{FailOnChanges: true},
			result: &app.Result{Changed: true},
			want:   1,
		},
		{
			name:   "unchanged allowed",
			cfg:    &config.Config{},
			result: &app.Result{},
			want:   0,
		},
		{
			name:   "unchanged fail on changes",
			cfg:    &config.Config{FailOnChanges: true},
			result: &app.Result{},
			want:   0,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			withMainHooks(
				t,
				func() (*config.Config, error) {
					return testCase.cfg, testCase.loadErr
				},
				func(context.Context, *config.Config) (*app.Result, error) {
					return testCase.result, testCase.runErr
				},
				func(*config.Config, *app.Result) error {
					return testCase.writeErr
				},
			)

			if got := run(); got != testCase.want {
				t.Fatalf("run() = %d, want %d", got, testCase.want)
			}
		})
	}
}
