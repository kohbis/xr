package parallel

import (
	"bytes"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

func TestJobs(t *testing.T) {
	tests := []struct{ jobs, n, want int }{
		{jobs: 0, n: 5, want: 1},
		{jobs: -2, n: 5, want: 1},
		{jobs: 1, n: 5, want: 1},
		{jobs: 3, n: 10, want: 3},
		{jobs: 8, n: 3, want: 3},
		{jobs: 4, n: 0, want: 0},
	}
	for _, tt := range tests {
		if got := Jobs(tt.jobs, tt.n); got != tt.want {
			t.Errorf("Jobs(%d, %d) = %d, want %d", tt.jobs, tt.n, got, tt.want)
		}
	}
}

func TestRun_NoItems(t *testing.T) {
	called := false
	Run(0, 4, io.Discard, io.Discard, func(int, io.Writer, io.Writer) { called = true })
	if called {
		t.Fatal("fn was called with no items")
	}
}

// TestRun_OutputOrderedRegardlessOfCompletion is the guarantee the package
// exists for: items that finish out of order still print in index order.
func TestRun_OutputOrderedRegardlessOfCompletion(t *testing.T) {
	const n = 8

	for _, jobs := range []int{1, 3, n} {
		t.Run(fmt.Sprintf("jobs=%d", jobs), func(t *testing.T) {
			var out, errOut bytes.Buffer
			Run(n, jobs, &out, &errOut, func(i int, o, e io.Writer) {
				// Later items finish first, so completion order is reversed.
				time.Sleep(time.Duration(n-i) * 2 * time.Millisecond)
				_, _ = fmt.Fprintf(o, "out %d\n", i)
				_, _ = fmt.Fprintf(e, "err %d\n", i)
			})

			var wantOut, wantErr bytes.Buffer
			for i := range n {
				_, _ = fmt.Fprintf(&wantOut, "out %d\n", i)
				_, _ = fmt.Fprintf(&wantErr, "err %d\n", i)
			}
			if out.String() != wantOut.String() {
				t.Errorf("stdout = %q, want %q", out.String(), wantOut.String())
			}
			if errOut.String() != wantErr.String() {
				t.Errorf("stderr = %q, want %q", errOut.String(), wantErr.String())
			}
		})
	}
}

// TestResults_OrderedRegardlessOfCompletion is the Run guarantee for work that
// returns values: the slice is in index order even when later items finish
// first.
func TestResults_OrderedRegardlessOfCompletion(t *testing.T) {
	const n = 8

	for _, jobs := range []int{1, 3, n} {
		t.Run(fmt.Sprintf("jobs=%d", jobs), func(t *testing.T) {
			got := Results(n, jobs, func(i int) int {
				time.Sleep(time.Duration(n-i) * 2 * time.Millisecond)
				return i * 10
			})

			if len(got) != n {
				t.Fatalf("len(Results()) = %d, want %d", len(got), n)
			}
			for i, v := range got {
				if v != i*10 {
					t.Errorf("Results()[%d] = %d, want %d", i, v, i*10)
				}
			}
		})
	}
}

func TestResults_EveryItemRunsExactlyOnce(t *testing.T) {
	const n = 50
	var calls atomic.Int64

	got := Results(n, 8, func(i int) int {
		calls.Add(1)
		return i
	})

	if calls.Load() != n {
		t.Errorf("fn called %d times, want %d", calls.Load(), n)
	}
	for i, v := range got {
		if v != i {
			t.Fatalf("Results()[%d] = %d, want %d", i, v, i)
		}
	}
}

func TestResults_NoItems(t *testing.T) {
	called := false
	got := Results(0, 4, func(int) int {
		called = true
		return 0
	})
	if called {
		t.Fatal("fn was called with no items")
	}
	if len(got) != 0 {
		t.Fatalf("len(Results()) = %d, want 0", len(got))
	}
}

func TestRun_StreamsStaySeparate(t *testing.T) {
	var out, errOut bytes.Buffer
	Run(3, 3, &out, &errOut, func(i int, o, e io.Writer) {
		_, _ = fmt.Fprintf(o, "O%d", i)
		_, _ = fmt.Fprintf(e, "E%d", i)
	})
	if out.String() != "O0O1O2" {
		t.Errorf("stdout = %q, want %q", out.String(), "O0O1O2")
	}
	if errOut.String() != "E0E1E2" {
		t.Errorf("stderr = %q, want %q", errOut.String(), "E0E1E2")
	}
}

func TestRun_EveryItemRunsExactlyOnce(t *testing.T) {
	const n = 50
	counts := make([]atomic.Int32, n)

	Run(n, 8, io.Discard, io.Discard, func(i int, _, _ io.Writer) {
		counts[i].Add(1)
	})

	for i := range counts {
		if got := counts[i].Load(); got != 1 {
			t.Fatalf("item %d ran %d times, want 1", i, got)
		}
	}
}

// TestRun_Concurrency checks that workers really do overlap; a sequential
// implementation would never see more than one in flight.
func TestRun_Concurrency(t *testing.T) {
	var inFlight, peak atomic.Int32

	Run(16, 4, io.Discard, io.Discard, func(int, io.Writer, io.Writer) {
		cur := inFlight.Add(1)
		for {
			old := peak.Load()
			if cur <= old || peak.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		inFlight.Add(-1)
	})

	if peak.Load() < 2 {
		t.Fatalf("peak concurrency = %d, want at least 2", peak.Load())
	}
	if peak.Load() > 4 {
		t.Fatalf("peak concurrency = %d, want at most the requested 4", peak.Load())
	}
}
