package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jwebster45206/story-engine/integration/runner"
)

func TestMain(m *testing.M) {
	// Check required environment variables
	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		fmt.Fprintf(os.Stderr, "API_BASE_URL environment variable is required\n")
		os.Exit(1)
	}

	fmt.Printf("Running Story Engine Integration Tests\n")
	fmt.Printf("   API Base URL: %s\n", apiBaseURL)

	// Run the tests
	code := m.Run()
	os.Exit(code)
}

func TestIntegrationSuites(t *testing.T) {
	apiBaseURL := os.Getenv("API_BASE_URL")
	timeoutSeconds := getIntEnv("TEST_TIMEOUT_SECONDS", 30)

	// Create runner (no concurrency)
	testRunner := runner.NewRunner(apiBaseURL)
	testRunner.Timeout = time.Duration(timeoutSeconds) * time.Second
	testRunner.Logger = func(format string, args ...interface{}) {
		fmt.Printf(format+"\n", args...)
	}

	// Discover test case files
	testFiles, err := discoverTestFiles("cases")
	if err != nil {
		t.Fatalf("Failed to discover test files: %v", err)
	}

	if len(testFiles) == 0 {
		t.Fatal("No test files found in cases directory")
	}

	// Load test suites
	var jobs []runner.TestJob
	for _, file := range testFiles {
		suite, err := runner.LoadTestSuite(file)
		if err != nil {
			t.Errorf("Failed to load test suite %s: %v", file, err)
			continue
		}

		jobs = append(jobs, runner.TestJob{
			Name:     suite.Name,
			Suite:    suite,
			CaseFile: file,
		})
	}

	if len(jobs) == 0 {
		t.Fatal("No valid test suites loaded")
	}

	t.Logf("Loaded %d test suites", len(jobs))
	for _, job := range jobs {
		t.Logf("   - %s (%d steps)", job.Name, len(job.Suite.Steps))
	}

	// Run tests sequentially with real-time progress
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	t.Logf("Running %d test suites sequentially...", len(jobs))

	var results []runner.TestRunResult
	var failed []string
	var passed []string

	for i, job := range jobs {
		t.Logf("[%d/%d] Starting test suite: %s (%d steps)", i+1, len(jobs), job.Name, len(job.Suite.Steps))

		result, err := testRunner.RunSuite(ctx, job.Suite)
		if err != nil && result.Error == nil {
			result.Error = err
		}
		result.Job = job
		results = append(results, result)

		// Process result immediately for real-time feedback
		t.Logf("GameState ID: %s", result.GameState.String())

		if result.Error != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", result.Job.Name, result.Error))
			t.Errorf("[%d/%d] FAILED: Test suite '%s' failed: %v", i+1, len(jobs), result.Job.Name, result.Error)
		} else {
			passed = append(passed, result.Job.Name)
			t.Logf("[%d/%d] PASSED: Test suite '%s' completed in %v", i+1, len(jobs), result.Job.Name, result.Duration)

			// Log step details for passed tests
			for _, stepResult := range result.Results {
				if stepResult.Success {
					t.Logf("   ✓ %s (%v)", stepResult.StepName, stepResult.Duration)
				} else {
					t.Errorf("   ✗ %s: %v", stepResult.StepName, stepResult.Error)
				}
			}
		}
		t.Logf("") // Empty line for readability between suites
	}

	// Summary
	t.Logf("\nIntegration Test Summary:")
	t.Logf("   Passed: %d", len(passed))
	t.Logf("   Failed: %d", len(failed))

	if len(failed) > 0 {
		t.Logf("\nFailed tests:")
		for _, failure := range failed {
			t.Logf("   - %s", failure)
		}
		t.Fatalf("Integration tests failed")
	}

	t.Logf("\nAll integration tests passed!")
}

// TestSingleSuite allows running individual test suites for debugging
func TestSingleSuite(t *testing.T) {
	// Skip if not explicitly requested
	suiteFile := os.Getenv("TEST_SUITE_FILE")
	if suiteFile == "" {
		t.Skip("Skipping single suite test (set TEST_SUITE_FILE to run)")
	}

	apiBaseURL := os.Getenv("API_BASE_URL")
	timeoutSeconds := getIntEnv("TEST_TIMEOUT_SECONDS", 30)

	testRunner := runner.NewRunner(apiBaseURL)
	testRunner.Timeout = time.Duration(timeoutSeconds) * time.Second
	testRunner.Logger = func(format string, args ...interface{}) {
		fmt.Printf(format+"\n", args...)
	}

	// Load the specific test suite
	suite, err := runner.LoadTestSuite(suiteFile)
	if err != nil {
		t.Fatalf("Failed to load test suite %s: %v", suiteFile, err)
	}

	t.Logf("Running single test suite: %s", suite.Name)

	// Run the test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := testRunner.RunSuite(ctx, suite)
	if err != nil {
		t.Fatalf("Test suite failed: %v", err)
	}

	// Log detailed results
	t.Logf("GameState ID: %s", result.GameState.String())
	t.Logf("Test suite '%s' completed in %v", suite.Name, result.Duration)
	for _, stepResult := range result.Results {
		if stepResult.Success {
			t.Logf("   PASS %s (%v)", stepResult.StepName, stepResult.Duration)
		} else {
			t.Errorf("   FAIL %s: %v", stepResult.StepName, stepResult.Error)
		}
	}

	if result.Error != nil {
		t.Fatalf("Test suite had errors: %v", result.Error)
	}
}

// Helper functions

func discoverTestFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

func getIntEnv(name string, defaultValue int) int {
	str := os.Getenv(name)
	if str == "" {
		return defaultValue
	}

	val, err := strconv.Atoi(str)
	if err != nil {
		return defaultValue
	}

	return val
}
