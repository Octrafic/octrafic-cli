package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProjectManagement(t *testing.T) {
	// Test temporary vs permanent projects
	tempProject := &Project{
		ID:          "temp-test-id",
		Name:        "",
		BaseURL:     "https://temp-api.com",
		IsTemporary: true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	permanentProject := &Project{
		ID:          "perm-test-id",
		Name:        "Test Project",
		BaseURL:     "https://perm-api.com",
		IsTemporary: false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Test saving temporary project
	err := SaveProject(tempProject)
	if err != nil {
		t.Fatalf("Failed to save temporary project: %v", err)
	}

	// Test saving permanent project
	err = SaveProject(permanentProject)
	if err != nil {
		t.Fatalf("Failed to save permanent project: %v", err)
	}

	// Test loading projects
	loadedTemp, err := LoadProject(tempProject.ID)
	if err != nil {
		t.Fatalf("Failed to load temporary project: %v", err)
	}
	if !loadedTemp.IsTemporary {
		t.Errorf("Expected temporary project, got permanent")
	}

	loadedPerm, err := LoadProject(permanentProject.ID)
	if err != nil {
		t.Fatalf("Failed to load permanent project: %v", err)
	}
	if loadedPerm.IsTemporary {
		t.Errorf("Expected permanent project, got temporary")
	}

	// Test FindProjectByName
	foundProject, err := FindProjectByName("Test Project")
	if err != nil {
		t.Fatalf("Failed to find project by name: %v", err)
	}
	if foundProject.ID != permanentProject.ID {
		t.Errorf("Found wrong project: expected %s, got %s", permanentProject.ID, foundProject.ID)
	}

	// Test ListNamedProjects
	namedProjects, err := ListNamedProjects()
	if err != nil {
		t.Fatalf("Failed to list named projects: %v", err)
	}

	foundNamed := false
	for _, p := range namedProjects {
		if p.ID == permanentProject.ID {
			foundNamed = true
		}
		if p.ID == tempProject.ID {
			t.Errorf("Temporary project should not be in named projects list")
		}
	}
	if !foundNamed {
		t.Errorf("Named project not found in list")
	}

	// Cleanup
	tempPath, _ := GetProjectPathByType(tempProject.ID, true)
	_ = os.RemoveAll(tempPath)

	permPath, _ := GetProjectPathByType(permanentProject.ID, false)
	_ = os.RemoveAll(permPath)
}

func TestGetProjectPathByType(t *testing.T) {
	testID := "test-project-id"

	// Test permanent project path
	permPath, err := GetProjectPathByType(testID, false)
	if err != nil {
		t.Fatalf("Failed to get permanent project path: %v", err)
	}
	if !filepath.IsAbs(permPath) {
		t.Errorf("Expected absolute path, got: %s", permPath)
	}

	// Test temporary project path
	tempPath, err := GetProjectPathByType(testID, true)
	if err != nil {
		t.Fatalf("Failed to get temporary project path: %v", err)
	}
	if tempPath[:4] != "/tmp" {
		t.Errorf("Expected temporary path to start with /tmp, got: %s", tempPath)
	}

	// Cleanup
	_ = os.RemoveAll(permPath)
	_ = os.RemoveAll(tempPath)
}

func TestCleanupTempProjects(t *testing.T) {
	// Create some temporary projects
	tempProject1 := &Project{
		ID:          "cleanup-test-1",
		Name:        "",
		BaseURL:     "https://test1.com",
		IsTemporary: true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	tempProject2 := &Project{
		ID:          "cleanup-test-2",
		Name:        "",
		BaseURL:     "https://test2.com",
		IsTemporary: true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_ = SaveProject(tempProject1)
	_ = SaveProject(tempProject2)

	// Clean up
	err := CleanupTempProjects()
	if err != nil {
		t.Fatalf("Failed to cleanup temp projects: %v", err)
	}

	// Verify they're gone
	tempPath, _ := GetTempProjectsDir()
	entries, _ := os.ReadDir(tempPath)

	if len(entries) > 0 {
		t.Errorf("Expected no temporary projects after cleanup, found %d", len(entries))
	}
}
