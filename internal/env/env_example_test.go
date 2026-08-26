package env

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestEnvExampleInSync keeps .env.example honest: every variable the code
// reads must be listed there, and nothing listed may be dead. This is what
// would have caught SHRL_ADMIN_KEY lingering in the template after it was
// removed in ADR 0006.
func TestEnvExampleInSync(t *testing.T) {
	// go test runs with the package directory as cwd.
	root := filepath.Clean("../..")

	codeVars := map[string]bool{}
	goRe := regexp.MustCompile(`(?:env\.(?:Or|Int|Duration)|os\.Getenv)\("(SHRL_[A-Z_]+)`)
	tsRe := regexp.MustCompile(`\benv\.(SHRL_[A-Z_]+)`)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "bin", "build":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		switch filepath.Ext(d.Name()) {
		case ".go":
			scanVars(t, path, goRe, codeVars)
		case ".ts", ".svelte":
			scanVars(t, path, tsRe, codeVars)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repo: %v", err)
	}

	exampleVars := map[string]bool{}
	lineRe := regexp.MustCompile(`^[# ]*(SHRL_[A-Z_]+)`)
	raw, err := os.ReadFile(filepath.Join(root, ".env.example"))
	if err != nil {
		t.Fatalf("reading .env.example: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if m := lineRe.FindStringSubmatch(line); m != nil {
			exampleVars[m[1]] = true
		}
	}

	var missing, dead []string
	for v := range codeVars {
		if !exampleVars[v] {
			missing = append(missing, v)
		}
	}
	for v := range exampleVars {
		if !codeVars[v] {
			dead = append(dead, v)
		}
	}
	if len(missing) > 0 || len(dead) > 0 {
		sort.Strings(missing)
		sort.Strings(dead)
		t.Errorf("code/.env.example drift: missing from .env.example: %v; dead in .env.example: %v", missing, dead)
	}
}

func scanVars(t *testing.T, path string, re *regexp.Regexp, out map[string]bool) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	for _, m := range re.FindAllSubmatch(content, -1) {
		out[string(m[1])] = true
	}
}
