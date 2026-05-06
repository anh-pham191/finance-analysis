package archtest

import (
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type goPackage struct {
	ImportPath string
	Imports    []string
}

func TestInternalImportGraph(t *testing.T) {
	t.Parallel()

	packages := listInternalPackages(t)
	for _, pkg := range packages {
		for _, imported := range pkg.Imports {
			assertAllowedImport(t, pkg.ImportPath, imported)
		}
	}
}

func listInternalPackages(t *testing.T) []goPackage {
	t.Helper()

	cmd := exec.Command("go", "list", "-json", "./internal/...")
	cmd.Dir = repoRoot(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list internal packages: %v", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var packages []goPackage
	for {
		var pkg goPackage
		if err := decoder.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("decode go list output: %v", err)
		}
		packages = append(packages, pkg)
	}
	return packages
}

func assertAllowedImport(t *testing.T, from, imported string) {
	t.Helper()

	if strings.Contains(from, "/internal/domain") && strings.Contains(imported, "/internal/") {
		t.Fatalf("%s must not import internal package %s", from, imported)
	}
	if strings.Contains(from, "/internal/ports") &&
		strings.Contains(imported, "/internal/") &&
		!strings.Contains(imported, "/internal/domain") {
		t.Fatalf("%s must only import internal/domain, got %s", from, imported)
	}
	if isUseCasePackage(from) && (strings.Contains(imported, "/internal/akahu") || strings.Contains(imported, "/internal/storage")) {
		t.Fatalf("%s must not import adapter package %s", from, imported)
	}
	if isCorePackage(from) && (imported == "github.com/spf13/cobra" || imported == "net/http") {
		t.Fatalf("%s must not import delivery package %s", from, imported)
	}
}

func isUseCasePackage(path string) bool {
	return strings.Contains(path, "/internal/ingest") ||
		strings.Contains(path, "/internal/categorise") ||
		strings.Contains(path, "/internal/report")
}

func isCorePackage(path string) bool {
	return isUseCasePackage(path) || strings.Contains(path, "/internal/render")
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}
