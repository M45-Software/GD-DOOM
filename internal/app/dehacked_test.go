package app

import (
	"bytes"
	"gddoom/internal/wad"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLauncherDehackedDiagnosticsAndCapabilityErrors(t *testing.T) {
	data := buildAppTestWAD("PWAD", []appTestLump{{name: "DEHACKED", data: []byte("[CODEPTR]\nFRAME 281 = Fire\n")}})
	file, err := wad.OpenData("states.wad", data)
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if defs, err := loadGameplayDehacked(file, nil, false, &stderr); err == nil || defs != nil || !strings.Contains(err.Error(), "281") {
		t.Fatal("unsupported embedded gameplay patch accepted")
	}
	if defs, err := loadGameplayDehacked(file, nil, true, &stderr); err != nil || defs != nil {
		t.Fatal("nodeh ignored")
	}
}

func TestLauncherDehFlagReportsPatchErrorsBeforeMapLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.wad")
	if err := os.WriteFile(path, buildAppTestWAD("IWAD", nil), 0600); err != nil {
		t.Fatal(err)
	}
	first, second := filepath.Join(dir, "first.deh"), filepath.Join(dir, "second.deh")
	if err := os.WriteFile(first, []byte("Thing 2\nHit points = 123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("Frame 281\nDuration = 2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	code := RunParse([]string{"-config", "", "-render=false", "-wad", path, "-deh", first, "-deh", second}, &out, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "DeHackEd") || !strings.Contains(stderr.String(), "281") {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}
