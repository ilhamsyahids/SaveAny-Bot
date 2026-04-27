package config

import ytdlp "github.com/lrstanley/go-ytdlp"

type ytdlpConfig struct {
	CookiesFile        string `toml:"cookies_file" mapstructure:"cookies_file" json:"cookies_file"`
	CookiesFromBrowser string `toml:"cookies_from_browser" mapstructure:"cookies_from_browser" json:"cookies_from_browser"`
	Proxy              string `toml:"proxy" mapstructure:"proxy" json:"proxy"`
}

func (c ytdlpConfig) ApplyTo(cmd *ytdlp.Command) *ytdlp.Command {
	if c.CookiesFile != "" {
		cmd = cmd.Cookies(c.CookiesFile)
	} else if c.CookiesFromBrowser != "" {
		cmd = cmd.CookiesFromBrowser(c.CookiesFromBrowser)
	}
	if c.Proxy != "" {
		cmd = cmd.Proxy(c.Proxy)
	}
	return cmd
}
