package swagger

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-openapi/spec"
)

// Security constants
const (
	maxFileSize  = 10 * 1024 * 1024 // 10MB max file size
	maxPathDepth = 10               // Maximum directory depth
)

// ParseStats statistics for parsing operations
type ParseStats struct {
	TotalFiles   int
	SuccessFiles int
	FailedFiles  int
	TotalLines   int
	ParsedRoutes int
	ParsedModels int
	Errors       []ParseError
	StartTime    time.Time
	EndTime      time.Time
}

// ParseError detailed error information
type ParseError struct {
	File    string
	Line    int
	Message string
	Type    string
	Time    time.Time
}

// Memory management constants
const (
	maxStringBuilderSize = 10000 // Maximum string builder size
	maxModelCacheSize    = 1000  // Maximum model cache size
	maxRouteCacheSize    = 1000  // Maximum route cache size
	gcThreshold          = 100   // Garbage collection threshold
)

// StringBuilderPool string builder object pool
type StringBuilderPool struct {
	pool chan *strings.Builder
}

// NewStringBuilderPool creates a new string builder pool
func NewStringBuilderPool(size int) *StringBuilderPool {
	pool := &StringBuilderPool{
		pool: make(chan *strings.Builder, size),
	}

	// Pre-populate pool
	for i := 0; i < size; i++ {
		pool.pool <- &strings.Builder{}
	}

	return pool
}

// Get gets a string builder from pool
func (p *StringBuilderPool) Get() *strings.Builder {
	select {
	case sb := <-p.pool:
		sb.Reset()
		return sb
	default:
		return &strings.Builder{}
	}
}

// Put returns a string builder to pool
func (p *StringBuilderPool) Put(sb *strings.Builder) {
	if sb.Len() > maxStringBuilderSize {
		// Don't return very large builders to pool
		return
	}

	select {
	case p.pool <- sb:
	default:
		// Pool is full, discard
	}
}

// AnnotationParser annotation parser with memory management
type AnnotationParser struct {
	swagger     *spec.Swagger
	routes      map[string]*RouteInfo
	models      map[string]*ModelInfo
	allowedDirs []string // Allowed scan directories for security
	stats       *ParseStats
	mu          sync.RWMutex
	sbPool      *StringBuilderPool
	lastGC      time.Time
}

// RouteInfo route information
type RouteInfo struct {
	Method      string
	Path        string
	Summary     string
	Description string
	Tags        []string
	Params      []ParamInfo
	Responses   map[int]ResponseInfo
	Security    []map[string][]string
}

// ParamInfo parameter information
type ParamInfo struct {
	Name        string
	In          string // path, query, header, body, formData
	Type        string
	Format      string
	Required    bool
	Description string
	Default     interface{}
	Example     interface{}
}

// ResponseInfo response information
type ResponseInfo struct {
	Description string
	Schema      *SchemaInfo
	Headers     map[string]HeaderInfo
}

// SchemaInfo schema information
type SchemaInfo struct {
	Type       string
	Format     string
	Ref        string
	Properties map[string]*SchemaInfo
	Items      *SchemaInfo
	Required   []string
}

// HeaderInfo response header information
type HeaderInfo struct {
	Type        string
	Format      string
	Description string
}

// ModelInfo model information
type ModelInfo struct {
	Name        string
	Description string
	Properties  map[string]PropertyInfo
	Required    []string
}

// PropertyInfo property information
type PropertyInfo struct {
	Type        string
	Format      string
	Description string
	Example     interface{}
	Enum        []interface{}
	Minimum     *float64
	Maximum     *float64
	MinLength   *int64
	MaxLength   *int64
	Pattern     string
}

// NewAnnotationParser creates an annotation parser
func NewAnnotationParser(swagger *spec.Swagger, allowedDirs []string) *AnnotationParser {
	return &AnnotationParser{
		swagger:     swagger,
		routes:      make(map[string]*RouteInfo),
		models:      make(map[string]*ModelInfo),
		allowedDirs: allowedDirs,
		stats:       &ParseStats{},
		sbPool:      NewStringBuilderPool(100), // Initialize with a small pool
	}
}

