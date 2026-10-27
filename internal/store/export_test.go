package store

import "database/sql"

// SQL exposes the connection to this package's tests only (export_test.go is compiled for tests).
func (db *DB) SQL() *sql.DB {
	return db.sql
}
