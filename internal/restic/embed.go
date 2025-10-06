package restic

import _ "embed"

//go:embed run.sh
var runScript string

//go:embed restic.rb
var resticScript string