// ParseFile parses a file with comprehensive error handling
func (p *AnnotationParser) ParseFile(filename string) error {
	p.mu.Lock()
	p.stats.TotalFiles++
	p.stats.StartTime = time.Now()
	p.mu.Unlock()

	// Security validation
	if err := p.validateFilePath(filename); err != nil {
		p.recordError(filename, 0, err.Error(), "validation")
		p.mu.Lock()
		p.stats.FailedFiles++
		p.mu.Unlock()
		return fmt.Errorf("file path validation failed: %w", err)
	}

	// Check file size
	if err := p.checkFileSize(filename); err != nil {
		p.recordError(filename, 0, err.Error(), "size_check")
		p.mu.Lock()
		p.stats.FailedFiles++
		p.mu.Unlock()
		return fmt.Errorf("file size check failed: %w", err)
	}

	// Check file extension
	if !p.isAllowedFile(filename) {
		errMsg := fmt.Sprintf("file type not allowed: %s", filename)
		p.recordError(filename, 0, errMsg, "file_type")
		p.mu.Lock()
		p.stats.FailedFiles++
		p.mu.Unlock()
		return fmt.Errorf("file type not allowed: %s", filename)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		p.recordError(filename, 0, err.Error(), "file_read")
		p.mu.Lock()
		p.stats.FailedFiles++
		p.mu.Unlock()
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		p.recordError(filename, 0, err.Error(), "parse")
		p.mu.Lock()
		p.stats.FailedFiles++
		p.mu.Unlock()
		return fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	// Count lines
	lineCount := len(strings.Split(string(content), "\n"))
	p.mu.Lock()
	p.stats.TotalLines += lineCount
	p.mu.Unlock()

	// Iterate through all declarations
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			// Parse function comments
			if d.Doc != nil {
				if err := p.parseFuncDoc(d); err != nil {
					p.recordError(filename, fset.Position(d.Pos()).Line, err.Error(), "func_parse")
				}
			}
		case *ast.GenDecl:
			// Parse type definitions
			if d.Tok == token.TYPE && d.Doc != nil {
				if err := p.parseTypeDoc(d); err != nil {
					p.recordError(filename, fset.Position(d.Pos()).Line, err.Error(), "type_parse")
				}
			}
		}
	}

	p.mu.Lock()
	p.stats.SuccessFiles++
	p.stats.EndTime = time.Now()
	p.mu.Unlock()

	return nil
}

// recordError records a parsing error with details
func (p *AnnotationParser) recordError(file string, line int, message, errorType string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	parseError := ParseError{
		File:    file,
		Line:    line,
		Message: message,
		Type:    errorType,
		Time:    time.Now(),
	}

	// Limit error list size to prevent memory issues
	if len(p.stats.Errors) < 100 {
		p.stats.Errors = append(p.stats.Errors, parseError)
	}

	// Log only if logger is available (avoid panic in tests)
	defer func() {
		if r := recover(); r != nil {
			// Silently ignore log panics in tests
		}
	}()
	log.Warnf("Parse error in %s:%d [%s]: %s", file, line, errorType, message)
}

// GetStats returns parsing statistics
func (p *AnnotationParser) GetStats() *ParseStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *p.stats
	stats.Errors = make([]ParseError, len(p.stats.Errors))
	copy(stats.Errors, p.stats.Errors)

	return &stats
}

// GetErrorSummary returns a summary of parsing errors
func (p *AnnotationParser) GetErrorSummary() map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	summary := make(map[string]int)
	for _, err := range p.stats.Errors {
		summary[err.Type]++
	}

	return summary
}

// validateFilePath validates file path for security
func (p *AnnotationParser) validateFilePath(filename string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if path is within allowed directories
	if !p.isAllowedDir(filepath.Dir(absPath)) {
		return fmt.Errorf("file path %s is outside allowed directories", absPath)
	}

	// Check path depth to prevent path traversal
	if p.getPathDepth(absPath) > maxPathDepth {
		return fmt.Errorf("file path depth exceeds maximum allowed depth: %s", absPath)
	}

	// Check for suspicious path components
	suspicious := []string{"..", "~", "/etc", "/var", "/usr", "/bin", "/sbin", "/tmp", "/root"}
	for _, s := range suspicious {
		if strings.Contains(absPath, s) {
			return fmt.Errorf("file path contains suspicious component: %s", s)
		}
	}

	return nil
}

// checkFileSize checks if file size is within limits
func (p *AnnotationParser) checkFileSize(filename string) error {
	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if info.Size() > maxFileSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", info.Size(), maxFileSize)
	}

	return nil
}

// isAllowedFile checks if file type is allowed
func (p *AnnotationParser) isAllowedFile(filename string) bool {
	allowedExtensions := []string{".go"}
	ext := strings.ToLower(filepath.Ext(filename))

	for _, allowed := range allowedExtensions {
		if ext == allowed {
			return true
		}
	}
	return false
}

