package repo

import (
	"testing"

	"github.com/kohbis/xr/internal/exitcode"
	"github.com/kohbis/xr/internal/workspace"
	"github.com/spf13/cobra"
)

func TestEffectiveSyncNetwork(t *testing.T) {
	tests := []struct {
		name      string
		update    bool
		wantFetch bool
		wantPull  bool
	}{
		{name: "checkout only", wantFetch: false, wantPull: false},
		{name: "update", update: true, wantFetch: true, wantPull: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncUpdate = tt.update

			gotFetch, gotPull := effectiveSyncNetwork()
			if gotFetch != tt.wantFetch || gotPull != tt.wantPull {
				t.Fatalf("effectiveSyncNetwork() = (%t,%t), want (%t,%t)",
					gotFetch, gotPull, tt.wantFetch, tt.wantPull)
			}
		})
	}
}

func TestValidateSyncFlags(t *testing.T) {
	syncUpdate = false
	syncPrune = true
	syncCreateBranchIfMissing = false

	if err := validateSyncFlags(false); err == nil {
		t.Fatal("expected error when prune without update")
	}

	syncPrune = false
	syncCreateBranchIfMissing = true
	if err := validateSyncFlags(false); err == nil {
		t.Fatal("expected error when create-branch-if-missing without update")
	}

	syncUpdate = true
	if err := validateSyncFlags(true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncExitCode(t *testing.T) {
	tests := []struct {
		name     string
		result   workspace.SyncResult
		wantCode int
		wantExit bool
	}{
		{name: "all synced", result: workspace.SyncResult{Synced: 2}},
		{name: "skipped only", result: workspace.SyncResult{Skipped: 2}},
		{name: "one failed", result: workspace.SyncResult{Synced: 1, Failed: 1}, wantCode: 1, wantExit: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "sync"}
			err := syncExitCode(cmd, &tt.result)
			if !tt.wantExit {
				if err != nil {
					t.Fatalf("syncExitCode() = %v, want nil", err)
				}
				if cmd.SilenceErrors || cmd.SilenceUsage {
					t.Error("cobra reporting silenced without a failure")
				}
				return
			}
			code, ok := exitcode.From(err)
			if !ok || code != tt.wantCode {
				t.Fatalf("exitcode.From(%v) = (%d,%t), want (%d,true)", err, code, ok, tt.wantCode)
			}
			// The summary already reported the failures, so cobra must stay quiet.
			if !cmd.SilenceErrors || !cmd.SilenceUsage {
				t.Error("cobra would print an error and usage on top of the summary")
			}
		})
	}
}
