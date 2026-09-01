package readme

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This is the README counterpart of internal/env's TestEnvExampleInSync:
// every claim that can drift silently is cross-checked against the source
// tree. It would have caught the stale "geographic maps are planned" roadmap
// note and the undocumented /v1/stats endpoints.

var root = filepath.Clean("../..")

func walkCode(t *testing.T, visit func(path string, content []byte)) {
	t.Helper()
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
		case ".go", ".ts", ".svelte":
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			visit(path, content)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repo: %v", err)
	}
}

func readmeVars(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	vars := map[string]bool{}
	rowRe := regexp.MustCompile("^[#| ]*`(SHRL_[A-Z_]+)`")
	for _, line := range strings.Split(string(raw), "\n") {
		if m := rowRe.FindStringSubmatch(line); m != nil {
			vars[m[1]] = true
		}
	}
	return vars
}

// TestReadmeEnvVarsInSync keeps the README configuration tables honest: every
// SHRL_* variable the code reads must be documented, and no documented
// variable may be dead. Numeric env.Int defaults must match the documented
// default (a non-numeric or indirect default, e.g. a named constant, is
// skipped).
func TestReadmeEnvVarsInSync(t *testing.T) {
	codeVars := map[string]bool{}
	codeDefaults := map[string]int{}
	goVarRe := regexp.MustCompile(`(?:env\.(?:Or|Int|Duration|Bool)|os\.Getenv)\("(SHRL_[A-Z_]+)`)
	tsVarRe := regexp.MustCompile(`\benv\.(SHRL_[A-Z_]+)`)
	goDefaultRe := regexp.MustCompile(`env\.Int\("(SHRL_[A-Z_]+)",\s*(\d+)\)`)
	walkCode(t, func(path string, content []byte) {
		re := goVarRe
		if !strings.HasSuffix(path, ".go") {
			re = tsVarRe
		}
		for _, m := range re.FindAllSubmatch(content, -1) {
			codeVars[string(m[1])] = true
		}
		for _, m := range goDefaultRe.FindAllSubmatch(content, -1) {
			n, err := strconv.Atoi(string(m[2]))
			if err == nil {
				codeDefaults[string(m[1])] = n
			}
		}
	})

	readmeVars := readmeVars(t)

	var missing, dead []string
	for v := range codeVars {
		if !readmeVars[v] {
			missing = append(missing, v)
		}
	}
	for v := range readmeVars {
		if !codeVars[v] {
			dead = append(dead, v)
		}
	}
	if len(missing) > 0 || len(dead) > 0 {
		sort.Strings(missing)
		sort.Strings(dead)
		t.Errorf("code/README drift: missing from README: %v; dead in README: %v", missing, dead)
	}

	defaultRe := regexp.MustCompile("^\\|\\s*`(SHRL_[A-Z_]+)`\\s*\\|\\s*`?(\\d+)`?")
	defaults := map[string]int{}
	raw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if m := defaultRe.FindStringSubmatch(line); m != nil {
			n, err := strconv.Atoi(m[2])
			if err == nil {
				defaults[m[1]] = n
			}
		}
	}
	var drifted []string
	for v, want := range codeDefaults {
		if got, ok := defaults[v]; ok && got != want {
			drifted = append(drifted, fmt.Sprintf("%s: code default %d, README says %d", v, want, got))
		}
	}
	if len(drifted) > 0 {
		sort.Strings(drifted)
		t.Errorf("code/README default drift: %v", drifted)
	}
}

// TestReadmeAuthRoutesInSync keeps the Auth API reference table honest: the
// documented method+path pairs must exactly match the routes the auth service
// registers.
func TestReadmeAuthRoutesInSync(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(root, "auth", "main.go"))
	if err != nil {
		t.Fatalf("reading auth/main.go: %v", err)
	}
	codeRoutes := map[string]bool{}
	for _, m := range regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) ([^"]+)"`).FindAllSubmatch(content, -1) {
		codeRoutes[string(m[1])+" "+string(m[2])] = true
	}

	raw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	readmeRoutes := map[string]bool{}
	rowRe := regexp.MustCompile("^\\|\\s*(GET|POST|PATCH|PUT|DELETE)\\s+\\|\\s*`([^`]+)`")
	for _, line := range strings.Split(string(raw), "\n") {
		if m := rowRe.FindStringSubmatch(line); m != nil {
			readmeRoutes[m[1]+" "+m[2]] = true
		}
	}

	var missing, dead []string
	for r := range codeRoutes {
		if !readmeRoutes[r] {
			missing = append(missing, r)
		}
	}
	for r := range readmeRoutes {
		if !codeRoutes[r] {
			dead = append(dead, r)
		}
	}
	if len(missing) > 0 || len(dead) > 0 {
		sort.Strings(missing)
		sort.Strings(dead)
		t.Errorf("auth route/README drift: missing from README: %v; dead in README: %v", missing, dead)
	}
}

// TestReadmeInternalLinksResolve checks that every relative markdown link in
// README.md points at a file that exists.
func TestReadmeInternalLinksResolve(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	linkRe := regexp.MustCompile(`\]\(([^)\s]+)\)`)
	var broken []string
	for _, m := range linkRe.FindAllSubmatch(raw, -1) {
		target := string(m[1])
		u, err := url.Parse(target)
		if err != nil || u.Scheme != "" || u.Host != "" || strings.HasPrefix(target, "#") {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(strings.SplitN(u.Path, "#", 2)[0]))
		if _, err := os.Stat(path); err != nil {
			broken = append(broken, target)
		}
	}
	if len(broken) > 0 {
		t.Errorf("broken internal links in README.md: %v", broken)
	}
}
