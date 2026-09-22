package cmd

import (
	"testing"

	"github.com/kohbis/xr/internal/output"
)

// scanRepoResult classifies one repository of a diff scan. The okStatus
// argument exists because "scanned, and it matched" is spelled differently by
// the two commands and both spellings are documented --json output.
func TestScanRepoResult(t *testing.T) {
	tests := []struct {
		name       string
		matches    int
		errMsg     string
		okStatus   string
		wantStatus string
	}{
		{name: "pattern with matches", matches: 3, okStatus: "matched", wantStatus: "matched"},
		{name: "history with matches", matches: 3, okStatus: "ok", wantStatus: "ok"},
		{name: "pattern without matches", okStatus: "matched", wantStatus: "no_matches"},
		{name: "history without matches", okStatus: "ok", wantStatus: "no_matches"},
		// An error wins over the match count, including the count it left behind.
		{name: "error outranks matches", matches: 2, errMsg: "git log: boom", okStatus: "ok", wantStatus: output.StatusFailed},
		{name: "error without matches", errMsg: "scan failed", okStatus: "matched", wantStatus: output.StatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanRepoResult("api", tt.matches, tt.errMsg, tt.okStatus)
			if got.Name != "api" {
				t.Errorf("scanRepoResult() name = %q, want api", got.Name)
			}
			if got.Status != tt.wantStatus {
				t.Errorf("scanRepoResult() status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.Error != tt.errMsg {
				t.Errorf("scanRepoResult() error = %q, want %q", got.Error, tt.errMsg)
			}
			if got.Metrics["matches"] != tt.matches {
				t.Errorf("scanRepoResult() matches = %d, want %d", got.Metrics["matches"], tt.matches)
			}
		})
	}
}
