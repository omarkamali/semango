package config

import _ "embed"

// embeddedCueSchema holds the compiled-in CUE schema so that the binary
// does not rely on an external docs/config.cue file at runtime.
//
//go:embed config_schema.cue
var embeddedCueSchema []byte
