package ytdlp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/log"
	ytdlp "github.com/lrstanley/go-ytdlp"

	"github.com/krau/SaveAny-Bot/config"
)

// noProxy means run without setting --proxy.
const noProxy = ""

// RunWithProxyFailover tries each configured proxy in order until one succeeds.
func RunWithProxyFailover(
	ctx context.Context,
	cmd *ytdlp.Command,
	cfg config.YtdlpConfig,
	userFlags []string,
	args ...string,
) (*ytdlp.Result, error) {
	logger := log.FromContext(ctx)
	candidates := proxyCandidates(cfg, userFlags)

	var lastErr error
	for attempt, proxy := range candidates {
		// Reusing the builder is safe because Run rebuilds the command.
		if proxy == noProxy {
			cmd = cmd.UnsetProxy()
		} else {
			cmd = cmd.Proxy(proxy)
		}

		result, err := cmd.Run(ctx, args...)
		switch {
		case err == nil && result.ExitCode == 0:
			if attempt > 0 {
				logger.Infof("yt-dlp succeeded via proxy %d/%d (%s)",
					attempt+1, len(candidates), redactProxy(proxy))
			}
			return result, nil
		case err != nil:
			lastErr = err
		default:
			lastErr = fmt.Errorf("yt-dlp exited with code %d: %s", result.ExitCode, result.Stderr)
		}

		// Do not retry after cancellation.
		if errors.Is(lastErr, context.Canceled) || ctx.Err() != nil {
			return nil, lastErr
		}

		if len(candidates) > 1 {
			logger.Warnf("yt-dlp failed via proxy %d/%d (%s): %v",
				attempt+1, len(candidates), redactProxy(proxy), lastErr)
		}
	}

	if len(candidates) > 1 {
		return nil, fmt.Errorf("all %d proxies failed, last error: %w", len(candidates), lastErr)
	}
	return nil, lastErr
}

// proxyCandidates returns unique, non-empty proxies in configured order.
func proxyCandidates(cfg config.YtdlpConfig, userFlags []string) []string {
	// A user-supplied proxy takes precedence and is run once.
	if flagsContain(userFlags, "--proxy") {
		return []string{noProxy}
	}

	seen := make(map[string]struct{}, len(cfg.Proxies))
	candidates := make([]string, 0, len(cfg.Proxies))
	for _, proxy := range cfg.Proxies {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			continue
		}
		if _, dup := seen[proxy]; dup {
			continue
		}
		seen[proxy] = struct{}{}
		candidates = append(candidates, proxy)
	}

	if len(candidates) == 0 {
		return []string{noProxy}
	}
	return candidates
}

// flagsContain matches both --flag value and --flag=value.
func flagsContain(flags []string, names ...string) bool {
	for _, flag := range flags {
		for _, name := range names {
			if flag == name || strings.HasPrefix(flag, name+"=") {
				return true
			}
		}
	}
	return false
}

// redactProxy removes credentials before a proxy URL is logged.
func redactProxy(proxy string) string {
	if proxy == noProxy {
		return "direct"
	}

	u, err := url.Parse(proxy)
	if err != nil || u.Host == "" {
		return "<proxy>"
	}
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "xxxxx")
		}
	}
	return u.Redacted()
}
