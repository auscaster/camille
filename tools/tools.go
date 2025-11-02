//go:build tools

package tools

// This file tracks tool dependencies for reproducible builds.
// Run `go mod tidy` after adding/removing tools here.

import (
    _ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
    _ "github.com/pressly/goose/v3/cmd/goose"
)
