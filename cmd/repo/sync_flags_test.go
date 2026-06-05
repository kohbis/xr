package repo

import "testing"

func TestEffectiveSyncNetwork(t *testing.T) {
	tests := []struct {
		name       string
		update     bool
		submod     bool
		wantFetch  bool
		wantPull   bool
		wantSubmod bool
	}{
		{name: "checkout only", wantFetch: false, wantPull: false, wantSubmod: false},
		{name: "update", update: true, wantFetch: true, wantPull: true, wantSubmod: false},
		{name: "submodules only", submod: true, wantFetch: false, wantPull: false, wantSubmod: true},
		{name: "update and submodules", update: true, submod: true, wantFetch: true, wantPull: true, wantSubmod: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncUpdate = tt.update
			syncSubmod = tt.submod

			gotFetch, gotPull, gotSubmod := effectiveSyncNetwork()
			if gotFetch != tt.wantFetch || gotPull != tt.wantPull || gotSubmod != tt.wantSubmod {
				t.Fatalf("effectiveSyncNetwork() = (%t,%t,%t), want (%t,%t,%t)",
					gotFetch, gotPull, gotSubmod, tt.wantFetch, tt.wantPull, tt.wantSubmod)
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
