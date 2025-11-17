package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtractFrontMatter(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected FrontMatterData
		wantErr  bool
	}{
		{
			name: "valid front matter",
			content: `---
title: "Test Event"
place: "Test Place"
city: "Test City"
---
Some content here
`,
			expected: FrontMatterData{
				Title: "Test Event",
				Place: "Test Place",
				City:  "Test City",
			},
			wantErr: false,
		},
		{
			name: "missing front matter",
			content: `No front matter
Just content
`,
			wantErr: true,
		},
		{
			name: "invalid yaml",
			content: `---
title: [invalid yaml
---
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary test file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.md")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test the function
			fmData, err := extractFrontMatter(tmpFile)
			if tt.wantErr {
				if err == nil {
					t.Error("extractFrontMatter() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("extractFrontMatter() unexpected error: %v", err)
			}

			// Check the results
			if fmData.Title != tt.expected.Title {
				t.Errorf("Title = %v, want %v", fmData.Title, tt.expected.Title)
			}
			if fmData.Place != tt.expected.Place {
				t.Errorf("Place = %v, want %v", fmData.Place, tt.expected.Place)
			}
			if fmData.City != tt.expected.City {
				t.Errorf("City = %v, want %v", fmData.City, tt.expected.City)
			}
		})
	}
}

func TestGetWeekdayName(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		lang     string
		expected string
	}{
		{
			name:     "monday in french",
			date:     time.Date(2024, 12, 23, 0, 0, 0, 0, time.UTC),
			lang:     "fr",
			expected: "lundi",
		},
		{
			name:     "monday in english",
			date:     time.Date(2024, 12, 23, 0, 0, 0, 0, time.UTC),
			lang:     "en",
			expected: "monday",
		},
		{
			name:     "tuesday in french",
			date:     time.Date(2024, 12, 24, 0, 0, 0, 0, time.UTC),
			lang:     "fr",
			expected: "mardi",
		},
		{
			name:     "tuesday in english",
			date:     time.Date(2024, 12, 24, 0, 0, 0, 0, time.UTC),
			lang:     "en",
			expected: "tuesday",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getWeekdayName(tt.date, tt.lang)
			if got != tt.expected {
				t.Errorf("getWeekdayName(%v, %s) = %v, want %v", tt.date, tt.lang, got, tt.expected)
			}
		})
	}
}

func TestGetMonthName(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		lang     string
		expected string
	}{
		{
			name:     "december in french",
			date:     time.Date(2024, 12, 23, 0, 0, 0, 0, time.UTC),
			lang:     "fr",
			expected: "décembre",
		},
		{
			name:     "december in english",
			date:     time.Date(2024, 12, 23, 0, 0, 0, 0, time.UTC),
			lang:     "en",
			expected: "december",
		},
		{
			name:     "january in french",
			date:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			lang:     "fr",
			expected: "janvier",
		},
		{
			name:     "january in english",
			date:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			lang:     "en",
			expected: "january",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMonthName(tt.date, tt.lang)
			if got != tt.expected {
				t.Errorf("getMonthName(%v, %s) = %v, want %v", tt.date, tt.lang, got, tt.expected)
			}
		})
	}
}

func TestCapitalizeFirstLetter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single letter",
			input:    "a",
			expected: "A",
		},
		{
			name:     "word",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "already capitalized",
			input:    "World",
			expected: "World",
		},
		{
			name:     "sentence",
			input:    "hello world",
			expected: "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := capitalizeFirstLetter(tt.input)
			if got != tt.expected {
				t.Errorf("capitalizeFirstLetter(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestWaitForEventPage(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		timeout       time.Duration
		interval      time.Duration
		wantErr       bool
	}{
		{
			name:           "success on first try",
			responseStatus: http.StatusOK,
			timeout:       100 * time.Millisecond,
			interval:      10 * time.Millisecond,
			wantErr:       false,
		},
		{
			name:           "not found",
			responseStatus: http.StatusNotFound,
			timeout:       100 * time.Millisecond,
			interval:      10 * time.Millisecond,
			wantErr:       true,
		},
		{
			name:           "server error",
			responseStatus: http.StatusInternalServerError,
			timeout:       100 * time.Millisecond,
			interval:      10 * time.Millisecond,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
			}))
			defer server.Close()

			err := waitForEventPage(server.URL, tt.timeout, tt.interval)
			if tt.wantErr {
				if err == nil {
					t.Error("waitForEventPage() expected error but got none")
				}
			} else if err != nil {
				t.Errorf("waitForEventPage() unexpected error: %v", err)
			}
		})
	}
}

func TestPublishEventOnFacebook(t *testing.T) {
	tests := []struct {
		name        string
		data        EventData
		fmData      FrontMatterData
		eventURL    string
		pageID      string
		dryRun      bool
		wantErr     bool
		errContains string
	}{
		{
			name: "dry run success",
			data: EventData{
				Date:                "23/12/2024",
				LongDate:            "lundi 23 décembre",
				LongDateCapitalized: "Lundi 23 décembre",
			},
			fmData: FrontMatterData{
				Title: "Test Event",
				Place: "Test Place",
				City:  "Test City",
			},
			eventURL: "http://example.com",
			pageID:   "123",
			dryRun:   true,
			wantErr:  false,
		},
		{
			name: "missing page ID",
			data: EventData{},
			fmData: FrontMatterData{},
			eventURL: "http://example.com",
			pageID:   "",
			dryRun:   false,
			wantErr:  true,
			errContains: "Invalid OAuth access token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := publishEventOnFacebook(tt.data, tt.fmData, tt.eventURL, tt.pageID, "dummy-token", tt.dryRun)
			if tt.wantErr {
				if err == nil {
					t.Error("publishEventOnFacebook() expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("publishEventOnFacebook() unexpected error: %v", err)
			}
		})
	}
}

func TestRunGitCommand(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		args        []string
		wantErr     bool
		errContains string
	}{
		{
			name: "init",
			cmd:  "init",
			args: []string{},
			wantErr: false,
		},
		{
			name: "status after init",
			cmd:  "status",
			args: []string{},
			wantErr: false,
		},
		{
			name: "invalid command",
			cmd:  "invalid",
			args: []string{},
			wantErr: true,
			errContains: "n'est pas une commande git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			
			// Initialize git repo first
			if _, err := runGitCommand(tmpDir, "init"); err != nil {
				t.Fatalf("Failed to initialize git repo: %v", err)
			}

			// Run the actual test command
			if tt.cmd != "init" {
				args := append([]string{tt.cmd}, tt.args...)
				_, err := runGitCommand(tmpDir, args...)
				if tt.wantErr {
					if err == nil {
						t.Error("runGitCommand() expected error but got none")
					} else if !strings.Contains(err.Error(), tt.errContains) {
						t.Errorf("error = %v, want error containing %v", err, tt.errContains)
					}
					return
				}
				if err != nil {
					t.Errorf("runGitCommand() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRunGitCheckChanges(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		setupFunc   func(dir string) error
		wantChanges bool
		wantErr     bool
		errContains string
	}{
		{
			name:     "no changes",
			filePath: "test.txt",
			setupFunc: func(dir string) error {
				// Create and commit a file
				if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test content"), 0644); err != nil {
					return err
				}
				if _, err := runGitCommand(dir, "add", "test.txt"); err != nil {
					return err
				}
				if _, err := runGitCommand(dir, "commit", "-m", "Initial commit"); err != nil {
					return err
				}
				return nil
			},
			wantChanges: false,
			wantErr:     false,
		},
		{
			name:     "changes",
			filePath: "test.txt",
			setupFunc: func(dir string) error {
				// Create a file and stage it but don't commit
				if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test content"), 0644); err != nil {
					return err
				}
				if _, err := runGitCommand(dir, "add", "test.txt"); err != nil {
					return err
				}
				return nil
			},
			wantChanges: true,
			wantErr:     false,
		},
		{
			name:     "invalid file path",
			filePath: "invalid/file/path",
			setupFunc: func(dir string) error {
				return nil
			},
			wantChanges: false,
			wantErr:     true,
			errContains: "révision inconnue ou chemin inexistant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			
			// Initialize git repo
			if _, err := runGitCommand(tmpDir, "init"); err != nil {
				t.Fatalf("Failed to initialize git repo: %v", err)
			}

			// Configure git user for commits
			if _, err := runGitCommand(tmpDir, "config", "user.email", "test@example.com"); err != nil {
				t.Fatalf("Failed to configure git user email: %v", err)
			}
			if _, err := runGitCommand(tmpDir, "config", "user.name", "Test User"); err != nil {
				t.Fatalf("Failed to configure git user name: %v", err)
			}

			// Run setup function
			if err := tt.setupFunc(tmpDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			hasChanges, err := runGitCheckChanges(tmpDir, tt.filePath)
			if tt.wantErr {
				if err == nil {
					t.Error("runGitCheckChanges() expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("runGitCheckChanges() unexpected error: %v", err)
			}
			if hasChanges != tt.wantChanges {
				t.Errorf("hasChanges = %v, want %v", hasChanges, tt.wantChanges)
			}
		})
	}
}

func TestPublishEventMarkdown(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Mock git functions
	mockGitCommand := func(dir string, args ...string) (string, error) {
		t.Logf("Mock git command in %s: git %v", dir, args)
		return "", nil
	}
	mockGitCheckChanges := func(dir, filePath string) (bool, error) {
		t.Logf("Mock git check changes in %s for file %s", dir, filePath)
		return true, nil
	}

	tests := []struct {
		name        string
		setup       func(string)
		expectError bool
	}{
		{
			name: "successful publish",
			setup: func(dir string) {
				templateContent := `---
title: Test Event
place: Test Place
city: Test City
---
Event on {{ .Date }}
Long date: {{ .LongDate }}
Capitalized: {{ .LongDateCapitalized }}`
				templatePath := filepath.Join(dir, "test.template.md")
				if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
					t.Fatalf("Failed to create template file: %v", err)
				}
			},
			expectError: false,
		},
		{
			name:        "dry run",
			setup:       func(string) {},
			expectError: false,
		},
		{
			name:        "invalid template path",
			setup:       func(string) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test directory
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// Run setup
			tt.setup(testDir)

			// Run the function
			date := time.Date(2024, 12, 23, 0, 0, 0, 0, time.UTC)
			templatePath := filepath.Join(testDir, "test.template.md")
			_, _, _, _, _, err := publishEventMarkdown(templatePath, date, date.Format("2006-01-02"), "fr", tt.name == "dry run", mockGitCommand, mockGitCheckChanges)

			// Check results
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestPublishEvent(t *testing.T) {
	// Save original git functions and restore after test
	origGitCommand := runGitCommand
	origGitCheckChanges := runGitCheckChanges
	defer func() {
		runGitCommand = origGitCommand
		runGitCheckChanges = origGitCheckChanges
	}()

	// Create a temporary template file
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "test.md.template")
	templateContent := `---
title: Test Event
place: Test Place
city: Test City
---
Event on {{ .Date }}
Long date: {{ .LongDate }}
Capitalized: {{ .LongDateCapitalized }}`
	err := os.WriteFile(templatePath, []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test template: %v", err)
	}

	// Create a path in a non-existent directory
	nonexistentDir := filepath.Join(tmpDir, "nonexistent")
	invalidPath := filepath.Join(nonexistentDir, "template.md")

	testDate := time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		ctx         EventContext
		expectError bool
		errorMsg    string
		mockGitErr  bool
		mockFileErr bool
	}{
		{
			name: "Valid context without Facebook",
			ctx: EventContext{
				Date:            testDate,
				TemplatePath:    templatePath,
				Language:        "fr",
				DryRun:         true,
				PublishFacebook: false,
			},
			expectError: false,
		},
		{
			name: "Facebook publish without token",
			ctx: EventContext{
				Date:            testDate,
				TemplatePath:    templatePath,
				Language:        "fr",
				DryRun:         true,
				PublishFacebook: true,
				PageAccessToken: "",
			},
			expectError: true,
			errorMsg:    "FACEBOOK_PAGE_ACCESS_TOKEN not set",
		},
		{
			name: "Facebook publish with token (all pages)",
			ctx: EventContext{
				Date:            testDate,
				TemplatePath:    templatePath,
				Language:        "fr",
				DryRun:         true,
				PublishFacebook: true,
				PageAccessToken: "test-token",
				FacebookPages:   "all",
			},
			expectError: false,
		},
		{
			name: "Facebook publish to specific page",
			ctx: EventContext{
				Date:            testDate,
				TemplatePath:    templatePath,
				Language:        "fr",
				DryRun:         true,
				PublishFacebook: true,
				PageAccessToken: "test-token",
				FacebookPages:   "forro-a-strasbourg",
			},
			expectError: false,
		},
		{
			name: "Facebook publish to multiple specific pages",
			ctx: EventContext{
				Date:            testDate,
				TemplatePath:    templatePath,
				Language:        "fr",
				DryRun:         true,
				PublishFacebook: true,
				PageAccessToken: "test-token",
				FacebookPages:   "forro-a-strasbourg,forro-stras",
			},
			expectError: false,
		},
		{
			name: "Facebook publish to unknown page",
			ctx: EventContext{
				Date:            testDate,
				TemplatePath:    templatePath,
				Language:        "fr",
				DryRun:         true,
				PublishFacebook: true,
				PageAccessToken: "test-token",
				FacebookPages:   "unknown-page",
			},
			expectError: false, // Should not error, just log a warning
		},
		{
			name: "Facebook publish to mix of valid and invalid pages",
			ctx: EventContext{
				Date:            testDate,
				TemplatePath:    templatePath,
				Language:        "fr",
				DryRun:         true,
				PublishFacebook: true,
				PageAccessToken: "test-token",
				FacebookPages:   "forro-a-strasbourg,unknown-page,forro-stras",
			},
			expectError: false, // Should not error, just log a warning for unknown page
		},
		{
			name: "Invalid template path",
			ctx: EventContext{
				Date:            testDate,
				TemplatePath:    invalidPath,
				Language:        "fr",
				DryRun:         true,
				PublishFacebook: false,
			},
			expectError: true,
			errorMsg:    "error publishing event",
			mockFileErr: true,
		},
		{
			name: "Git command error",
			ctx: EventContext{
				Date:            testDate,
				TemplatePath:    templatePath,
				Language:        "fr",
				DryRun:         false,
				PublishFacebook: false,
			},
			expectError: true,
			errorMsg:    "error publishing event",
			mockGitErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockGitErr {
				runGitCommand = func(dir string, args ...string) (string, error) {
					return "", fmt.Errorf("mock git error")
				}
			} else if tt.mockFileErr {
				// Mock the file error by making the template path inaccessible
				runGitCommand = func(dir string, args ...string) (string, error) {
					return "", fmt.Errorf("mock file error: no such file or directory")
				}
				runGitCheckChanges = func(dir, filePath string) (bool, error) {
					return false, fmt.Errorf("mock file error: no such file or directory")
				}
			} else {
				runGitCommand = func(dir string, args ...string) (string, error) {
					return "", nil
				}
				runGitCheckChanges = func(dir, filePath string) (bool, error) {
					return true, nil
				}
			}

			err := publishEvent(tt.ctx)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q but got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
