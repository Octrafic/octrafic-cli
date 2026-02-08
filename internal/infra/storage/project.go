package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Octrafic/octrafic-cli/internal/agents"
	"github.com/Octrafic/octrafic-cli/internal/core/parser"
)

const projectsDir = "projects"
const tempProjectsDir = "/tmp/octrafic-projects"

// Project represents a single API testing project
type Project struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	BaseURL        string      `json:"base_url"`
	SpecPath       string      `json:"spec_path,omitempty"`
	SpecHash       string      `json:"spec_hash,omitempty"`
	IsTemporary    bool        `json:"is_temporary"`
	AuthConfig     *AuthConfig `json:"auth_config,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	LastAccessedAt time.Time   `json:"last_accessed_at"`
}

// AuthConfig stores authentication configuration for a project
// WARNING: Credentials are stored in plain text
type AuthConfig struct {
	Type     string `json:"type"`               // none, bearer, apikey, basic
	Token    string `json:"token,omitempty"`    // Bearer token
	KeyName  string `json:"key_name,omitempty"` // API key name (e.g., X-API-Key)
	KeyValue string `json:"key_value,omitempty"`// API key value
	Location string `json:"location,omitempty"` // header or query
	Username string `json:"username,omitempty"` // Basic auth username
	Password string `json:"password,omitempty"` // Basic auth password
}

// ClearAuth removes authentication configuration from project
func (p *Project) ClearAuth() {
	p.AuthConfig = nil
}

// HasAuth returns true if project has saved authentication
func (p *Project) HasAuth() bool {
	return p.AuthConfig != nil && p.AuthConfig.Type != "" && p.AuthConfig.Type != "none"
}

// ProjectContext encapsulates project data with pre-computed paths
type ProjectContext struct {
	Project     *Project
	ProjectPath string
	EndpointsPath string
	HashPath    string
}

// NewProjectContext creates a ProjectContext from a loaded or new project
func NewProjectContext(project *Project) (*ProjectContext, error) {
	projectPath, err := GetProjectPathByType(project.ID, project.IsTemporary)
	if err != nil {
		return nil, fmt.Errorf("failed to get project path: %w", err)
	}

	return &ProjectContext{
		Project:       project,
		ProjectPath:   projectPath,
		EndpointsPath: filepath.Join(projectPath, "endpoints.json"),
		HashPath:      filepath.Join(projectPath, "spec.hash"),
	}, nil
}

// LoadProjectContext loads a project and creates its context
func LoadProjectContext(projectID string) (*ProjectContext, error) {
	project, err := LoadProject(projectID)
	if err != nil {
		return nil, err
	}

	// Update last accessed time
	project.LastAccessedAt = time.Now()
	_ = SaveProject(project) // Non-fatal - just continue

	return NewProjectContext(project)
}

// GetProjectsDir returns the path to the projects directory
func GetProjectsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	projectsPath := filepath.Join(homeDir, storageDir, projectsDir)
	if err := os.MkdirAll(projectsPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create projects directory: %w", err)
	}

	return projectsPath, nil
}

// GetTempProjectsDir returns the path to the temporary projects directory
func GetTempProjectsDir() (string, error) {
	if err := os.MkdirAll(tempProjectsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp projects directory: %w", err)
	}
	return tempProjectsDir, nil
}

// GetProjectPathByType returns the path to a specific project directory based on type
func GetProjectPathByType(projectID string, isTemporary bool) (string, error) {
	var projectsPath string
	var err error

	if isTemporary {
		projectsPath, err = GetTempProjectsDir()
	} else {
		projectsPath, err = GetProjectsDir()
	}

	if err != nil {
		return "", err
	}

	projectPath := filepath.Join(projectsPath, projectID)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create project directory: %w", err)
	}

	return projectPath, nil
}

// SaveProject saves a project to disk
func SaveProject(project *Project) error {
	projectPath, err := GetProjectPathByType(project.ID, project.IsTemporary)
	if err != nil {
		return err
	}

	project.UpdatedAt = time.Now()

	filePath := filepath.Join(projectPath, "project.json")
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write project file: %w", err)
	}

	return nil
}

// LoadProject loads a project from disk (checks both permanent and temporary locations)
func LoadProject(projectID string) (*Project, error) {
	// Try permanent location first (without creating directory)
	permanentPath, err := GetProjectsDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(permanentPath, projectID, "project.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		// Try temporary location
		tempPath, err2 := GetTempProjectsDir()
		if err2 != nil {
			return nil, fmt.Errorf("failed to read project file: %w", err)
		}

		tempFilePath := filepath.Join(tempPath, projectID, "project.json")
		data, err = os.ReadFile(tempFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read project file: %w", err)
		}
	}

	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project: %w", err)
	}

	return &project, nil
}

// ListProjects returns all projects
func ListProjects() ([]*Project, error) {
	projectsPath, err := GetProjectsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(projectsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	var projects []*Project
	for _, entry := range entries {
		if entry.IsDir() {
			project, err := LoadProject(entry.Name())
			if err != nil {
				// Skip invalid projects
				continue
			}
			projects = append(projects, project)
		}
	}

	return projects, nil
}

// DeleteProject deletes a project from disk
func DeleteProject(project *Project) error {
	projectPath, err := GetProjectPathByType(project.ID, project.IsTemporary)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(projectPath); err != nil {
		return fmt.Errorf("failed to delete project directory: %w", err)
	}

	return nil
}

// ComputeFileHash computes SHA256 hash of a file
func ComputeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// LoadOrParseSpec parses the spec file and returns endpoints with hash
// Now uses endpoints.json storage instead of Bleve index
func LoadOrParseSpec(specPath, projectID, baseURL, apiKey string, isTemporary bool) ([]parser.Endpoint, string, error) {
	// Compute current hash
	currentHash, err := ComputeFileHash(specPath)
	if err != nil {
		return nil, "", err
	}

	// Check if we have cached endpoints with matching hash
	if HasEndpoints(projectID, isTemporary) {
		storedHash, err := getStoredHash(projectID, isTemporary)
		if err == nil && storedHash == currentHash {
			// Hash matches, load cached endpoints
			endpoints, err := LoadEndpoints(projectID, isTemporary)
			return endpoints, currentHash, err
		}
	}

	// Parse spec file
	ext := strings.ToLower(filepath.Ext(specPath))

	var endpoints []parser.Endpoint

	// For JSON/YAML/GraphQL/Markdown, use local parser (fast, no backend needed)
	if ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".graphql" || ext == ".gql" || ext == ".md" || ext == ".markdown" {
		spec, err := parser.ParseSpecification(specPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to parse spec: %w", err)
		}
		endpoints = spec.Endpoints
	} else {
		// For other formats (RAML, Proto, etc), use local AI processing
		// Read spec file content
		specContent, err := os.ReadFile(specPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read spec file: %w", err)
		}

		// Create local agent and process spec
		localAgent, err := agent.NewAgent(baseURL)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create agent: %w", err)
		}

		apiEndpoints, err := localAgent.ProcessSpecification(string(specContent), baseURL)
		if err != nil {
			return nil, "", fmt.Errorf("failed to process spec with AI: %w", err)
		}

		// Convert agent response to parser.Endpoint format
		for _, ep := range apiEndpoints {
			endpoints = append(endpoints, parser.Endpoint{
				Method:       ep.Method,
				Path:         ep.Path,
				Description:  ep.Description,
				RequiresAuth: ep.RequiresAuth,
				AuthType:     ep.AuthType,
			})
		}
	}

	// Save endpoints to JSON
	if err := SaveEndpoints(projectID, endpoints, isTemporary); err != nil {
		return nil, "", fmt.Errorf("failed to save endpoints: %w", err)
	}

	// Store the hash
	if err := storeHash(projectID, currentHash, isTemporary); err != nil {
		return nil, "", fmt.Errorf("failed to store hash: %w", err)
	}

	return endpoints, currentHash, nil
}

// CreateOrUpdateProject creates or updates a project with spec parsing
// Returns the project and parsed endpoints
func CreateOrUpdateProject(projectID, name, baseURL, specPath, apiKey string, isTemporary bool) (*Project, []parser.Endpoint, error) {
	var endpoints []parser.Endpoint
	var specHash string

	if specPath != "" {
		// Parse spec and save endpoints to JSON
		var err error
		endpoints, specHash, err = LoadOrParseSpec(specPath, projectID, baseURL, apiKey, isTemporary)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse spec: %w", err)
		}
	}

	// Try to load existing project
	project, err := LoadProject(projectID)
	if err != nil {
		// Create new project
		project = &Project{
			ID:          projectID,
			Name:        name,
			BaseURL:     baseURL,
			SpecPath:    specPath,
			SpecHash:    specHash,
			IsTemporary: isTemporary,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
	} else {
		// Update existing project
		if name != "" {
			project.Name = name
		}
		if baseURL != "" {
			project.BaseURL = baseURL
		}
		if specPath != "" {
			project.SpecPath = specPath
			project.SpecHash = specHash
		}
		project.UpdatedAt = time.Now()
	}

	// Save project
	if err := SaveProject(project); err != nil {
		return nil, nil, err
	}

	return project, endpoints, nil
}

// ListNamedProjects returns only named (non-temporary) projects
func ListNamedProjects() ([]*Project, error) {
	projects, err := ListProjects()
	if err != nil {
		return nil, err
	}

	var namedProjects []*Project
	for _, project := range projects {
		if !project.IsTemporary && project.Name != "" {
			namedProjects = append(namedProjects, project)
		}
	}

	return namedProjects, nil
}

// FindProjectByName searches for a project by name in permanent storage
func FindProjectByName(name string) (*Project, error) {
	projects, err := ListNamedProjects()
	if err != nil {
		return nil, err
	}

	for _, project := range projects {
		if project.Name == name {
			return project, nil
		}
	}

	return nil, fmt.Errorf("project not found: %s", name)
}

// ValidateSpecPath checks if spec file exists and is readable
func ValidateSpecPath(specPath string) error {
	if specPath == "" {
		return fmt.Errorf("spec path is empty")
	}

	// Check if file exists
	info, err := os.Stat(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("spec file does not exist: %s", specPath)
		}
		return fmt.Errorf("failed to access spec file: %w", err)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("spec path is not a regular file: %s", specPath)
	}

	// Check if readable
	file, err := os.Open(specPath)
	if err != nil {
		return fmt.Errorf("spec file is not readable: %w", err)
	}
	_ = file.Close()

	return nil
}

// CheckNameConflict checks if a project with the given name already exists (excluding given projectID)
func CheckNameConflict(name string, excludeProjectID string) (*Project, error) {
	if name == "" {
		return nil, nil // No conflict for unnamed projects
	}

	projects, err := ListNamedProjects()
	if err != nil {
		return nil, err
	}

	for _, project := range projects {
		if project.Name == name && project.ID != excludeProjectID {
			return project, nil // Conflict found
		}
	}

	return nil, nil // No conflict
}

// CleanupTempProjects removes all temporary projects
func CleanupTempProjects() error {
	tempPath, err := GetTempProjectsDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(tempPath)
	if err != nil {
		return fmt.Errorf("failed to read temp projects directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			projectPath := filepath.Join(tempPath, entry.Name())
			if err := os.RemoveAll(projectPath); err != nil {
				// Continue cleaning up other projects even if one fails
				continue
			}
		}
	}

	return nil
}

// getStoredHash retrieves the stored spec hash for a project
func getStoredHash(projectID string, isTemporary bool) (string, error) {
	projectPath, err := GetProjectPathByType(projectID, isTemporary)
	if err != nil {
		return "", err
	}
	hashPath := filepath.Join(projectPath, "spec.hash")
	content, err := os.ReadFile(hashPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

// storeHash stores the spec hash for a project
func storeHash(projectID, hash string, isTemporary bool) error {
	projectPath, err := GetProjectPathByType(projectID, isTemporary)
	if err != nil {
		return err
	}
	hashPath := filepath.Join(projectPath, "spec.hash")
	return os.WriteFile(hashPath, []byte(hash), 0644)
}
