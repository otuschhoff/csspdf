package docflowpdf

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfinedFileResolverEnforcesRootAndRegularFiles(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	resolver := ConfinedFileResolver{Root: root}
	data, err := resolver.ReadFile(context.Background(), "inside.txt", 16)
	if err != nil || string(data) != "inside" {
		t.Fatalf("read in-root file = %q, %v", data, err)
	}

	for _, name := range []string{outside, "../outside.txt", "escape.txt", "."} {
		if _, err := resolver.ReadFile(context.Background(), name, 16); err == nil {
			t.Fatalf("ReadFile(%q) unexpectedly succeeded", name)
		}
	}
}

func TestConfinedFileResolverEnforcesByteLimitAndCancellation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "large.txt"), []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolver := ConfinedFileResolver{Root: root}
	_, err := resolver.ReadFile(context.Background(), "large.txt", 4)
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Limit != 4 {
		t.Fatalf("expected limit error, got %v", err)
	}
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = resolver.ReadFile(ctx, "large.txt", 16)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if strings.Contains(err.Error(), root) {
		t.Fatalf("cancellation error leaked root: %v", err)
	}
}
