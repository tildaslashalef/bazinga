package tools

import (
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMain sets up and tears down the test environment
func TestMain(m *testing.M) {
	// Initialize test logger
	logger := loggy.NewTestLogger()
	_ = logger

	// Run tests
	exitCode := m.Run()

	// Exit with code from test run
	os.Exit(exitCode)
}

// TestGitStatus tests the gitStatus function
func TestGitStatus(t *testing.T) {
	// Create a temp directory to use for tests
	tmpDir := t.TempDir()

	// Create a tool executor with the temp path
	te := &ToolExecutor{rootPath: tmpDir}

	// Setup test cases
	tests := []struct {
		name           string
		mockOutput     string
		mockError      bool
		expectedResult string
		expectError    bool
	}{
		{
			name:           "clean_repo",
			mockOutput:     "", // Empty output results in "Working tree clean"
			mockError:      false,
			expectedResult: "Working tree clean",
			expectError:    false,
		},
		{
			name:           "branch_only",
			mockOutput:     "## master...origin/master",
			mockError:      false,
			expectedResult: "Branch: master...origin/master",
			expectError:    false,
		},
		{
			name:           "modified_files",
			mockOutput:     "## master...origin/master\nM  file1.go\n D file2.go\n?? file3.go",
			mockError:      false,
			expectedResult: "Branch: master...origin/master\n  Modified: file1.go\n  Deleted (unstaged): file2.go\n  Untracked: file3.go",
			expectError:    false,
		},
		{
			name:           "git_error",
			mockOutput:     "fatal: not a git repository",
			mockError:      true,
			expectedResult: "",
			expectError:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Save the original execCommand
			origExecCommand := execCommand
			defer func() { execCommand = origExecCommand }()

			// Create mock script file
			mockScriptFile := filepath.Join(tmpDir, "mock_git.sh")
			mockScript := `#!/bin/sh
if [ "$MOCK_ERROR" = "1" ]; then
  echo "$MOCK_OUTPUT" >&2
  exit 1
fi
echo "$MOCK_OUTPUT"
exit 0
`
			if err := os.WriteFile(mockScriptFile, []byte(mockScript), 0o755); err != nil {
				t.Fatal(err)
			}

			// Mock exec.Command
			execCommand = func(command string, args ...string) *exec.Cmd {
				cs := []string{"-c"}

				// Set the mock's environment
				env := []string{
					"MOCK_OUTPUT=" + test.mockOutput,
				}

				if test.mockError {
					env = append(env, "MOCK_ERROR=1")
				}

				cmd := exec.Command("/bin/sh", cs...)
				cmd.Env = append(os.Environ(), env...)
				cmd.Args = append(cmd.Args, mockScriptFile)
				return cmd
			}

			// Call the function
			result, err := te.gitStatus(map[string]interface{}{})

			// Check error
			if test.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !test.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check result
			if !test.expectError && result != test.expectedResult {
				t.Errorf("expected result %q, got %q", test.expectedResult, result)
			}
		})
	}
}

// TestGitDiff tests the gitDiff function
func TestGitDiff(t *testing.T) {
	// Create a temp directory to use for tests
	tmpDir := t.TempDir()

	// Create a tool executor with the temp path
	te := &ToolExecutor{rootPath: tmpDir}

	// Setup test cases
	tests := []struct {
		name           string
		input          map[string]interface{}
		mockOutput     string
		mockError      bool
		expectedResult string
		expectError    bool
	}{
		{
			name:           "no_changes",
			input:          map[string]interface{}{},
			mockOutput:     "",
			mockError:      false,
			expectedResult: "No changes to show",
			expectError:    false,
		},
		{
			name:           "with_changes",
			input:          map[string]interface{}{},
			mockOutput:     "diff --git a/file.go b/file.go\nindex 123..456 100644\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,3 @@\n-old line\n+new line",
			mockError:      false,
			expectedResult: "diff --git a/file.go b/file.go\nindex 123..456 100644\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,3 @@\n-old line\n+new line",
			expectError:    false,
		},
		{
			name:           "staged_changes",
			input:          map[string]interface{}{"staged": true},
			mockOutput:     "diff --git a/file.go b/file.go\nindex 123..456 100644\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,3 @@\n-old line\n+new line",
			mockError:      false,
			expectedResult: "diff --git a/file.go b/file.go\nindex 123..456 100644\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,3 @@\n-old line\n+new line",
			expectError:    false,
		},
		{
			name:           "specific_file",
			input:          map[string]interface{}{"file_path": "file.go"},
			mockOutput:     "diff for file.go",
			mockError:      false,
			expectedResult: "diff for file.go",
			expectError:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Save the original execCommand
			origExecCommand := execCommand
			defer func() { execCommand = origExecCommand }()

			// Create mock script file
			mockScriptFile := filepath.Join(tmpDir, "mock_git.sh")
			mockScript := `#!/bin/sh
if [ "$MOCK_ERROR" = "1" ]; then
  echo "$MOCK_OUTPUT" >&2
  exit 1
fi
echo "$MOCK_OUTPUT"
exit 0
`
			if err := os.WriteFile(mockScriptFile, []byte(mockScript), 0o755); err != nil {
				t.Fatal(err)
			}

			// Mock exec.Command
			execCommand = func(command string, args ...string) *exec.Cmd {
				cs := []string{"-c"}

				// Set the mock's environment
				env := []string{
					"MOCK_OUTPUT=" + test.mockOutput,
				}

				if test.mockError {
					env = append(env, "MOCK_ERROR=1")
				}

				cmd := exec.Command("/bin/sh", cs...)
				cmd.Env = append(os.Environ(), env...)
				cmd.Args = append(cmd.Args, mockScriptFile)
				return cmd
			}

			// Call the function
			result, err := te.gitDiff(test.input)

			// Check error
			if test.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !test.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check result
			if !test.expectError && result != test.expectedResult {
				t.Errorf("expected result %q, got %q", test.expectedResult, result)
			}
		})
	}
}

// TestHelperProcess isn't a real test. It's used as a helper process for TestGitStatus and other tests.
// This allows us to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	// Get the mock output
	mockOutput := os.Getenv("MOCK_OUTPUT")

	// Check if we should return an error
	if os.Getenv("MOCK_ERROR") == "1" {
		os.Exit(1)
	}

	// Print the mock output
	if _, err := os.Stdout.Write([]byte(mockOutput)); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
