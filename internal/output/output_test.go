package output

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestPrintDiffLine_AddedLine(t *testing.T) {
	out := captureStdout(t, func() {
		PrintDiffLine("+added line")
	})

	if !strings.Contains(out, colorGreen) {
		t.Error("added line should be colored green")
	}
	if !strings.Contains(out, "+added line") {
		t.Error("added line content missing")
	}
}

func TestPrintDiffLine_RemovedLine(t *testing.T) {
	out := captureStdout(t, func() {
		PrintDiffLine("-removed line")
	})

	if !strings.Contains(out, colorRed) {
		t.Error("removed line should be colored red")
	}
	if !strings.Contains(out, "-removed line") {
		t.Error("removed line content missing")
	}
}

func TestPrintDiffLine_HunkHeader(t *testing.T) {
	out := captureStdout(t, func() {
		PrintDiffLine("@@ -1,3 +1,4 @@")
	})

	if !strings.Contains(out, colorCyan) {
		t.Error("hunk header should be colored cyan")
	}
	if !strings.Contains(out, "@@ -1,3 +1,4 @@") {
		t.Error("hunk header content missing")
	}
}

func TestPrintDiffLine_ContextLine(t *testing.T) {
	out := captureStdout(t, func() {
		PrintDiffLine(" unchanged context")
	})

	if strings.Contains(out, colorGreen) || strings.Contains(out, colorRed) || strings.Contains(out, colorCyan) {
		t.Error("context line should not have diff-specific colors")
	}
	if !strings.Contains(out, " unchanged context") {
		t.Error("context line content missing")
	}
}

func TestPrintRepoHeader_ContainsName(t *testing.T) {
	out := captureStdout(t, func() {
		PrintRepoHeader("my-repo")
	})

	if !strings.Contains(out, "my-repo") {
		t.Error("repo header should contain repo name")
	}
	if !strings.Contains(out, colorBold) {
		t.Error("repo header should be bold")
	}
}

func TestPrintWarning_Format(t *testing.T) {
	out := captureStdout(t, func() {
		PrintWarning("something went wrong")
	})

	if !strings.Contains(out, "warning:") {
		t.Error("warning output should contain 'warning:' prefix")
	}
	if !strings.Contains(out, "something went wrong") {
		t.Error("warning message missing")
	}
	if !strings.Contains(out, colorYellow) {
		t.Error("warning should be yellow")
	}
}

func TestPrintMatchSimple_MatchLine(t *testing.T) {
	out := captureStdout(t, func() {
		PrintMatchSimple("repo-b", "lib.go", 7, "  func helper()  ", false)
	})

	if !strings.Contains(out, "repo-b") {
		t.Error("match should contain repo name")
	}
	if !strings.Contains(out, "func helper()") {
		t.Error("match content should be trimmed")
	}
}
