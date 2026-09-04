package marketdataapi

import _ "embed"

//go:embed openapi.yaml
var openAPISpec []byte

// OpenAPISpec returns the OpenAPI document embedded in the application binary.
func OpenAPISpec() []byte {
	return openAPISpec
}
