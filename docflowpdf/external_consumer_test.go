package docflowpdf_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExternalConsumerCompilesPublicConfiguration(t *testing.T) {
	fixture := filepath.Join("testdata", "external-consumer")
	command := exec.Command("go", "test", "./...")
	command.Dir = fixture
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("external consumer compile failed: %v\n%s", err, output)
	}
}
