package ui

import (
	_ "embed"
	"html/template"
	"net/http"
)

// SwaggerUIConfig Swagger UI configuration
type SwaggerUIConfig struct {
	Title           string
	SpecURL         string
	AutoRefresh     bool
	RefreshInterval int // milliseconds
}

//go:embed index.html
var indexHTML string

// indexTemplate template
var indexTemplate = template.Must(template.New("index").Parse(indexHTML))

// Handler returns HTTP handler for Swagger UI
func Handler(config SwaggerUIConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := indexTemplate.Execute(w, config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// DefaultHandler returns handler with default configuration
func DefaultHandler(specURL string) http.HandlerFunc {
	return Handler(SwaggerUIConfig{
		Title:           "API Documentation",
		SpecURL:         specURL,
		AutoRefresh:     false,
		RefreshInterval: 5000,
	})
}
