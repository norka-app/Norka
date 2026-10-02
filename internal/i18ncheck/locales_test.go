package i18ncheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"testing"
)

// placeholder matches vue-i18n variables such as {name}. Literal braces
// written as {'@'} are not placeholders.
var placeholder = regexp.MustCompile(`\{[A-Za-z][A-Za-z0-9_]*\}`)

func TestFrontendLocaleKeysMatch(t *testing.T) {
	dir := localeDir(t)
	ru := mustFlatten(t, filepath.Join(dir, "ru.json"))
	en := mustFlatten(t, filepath.Join(dir, "en.json"))

	var missingEn, missingRu, placeholderDiff []string
	for key, ruText := range ru {
		enText, ok := en[key]
		if !ok {
			missingEn = append(missingEn, key)
			continue
		}
		if !samePlaceholders(ruText, enText) {
			placeholderDiff = append(placeholderDiff, fmt.Sprintf("%s ru=%q en=%q", key, ruText, enText))
		}
	}
	for key := range en {
		if _, ok := ru[key]; !ok {
			missingRu = append(missingRu, key)
		}
	}
	sort.Strings(missingEn)
	sort.Strings(missingRu)
	sort.Strings(placeholderDiff)
	if len(missingEn) > 0 {
		t.Errorf("en.json is missing %d keys from ru.json:\n%s", len(missingEn), stringsJoin(missingEn))
	}
	if len(missingRu) > 0 {
		t.Errorf("ru.json is missing %d keys from en.json:\n%s", len(missingRu), stringsJoin(missingRu))
	}
	if len(placeholderDiff) > 0 {
		t.Errorf("placeholder mismatch:\n%s", stringsJoin(placeholderDiff))
	}
}

func localeDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "frontend", "src", "locales")
}

func mustFlatten(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	out := map[string]string{}
	flatten("", root, out)
	if len(out) == 0 {
		t.Fatalf("%s has no string keys", path)
	}
	return out
}

func flatten(prefix string, value any, out map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flatten(next, child, out)
		}
	case string:
		out[prefix] = typed
	default:
		out[prefix] = fmt.Sprint(typed)
	}
}

func samePlaceholders(left, right string) bool {
	leftSet := placeholders(left)
	rightSet := placeholders(right)
	if len(leftSet) != len(rightSet) {
		return false
	}
	for key := range leftSet {
		if _, ok := rightSet[key]; !ok {
			return false
		}
	}
	return true
}

func placeholders(text string) map[string]struct{} {
	found := placeholder.FindAllString(text, -1)
	out := make(map[string]struct{}, len(found))
	for _, item := range found {
		out[item] = struct{}{}
	}
	return out
}

func stringsJoin(items []string) string {
	return fmt.Sprintf("  %s", joinLines(items))
}

func joinLines(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += "\n  "
		}
		out += item
	}
	return out
}