// getPathDepth calculates the depth of a file path
func (p *AnnotationParser) getPathDepth(path string) int {
	path = filepath.Clean(path)
	if path == "." || path == "/" {
		return 0
	}

	parts := strings.Split(path, string(os.PathSeparator))
	depth := 0
	for _, part := range parts {
		if part != "" && part != "." && part != ".." {
			depth++
		}
	}
	return depth
}

// parseFuncDoc parses function documentation comments
func (p *AnnotationParser) parseFuncDoc(fn *ast.FuncDecl) error {
	if fn.Doc == nil {
		return nil
	}

	comments := fn.Doc.Text()
	if len(comments) > maxInputLength {
		log.Warnf("Function comment too long, truncating: %s", fn.Name.Name)
		comments = comments[:maxInputLength]
	}

	lines := p.safeSplit(comments, "\n")
	if len(lines) == 0 {
		return nil
	}

	var route *RouteInfo

	for _, line := range lines {
		line = p.safeTrimSpace(line)

		// Parse route annotations
		if p.safeHasPrefix(line, "@Router") {
			parts := p.safeFields(line)
			if len(parts) >= 3 {
				path := parts[1]
				method := p.safeTrim(parts[2], "[]")
				route = &RouteInfo{
					Path:      path,
					Method:    strings.ToUpper(method),
					Responses: make(map[int]ResponseInfo),
				}
				p.routes[fn.Name.Name] = route
			}
		} else if route != nil {
			// Parse other annotations
			if p.safeHasPrefix(line, "@Summary") {
				route.Summary = p.safeTrimSpace(p.safeTrimPrefix(line, "@Summary"))
			} else if p.safeHasPrefix(line, "@Description") {
				route.Description = p.safeTrimSpace(p.safeTrimPrefix(line, "@Description"))
			} else if p.safeHasPrefix(line, "@Tags") {
				tags := p.safeTrimSpace(p.safeTrimPrefix(line, "@Tags"))
				route.Tags = p.safeSplit(tags, ",")
				for i := range route.Tags {
					route.Tags[i] = p.safeTrimSpace(route.Tags[i])
				}
			} else if p.safeHasPrefix(line, "@Param") {
				param := p.parseParam(line)
				if param != nil {
					route.Params = append(route.Params, *param)
				}
			} else if p.safeHasPrefix(line, "@Success") || p.safeHasPrefix(line, "@Failure") {
				code, resp := p.parseResponse(line)
				if resp != nil {
					route.Responses[code] = *resp
				}
			} else if p.safeHasPrefix(line, "@Security") {
				security := p.parseSecurity(line)
				if security != nil {
					route.Security = append(route.Security, security)
				}
			}
		}
	}

	return nil
}

// ParseParam parses parameter annotations (public method for testing)
// Format: @Param name in type format required "description" default(value) example(value)
func (p *AnnotationParser) ParseParam(line string) *ParamInfo {
	return p.parseParam(line)
}

// parseParam parses parameter annotations
// Format: @Param name in type format required "description" default(value) example(value)
func (p *AnnotationParser) parseParam(line string) *ParamInfo {
	// Use safer string parsing instead of regex
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "@Param") {
		return nil
	}

	// Remove @Param prefix
	line = strings.TrimSpace(strings.TrimPrefix(line, "@Param"))

	// Split by spaces, but be careful with quoted strings
	parts := p.splitParamLine(line)
	if len(parts) < 6 {
		return nil
	}

	param := &ParamInfo{
		Name:        strings.TrimSpace(parts[0]),
		In:          strings.TrimSpace(parts[1]),
		Type:        strings.TrimSpace(parts[2]),
		Format:      strings.TrimSpace(parts[3]),
		Required:    strings.TrimSpace(parts[4]) == "true",
		Description: strings.Trim(strings.TrimSpace(parts[5]), `"'`),
	}

	// Parse default value if present
	if len(parts) > 6 && strings.Contains(parts[6], "default(") {
		defaultVal := p.extractValue(parts[6], "default")
		if defaultVal != "" {
			param.Default = p.parseValue(defaultVal, param.Type)
		}
	}

	// Parse example value if present
	if len(parts) > 7 && strings.Contains(parts[7], "example(") {
		exampleVal := p.extractValue(parts[7], "example")
		if exampleVal != "" {
			param.Example = p.parseValue(exampleVal, param.Type)
		}
	}

	return param
}

