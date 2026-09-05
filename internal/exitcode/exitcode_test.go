package exitcode

import (
	"errors"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
)

func TestFrom(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantOK   bool
	}{
		{name: "nil", err: nil},
		{name: "plain error", err: errors.New("boom")},
		{name: "silent", err: Silent(1), wantCode: 1, wantOK: true},
		{name: "silent other code", err: Silent(3), wantCode: 3, wantOK: true},
		{name: "wrapped", err: fmt.Errorf("syncing: %w", Silent(2)), wantCode: 2, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, ok := From(tt.err)
			if ok != tt.wantOK || code != tt.wantCode {
				t.Fatalf("From() = (%d,%t), want (%d,%t)", code, ok, tt.wantCode, tt.wantOK)
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	if got := Silent(1).Error(); got != "exit status 1" {
		t.Fatalf("Error() = %q, want %q", got, "exit status 1")
	}
}

func TestFailed(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	err := Failed(cmd)
	code, ok := From(err)
	if !ok || code != 1 {
		t.Fatalf("From(Failed()) = (%d,%t), want (1,true)", code, ok)
	}
	if !cmd.SilenceErrors || !cmd.SilenceUsage {
		t.Fatal("Failed() must silence cobra's error and usage output")
	}
}
