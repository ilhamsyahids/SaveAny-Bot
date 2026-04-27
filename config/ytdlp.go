package config

type ytdlpConfig struct {
	CookiesFile        string `toml:"cookies_file" mapstructure:"cookies_file" json:"cookies_file"`
	CookiesFromBrowser string `toml:"cookies_from_browser" mapstructure:"cookies_from_browser" json:"cookies_from_browser"`
	Proxy              string `toml:"proxy" mapstructure:"proxy" json:"proxy"`
}
