package goreleasercheck

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"
)

// Та же проверка, что в GoReleaser internal/tmpl.ApplySingleEnvOnly:
// в поле token допускается только {{ .Env.ИМЯ }}.
var envOnlyRe = regexp.MustCompile(`^{{\s*\.Env\.[^.\s}]+\s*}}$`)

const skipUploadTpl = `{{ if eq (index .Env "TAP_GITHUB_TOKEN") "" }}true{{ else }}false{{ end }}`

func TestIndexTokenIsRejected(t *testing.T) {
	bad := `{{ index .Env "TAP_GITHUB_TOKEN" }}`
	if envOnlyRe.MatchString(bad) {
		t.Fatal("форма index не должна проходить")
	}
	_, err := apply(bad, map[string]string{"TAP_GITHUB_TOKEN": "fake-token-for-ci"}, true)
	if err == nil || err.Error() != errTokenForm.Error() {
		t.Fatalf("err = %v", err)
	}
}

func TestTapPublishTemplates(t *testing.T) {
	body := readGoreleaser(t)
	sections := topLevel(body)
	for _, name := range []string{"homebrew_casks", "scoops"} {
		t.Run(name, func(t *testing.T) {
			section, ok := sections[name]
			if !ok {
				t.Fatalf("в .goreleaser.yaml нет секции %s", name)
			}
			tokens := scalars(section, "token")
			skips := scalars(section, "skip_upload")
			if len(tokens) != 1 || len(skips) != 1 {
				t.Fatalf("token=%q skip_upload=%q, нужна ровно одна пара", tokens, skips)
			}
			token := tokens[0]
			if !envOnlyRe.MatchString(token) {
				t.Fatalf("token %q: GoReleaser ждёт только {{ .Env.ИМЯ }}", token)
			}
			if token != "{{ .Env.TAP_GITHUB_TOKEN }}" {
				t.Fatalf("token = %q", token)
			}
			if skips[0] != skipUploadTpl {
				t.Fatalf("skip_upload = %q", skips[0])
			}

			assertPublish(t, "unset", nil, "true", false, "")
			assertPublish(t, "empty", map[string]string{"TAP_GITHUB_TOKEN": ""}, "true", true, "")
			assertPublish(t, "fake", map[string]string{"TAP_GITHUB_TOKEN": "fake-token-for-ci"}, "false", true, "fake-token-for-ci")
			if value, ok := os.LookupEnv("TAP_GITHUB_TOKEN"); ok && value != "" {
				assertPublish(t, "process", map[string]string{"TAP_GITHUB_TOKEN": value}, "false", true, value)
			}
		})
	}
}

func assertPublish(t *testing.T, name string, env map[string]string, wantSkip string, tokenOK bool, wantToken string) {
	t.Helper()
	skip, err := apply(skipUploadTpl, env, false)
	if err != nil {
		t.Fatalf("%s: skip_upload: %v", name, err)
	}
	if skip != wantSkip {
		t.Fatalf("%s: skip_upload = %q, want %q", name, skip, wantSkip)
	}
	got, err := apply("{{ .Env.TAP_GITHUB_TOKEN }}", env, true)
	if tokenOK {
		if err != nil {
			t.Fatalf("%s: token: %v", name, err)
		}
		if got != wantToken {
			t.Fatalf("%s: token = %q, want %q", name, got, wantToken)
		}
		return
	}
	if err == nil {
		t.Fatalf("%s: отсутствующая переменная не должна проходить разбор токена", name)
	}
	if skip != "true" {
		t.Fatalf("%s: токен не разобрался, а skip_upload = %q", name, skip)
	}
}

func apply(src string, env map[string]string, singleEnv bool) (string, error) {
	src = strings.TrimSpace(src)
	if singleEnv && !envOnlyRe.MatchString(src) {
		return "", errTokenForm
	}
	var out bytes.Buffer
	tmpl, err := template.New("tmpl").Option("missingkey=error").Parse(src)
	if err != nil {
		return "", err
	}
	if env == nil {
		env = map[string]string{}
	}
	err = tmpl.Execute(&out, map[string]any{"Env": env})
	return out.String(), err
}

type tokenFormError struct{}

func (tokenFormError) Error() string {
	return "expected {{ .Env.VAR_NAME }} only (no plain-text or other interpolation)"
}

var errTokenForm = tokenFormError{}

func readGoreleaser(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", ".goreleaser.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func topLevel(body string) map[string]string {
	out := map[string]string{}
	var name string
	var buf []string
	flush := func() {
		if name != "" {
			out[name] = strings.Join(buf, "\n")
		}
	}
	for _, line := range strings.Split(body, "\n") {
		if line != "" && line[0] != ' ' && line[0] != '#' && strings.HasSuffix(strings.TrimSpace(line), ":") {
			flush()
			name = strings.TrimSuffix(strings.TrimSpace(line), ":")
			buf = nil
			continue
		}
		if name != "" {
			buf = append(buf, line)
		}
	}
	flush()
	return out
}

func scalars(section, key string) []string {
	var out []string
	prefix := key + ":"
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		if len(val) >= 2 {
			quote := val[0]
			if (quote == '"' || quote == '\'') && val[len(val)-1] == quote {
				val = val[1 : len(val)-1]
			}
		}
		out = append(out, val)
	}
	return out
}
