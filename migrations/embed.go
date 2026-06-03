// Package migrations holds the SQL schema migrations, embedded into the binary
// so the CLI can apply them without an external migrate tool or loose .sql files.
package migrations

import "embed"

// FS holds every <version>_<title>.<up|down>.sql migration, consumed by the
// golang-migrate iofs source in internal/migrate.
//
//go:embed *.sql
var FS embed.FS
