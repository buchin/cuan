// Package workflow embeds the internal Cuan collaboration contracts.
package workflow

import _ "embed"

//go:embed coordinator.md
var Coordinator string

//go:embed developer.md
var Developer string
