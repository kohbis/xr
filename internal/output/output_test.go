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

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stderr = old

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
	out := captureStderr(t, func() {
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

func TestPrintWarning_KeepsStdoutClean(t *testing.T) {
	out := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			PrintWarning("something went wrong")
		})
	})

	if out != "" {
		t.Errorf("warning must not reach stdout, got %q", out)
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

func TestPrintMatchSimple_ContextLine(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	out := captureStdout(t, func() {
		PrintMatchSimple("repo-b", "lib.go", 6, "  return nil  ", true)
	})

	// A context line is only useful if it says where it is and what it says.
	if !strings.Contains(out, "lib.go") {
		t.Errorf("context line should name the file, got %q", out)
	}
	if !strings.Contains(out, "return nil") {
		t.Errorf("context line should carry its content, got %q", out)
	}
	// '-' instead of ':' is what tells a context line apart from a match.
	if !strings.Contains(out, "lib.go-6-") {
		t.Errorf("context line should be marked with '-', got %q", out)
	}
}
