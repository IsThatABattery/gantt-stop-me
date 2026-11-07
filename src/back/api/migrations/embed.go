// Package migrations holds the SQL migrations, applied in file-name order at startup.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
