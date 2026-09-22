package cmd

import (
	"strings"
	"testing"
)

// Every command carrying --jobs must reject a value below 1 rather than let
// parallel.Jobs clamp it into a silent sequential run. exec had no such check
// until it was routed through parallel.ValidateJobs, so this pins the wiring
// rather than the rule, which internal/parallel tests on its own.
func TestJobsFlagRejectedBelowOne(t *testing.T) {
	tests := []struct {
		name string
		set  func(int)
		run  func() error
	}{
		{
			name: "exec",
			set:  func(n int) { execJobs = n },
			run:  func() error { return execCmd.RunE(execCmd, []string{"true"}) },
		},
		{
			name: "search",
			set:  func(n int) { searchJobs = n },
			run:  func() error { return searchCmd.RunE(searchCmd, []string{"pattern"}) },
		},
		{
			name: "diff",
			set:  func(n int) { diffJobs = n },
			run:  func() error { _, _, err := loadDiffWorkspace(); return err },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { tt.set(1) })

			tt.set(0)
			err := tt.run()
			if err == nil || !strings.Contains(err.Error(), "--jobs must be at least 1") {
				t.Fatalf("with --jobs 0: error = %v, want it to mention --jobs must be at least 1", err)
			}

			// A valid value must get past the check; the command then fails for
			// its own reasons (no config here), which is not what is under test.
			tt.set(2)
			if err := tt.run(); err != nil && strings.Contains(err.Error(), "--jobs") {
				t.Errorf("with --jobs 2: error = %v, want the jobs check to pass", err)
			}
		})
	}
}