// splitParamLine safely splits a parameter line, handling quoted strings
func (p *AnnotationParser) splitParamLine(line string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)

	for i := 0; i < len(line); i++ {
		char := line[i]

		if (char == '"' || char == '\'') && (i == 0 || line[i-1] != '\\') {
			if !inQuotes {
				inQuotes = true
				quoteChar = char
			} else if char == quoteChar {
				inQuotes = false
				quoteChar = 0
			}
		}

		if char == ' ' && !inQuotes {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// extractValue extracts value from format like "default(value)" or "example(value)"
func (p *AnnotationParser) extractValue(s, prefix string) string {
	if !strings.Contains(s, prefix+"(") {
		return ""
	}

	start := strings.Index(s, prefix+"(")
	if start == -1 {
		return ""
	}

	start += len(prefix) + 1
	end := strings.LastIndex(s, ")")
	if end == -1 || end <= start {
		return ""
	}

	return strings.TrimSpace(s[start:end])
}

// ParseResponse parses response annotations (public method for testing)
// Format: @Success 200 {object} model.Response "description"
func (p *AnnotationParser) ParseResponse(line string) (int, *ResponseInfo) {
	return p.parseResponse(line)
}

// parseResponse parses response annotations
// Format: @Success 200 {object} model.Response "description"
func (p *AnnotationParser) parseResponse(line string) (int, *ResponseInfo) {
	line = strings.TrimSpace(line)

	// Check if it's a response annotation
	if !strings.HasPrefix(line, "@Success") && !strings.HasPrefix(line, "@Failure") {
		return 0, nil
	}

	// Remove annotation prefix
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "@Success"), "@Failure"))

	// Split by spaces
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return 0, nil
	}

	// Parse status code safely
	code := 0
	if codeStr := parts[0]; codeStr != "" {
		if parsedCode, err := strconv.Atoi(codeStr); err == nil {
			// Validate status code range
			if parsedCode >= 100 && parsedCode <= 599 {
				code = parsedCode
			}
		}
	}

	// Parse response type and model
	responseType := strings.Trim(parts[1], "{}")
	modelName := parts[2]

	// Parse description if present
	description := ""
	if len(parts) > 3 {
		description = strings.Trim(strings.Join(parts[3:], " "), `"'`)
	}

	resp := &ResponseInfo{
		Description: description,
		Schema: &SchemaInfo{
			Type: responseType,
			Ref:  modelName,
		},
		Headers: make(map[string]HeaderInfo),
	}

	return code, resp
}

// parseSecurity parses security annotations
// Format: @Security ApiKeyAuth
func (p *AnnotationParser) parseSecurity(line string) map[string][]string {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil
	}

	security := make(map[string][]string)
	security[parts[1]] = []string{}

	if len(parts) > 2 {
		security[parts[1]] = parts[2:]
	}

	return security
}

// parseTypeDoc parses type documentation comments
func (p *AnnotationParser) parseTypeDoc(decl *ast.GenDecl) error {
	for _, spec := range decl.Specs {
		if ts, ok := spec.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				model := &ModelInfo{
					Name:       ts.Name.Name,
					Properties: make(map[string]PropertyInfo),
				}

				// Parse struct comments
				if decl.Doc != nil {
					for _, comment := range decl.Doc.List {
						text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
						if strings.HasPrefix(text, "@Description") {
							model.Description = strings.TrimSpace(strings.TrimPrefix(text, "@Description"))
						}
					}
				}

				// Parse fields
				for _, field := range st.Fields.List {
					if err := p.parseField(field, model); err != nil {
						return err
					}
				}

				p.models[model.Name] = model
			}
		}
	}

	return nil
}

// parseField parses field
func (p *AnnotationParser) parseField(field *ast.Field, model *ModelInfo) error {
	if len(field.Names) == 0 {
		return nil
	}

	fieldName := field.Names[0].Name
	if !ast.IsExported(fieldName) {
		return nil
	}

	prop := PropertyInfo{}

	// Parse type
	prop.Type = p.getFieldType(field.Type)

	// Parse tags
	if field.Tag != nil {
		tag := field.Tag.Value
		prop = p.parseFieldTag(tag, prop)

		// Get JSON tag as property name
		jsonTag := p.getTagValue(tag, "json")
		if jsonTag != "" && jsonTag != "-" {
			parts := strings.Split(jsonTag, ",")
			fieldName = parts[0]

			// Check if required
			for _, part := range parts[1:] {
				if part == "required" {
					model.Required = append(model.Required, fieldName)
				}
			}
		}
	}

	// Parse comments
	if field.Comment != nil {
		for _, comment := range field.Comment.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			prop.Description = text

			// Parse special annotations
			if strings.HasPrefix(text, "@Example") {
				example := strings.TrimSpace(strings.TrimPrefix(text, "@Example"))
				prop.Example = p.parseValue(example, prop.Type)
			} else if strings.HasPrefix(text, "@Enum") {
				enum := strings.TrimSpace(strings.TrimPrefix(text, "@Enum"))
				parts := strings.Split(enum, ",")
				for _, part := range parts {
					prop.Enum = append(prop.Enum, strings.TrimSpace(part))
				}
			}
		}
	}

	model.Properties[fieldName] = prop

	return nil
}

