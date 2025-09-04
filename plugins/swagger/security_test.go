package swagger

import (
	"os"
	"testing"
)

func TestEnvironmentDetection(t *testing.T) {
	plugin := &PlugSwagger{
		config: &SwaggerConfig{
			Security: SecurityConfig{
				Environment: "",
			},
		},
	}

	// Test default environment
	env := plugin.getCurrentEnvironment()
	if env != EnvDevelopment {
		t.Errorf("Expected default environment to be %s, got %s", EnvDevelopment, env)
	}

	// Test environment variable detection
	os.Setenv("ENV", "testing")
	defer os.Unsetenv("ENV")

	env = plugin.getCurrentEnvironment()
	if env != "testing" {
		t.Errorf("Expected environment from ENV to be testing, got %s", env)
	}
}

func TestEnvironmentRestrictions(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		expected bool
	}{
		{"development_allowed", "development", true},
		{"testing_allowed", "testing", true},
		{"staging_denied", "staging", false},
		{"production_denied", "production", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &PlugSwagger{
				config: &SwaggerConfig{
					Security: SecurityConfig{
						Environment:   tt.env,
						AllowedEnvs:   []string{"development", "testing"},
						DisableInProd: true,
					},
				},
			}

			allowed := plugin.isEnvironmentAllowed()
			if allowed != tt.expected {
				t.Errorf("Environment %s: expected %v, got %v", tt.env, tt.expected, allowed)
			}
		})
	}
}

func TestPathValidation(t *testing.T) {
	plugin := &PlugSwagger{
		config: &SwaggerConfig{
			Gen: GenConfig{
				ScanDirs: []string{"./"},
			},
		},
	}

	// Test valid directory (current directory should exist)
	err := plugin.validateScanDirectory("./")
	if err != nil {
		t.Errorf("Valid directory should not cause error: %v", err)
	}

	// Test suspicious directory
	err = plugin.validateScanDirectory("/etc")
	if err == nil {
		t.Error("Suspicious directory should cause error")
	}

	// Test non-existent directory
	err = plugin.validateScanDirectory("./nonexistent")
	if err == nil {
		t.Error("Non-existent directory should cause error")
	}
}

func TestCORSValidation(t *testing.T) {
	plugin := &PlugSwagger{
		config: &SwaggerConfig{
			Security: SecurityConfig{
				TrustedOrigins: []string{"http://localhost:8080"},
			},
		},
	}

	// Test trusted origin
	if !plugin.isCORSAllowed("http://localhost:8080") {
		t.Error("Trusted origin should be allowed")
	}

	// Test untrusted origin
	if plugin.isCORSAllowed("http://malicious.com") {
		t.Error("Untrusted origin should not be allowed")
	}

	// Test default localhost origins when no trusted origins configured
	plugin.config.Security.TrustedOrigins = nil
	if !plugin.isCORSAllowed("http://localhost:8081") {
		t.Error("Localhost should be allowed by default")
	}
}

func TestHTMLEscaping(t *testing.T) {
	plugin := &PlugSwagger{}

	tests := []struct {
		input    string
		expected string
	}{
		{"<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"&<>\"'", "&amp;&lt;&gt;&quot;&#39;"},
		{"normal text", "normal text"},
		{"", ""},
	}

	for _, tt := range tests {
		result := plugin.escapeHTML(tt.input)
		if result != tt.expected {
			t.Errorf("Input: %s, Expected: %s, Got: %s", tt.input, tt.expected, result)
		}
	}
}
