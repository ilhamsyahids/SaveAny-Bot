package ytdlp

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	ytdlp "github.com/lrstanley/go-ytdlp"

	"github.com/krau/SaveAny-Bot/config"
)

// args returns the command arguments for assertion.
func args(t *testing.T, cmd *ytdlp.Command) string {
	t.Helper()
	return strings.Join(cmd.SetExecutable("yt-dlp").BuildCommand(t.Context()).Args, " ")
}

// fakeYtdlp creates a stub that records invocations without network access.
func fakeYtdlp(t *testing.T, script string) (bin, logPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stub is POSIX-only")
	}

	dir := t.TempDir()
	bin = filepath.Join(dir, "yt-dlp")
	logPath = filepath.Join(dir, "invocations.log")

	body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + logPath + "\n" + script
	if err := os.WriteFile(bin, []byte(body), 0700); err != nil {
		t.Fatalf("failed to write fake yt-dlp: %v", err)
	}
	return bin, logPath
}

func invocations(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("failed to read invocation log: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func TestProxyCandidates(t *testing.T) {
	cases := []struct {
		name  string
		cfg   config.YtdlpConfig
		flags []string
		want  []string
	}{
		{
			name: "none configured runs once without a proxy",
			want: []string{""},
		},
		{
			name: "single proxy",
			cfg:  config.YtdlpConfig{Proxies: []string{"http://a:1"}},
			want: []string{"http://a:1"},
		},
		{
			name: "order is preserved so the first entry is preferred",
			cfg:  config.YtdlpConfig{Proxies: []string{"http://a:1", "socks5://b:2", "http://c:3"}},
			want: []string{"http://a:1", "socks5://b:2", "http://c:3"},
		},
		{
			name: "blank entries dropped",
			cfg:  config.YtdlpConfig{Proxies: []string{"", "  ", "http://a:1"}},
			want: []string{"http://a:1"},
		},
		{
			name: "surrounding whitespace trimmed",
			cfg:  config.YtdlpConfig{Proxies: []string{" http://a:1 "}},
			want: []string{"http://a:1"},
		},
		{
			// Duplicates should not trigger another attempt.
			name: "duplicates dropped, first position kept",
			cfg:  config.YtdlpConfig{Proxies: []string{"http://a:1", "http://b:2", "http://a:1"}},
			want: []string{"http://a:1", "http://b:2"},
		},
		{
			name: "all entries blank falls back to a direct attempt",
			cfg:  config.YtdlpConfig{Proxies: []string{"", " "}},
			want: []string{""},
		},
		{
			name:  "user --proxy flag wins and runs once",
			cfg:   config.YtdlpConfig{Proxies: []string{"http://a:1", "http://b:2"}},
			flags: []string{"--proxy", "http://mine:9"},
			want:  []string{""},
		},
		{
			name:  "user --proxy=value form also wins",
			cfg:   config.YtdlpConfig{Proxies: []string{"http://a:1"}},
			flags: []string{"--proxy=http://mine:9"},
			want:  []string{""},
		},
		{
			name:  "unrelated user flag leaves the proxies in place",
			cfg:   config.YtdlpConfig{Proxies: []string{"http://a:1"}},
			flags: []string{"--format", "bestaudio"},
			want:  []string{"http://a:1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := proxyCandidates(tc.cfg, tc.flags); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("proxyCandidates() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestRunWithProxyFailoverUsesFirstProxy(t *testing.T) {
	bin, logPath := fakeYtdlp(t, "exit 0\n")

	cmd := ytdlp.New().SetExecutable(bin)
	cfg := config.YtdlpConfig{Proxies: []string{"http://first:1", "http://second:2"}}

	if _, err := RunWithProxyFailover(t.Context(), cmd, cfg, nil, "URL"); err != nil {
		t.Fatalf("RunWithProxyFailover failed: %v", err)
	}

	got := invocations(t, logPath)
	if len(got) != 1 {
		t.Fatalf("expected 1 invocation, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "--proxy http://first:1") {
		t.Errorf("expected the first proxy to be used, got %q", got[0])
	}
}

// A failed proxy should fall through to the next one.
func TestRunWithProxyFailoverFallsThroughToLater(t *testing.T) {
	bin, logPath := fakeYtdlp(t, `
case "$*" in
  *--proxy\ http://dead:1*) exit 1 ;;
  *--proxy\ http://alsodead:2*) exit 1 ;;
  *--proxy\ http://alive:3*) exit 0 ;;
esac
exit 1
`)

	cmd := ytdlp.New().SetExecutable(bin)
	cfg := config.YtdlpConfig{Proxies: []string{
		"http://dead:1", "http://alsodead:2", "http://alive:3",
	}}

	if _, err := RunWithProxyFailover(t.Context(), cmd, cfg, nil, "URL"); err != nil {
		t.Fatalf("expected the third proxy to succeed, got %v", err)
	}

	got := invocations(t, logPath)
	if len(got) != 3 {
		t.Fatalf("expected 3 invocations, got %d: %v", len(got), got)
	}
	for i, want := range []string{"http://dead:1", "http://alsodead:2", "http://alive:3"} {
		if !strings.Contains(got[i], "--proxy "+want) {
			t.Errorf("invocation %d: expected proxy %s, got %q", i+1, want, got[i])
		}
	}
}

// Each attempt should use only its own proxy.
func TestRunWithProxyFailoverReplacesRatherThanAppends(t *testing.T) {
	bin, logPath := fakeYtdlp(t, "if echo \"$*\" | grep -q 'http://second:2'; then exit 0; fi\nexit 1\n")

	cmd := ytdlp.New().SetExecutable(bin)
	cfg := config.YtdlpConfig{Proxies: []string{"http://first:1", "http://second:2"}}

	if _, err := RunWithProxyFailover(t.Context(), cmd, cfg, nil, "URL"); err != nil {
		t.Fatalf("RunWithProxyFailover failed: %v", err)
	}

	second := invocations(t, logPath)[1]
	if strings.Contains(second, "http://first:1") {
		t.Errorf("second attempt still carried the first proxy: %q", second)
	}
	if strings.Count(second, "--proxy") != 1 {
		t.Errorf("expected exactly one --proxy flag, got %q", second)
	}
}

func TestRunWithProxyFailoverAllFail(t *testing.T) {
	bin, logPath := fakeYtdlp(t, "echo 'boom' >&2\nexit 1\n")

	cmd := ytdlp.New().SetExecutable(bin)
	cfg := config.YtdlpConfig{Proxies: []string{"http://a:1", "http://b:2"}}

	_, err := RunWithProxyFailover(t.Context(), cmd, cfg, nil, "URL")
	if err == nil {
		t.Fatal("expected an error when every proxy fails")
	}
	if !strings.Contains(err.Error(), "all 2 proxies failed") {
		t.Errorf("error should say how many were tried, got %v", err)
	}
	if n := len(invocations(t, logPath)); n != 2 {
		t.Errorf("expected 2 invocations, got %d", n)
	}
}

func TestRunWithProxyFailoverNoProxyConfigured(t *testing.T) {
	bin, logPath := fakeYtdlp(t, "exit 0\n")

	cmd := ytdlp.New().SetExecutable(bin)

	if _, err := RunWithProxyFailover(t.Context(), cmd, config.YtdlpConfig{}, nil, "URL"); err != nil {
		t.Fatalf("RunWithProxyFailover failed: %v", err)
	}

	got := invocations(t, logPath)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 invocation, got %d: %v", len(got), got)
	}
	if strings.Contains(got[0], "--proxy") {
		t.Errorf("no proxy configured, but --proxy was passed: %q", got[0])
	}
}

// Cancellation should stop retries.
func TestRunWithProxyFailoverStopsOnCancel(t *testing.T) {
	bin, logPath := fakeYtdlp(t, "exit 1\n")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	cmd := ytdlp.New().SetExecutable(bin)
	cfg := config.YtdlpConfig{Proxies: []string{"http://a:1", "http://b:2", "http://c:3"}}

	if _, err := RunWithProxyFailover(ctx, cmd, cfg, nil, "URL"); err == nil {
		t.Fatal("expected an error from a cancelled context")
	}

	if n := len(invocations(t, logPath)); n > 1 {
		t.Errorf("cancelled run kept trying proxies: %d invocations", n)
	}
}

func TestRunWithProxyFailoverRespectsUserFlag(t *testing.T) {
	bin, logPath := fakeYtdlp(t, "exit 0\n")

	cmd := ytdlp.New().SetExecutable(bin)
	cfg := config.YtdlpConfig{Proxies: []string{"http://config:1"}}

	_, err := RunWithProxyFailover(t.Context(), cmd, cfg, []string{"--proxy", "http://mine:9"},
		"--proxy", "http://mine:9", "URL")
	if err != nil {
		t.Fatalf("RunWithProxyFailover failed: %v", err)
	}

	got := invocations(t, logPath)
	if len(got) != 1 {
		t.Fatalf("expected 1 invocation, got %d: %v", len(got), got)
	}
	if strings.Contains(got[0], "http://config:1") {
		t.Errorf("configured proxy overrode the user's own flag: %q", got[0])
	}
}

// Credentials must not appear in logs or errors.
func TestRedactProxy(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "direct"},
		{"http://host:8080", "http://host:8080"},
		{"socks5://user:s3cret@host:1080", "socks5://user:xxxxx@host:1080"},
		{"http://user@host:8080", "http://user@host:8080"},
		{"not a url at all", "<proxy>"},
	}

	for _, tc := range cases {
		got := redactProxy(tc.in)
		if got != tc.want {
			t.Errorf("redactProxy(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if strings.Contains(got, "s3cret") {
			t.Errorf("redactProxy(%q) leaked the password: %q", tc.in, got)
		}
	}
}

func TestFlagsContain(t *testing.T) {
	cases := []struct {
		flags []string
		name  string
		want  bool
	}{
		{[]string{"--proxy", "x"}, "--proxy", true},
		{[]string{"--proxy=x"}, "--proxy", true},
		{[]string{"--format", "b"}, "--proxy", false},
		{nil, "--proxy", false},
		// Similar flag names must not match.
		{[]string{"--proxy-something=1"}, "--proxy", false},
	}

	for _, tc := range cases {
		if got := flagsContain(tc.flags, tc.name); got != tc.want {
			t.Errorf("flagsContain(%q, %q) = %v, want %v", tc.flags, tc.name, got, tc.want)
		}
	}
}

func TestIsYtdlpTempFile(t *testing.T) {
	cases := map[string]bool{
		"video.mp4":       false,
		"video.mp4.part":  true,
		"video.info.ytdl": true,
		"video.mp4.temp":  true,
		"partial.mp3":     false,
	}

	for name, want := range cases {
		if got := isYtdlpTempFile(name); got != want {
			t.Errorf("isYtdlpTempFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestApplyFormatConfigUntouchedByProxy(t *testing.T) {
	// Proxy rotation must not alter format flags.
	cmd := applyFormatConfig(ytdlp.New(), config.YtdlpConfig{
		MaxHeight: 720,
		Proxies:   []string{"http://a:1"},
	})

	if got := args(t, cmd); strings.Contains(got, "--proxy") {
		t.Errorf("applyFormatConfig set a proxy flag: %s", got)
	}
}