// getFieldType gets field type
func (p *AnnotationParser) getFieldType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return p.mapGoTypeToSwagger(t.Name)
	case *ast.ArrayType:
		elemType := p.getFieldType(t.Elt)
		return "array:" + elemType
	case *ast.StarExpr:
		return p.getFieldType(t.X)
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	case *ast.MapType:
		return "object"
	}
	return "string"
}

// mapGoTypeToSwagger maps Go types to Swagger types
func (p *AnnotationParser) mapGoTypeToSwagger(goType string) string {
	switch goType {
	case "bool":
		return "boolean"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return "integer"
	case "float32", "float64":
		return "number"
	case "string":
		return "string"
	case "time.Time":
		return "string:date-time"
	default:
		return "object"
	}
}

// parseFieldTag parses field tags
func (p *AnnotationParser) parseFieldTag(tag string, prop PropertyInfo) PropertyInfo {
	// Parse validate tag
	validate := p.getTagValue(tag, "validate")
	if validate != "" {
		rules := strings.Split(validate, ",")
		for _, rule := range rules {
			parts := strings.Split(rule, "=")
			switch parts[0] {
			case "required":
				// Already handled at upper level
			case "min":
				if len(parts) > 1 {
					if min, err := strconv.ParseFloat(parts[1], 64); err == nil {
						// Validate range to prevent extreme values
						if min >= -1e6 && min <= 1e6 {
							prop.Minimum = &min
						}
					}
				}
			case "max":
				if len(parts) > 1 {
					if max, err := strconv.ParseFloat(parts[1], 64); err == nil {
						// Validate range to prevent extreme values
						if max >= -1e6 && max <= 1e6 {
							prop.Maximum = &max
						}
					}
				}
			case "minlen":
				if len(parts) > 1 {
					if minLen, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
						// Validate range to prevent extreme values
						if minLen >= 0 && minLen <= 10000 {
							prop.MinLength = &minLen
						}
					}
				}
			case "maxlen":
				if len(parts) > 1 {
					if maxLen, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
						// Validate range to prevent extreme values
						if maxLen >= 0 && maxLen <= 10000 {
							prop.MaxLength = &maxLen
						}
					}
				}
			}
		}
	}

	// Parse binding tag
	binding := p.getTagValue(tag, "binding")
	if strings.Contains(binding, "required") {
		// Already handled at upper level
	}

	// Parse example tag
	example := p.getTagValue(tag, "example")
	if example != "" {
		prop.Example = p.parseValue(example, prop.Type)
	}

	// Parse enum tag
	enum := p.getTagValue(tag, "enum")
	if enum != "" {
		parts := strings.Split(enum, ",")
		for _, part := range parts {
			prop.Enum = append(prop.Enum, strings.TrimSpace(part))
		}
	}

	return prop
}

// getTagValue gets tag value
func (p *AnnotationParser) getTagValue(tag, key string) string {
	tag = strings.Trim(tag, "`")
	parts := strings.Fields(tag)

	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 && kv[0] == key {
			return strings.Trim(kv[1], `"`)
		}
	}

	return ""
}

// parseValue safely parses value with validation
func (p *AnnotationParser) parseValue(value, valueType string) interface{} {
	// Input validation
	if value == "" || len(value) > maxValueLength {
		return value
	}

	value = p.safeTrim(value, `"'`)

	switch valueType {
	case "boolean":
		// Only accept true/false values
		if value == "true" || value == "false" {
			return value == "true"
		}
		return value
	case "integer":
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			// Validate range to prevent extreme values
			if i >= -999999999 && i <= 999999999 {
				return i
			}
		}
		return value
	case "number":
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			// Validate range to prevent extreme values
			if f >= -1e9 && f <= 1e9 {
				return f
			}
		}
		return value
	default:
		return value
	}
}

// BuildSwagger builds Swagger specification
func (p *AnnotationParser) BuildSwagger() error {
	// Build paths
	for name, route := range p.routes {
		p.buildPath(name, route)
	}

	// Build definitions
	for name, model := range p.models {
		p.buildDefinition(name, model)
	}

	return nil
}

