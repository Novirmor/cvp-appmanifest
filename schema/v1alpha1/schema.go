// Package v1alpha1 embeds the immutable v1alpha1 deployment schema.
package v1alpha1

import _ "embed"

//go:embed appmanifest.schema.json
var Schema []byte

// APIVersion is the exact apiVersion value this schema accepts.
const APIVersion = "appmanifest.mgconsulting.io/v1alpha1"
