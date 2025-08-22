# Swagger Documentation Directory

This directory is used to store generated Swagger API documentation.

## File Description

- `swagger.json` - Auto-generated Swagger 2.0 specification document (JSON format)
- `swagger.yaml` - Auto-generated Swagger 2.0 specification document (YAML format)

## Generation Method

Documentation is automatically generated in the following cases:
1. When scanning code annotations during application startup
2. When files change (if monitoring is enabled)
3. When manually calling the generation interface

## Configuration

Configure output path in `config.yaml`:

```yaml
lynx:
  swagger:
    gen:
      output_path: "./plugins/swagger/docs/swagger.json"
```
