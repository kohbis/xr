package repo

import "testing"

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
