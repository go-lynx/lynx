package swagger

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-openapi/spec"
)

const (
	pluginName        = "swagger"
	pluginVersion     = "v1.0.0"
	pluginDescription = "Swagger API documentation generator and UI server"
	confPrefix        = "lynx.swagger"
)

// SwaggerConfig plugin configuration
type SwaggerConfig struct {
	Enabled bool       `json:"enabled" yaml:"enabled"`
	Info    InfoConfig `json:"info" yaml:"info"`
	UI      UIConfig   `json:"ui" yaml:"ui"`
	Gen     GenConfig  `json:"generator" yaml:"generator"`
}

// InfoConfig API basic information
type InfoConfig struct {
	Title          string `json:"title" yaml:"title"`
	Description    string `json:"description" yaml:"description"`
	Version        string `json:"version" yaml:"version"`
	TermsOfService string `json:"termsOfService" yaml:"termsOfService"`
	Contact        struct {
		Name  string `json:"name" yaml:"name"`
		Email string `json:"email" yaml:"email"`
		URL   string `json:"url" yaml:"url"`
	} `json:"contact" yaml:"contact"`
	License struct {
		Name string `json:"name" yaml:"name"`
		URL  string `json:"url" yaml:"url"`
	} `json:"license" yaml:"license"`
}

// UIConfig UI configuration
type UIConfig struct {
	Path                     string `json:"path" yaml:"path"`
	Enabled                  bool   `json:"enabled" yaml:"enabled"`
	DeepLinking              bool   `json:"deepLinking" yaml:"deepLinking"`
	DisplayRequestDuration   bool   `json:"displayRequestDuration" yaml:"displayRequestDuration"`
	DocExpansion             string `json:"docExpansion" yaml:"docExpansion"`
	DefaultModelsExpandDepth int    `json:"defaultModelsExpandDepth" yaml:"defaultModelsExpandDepth"`
	Port                     int    `json:"port" yaml:"port"`
}

// GenConfig generator configuration
type GenConfig struct {
	ScanDirs         []string `json:"scanDirs" yaml:"scanDirs"`
	ExcludeDirs      []string `json:"excludeDirs" yaml:"excludeDirs"`
	AutoGenerate     bool     `json:"autoGenerate" yaml:"autoGenerate"`
	OutputPath       string   `json:"outputPath" yaml:"outputPath"`
	ParseComments    bool     `json:"parseComments" yaml:"parseComments"`
	GenerateExamples bool     `json:"generateExamples" yaml:"generateExamples"`
}

// PlugSwagger Swagger plugin
type PlugSwagger struct {
	*plugins.BasePlugin
	config    *SwaggerConfig
	swagger   *spec.Swagger
	mu        sync.RWMutex
	server    *http.Server
	generator *Generator
	watcher   *FileWatcher
	uiServer  *http.Server
}

// Generator Swagger documentation generator
type Generator struct {
	config *GenConfig
	parser *Parser
	mu     sync.Mutex
}

// Parser code parser
type Parser struct {
	packages map[string]*Package
	routes   []*Route
}

// Package package information
type Package struct {
	Name    string
	Structs map[string]*Struct
}

// Struct struct information
type Struct struct {
	Name   string
	Fields []Field
	Doc    string
}

// Field field information
type Field struct {
	Name     string
	Type     string
	Tag      string
	Required bool
	Doc      string
}

// Route route information
type Route struct {
	Method      string
	Path        string
	Handler     string
	Summary     string
	Description string
	Tags        []string
	Parameters  []Parameter
	Responses   map[string]Response
}

// Parameter parameter information
type Parameter struct {
	Name        string
	In          string // path, query, header, body
	Type        string
	Required    bool
	Description string
	Example     interface{}
}

// Response response information
type Response struct {
	Description string
	Schema      interface{}
	Examples    map[string]interface{}
}

// FileWatcher file watcher
type FileWatcher struct {
	paths    []string
	callback func()
	stop     chan struct{}
}

// NewSwaggerPlugin creates a Swagger plugin
func NewSwaggerPlugin() *PlugSwagger {
	return &PlugSwagger{
		BasePlugin: plugins.NewBasePlugin(
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			pluginName,
			pluginDescription,
			pluginVersion,
			confPrefix,
			100,
		),
		config: &SwaggerConfig{
			Enabled: true,
			UI: UIConfig{
				Path:    "/swagger",
				Enabled: true,
			},
			Gen: GenConfig{
				AutoGenerate:  true,
				OutputPath:    "./docs/swagger.json",
				ParseComments: true,
			},
		},
	}
}

