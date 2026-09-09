package fileout

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeFileInfo struct {
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return "document.pdf" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

type fakeOutputFile struct {
	name       string
	writeErr   error
	chmodErr   error
	closeErr   error
	mode       os.FileMode
	data       []byte
	closeCalls int
}

func (f *fakeOutputFile) Write(data []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	f.data = append(f.data, data...)
	return len(data), nil
}

func (f *fakeOutputFile) Chmod(mode os.FileMode) error {
	f.mode = mode
	return f.chmodErr
}

func (f *fakeOutputFile) Name() string { return f.name }

func (f *fakeOutputFile) Close() error {
	f.closeCalls++
	return f.closeErr
}

func TestWriteAtomically_ReplacesDestinationAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "document.pdf")
	if err := os.WriteFile(outputPath, []byte("old"), 0640); err != nil {
		t.Fatalf("write destination: %v", err)
	}

	if err := WriteAtomically(outputPath, []byte("new")); err != nil {
		t.Fatalf("WriteAtomically returned error: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("destination = %q, want new", data)
	}
	if info, err := os.Stat(outputPath); err != nil {
		t.Fatalf("stat destination: %v", err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
		got := info.Mode().Perm()
		t.Fatalf("destination mode = %o, want 640", got)
	}
	assertNoTemps(t, dir)
}

func TestWriteAtomically_NewDestinationIsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	outputPath := filepath.Join(t.TempDir(), "document.pdf")
	if err := WriteAtomically(outputPath, []byte("new")); err != nil {
		t.Fatalf("WriteAtomically returned error: %v", err)
	}
	if info, err := os.Stat(outputPath); err != nil {
		t.Fatalf("stat destination: %v", err)
	} else if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("destination mode = %o, want 600", got)
	}
}

func TestWriteAtomically_ReportsOperationFailuresAndCleansUp(t *testing.T) {
	createErr := errors.New("create failed")
	statErr := errors.New("stat failed")
	chmodErr := errors.New("chmod failed")
	writeErr := errors.New("write failed")
	closeErr := errors.New("close failed")
	renameErr := errors.New("rename failed")

	testCases := []struct {
		name      string
		createErr error
		statErr   error
		chmodErr  error
		writeErr  error
		closeErr  error
		renameErr error
		wantCause error
		wantText  string
		wantClose int
		wantTemp  bool
	}{
		{name: "create", createErr: createErr, wantCause: createErr, wantText: "failed to create temporary output", wantClose: 0, wantTemp: false},
		{name: "stat", statErr: statErr, wantCause: statErr, wantText: "failed to inspect output file", wantClose: 1, wantTemp: true},
		{name: "chmod", chmodErr: chmodErr, wantCause: chmodErr, wantText: "failed to preserve output permissions", wantClose: 1, wantTemp: true},
		{name: "write", writeErr: writeErr, wantCause: writeErr, wantText: "failed to write temporary PDF", wantClose: 1, wantTemp: true},
		{name: "close", closeErr: closeErr, wantCause: closeErr, wantText: "failed to close temporary PDF", wantClose: 1, wantTemp: true},
		{name: "rename", renameErr: renameErr, wantCause: renameErr, wantText: "failed to replace output file", wantClose: 1, wantTemp: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), "document.pdf")
			temp := &fakeOutputFile{
				name:     filepath.Join(filepath.Dir(outputPath), ".document.pdf.tmp-test"),
				chmodErr: testCase.chmodErr,
				writeErr: testCase.writeErr,
				closeErr: testCase.closeErr,
			}
			removed := false
			renameCalled := false
			err := writeAtomically(outputPath, []byte("new output"), operations{
				createTemp: func(dir, pattern string) (outputFile, error) {
					if dir != filepath.Dir(outputPath) || pattern != ".document.pdf.tmp-*" {
						t.Fatalf("createTemp(%q, %q) used unexpected location", dir, pattern)
					}
					return temp, testCase.createErr
				},
				stat: func(string) (os.FileInfo, error) {
					if testCase.statErr != nil {
						return nil, testCase.statErr
					}
					return fakeFileInfo{mode: 0640}, nil
				},
				rename: func(source, destination string) error {
					renameCalled = true
					if source != temp.name || destination != outputPath {
						t.Fatalf("rename(%q, %q) used unexpected paths", source, destination)
					}
					return testCase.renameErr
				},
				remove: func(path string) error {
					if path != temp.name {
						t.Fatalf("remove(%q), want %q", path, temp.name)
					}
					removed = true
					return nil
				},
			})
			if err == nil || !strings.Contains(err.Error(), testCase.wantText) {
				t.Fatalf("error = %v, want containing %q", err, testCase.wantText)
			}
			if !errors.Is(err, testCase.wantCause) {
				t.Fatalf("error = %v, want wrapped cause %v", err, testCase.wantCause)
			}
			if temp.closeCalls != testCase.wantClose {
				t.Fatalf("close calls = %d, want %d", temp.closeCalls, testCase.wantClose)
			}
			if removed != testCase.wantTemp {
				t.Fatalf("removed = %t, want %t", removed, testCase.wantTemp)
			}
			if renameCalled != (testCase.name == "rename") {
				t.Fatalf("rename called = %t for %s failure", renameCalled, testCase.name)
			}
		})
	}
}

func TestWriteAtomically_MissingDestinationSkipsChmod(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "document.pdf")
	temp := &fakeOutputFile{name: filepath.Join(filepath.Dir(outputPath), ".document.pdf.tmp-test")}

	err := writeAtomically(outputPath, []byte("new output"), operations{
		createTemp: func(string, string) (outputFile, error) { return temp, nil },
		stat:       func(string) (os.FileInfo, error) { return nil, fs.ErrNotExist },
		rename:     func(string, string) error { return nil },
		remove:     func(string) error { return nil },
	})
	if err != nil {
		t.Fatalf("writeAtomically returned error: %v", err)
	}
	if temp.mode != 0 {
		t.Fatalf("chmod mode = %o, want no chmod", temp.mode)
	}
	if string(temp.data) != "new output" {
		t.Fatalf("temporary data = %q, want new output", temp.data)
	}
}

func assertNoTemps(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".document.pdf.tmp-*"))
	if err != nil {
		t.Fatalf("glob temporary outputs: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary outputs were not cleaned up: %v", matches)
	}
}