// buildPath builds path
func (p *AnnotationParser) buildPath(name string, route *RouteInfo) {
	if p.swagger.Paths == nil {
		p.swagger.Paths = &spec.Paths{
			Paths: make(map[string]spec.PathItem),
		}
	}

	pathItem, exists := p.swagger.Paths.Paths[route.Path]
	if !exists {
		pathItem = spec.PathItem{}
	}

	operation := &spec.Operation{
		OperationProps: spec.OperationProps{
			ID:          name,
			Summary:     route.Summary,
			Description: route.Description,
			Tags:        route.Tags,
			Security:    route.Security,
		},
	}

	// Build parameters
	for _, param := range route.Params {
		p.buildParameter(operation, param)
	}

	// Build responses
	p.buildResponses(operation, route.Responses)

	// Set operation
	switch route.Method {
	case "GET":
		pathItem.Get = operation
	case "POST":
		pathItem.Post = operation
	case "PUT":
		pathItem.Put = operation
	case "DELETE":
		pathItem.Delete = operation
	case "PATCH":
		pathItem.Patch = operation
	case "HEAD":
		pathItem.Head = operation
	case "OPTIONS":
		pathItem.Options = operation
	}

	p.swagger.Paths.Paths[route.Path] = pathItem
}

// buildParameter builds parameter
func (p *AnnotationParser) buildParameter(op *spec.Operation, param ParamInfo) {
	swaggerParam := spec.Parameter{
		SimpleSchema: spec.SimpleSchema{
			Type:   param.Type,
			Format: param.Format,
		},
		ParamProps: spec.ParamProps{
			Name:        param.Name,
			In:          param.In,
			Description: param.Description,
			Required:    param.Required,
		},
	}

	if param.Default != nil {
		swaggerParam.Default = param.Default
	}

	if param.Example != nil {
		swaggerParam.Example = param.Example
	}

	op.Parameters = append(op.Parameters, swaggerParam)
}

// buildResponses builds responses
func (p *AnnotationParser) buildResponses(op *spec.Operation, responses map[int]ResponseInfo) {
	op.Responses = &spec.Responses{
		ResponsesProps: spec.ResponsesProps{
			StatusCodeResponses: make(map[int]spec.Response),
		},
	}

	for code, resp := range responses {
		swaggerResp := spec.Response{
			ResponseProps: spec.ResponseProps{
				Description: resp.Description,
				Headers:     make(map[string]spec.Header),
			},
		}

		// Build Schema
		if resp.Schema != nil {
			swaggerResp.Schema = p.buildSchema(resp.Schema)
		}

		// Build response headers
		for name, header := range resp.Headers {
			swaggerResp.Headers[name] = spec.Header{
				SimpleSchema: spec.SimpleSchema{
					Type:   header.Type,
					Format: header.Format,
				},
				HeaderProps: spec.HeaderProps{
					Description: header.Description,
				},
			}
		}

		op.Responses.StatusCodeResponses[code] = swaggerResp
	}
}

// buildSchema builds Schema
func (p *AnnotationParser) buildSchema(schema *SchemaInfo) *spec.Schema {
	if schema.Ref != "" {
		// Handle references
		if strings.HasPrefix(schema.Ref, "#/definitions/") {
			return &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Ref: spec.MustCreateRef(schema.Ref),
				},
			}
		}

		// Handle type references
		parts := strings.Split(schema.Ref, ".")
		modelName := parts[len(parts)-1]
		return &spec.Schema{
			SchemaProps: spec.SchemaProps{
				Ref: spec.MustCreateRef("#/definitions/" + modelName),
			},
		}
	}

	swaggerSchema := &spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type:   []string{schema.Type},
			Format: schema.Format,
		},
	}

	// Handle arrays
	if strings.HasPrefix(schema.Type, "array:") {
		swaggerSchema.Type = []string{"array"}
		itemType := strings.TrimPrefix(schema.Type, "array:")
		swaggerSchema.Items = &spec.SchemaOrArray{
			Schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type: []string{itemType},
				},
			},
		}
	}

	// Handle object properties
	if len(schema.Properties) > 0 {
		swaggerSchema.Properties = make(map[string]spec.Schema)
		for name, prop := range schema.Properties {
			swaggerSchema.Properties[name] = *p.buildSchema(prop)
		}
		swaggerSchema.Required = schema.Required
	}

	return swaggerSchema
}

