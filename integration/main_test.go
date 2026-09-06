//go:build integration

package integration

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/integration/runner"
	"github.com/jwebster45206/story-engine/internal/auth"
)

var errFlag = flag.String("err", "continue", "Error handling mode: 'continue' (run all steps) or 'exit' (stop on first failure)")
var scenarioFlag = flag.String("scenario", "", "Override scenario for all test cases (e.g. pirate.json)")

func TestMain(m *testing.M) {
	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost:8080"
	}

	fmt.Printf("Running Story Engine Integration Tests\n")
	fmt.Printf("   API Base URL: %s\n", apiBaseURL)

	os.Exit(m.Run())
}

func TestIntegration(t *testing.T) {
	if *errFlag != "exit" && *errFlag != "continue" {
		t.Fatalf("Invalid -err flag value: %s (must be 'exit' or 'continue')", *errFlag)
	}

	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost:8080"
	}
	timeoutSeconds := getIntEnv("TEST_TIMEOUT_SECONDS", 30)

	if *scenarioFlag != "" {
		t.Logf("Scenario override enabled: %s", *scenarioFlag)
	}

	testFiles, err := runner.DiscoverCaseFiles("cases")
	if err != nil {
		t.Fatalf("Failed to discover test files: %v", err)
	}
	if len(testFiles) == 0 {
		t.Fatal("No test files found in cases directory")
	}

	t.Logf("Loaded %d test suites", len(testFiles))

	for _, file := range testFiles {
		suite, err := runner.LoadTestSuite(file)
		if err != nil {
			t.Errorf("Failed to load test suite %s: %v", file, err)
			continue
		}
		name := strings.TrimSuffix(filepath.Base(file), ".json")
		t.Run(name, func(t *testing.T) {
			testRunner := runner.NewRunner(apiBaseURL)
			testRunner.Timeout = time.Duration(timeoutSeconds) * time.Second
			testRunner.ErrorHandlingMode = runner.ErrorHandlingMode(*errFlag)
			testRunner.ScenarioOverride = *scenarioFlag
			testRunner.Logger = func(format string, args ...interface{}) {
				t.Logf(format, args...)
			}
			priv, principal := mustJWT(t)
			testRunner.UseJWT(priv, principal)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			t.Logf("Starting test suite: %s (%d steps)", suite.Name, len(suite.Steps))

			result, err := testRunner.RunSuite(ctx, suite)
			if err != nil && result.Error == nil {
				result.Error = err
			}

			t.Logf("GameState ID: %s", result.GameState.String())

			for _, stepResult := range result.Results {
				switch {
				case stepResult.IsReset:
					t.Logf("   ↻ %s (%v)", stepResult.StepName, stepResult.Duration)
				case stepResult.Success:
					t.Logf("   ✓ %s (%v)", stepResult.StepName, stepResult.Duration)
				default:
					t.Errorf("   ✗ %s: %v", stepResult.StepName, stepResult.Error)
				}
			}

			if result.Error != nil {
				t.Fatalf("Test suite '%s' failed: %v", suite.Name, result.Error)
			}
			t.Logf("PASSED: Test suite '%s' completed in %v", suite.Name, result.Duration)
		})
	}
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

func mustJWT(t *testing.T) (*ecdsa.PrivateKey, uuid.UUID) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
	key, err := auth.LoadPrivateKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.TrimSpace(os.Getenv("STORY_ENGINE_PRINCIPAL"))
	if raw == "" {
		return key, uuid.MustParse("22222222-2222-4222-8222-222222222222")
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		t.Fatal("STORY_ENGINE_PRINCIPAL must be a non-nil UUID")
	}
	return key, id
}
