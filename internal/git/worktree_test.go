package git

import "testing"

func TestParseWorktreePorcelain(t *testing.T) {
	out := `worktree /ws/repos/api
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main

worktree /ws/worktrees/api/feat-x
HEAD 2222222222222222222222222222222222222222
branch refs/heads/feat-x

worktree /ws/worktrees/api/feature/foo
HEAD 3333333333333333333333333333333333333333
detached
locked

worktree /ws/worktrees/api/stale
HEAD 4444444444444444444444444444444444444444
branch refs/heads/stale
prunable gitdir file points to non-existent location
`

	got := parseWorktreePorcelain(out)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}

	want := []Worktree{
		{Path: "/ws/repos/api", Head: "1111111111111111111111111111111111111111", Branch: "main", Main: true},
		{Path: "/ws/worktrees/api/feat-x", Head: "2222222222222222222222222222222222222222", Branch: "feat-x"},
		{Path: "/ws/worktrees/api/feature/foo", Head: "3333333333333333333333333333333333333333", Detached: true, Locked: true},
		{Path: "/ws/worktrees/api/stale", Head: "4444444444444444444444444444444444444444", Branch: "stale", Prunable: true},
	}

	for i, w := range want {
		if got[i] != w {
			t.Errorf("worktree[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestParseWorktreePorcelain_BareAndNoTrailingBlankLine(t *testing.T) {
	out := `worktree /ws/repos/api.git
bare`

	got := parseWorktreePorcelain(out)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !got[0].Bare || !got[0].Main {
		t.Errorf("got %+v, want bare main worktree", got[0])
	}
}

func TestParseWorktreePorcelain_Empty(t *testing.T) {
	if got := parseWorktreePorcelain(""); len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}