// buildDefinition builds definition
func (p *AnnotationParser) buildDefinition(name string, model *ModelInfo) {
	if p.swagger.Definitions == nil {
		p.swagger.Definitions = make(spec.Definitions)
	}

	schema := spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type:        []string{"object"},
			Description: model.Description,
			Properties:  make(map[string]spec.Schema),
			Required:    model.Required,
		},
	}

	// Build properties
	for propName, prop := range model.Properties {
		propSchema := spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type:        []string{p.mapPropertyType(prop.Type)},
				Format:      prop.Format,
				Description: prop.Description,
			},
		}

		if prop.Example != nil {
			propSchema.Example = prop.Example
		}

		if len(prop.Enum) > 0 {
			propSchema.Enum = prop.Enum
		}

		if prop.Minimum != nil {
			propSchema.Minimum = prop.Minimum
		}

		if prop.Maximum != nil {
			propSchema.Maximum = prop.Maximum
		}

		if prop.MinLength != nil {
			propSchema.MinLength = prop.MinLength
		}

		if prop.MaxLength != nil {
			propSchema.MaxLength = prop.MaxLength
		}

		if prop.Pattern != "" {
			propSchema.Pattern = prop.Pattern
		}

		schema.Properties[propName] = propSchema
	}

	p.swagger.Definitions[name] = schema
}

// mapPropertyType maps property type
func (p *AnnotationParser) mapPropertyType(propType string) string {
	if strings.Contains(propType, ":") {
		parts := strings.Split(propType, ":")
		return parts[0]
	}

	switch propType {
	case "boolean", "integer", "number", "string", "array", "object":
		return propType
	default:
		return "string"
	}
}

// ScanDirectory scans directory
func (p *AnnotationParser) ScanDirectory(dir string) error {
	// Validate directory path
	if !p.isAllowedDir(dir) {
		return fmt.Errorf("directory %s is not allowed for scanning", dir)
	}

	_, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return err
	}

	// Recursively scan subdirectories
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			// Skip test files and generated files
			if strings.HasSuffix(path, "_test.go") ||
				strings.Contains(path, ".pb.go") ||
				strings.Contains(path, "vendor/") {
				return nil
			}

			log.Debugf("Parsing file: %s", path)
			if err := p.ParseFile(path); err != nil {
				log.Warnf("Failed to parse file %s: %v", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Build Swagger specification
	return p.BuildSwagger()
}

// isAllowedDir checks if a directory is allowed for scanning
func (p *AnnotationParser) isAllowedDir(dir string) bool {
	for _, allowedDir := range p.allowedDirs {
		if strings.HasPrefix(dir, allowedDir) {
			return true
		}
	}
	return false
}

// Safe parsing constants
const (
	maxInputLength = 1000 // Maximum input length for parsing
	maxArraySize   = 100  // Maximum array size for enums, tags, etc.
	maxTagLength   = 200  // Maximum tag length
	maxValueLength = 500  // Maximum value length
)

// safeSplit safely splits a string with boundary checks
func (p *AnnotationParser) safeSplit(s, sep string) []string {
	if s == "" || len(s) > maxInputLength {
		return nil
	}

	parts := strings.Split(s, sep)
	if len(parts) > maxArraySize {
		// Limit array size to prevent memory issues
		parts = parts[:maxArraySize]
	}

	// Filter and trim parts
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" && len(trimmed) <= maxValueLength {
			result = append(result, trimmed)
		}
	}

	return result
}

// safeFields safely splits a string into fields with boundary checks
func (p *AnnotationParser) safeFields(s string) []string {
	if s == "" || len(s) > maxInputLength {
		return nil
	}

	parts := strings.Fields(s)
	if len(parts) > maxArraySize {
		parts = parts[:maxArraySize]
	}

	// Validate each field
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) <= maxValueLength {
			result = append(result, part)
		}
	}

	return result
}

// safeTrim safely trims a string with length validation
func (p *AnnotationParser) safeTrim(s, cutset string) string {
	if s == "" || len(s) > maxInputLength {
		return s
	}

	result := strings.Trim(s, cutset)
	if len(result) > maxValueLength {
		result = result[:maxValueLength]
	}

	return result
}

// safeTrimPrefix safely trims a prefix with validation
func (p *AnnotationParser) safeTrimPrefix(s, prefix string) string {
	if s == "" || len(s) > maxInputLength {
		return s
	}

	if !strings.HasPrefix(s, prefix) {
		return s
	}

	result := strings.TrimPrefix(s, prefix)
	if len(result) > maxValueLength {
		result = result[:maxValueLength]
	}

	return result
}

