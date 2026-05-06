package database

import "testing"

func TestMigrationVersion(t *testing.T) {
	version, err := migrationVersion("migrations/000002_create_users_table.up.sql")
	if err != nil {
		t.Fatalf("migrationVersion returned error: %v", err)
	}
	if version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}
}
