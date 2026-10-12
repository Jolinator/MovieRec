package database

import (
	"os"
	"path/filepath"
	"testing"
)

// chdirWithSecrets creates <tmp>/secrets and <tmp>/work, and runs the test from <tmp>/work.
func chdirWithSecrets(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	secrets := filepath.Join(root, "secrets")
	work := filepath.Join(root, "work")
	for _, dir := range []string{secrets, work} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(work)
	return secrets
}

func TestLoadEnvironmentFilesLoadsExistingAndSkipsMissing(t *testing.T) {
	secrets := chdirWithSecrets(t)
	if err := os.WriteFile(filepath.Join(secrets, "a.env"), []byte("DB_ENV_TEST_A=hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DB_ENV_TEST_A", "")
	os.Unsetenv("DB_ENV_TEST_A")

	if err := LoadEnvironmentFiles("missing.env", "a.env"); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("DB_ENV_TEST_A"); got != "hello" {
		t.Fatalf("DB_ENV_TEST_A = %q", got)
	}
}

func TestLoadEnvironmentFilesDoesNotOverrideExisting(t *testing.T) {
	secrets := chdirWithSecrets(t)
	if err := os.WriteFile(filepath.Join(secrets, "a.env"), []byte("DB_ENV_TEST_B=file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DB_ENV_TEST_B", "process")

	if err := LoadEnvironmentFiles("a.env"); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("DB_ENV_TEST_B"); got != "process" {
		t.Fatalf("DB_ENV_TEST_B = %q", got)
	}
}

func TestLoadEnvironmentFilesReportsUnreadableFile(t *testing.T) {
	secrets := chdirWithSecrets(t)
	// A directory exists at the path but cannot be parsed as an env file.
	if err := os.Mkdir(filepath.Join(secrets, "dir.env"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnvironmentFiles("dir.env"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadLocalEnvironmentWithoutFiles(t *testing.T) {
	chdirWithSecrets(t)
	if err := LoadLocalEnvironment(); err != nil {
		t.Fatal(err)
	}
}