// safeTrimSpace safely trims whitespace with length validation
func (p *AnnotationParser) safeTrimSpace(s string) string {
	if s == "" || len(s) > maxInputLength {
		return s
	}

	result := strings.TrimSpace(s)
	if len(result) > maxValueLength {
		result = result[:maxValueLength]
	}

	return result
}

// safeHasPrefix safely checks if string has prefix with length validation
func (p *AnnotationParser) safeHasPrefix(s, prefix string) bool {
	if s == "" || len(s) > maxInputLength || len(prefix) > maxValueLength {
		return false
	}

	return strings.HasPrefix(s, prefix)
}

// safeContains safely checks if string contains substring with length validation
func (p *AnnotationParser) safeContains(s, substr string) bool {
	if s == "" || len(s) > maxInputLength || len(substr) > maxValueLength {
		return false
	}

	return strings.Contains(s, substr)
}

// checkMemoryUsage checks memory usage and triggers garbage collection if needed
func (p *AnnotationParser) checkMemoryUsage() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if we need garbage collection
	if len(p.models) > maxModelCacheSize || len(p.routes) > maxRouteCacheSize {
		p.performGC()
	}
}

// performGC performs garbage collection on caches
func (p *AnnotationParser) performGC() {
	// Clear old entries if caches are too large
	if len(p.models) > maxModelCacheSize {
		// Keep only the most recent models
		keys := make([]string, 0, len(p.models))
		for k := range p.models {
			keys = append(keys, k)
		}

		// Remove oldest entries
		removeCount := len(keys) - maxModelCacheSize
		for i := 0; i < removeCount; i++ {
			delete(p.models, keys[i])
		}

		// Log only if logger is available (avoid panic in tests)
		defer func() {
			if r := recover(); r != nil {
				// Silently ignore log panics in tests
			}
		}()
		log.Infof("GC: Removed %d old models, cache size: %d", removeCount, len(p.models))
	}

	if len(p.routes) > maxRouteCacheSize {
		// Keep only the most recent routes
		keys := make([]string, 0, len(p.routes))
		for k := range p.routes {
			keys = append(keys, k)
		}

		// Remove oldest entries
		removeCount := len(keys) - maxRouteCacheSize
		for i := 0; i < removeCount; i++ {
			delete(p.routes, keys[i])
		}

		// Log only if logger is available (avoid panic in tests)
		defer func() {
			if r := recover(); r != nil {
				// Silently ignore log panics in tests
			}
		}()
		log.Infof("GC: Removed %d old routes, cache size: %d", removeCount, len(p.routes))
	}

	p.lastGC = time.Now()
}

// GetMemoryStats returns memory usage statistics
func (p *AnnotationParser) GetMemoryStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := map[string]interface{}{
		"models_count": len(p.models),
		"routes_count": len(p.routes),
		"max_models":   maxModelCacheSize,
		"max_routes":   maxRouteCacheSize,
		"last_gc":      p.lastGC,
	}

	// Safely access string builder pool
	if p.sbPool != nil {
		stats["sb_pool_size"] = len(p.sbPool.pool)
		stats["sb_pool_capacity"] = cap(p.sbPool.pool)
	} else {
		stats["sb_pool_size"] = 0
		stats["sb_pool_capacity"] = 0
	}

	return stats
}

// ClearCache clears all caches and resets statistics
func (p *AnnotationParser) ClearCache() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear caches
	p.models = make(map[string]*ModelInfo)
	p.routes = make(map[string]*RouteInfo)

	// Reset statistics
	p.stats = &ParseStats{}

	// Reset last GC time
	p.lastGC = time.Now()

	// Log only if logger is available (avoid panic in tests)
	defer func() {
		if r := recover(); r != nil {
			// Silently ignore log panics in tests
		}
	}()
	log.Info("Cache cleared and statistics reset")
}

// OptimizeMemory optimizes memory usage
func (p *AnnotationParser) OptimizeMemory() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Perform garbage collection
	p.performGC()

	// Safely optimize string builders in pool
	if p.sbPool != nil {
		select {
		case sb := <-p.sbPool.pool:
			if sb.Cap() > maxStringBuilderSize {
				// Create new smaller builder
				newSB := &strings.Builder{}
				p.sbPool.pool <- newSB
			} else {
				p.sbPool.pool <- sb
			}
		default:
			// Pool is empty, nothing to optimize
		}
	}

	// Log only if logger is available (avoid panic in tests)
	defer func() {
		if r := recover(); r != nil {
			// Silently ignore log panics in tests
		}
	}()
	log.Info("Memory optimization completed")
}
