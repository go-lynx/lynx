package swagger_test

import (
	"encoding/json"
	"testing"
	
	"github.com/go-lynx/lynx/plugins/swagger"
	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
)

func TestSwaggerPlugin(t *testing.T) {
	// Create plugin instance
	plugin := swagger.NewSwaggerPlugin()
	assert.NotNil(t, plugin)
	
	// Test plugin information
	assert.Equal(t, "swagger", plugin.Name())
	assert.Equal(t, "v1.0.0", plugin.Version())
	assert.NotEmpty(t, plugin.Description())
}

func TestAnnotationParser(t *testing.T) {
	// Create parser
	parser := &swagger.AnnotationParser{}
	
	// Test parameter parsing
	paramLine := "@Param id path int true \"User ID\""
	param := parser.ParseParam(paramLine)
	assert.NotNil(t, param)
	assert.Equal(t, "id", param.Name)
	assert.Equal(t, "path", param.In)
	assert.Equal(t, "integer", param.Type)
	assert.True(t, param.Required)
	assert.Equal(t, "User ID", param.Description)
	
	// Test response parsing
	responseLine := "@Success 200 {object} UserResponse \"Success\""
	code, resp := parser.ParseResponse(responseLine)
	assert.Equal(t, 200, code)
	assert.NotNil(t, resp)
	assert.Equal(t, "Success", resp.Description)
}

func TestSwaggerGeneration(t *testing.T) {
	// Create Swagger specification
	swagger := &spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Swagger: "2.0",
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:       "Test API",
					Version:     "1.0.0",
					Description: "Test API",
				},
			},
			Paths: &spec.Paths{
				Paths: map[string]spec.PathItem{
					"/api/v1/users": {
						PathItemProps: spec.PathItemProps{
							Get: &spec.Operation{
								OperationProps: spec.OperationProps{
									ID:          "listUsers",
									Summary:     "Get user list",
									Description: "Get user list with pagination",
									Tags:        []string{"User Management"},
									Produces:    []string{"application/json"},
									Responses: &spec.Responses{
										ResponsesProps: spec.ResponsesProps{
											StatusCodeResponses: map[int]spec.Response{
												200: {
													ResponseProps: spec.ResponseProps{
														Description: "Success",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	
	// Convert to JSON
	data, err := json.MarshalIndent(swagger, "", "  ")
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	
	// Validate JSON structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Equal(t, "2.0", result["swagger"])
	
	info := result["info"].(map[string]interface{})
	assert.Equal(t, "Test API", info["title"])
	assert.Equal(t, "1.0.0", info["version"])
}

func TestSwaggerUIHTML(t *testing.T) {
	// Test generate Swagger UI HTML
	html := swagger.GenerateSwaggerUIHTML("/api/swagger.json", "API Documentation")
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "swagger-ui")
	assert.Contains(t, html, "/api/swagger.json")
	assert.Contains(t, html, "API Documentation")
}
