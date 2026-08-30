// Package v1alpha2 embeds the v1alpha2 deployment schema.
package v1alpha2

import _ "embed"

//go:embed appmanifest.schema.json
var Schema []byte

// APIVersion is the exact apiVersion value this schema accepts.
const APIVersion = "appmanifest.mgconsulting.io/v1alpha2"
