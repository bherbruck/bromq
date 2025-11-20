package swagger

import (
	_ "embed"
)

// SwaggerJSON contains the embedded OpenAPI specification in JSON format
//
//go:embed swagger.json
var SwaggerJSON []byte

// SwaggerYAML contains the embedded OpenAPI specification in YAML format
//
//go:embed swagger.yaml
var SwaggerYAML []byte
