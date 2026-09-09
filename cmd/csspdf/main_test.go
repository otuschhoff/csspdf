package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunCLIHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if exitCode := runCLI("/tmp/csspdf", []string{"help"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("runCLI exit code = %d, want 0", exitCode)
	}
	for _, expected := range []string{"Usage:", "csspdf <command>", "dom-parse", "gen-example", "pdfdump"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("help does not contain %q: %q", expected, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCLIVersion(t *testing.T) {
	var stdout bytes.Buffer
	if exitCode := runCLI("csspdf", []string{"version"}, &stdout, &bytes.Buffer{}); exitCode != 0 {
		t.Fatalf("runCLI exit code = %d, want 0", exitCode)
	}
	if got, want := stdout.String(), "csspdf version "+version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunCLIRejectsMissingAndUnknownCommands(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing", want: "csspdf <command> [options]"},
		{name: "unknown", args: []string{"unknown"}, want: `unknown command "unknown"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if exitCode := runCLI("csspdf", test.args, &bytes.Buffer{}, &stderr); exitCode != 2 {
				t.Fatalf("runCLI exit code = %d, want 2", exitCode)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr does not contain %q: %q", test.want, stderr.String())
			}
		})
	}
}

func TestRunCLIDispatchesDOMParse(t *testing.T) {
	var stderr bytes.Buffer
	if exitCode := runCLI("csspdf", []string{"dom-parse"}, &bytes.Buffer{}, &stderr); exitCode != 2 {
		t.Fatalf("runCLI exit code = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "Usage: csspdf dom-parse") {
		t.Fatalf("missing subcommand usage: %q", stderr.String())
	}
}

func TestRunCLIDispatchesPDFDump(t *testing.T) {
	var stderr bytes.Buffer
	if exitCode := runCLI("csspdf", []string{"pdfdump"}, &bytes.Buffer{}, &stderr); exitCode != 2 {
		t.Fatalf("runCLI exit code = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "Usage: csspdf pdfdump") {
		t.Fatalf("missing subcommand usage: %q", stderr.String())
	}
}

func TestRunCLICommandHelp(t *testing.T) {
	for _, command := range []string{"dom-parse", "pdfdump"} {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if exitCode := runCLI("csspdf", []string{command, "-h"}, &stdout, &stderr); exitCode != 0 {
				t.Fatalf("runCLI exit code = %d, want 0", exitCode)
			}
			if !strings.Contains(stdout.String(), "Usage: csspdf "+command) {
				t.Fatalf("missing command usage: %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunCLIDispatchesGenExample(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if exitCode := runCLI("csspdf", []string{"gen-example", "version"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("runCLI exit code = %d, want 0", exitCode)
	}
	if got, want := stdout.String(), "csspdf gen-example version "+version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
