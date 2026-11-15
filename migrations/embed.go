// Package migrations embeds the SQL schema migrations so the API ships as a
// single binary with no files to deploy alongside it.
package migrations

import "embed"

// FS contains every goose migration file.
//
//go:embed *.sql
var FS embed.FS
