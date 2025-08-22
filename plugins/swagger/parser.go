package swagger

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-openapi/spec"
)

// AnnotationParser annotation parser
type AnnotationParser struct {
	swagger *spec.Swagger
	routes  map[string]*RouteInfo
	models  map[string]*ModelInfo
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
func NewAnnotationParser(swagger *spec.Swagger) *AnnotationParser {
	return &AnnotationParser{
		swagger: swagger,
		routes:  make(map[string]*RouteInfo),
		models:  make(map[string]*ModelInfo),
	}
}

// ParseFile parses a file
func (p *AnnotationParser) ParseFile(filename string) error {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	// Iterate through all declarations
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			// Parse function comments
			if d.Doc != nil {
				p.parseFuncDoc(d)
			}
		case *ast.GenDecl:
			// Parse type definitions
			if d.Tok == token.TYPE && d.Doc != nil {
				p.parseTypeDoc(d)
			}
		}
	}

	return nil
}

// parseFuncDoc parses function documentation comments
func (p *AnnotationParser) parseFuncDoc(fn *ast.FuncDecl) {
	comments := fn.Doc.Text()
	lines := strings.Split(comments, "\n")

	var route *RouteInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Parse route annotations
		if strings.HasPrefix(line, "@Router") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				path := parts[1]
				method := strings.Trim(parts[2], "[]")
				route = &RouteInfo{
					Path:      path,
					Method:    strings.ToUpper(method),
					Responses: make(map[int]ResponseInfo),
				}
				p.routes[fn.Name.Name] = route
			}
		} else if route != nil {
			// Parse other annotations
			if strings.HasPrefix(line, "@Summary") {
				route.Summary = strings.TrimSpace(strings.TrimPrefix(line, "@Summary"))
			} else if strings.HasPrefix(line, "@Description") {
				route.Description = strings.TrimSpace(strings.TrimPrefix(line, "@Description"))
			} else if strings.HasPrefix(line, "@Tags") {
				tags := strings.TrimSpace(strings.TrimPrefix(line, "@Tags"))
				route.Tags = strings.Split(tags, ",")
				for i := range route.Tags {
					route.Tags[i] = strings.TrimSpace(route.Tags[i])
				}
			} else if strings.HasPrefix(line, "@Param") {
				param := p.parseParam(line)
				if param != nil {
					route.Params = append(route.Params, *param)
				}
			} else if strings.HasPrefix(line, "@Success") || strings.HasPrefix(line, "@Failure") {
				code, resp := p.parseResponse(line)
				if resp != nil {
					route.Responses[code] = *resp
				}
			} else if strings.HasPrefix(line, "@Security") {
				security := p.parseSecurity(line)
				if security != nil {
					route.Security = append(route.Security, security)
				}
			}
		}
	}
}

// parseParam parses parameter annotations
// Format: @Param name in type format required "description" default(value) example(value)
func (p *AnnotationParser) parseParam(line string) *ParamInfo {
	re := regexp.MustCompile(`@Param\s+(\S+)\s+(\S+)\s+(\S+)(?:\s+(\S+))?\s+(\S+)\s+"([^"]*)"(?:\s+default\(([^)]*)\))?(?:\s+example\(([^)]*)\))?`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 7 {
		return nil
	}

	param := &ParamInfo{
		Name:        matches[1],
		In:          matches[2],
		Type:        matches[3],
		Required:    matches[5] == "true",
		Description: matches[6],
	}

	if matches[4] != "" {
		param.Format = matches[4]
	}

	if len(matches) > 7 && matches[7] != "" {
		param.Default = p.parseValue(matches[7], param.Type)
	}

	if len(matches) > 8 && matches[8] != "" {
		param.Example = p.parseValue(matches[8], param.Type)
	}

	return param
}

// parseResponse parses response annotations
// Format: @Success 200 {object} model.Response "description"
func (p *AnnotationParser) parseResponse(line string) (int, *ResponseInfo) {
	re := regexp.MustCompile(`@(Success|Failure)\s+(\d+)\s+\{(\w+)\}\s+(\S+)(?:\s+"([^"]*)")?`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 5 {
		return 0, nil
	}

	code := 0
	fmt.Sscanf(matches[2], "%d", &code)

	resp := &ResponseInfo{
		Description: matches[5],
		Schema: &SchemaInfo{
			Type: matches[3],
			Ref:  matches[4],
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
func (p *AnnotationParser) parseTypeDoc(decl *ast.GenDecl) {
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
					p.parseField(field, model)
				}

				p.models[model.Name] = model
			}
		}
	}
}

// parseField parses field
func (p *AnnotationParser) parseField(field *ast.Field, model *ModelInfo) {
	if len(field.Names) == 0 {
		return
	}

	fieldName := field.Names[0].Name
	if !ast.IsExported(fieldName) {
		return
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
					var min float64
					fmt.Sscanf(parts[1], "%f", &min)
					prop.Minimum = &min
				}
			case "max":
				if len(parts) > 1 {
					var max float64
					fmt.Sscanf(parts[1], "%f", &max)
					prop.Maximum = &max
				}
			case "minlen":
				if len(parts) > 1 {
					var minLen int64
					fmt.Sscanf(parts[1], "%d", &minLen)
					prop.MinLength = &minLen
				}
			case "maxlen":
				if len(parts) > 1 {
					var maxLen int64
					fmt.Sscanf(parts[1], "%d", &maxLen)
					prop.MaxLength = &maxLen
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

// parseValue parses value
func (p *AnnotationParser) parseValue(value, valueType string) interface{} {
	value = strings.Trim(value, `"'`)

	switch valueType {
	case "boolean":
		return value == "true"
	case "integer":
		var i int64
		fmt.Sscanf(value, "%d", &i)
		return i
	case "number":
		var f float64
		fmt.Sscanf(value, "%f", &f)
		return f
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
