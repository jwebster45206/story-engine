package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiscoverCaseFiles(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	casesDir := filepath.Join(filepath.Dir(thisFile), "..", "cases")

	files, err := DiscoverCaseFiles(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no test files found")
	}

	entries, err := os.ReadDir(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	var jsonCount int
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			jsonCount++
		}
	}
	if len(files) != jsonCount {
		t.Fatalf("DiscoverCaseFiles returned %d files but cases/ has %d JSON files", len(files), jsonCount)
	}

	for _, f := range files {
		base := filepath.Base(f)
		suite, err := LoadTestSuite(f)
		if err != nil {
			t.Errorf("load %s: %v", base, err)
			continue
		}
		if len(suite.Steps) == 0 {
			t.Errorf("%s has no steps", base)
		}
	}
}
