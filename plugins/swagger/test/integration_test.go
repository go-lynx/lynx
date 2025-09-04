package swagger_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
	
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/swagger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRuntime mock runtime
type MockRuntime struct {
	config map[string]interface{}
}

func (m *MockRuntime) GetConfig() plugins.Config {
	return &MockConfig{data: m.config}
}

func (m *MockRuntime) GetLogger() interface{} {
	return nil
}

// MockConfig mock configuration
type MockConfig struct {
	data map[string]interface{}
}

func (m *MockConfig) Value(key string) plugins.Value {
	return &MockValue{data: m.data}
}

// MockValue mock value
type MockValue struct {
	data interface{}
}

func (m *MockValue) Scan(dest interface{}) error {
	// Simple configuration scanning implementation
	return nil
}

func TestSwaggerPluginIntegration(t *testing.T) {
	// Create temporary directory
	tempDir, err := ioutil.TempDir("", "swagger-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	
	// Create test file
	testFile := filepath.Join(tempDir, "test.go")
	testCode := `package test

// UserController user controller
type UserController struct{}

// GetUser get user
// @Summary Get user information
// @Description Get user details by ID
// @Tags User Management
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} User "Success"
// @Router /api/v1/users/{id} [get]
func (c *UserController) GetUser() {}

// User user model
type User struct {
	ID   int    ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}
`
	err = ioutil.WriteFile(testFile, []byte(testCode), 0644)
	require.NoError(t, err)
	
	// Create plugin
	plugin := swagger.NewSwaggerPlugin()
	assert.NotNil(t, plugin)
	
	// Mock runtime configuration
	runtime := &MockRuntime{
		config: map[string]interface{}{
			"lynx.swagger": map[string]interface{}{
				"enabled": true,
				"gen": map[string]interface{}{
					"enabled":      true,
					"scan_dirs":    []string{tempDir},
					"output_path":  filepath.Join(tempDir, "swagger.json"),
				},
				"ui": map[string]interface{}{
					"enabled": false, // Don't start UI during testing
				},
			},
		},
	}
	
	// Initialize resources
	err = plugin.InitializeResources(runtime)
	assert.NoError(t, err)
	
	// Wait for document generation
	time.Sleep(100 * time.Millisecond)
	
	// Check generated file
	swaggerFile := filepath.Join(tempDir, "swagger.json")
	if _, err := os.Stat(swaggerFile); err == nil {
		// File exists, read and validate
		data, err := ioutil.ReadFile(swaggerFile)
		assert.NoError(t, err)
		assert.Contains(t, string(data), "swagger")
		assert.Contains(t, string(data), "2.0")
	}
	
	// Clean up tasks
	err = plugin.CleanupTasks()
	assert.NoError(t, err)
}

func TestSwaggerAnnotationParsing(t *testing.T) {
	parser := &swagger.AnnotationParser{}
	
	testCases := []struct {
		name     string
		line     string
		expected interface{}
	}{
		{
			name: "Parse path parameter",
			line: "@Param id path int true \"User ID\"",
			expected: map[string]interface{}{
				"name":     "id",
				"in":       "path",
				"type":     "integer",
				"required": true,
			},
		},
		{
			name: "Parse query parameter",
			line: "@Param page query int false \"Page number\" default(1)",
			expected: map[string]interface{}{
				"name":     "page",
				"in":       "query",
				"type":     "integer",
				"required": false,
			},
		},
		{
			name: "Parse request body parameter",
			line: "@Param user body UserRequest true \"User information\"",
			expected: map[string]interface{}{
				"name":     "user",
				"in":       "body",
				"required": true,
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			param := parser.ParseParam(tc.line)
			assert.NotNil(t, param)
			
			expected := tc.expected.(map[string]interface{})
			if name, ok := expected["name"]; ok {
				assert.Equal(t, name, param.Name)
			}
			if in, ok := expected["in"]; ok {
				assert.Equal(t, in, param.In)
			}
			if paramType, ok := expected["type"]; ok {
				assert.Equal(t, paramType, param.Type)
			}
			if required, ok := expected["required"]; ok {
				assert.Equal(t, required, param.Required)
			}
		})
	}
}

func TestSwaggerResponseParsing(t *testing.T) {
	parser := &swagger.AnnotationParser{}
	
	testCases := []struct {
		name         string
		line         string
		expectedCode int
		expectedDesc string
	}{
		{
			name:         "Success response",
			line:         "@Success 200 {object} UserResponse \"Retrieved successfully\"",
			expectedCode: 200,
			expectedDesc: "Retrieved successfully",
		},
		{
			name:         "Error response",
			line:         "@Failure 404 {object} ErrorResponse \"User not found\"",
			expectedCode: 404,
			expectedDesc: "User not found",
		},
		{
			name:         "No content response",
			line:         "@Success 204 \"Deleted successfully\"",
			expectedCode: 204,
			expectedDesc: "Deleted successfully",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			code, resp := parser.ParseResponse(tc.line)
			assert.Equal(t, tc.expectedCode, code)
			assert.NotNil(t, resp)
			assert.Equal(t, tc.expectedDesc, resp.Description)
		})
	}
}
