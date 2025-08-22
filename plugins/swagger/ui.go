package swagger

import (
	"fmt"
	"net/http"
	
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/swagger/ui"
)

// SwaggerUIServer Swagger UI server
type SwaggerUIServer struct {
	port    int
	path    string
	specURL string
	title   string
	server  *http.Server
}

// NewSwaggerUIServer creates a Swagger UI server
func NewSwaggerUIServer(port int, path, specURL, title string) *SwaggerUIServer {
	return &SwaggerUIServer{
		port:    port,
		path:    path,
		specURL: specURL,
		title:   title,
	}
}

// Start starts the UI server
func (s *SwaggerUIServer) Start() error {
	mux := http.NewServeMux()
	
	// Register Swagger UI route
	mux.HandleFunc(s.path, s.serveSwaggerUI)
	
	// Register Swagger JSON route
	mux.HandleFunc(s.path+".json", s.serveSwaggerJSON)
	
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}
	
	log.Infof("Starting Swagger UI server on http://localhost:%d%s", s.port, s.path)
	
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("Swagger UI server error: %v", err)
		}
	}()
	
	return nil
}

// Stop stops the UI server
func (s *SwaggerUIServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// serveSwaggerUI serves Swagger UI page
func (s *SwaggerUIServer) serveSwaggerUI(w http.ResponseWriter, r *http.Request) {
	config := ui.SwaggerUIConfig{
		Title:           s.title,
		SpecURL:         s.specURL,
		AutoRefresh:     false,
		RefreshInterval: 5000,
	}
	handler := ui.Handler(config)
	handler(w, r)
}

// serveSwaggerJSON serves Swagger JSON documentation
func (s *SwaggerUIServer) serveSwaggerJSON(w http.ResponseWriter, r *http.Request) {
	// This should return actual Swagger JSON
	// Temporarily return an example
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"swagger":"2.0","info":{"title":"API","version":"1.0.0"}}`))
}

// GenerateSwaggerUIHTML generates Swagger UI HTML (kept for backward compatibility)
// Deprecated: Use ui.Handler instead
func GenerateSwaggerUIHTML(specURL, title string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    <link rel="stylesheet" type="text/css" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.9.0/swagger-ui.css">
    <style>
        html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin: 0; background: #fafafa; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.9.0/swagger-ui-bundle.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.9.0/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: "%s",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
            window.ui = ui;
        };
    </script>
</body>
</html>`, title, specURL)
}
