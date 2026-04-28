package config

import (
	"fmt"
	"os"
	"path/filepath"

	ytdlp "github.com/lrstanley/go-ytdlp"
)

type ytdlpConfig struct {
	CookiesFile        string `toml:"cookies_file" mapstructure:"cookies_file" json:"cookies_file"`
	CookiesFromBrowser string `toml:"cookies_from_browser" mapstructure:"cookies_from_browser" json:"cookies_from_browser"`
	Proxy              string `toml:"proxy" mapstructure:"proxy" json:"proxy"`
}

// ApplyTo applies ytdlp config to cmd. If tempDir is provided, cookie file is
// copied there so yt-dlp mutations don't corrupt the original.
func (c ytdlpConfig) ApplyTo(cmd *ytdlp.Command) *ytdlp.Command {
	fmt.Printf("Applying ytdlp config: %+v\n", c)
	if c.CookiesFile != "" {
		cookiePath := c.CookiesFile
		tmp := filepath.Join(os.TempDir(), "cookies.txt")
		if data, err := os.ReadFile(c.CookiesFile); err == nil {
			if err := os.WriteFile(tmp, data, 0600); err == nil {
				cookiePath = tmp
			}
		}
		cmd = cmd.Cookies(cookiePath)
	} else if c.CookiesFromBrowser != "" {
		cmd = cmd.CookiesFromBrowser(c.CookiesFromBrowser)
	}
	if c.Proxy != "" {
		cmd = cmd.Proxy(c.Proxy)
	}
	return cmd
}