// InitializeResources initializes resources
func (p *PlugSwagger) InitializeResources(rt plugins.Runtime) error {
	// Load configuration
	if err := rt.GetConfig().Value(confPrefix).Scan(p.config); err != nil {
		log.Warnf("Failed to load swagger config, using defaults: %v", err)
	}

	// Set default values
	if p.config.Info.Title == "" {
		p.config.Info.Title = "API Documentation"
	}
	if p.config.Info.Version == "" {
		p.config.Info.Version = "1.0.0"
	}
	if p.config.UI.Path == "" {
		p.config.UI.Path = "/swagger"
	}
	if p.config.Gen.OutputPath == "" {
		p.config.Gen.OutputPath = "./docs/swagger.json"
	}

	// Initialize generator
	p.generator = &Generator{
		config: &p.config.Gen,
		parser: &Parser{
			packages: make(map[string]*Package),
			routes:   make([]*Route, 0),
		},
	}

	// Initialize Swagger specification
	p.swagger = &spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Swagger: "2.0",
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:          p.config.Info.Title,
					Description:    p.config.Info.Description,
					Version:        p.config.Info.Version,
					TermsOfService: p.config.Info.TermsOfService,
				},
			},
			Schemes:     []string{"http", "https"},
			Paths:       &spec.Paths{Paths: make(map[string]spec.PathItem)},
			Definitions: spec.Definitions{},
		},
	}

	// Set contact information
	if p.config.Info.Contact.Name != "" {
		p.swagger.Info.Contact = &spec.ContactInfo{
			ContactInfoProps: spec.ContactInfoProps{},
		}
	}

	// Set license information
	if p.config.Info.License.Name != "" {
		p.swagger.Info.License = &spec.License{
			LicenseProps: spec.LicenseProps{},
		}
	}

	return nil
}

// StartupTasks startup tasks
func (p *PlugSwagger) StartupTasks() error {
	if !p.config.Enabled {
		log.Info("Swagger plugin is disabled")
		return nil
	}

	// Generate initial documentation
	if p.config.Gen.AutoGenerate {
		if err := p.generateSwaggerDocs(); err != nil {
			log.Errorf("Failed to generate swagger docs: %v", err)
		}
	}

	// Start UI service
	if p.config.UI.Enabled {
		r := p.startSwaggerUI()
		if r != nil {
			return fmt.Errorf("failed to start swagger UI server: %w", r)
		}
	}

	// Start file monitoring
	if p.config.Gen.AutoGenerate {
		p.startFileWatcher()
	}

	// Log that plugin has started
	log.Infof("Swagger plugin registered to application")

	log.Infof("Swagger plugin started successfully. UI available at %s", p.config.UI.Path)
	return nil
}

// CleanupTasks cleanup tasks
func (p *PlugSwagger) CleanupTasks() error {
	// Stop file monitoring
	if p.watcher != nil {
		close(p.watcher.stop)
	}

	// Stop UI server
	if p.uiServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.uiServer.Shutdown(ctx); err != nil {
			log.Errorf("Failed to shutdown swagger UI server: %v", err)
		}
	}

	return nil
}

