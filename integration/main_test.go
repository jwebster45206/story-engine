//go:build integration
// +build integration

package integration

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jwebster45206/story-engine/integration/runner"
)

var caseFlag = flag.String("case", "", "Name of test case to run (from integration/cases/)")
var errFlag = flag.String("err", "continue", "Error handling mode: 'continue' (run all steps) or 'exit' (stop on first failure)")
var runsFlag = flag.Int("runs", 1, "Number of times to run each test suite (useful for testing non-deterministic behavior)")
var scenarioFlag = flag.String("scenario", "", "Override scenario for all test cases (e.g., 'pirate.json', 'pirate.vars.json', 'pirate.both.json')")

func TestMain(m *testing.M) {
	// Check required environment variables
	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost:8080" // Default to localhost
	}

	fmt.Printf("Running Story Engine Integration Tests\n")
	fmt.Printf("   API Base URL: %s\n", apiBaseURL)

	// Run the tests
	code := m.Run()
	os.Exit(code)
}

func TestIntegrationSuites(t *testing.T) {
	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost:8080" // Default to localhost
	}
	timeoutSeconds := getIntEnv("TEST_TIMEOUT_SECONDS", 30)

	// Create runner (no concurrency)
	testRunner := runner.NewRunner(apiBaseURL)
	testRunner.Timeout = time.Duration(timeoutSeconds) * time.Second
	testRunner.ErrorHandlingMode = "continue" // Use continue mode for bulk tests to see all results
	testRunner.ScenarioOverride = *scenarioFlag
	testRunner.Logger = func(format string, args ...interface{}) {
		fmt.Printf(format+"\n", args...)
	}

	if testRunner.ScenarioOverride != "" {
		t.Logf("Scenario override enabled: %s", testRunner.ScenarioOverride)
	}

	// Discover test case files
	testFiles, err := discoverTestFiles("cases")
	if err != nil {
		t.Fatalf("Failed to discover test files: %v", err)
	}

	if len(testFiles) == 0 {
		t.Fatal("No test files found in cases directory")
	}

	// Load test suites (with expansion for sequences)
	var jobs []runner.TestJob
	for _, file := range testFiles {
		expandedJobs, err := runner.LoadTestSuiteWithExpansion(file, "cases")
		if err != nil {
			t.Errorf("Failed to load test suite %s: %v", file, err)
			continue
		}

		jobs = append(jobs, expandedJobs...)
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

	var failed []string
	var passed []string

	for i, job := range jobs {
		t.Logf("[%d/%d] Starting test suite: %s (%d steps)", i+1, len(jobs), job.Name, len(job.Suite.Steps))

		result, err := testRunner.RunSuite(ctx, job.Suite)
		if err != nil && result.Error == nil {
			result.Error = err
		}
		result.Job = job

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
				if stepResult.IsReset {
					// Reset steps don't count toward pass/fail metrics
					t.Logf("   ↻ %s (%v)", stepResult.StepName, stepResult.Duration)
				} else if stepResult.IsStoryEventWait {
					// Story event steps
					t.Logf("   ✓ %s (%v)", stepResult.StepName, stepResult.Duration)
				} else if stepResult.Success {
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
// Supports multiple cases comma-separated: -case "case1,case2,case3"
func TestSingleSuite(t *testing.T) {
	// Parse command line flags
	flag.Parse()

	// Skip if not explicitly requested
	if *caseFlag == "" {
		t.Skip("Skipping single suite test (use -case flag to run)")
	}

	// Parse comma-separated case names
	caseNames := strings.Split(*caseFlag, ",")
	var suiteFiles []string
	for _, caseName := range caseNames {
		caseName = strings.TrimSpace(caseName)
		if caseName == "" {
			continue
		}

		// Build the full path to the test case
		suiteFile := "cases/" + caseName
		if !strings.HasSuffix(suiteFile, ".json") {
			suiteFile += ".json"
		}
		suiteFiles = append(suiteFiles, suiteFile)
	}

	if len(suiteFiles) == 0 {
		t.Fatalf("No valid test cases found in -case flag: %s", *caseFlag)
	}

	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost:8080" // Default to localhost
	}
	timeoutSeconds := getIntEnv("TEST_TIMEOUT_SECONDS", 30)

	// Validate error handling mode
	if *errFlag != "exit" && *errFlag != "continue" {
		t.Fatalf("Invalid -err flag value: %s (must be 'exit' or 'continue')", *errFlag)
	}

	runs := *runsFlag
	if runs < 1 {
		t.Fatalf("Number of runs must be >= 1, got: %d", runs)
	}

	testRunner := runner.NewRunner(apiBaseURL)
	testRunner.Timeout = time.Duration(timeoutSeconds) * time.Second
	// For multi-run, always use continue mode to collect complete data
	// For single run, respect the user's error flag
	if runs > 1 {
		testRunner.ErrorHandlingMode = runner.ErrorHandlingContinue
	} else {
		testRunner.ErrorHandlingMode = runner.ErrorHandlingMode(*errFlag)
	}
	testRunner.ScenarioOverride = *scenarioFlag
	testRunner.Logger = func(format string, args ...interface{}) {
		fmt.Printf(format+"\n", args...)
	}

	errorMode := *errFlag
	if runs > 1 {
		errorMode = "continue (forced for multi-run statistics)"
	}

	scenarioInfo := ""
	if testRunner.ScenarioOverride != "" {
		scenarioInfo = fmt.Sprintf(" [scenario override: %s]", testRunner.ScenarioOverride)
	}
	t.Logf("Running %d test suite(s) %d time(s) each with error mode '%s'%s: %s", len(suiteFiles), runs, errorMode, scenarioInfo, strings.Join(caseNames, ", "))

	// Track overall statistics
	totalTests := 0
	totalPasses := 0
	totalFailures := 0
	caseStats := make(map[string]struct{ passes, failures int })

	// Track detailed failures per case and step
	var allFailures []failureDetail

	// Run test suites multiple times
	for run := 1; run <= runs; run++ {
		if runs > 1 {
			t.Logf("=== RUN %d/%d ===", run, runs)
		}

		// Run each test case
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		var failed []string
		var passed []string

		for i, suiteFile := range suiteFiles {
			// Load the specific test suite (with expansion for sequences)
			jobs, err := runner.LoadTestSuiteWithExpansion(suiteFile, "cases")
			if err != nil {
				t.Errorf("[%d/%d] Failed to load test suite %s: %v", i+1, len(suiteFiles), suiteFile, err)
				failed = append(failed, fmt.Sprintf("%s: load error", suiteFile))
				continue
			}

			// Run all jobs from this file (could be 1 regular test or N from a sequence)
			for _, job := range jobs {
				t.Logf("[%d/%d] Running test suite: %s", i+1, len(suiteFiles), job.Name)

				// Run the test
				result, err := testRunner.RunSuite(ctx, job.Suite)
				if err != nil && result.Error == nil {
					result.Error = err
				}
				result.Job = job

				// Log detailed results
				t.Logf("GameState ID: %s", result.GameState.String())

				totalTests++
				stats := caseStats[job.Name]

				if result.Error != nil {
					totalFailures++
					stats.failures++
					caseStats[job.Name] = stats

					failed = append(failed, fmt.Sprintf("%s: %v", job.Name, result.Error))
					t.Errorf("[%d/%d] FAILED: Test suite '%s' failed: %v", i+1, len(suiteFiles), job.Name, result.Error)

					if runs > 1 {
						t.Logf("Test suite '%s' failed (run %d/%d): %v", job.Name, run, runs, result.Error)
					} else if *errFlag == "exit" {
						t.Fatalf("Test suite(s) had errors")
					}
				} else {
					totalPasses++
					stats.passes++
					caseStats[job.Name] = stats

					passed = append(passed, job.Name)
					t.Logf("[%d/%d] PASSED: Test suite '%s' completed in %v", i+1, len(suiteFiles), job.Name, result.Duration)
				}

				// Log step details
				for _, stepResult := range result.Results {
					if stepResult.IsReset {
						// Reset steps don't count toward pass/fail metrics
						t.Logf("   ↻ %s (%v)", stepResult.StepName, stepResult.Duration)
					} else if stepResult.IsStoryEventWait {
						// Story event wait steps
						t.Logf("   ✓ %s (%v)", stepResult.StepName, stepResult.Duration)
					} else if stepResult.Success {
						t.Logf("   ✓ %s (%v)", stepResult.StepName, stepResult.Duration)
					} else {
						t.Errorf("   ✗ %s: %v", stepResult.StepName, stepResult.Error)
						// Record failure detail
						allFailures = append(allFailures, failureDetail{
							caseName: result.Job.Name,
							stepName: stepResult.StepName,
							error:    stepResult.Error.Error(),
							run:      run,
						})
					}
				}

				t.Logf("--------------------------------") // Separator between suites
			}
		}

		// Summary for multiple cases within this run
		if len(suiteFiles) > 1 {
			t.Logf("Run %d Summary:", run)
			t.Logf("   Passed: %d", len(passed))
			t.Logf("   Failed: %d", len(failed))

			if len(failed) > 0 {
				t.Logf("Failed suites:")
				for _, failure := range failed {
					t.Logf("   - %s", failure)
				}
			}
		}

		// For single run with exit mode, fail immediately if any test failed
		// For multi-run, we always continue to gather complete statistics
		if len(failed) > 0 && *errFlag == "exit" && runs == 1 {
			t.Fatalf("Test suite(s) had errors")
		}
	}

	// Report final statistics
	summary := buildFinalReport(runs, len(suiteFiles), totalTests, totalPasses, totalFailures, caseNames, caseStats)
	if summary != "" {
		t.Log(summary)
	}

	// Detailed failure report
	if len(allFailures) > 0 {
		t.Log(buildFailureReport(allFailures, totalTests, totalPasses, totalFailures))
	}

	if totalFailures > 0 {
		t.Fatalf("Test suite(s) had errors")
	}
}

// buildFinalReport creates the final statistics summary
func buildFinalReport(runs int, numSuites int, totalTests int, totalPasses int, totalFailures int, caseNames []string, caseStats map[string]struct{ passes, failures int }) string {
	var sb strings.Builder

	// Multi-run statistics
	if runs > 1 {
		sb.WriteString("\n=== FINAL MULTI-RUN STATISTICS ===\n")
		sb.WriteString(fmt.Sprintf("Total test executions: %d\n", totalTests))
		sb.WriteString(fmt.Sprintf("Total passes: %d (%.1f%%)\n", totalPasses, float64(totalPasses)/float64(totalTests)*100))
		sb.WriteString(fmt.Sprintf("Total failures: %d (%.1f%%)\n", totalFailures, float64(totalFailures)/float64(totalTests)*100))

		sb.WriteString("\nPer-suite statistics:\n")
		for _, caseName := range caseNames {
			stats := caseStats[caseName]
			total := stats.passes + stats.failures
			if total > 0 {
				passRate := float64(stats.passes) / float64(total) * 100
				sb.WriteString(fmt.Sprintf("  %s: %d/%d passes (%.1f%%)\n", caseName, stats.passes, total, passRate))

				// Flag potentially flaky tests
				if stats.passes > 0 && stats.failures > 0 {
					sb.WriteString("    ⚠️  FLAKY: This test both passed and failed across runs\n")
				}
			}
		}
	} else {
		// Single run summary (only if multiple suites)
		if numSuites > 1 {
			sb.WriteString("Test Suite Summary:\n")
			sb.WriteString(fmt.Sprintf("   Passed: %d\n", totalPasses))
			sb.WriteString(fmt.Sprintf("   Failed: %d\n", totalFailures))
		}
	}

	return sb.String()
}

// buildFailureReport creates a detailed failure report from collected failure details
func buildFailureReport(allFailures []failureDetail, totalTests, totalPasses, totalFailures int) string {
	var sb strings.Builder

	sb.WriteString("\n========================================\n")
	sb.WriteString("Detailed Failure Report\n")
	sb.WriteString("========================================\n")

	// Overall pass/fail rate
	if totalTests > 0 {
		passRate := float64(totalPasses) / float64(totalTests) * 100
		failRate := float64(totalFailures) / float64(totalTests) * 100
		sb.WriteString(fmt.Sprintf("\nOverall: %d/%d passed (%.1f%%), %d failed (%.1f%%)\n",
			totalPasses, totalTests, passRate, totalFailures, failRate))
	}

	// Group failures by case name
	failuresByCase := make(map[string][]failureDetail)
	for _, failure := range allFailures {
		failuresByCase[failure.caseName] = append(failuresByCase[failure.caseName], failure)
	}

	// Sort case names for consistent output
	var caseNames []string
	for caseName := range failuresByCase {
		caseNames = append(caseNames, caseName)
	}
	sort.Strings(caseNames)

	// Report failures grouped by case
	for _, caseName := range caseNames {
		failures := failuresByCase[caseName]
		sb.WriteString(fmt.Sprintf("\n%s (%d step failure(s)):\n", caseName, len(failures)))

		// Group by step name to show which steps failed across runs
		stepFailures := make(map[string][]failureDetail)
		for _, failure := range failures {
			stepFailures[failure.stepName] = append(stepFailures[failure.stepName], failure)
		}

		// Sort step names for consistent output
		var stepNames []string
		for stepName := range stepFailures {
			stepNames = append(stepNames, stepName)
		}
		sort.Strings(stepNames)

		// Report each step's failures
		for _, stepName := range stepNames {
			stepFails := stepFailures[stepName]
			if len(stepFails) == 1 {
				sb.WriteString(fmt.Sprintf("  ✗ %s (run %d):\n", stepName, stepFails[0].run))
				sb.WriteString(fmt.Sprintf("      %s\n", stepFails[0].error))
			} else {
				sb.WriteString(fmt.Sprintf("  ✗ %s (failed %d times):\n", stepName, len(stepFails)))
				for _, fail := range stepFails {
					sb.WriteString(fmt.Sprintf("      Run %d: %s\n", fail.run, fail.error))
				}
			}
		}
	}
	sb.WriteString("\n========================================\n")
	return sb.String()
}

// failureDetail tracks information about a specific step failure
type failureDetail struct {
	caseName string
	stepName string
	error    string
	run      int
}

// Helper functions

func discoverTestFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".json") {
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
