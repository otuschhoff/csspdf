package csspdf_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExternalConsumerBuildsAndRendersFromCleanDirectory(t *testing.T) {
	fixture := filepath.Join("testdata", "external-consumer")
	binary := filepath.Join(t.TempDir(), "external-consumer")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = fixture
	build.Env = append(os.Environ(), "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("external consumer build failed: %v\n%s", err, output)
	}
	runDirectory := t.TempDir()
	run := exec.Command(binary)
	run.Dir = runDirectory
	if output, err := run.CombinedOutput(); err != nil {
		t.Fatalf("external consumer render failed from clean directory: %v\n%s", err, output)
	}
	pdf, err := os.ReadFile(filepath.Join(runDirectory, "output.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("external consumer output is not a PDF: %q", pdf)
	}
}