// generateSwaggerDocs generates Swagger documentation
func (p *PlugSwagger) generateSwaggerDocs() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Info("Starting Swagger documentation generation...")

	// Create annotation parser
	parser := NewAnnotationParser(p.swagger)

	// Scan directories
	for _, dir := range p.config.Gen.ScanDirs {
		log.Infof("Scanning directory: %s", dir)
		if err := parser.ScanDirectory(dir); err != nil {
			log.Warnf("Failed to scan directory %s: %v", dir, err)
		}
	}

	// If no paths are scanned, add example endpoint
	if p.swagger.Paths == nil || len(p.swagger.Paths.Paths) == 0 {
		p.swagger.Paths = &spec.Paths{
			Paths: map[string]spec.PathItem{
				"/health": {
					PathItemProps: spec.PathItemProps{
						Get: &spec.Operation{
							OperationProps: spec.OperationProps{
								ID:          "health-check",
								Summary:     "Health Check",
								Description: "Check the health status of the service",
								Tags:        []string{"Health"},
								Responses: &spec.Responses{
									ResponsesProps: spec.ResponsesProps{
										StatusCodeResponses: map[int]spec.Response{
											200: {
												ResponseProps: spec.ResponseProps{
													Description: "Service is healthy",
													Schema: &spec.Schema{
														SchemaProps: spec.SchemaProps{
															Type: []string{"object"},
															Properties: map[string]spec.Schema{
																"status": {
																	SchemaProps: spec.SchemaProps{
																		Type: []string{"string"},
																	},
																},
																"timestamp": {
																	SchemaProps: spec.SchemaProps{
																		Type:   []string{"string"},
																		Format: "date-time",
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
					},
				},
			},
		}
	}

	// Save documentation
	if err := p.saveSwaggerJSON(); err != nil {
		return fmt.Errorf("failed to save swagger json: %w", err)
	}

	// Register to internal registry
	if err := p.registerSwaggerToSwag(); err != nil {
		return fmt.Errorf("failed to register swagger: %w", err)
	}

	log.Info("Swagger documentation generated successfully")
	return nil
}

// startSwaggerUI starts Swagger UI service
func (p *PlugSwagger) startSwaggerUI() error {
	if !p.config.UI.Enabled {
		return nil
	}

	// Create independent HTTP server for Swagger UI
	swaggerPath := p.config.UI.Path
	if swaggerPath == "" {
		swaggerPath = "/swagger"
	}

	// Create HTTP multiplexer
	mux := http.NewServeMux()

	// Register JSON documentation route
	mux.HandleFunc(swaggerPath+"/doc.json", p.serveSwaggerJSON)
	mux.HandleFunc("/swagger.json", p.serveSwaggerJSON)
	mux.HandleFunc("/api-docs", p.serveSwaggerJSON)

	// Register Swagger UI static file service
	// Note: Simplified handling here, actual Swagger UI static files need to be embedded
	mux.HandleFunc(swaggerPath+"/", p.serveSwaggerUI)

	// Get port configuration
	port := p.config.UI.Port
	if port == 0 {
		port = 8081 // Default port
	}

	// Create HTTP server
	p.uiServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server in the background
	go func() {
		log.Infof("Starting Swagger UI server on http://localhost:%d%s", port, swaggerPath)
		if err := p.uiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("Failed to start Swagger UI server: %v", err)
		}
	}()

	return nil
}

// serveSwaggerJSON provides Swagger JSON documentation
func (p *PlugSwagger) serveSwaggerJSON(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")

	if err := json.NewEncoder(w).Encode(p.swagger); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// serveSwaggerUI provides Swagger UI page
func (p *PlugSwagger) serveSwaggerUI(w http.ResponseWriter, r *http.Request) {
	// Simple Swagger UI HTML page
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>` + p.config.Info.Title + ` - Swagger UI</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; }
        #swagger-ui { padding: 20px; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
                url: "` + p.config.UI.Path + `/doc.json",
                dom_id: '#swagger-ui',
                deepLinking: ` + fmt.Sprintf("%v", p.config.UI.DeepLinking) + `,
                docExpansion: "` + p.config.UI.DocExpansion + `",
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                layout: "StandaloneLayout"
            });
        };
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// startFileWatcher starts file monitoring
func (p *PlugSwagger) startFileWatcher() {
	p.watcher = &FileWatcher{
		paths: p.config.Gen.ScanDirs,
		callback: func() {
			if err := p.generateSwaggerDocs(); err != nil {
				log.Errorf("Failed to regenerate docs: %v", err)
			}
		},
		stop: make(chan struct{}),
	}

	go p.watcher.watch()
}

// watch monitors file changes
func (w *FileWatcher) watch() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Simple periodic regeneration, actual file change monitoring can use fsnotify
			w.callback()
		case <-w.stop:
			return
		}
	}
}

// saveSwaggerJSON saves Swagger JSON documentation
func (p *PlugSwagger) saveSwaggerJSON() error {
	if p.config.Gen.OutputPath == "" {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(p.config.Gen.OutputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Serialize Swagger specification
	data, err := json.MarshalIndent(p.swagger, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal swagger: %w", err)
	}

	// Write to file
	if err := ioutil.WriteFile(p.config.Gen.OutputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write swagger file: %w", err)
	}

	log.Infof("Swagger JSON saved to %s", p.config.Gen.OutputPath)
	return nil
}

// registerSwaggerToSwag registers Swagger documentation to internal registry
func (p *PlugSwagger) registerSwaggerToSwag() error {
	// Simplified handling here, actual project can integrate swaggo/swag package
	// Or use other methods to manage Swagger documentation

	log.Infof("Swagger specification registered successfully")
	return nil
}

// Health health check
func (p *PlugSwagger) Health() error {
	return nil
}

// swagDoc Swagger documentation implementation
type swagDoc struct {
	swagger *spec.Swagger
}

func (s *swagDoc) ReadDoc() string {
	data, _ := json.Marshal(s.swagger)
	return string(data)
}

// GetSwagger gets Swagger specification
func (p *PlugSwagger) GetSwagger() *spec.Swagger {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.swagger
}

// UpdateSwagger updates Swagger specification
func (p *PlugSwagger) UpdateSwagger(swagger *spec.Swagger) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.swagger = swagger
}

// AddPath adds path
func (p *PlugSwagger) AddPath(path string, item spec.PathItem) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.swagger.Paths == nil {
		p.swagger.Paths = &spec.Paths{Paths: make(map[string]spec.PathItem)}
	}
	p.swagger.Paths.Paths[path] = item
}

// AddDefinition adds definition
func (p *PlugSwagger) AddDefinition(name string, schema spec.Schema) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.swagger.Definitions == nil {
		p.swagger.Definitions = make(spec.Definitions)
	}
	p.swagger.Definitions[name] = schema
}

// CheckHealth health check
func (p *PlugSwagger) CheckHealth() error {
	if !p.config.Enabled {
		return nil
	}

	// Check if documentation is generated
	if p.swagger == nil || p.swagger.Paths == nil {
		return fmt.Errorf("swagger documentation not generated")
	}

	return nil
}
