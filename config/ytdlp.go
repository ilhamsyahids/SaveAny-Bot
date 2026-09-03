package config

type YtdlpConfig struct {
	// MaxHeight limits the video resolution by height in pixels (e.g. 1080, 720).
	// 0 means no limit (best available). Ignored when Format is set.
	MaxHeight int `toml:"max_height" mapstructure:"max_height" json:"max_height"`
	// Format is a raw yt-dlp format selector (-f). When set, it takes precedence
	// over MaxHeight and gives the user full control.
	Format string `toml:"format" mapstructure:"format" json:"format"`
	// Recode is the target video container yt-dlp recodes into (e.g. mp4).
	// Empty disables recoding.
	Recode string `toml:"recode" mapstructure:"recode" json:"recode"`
	// Proxies are HTTP/HTTPS/SOCKS proxy URLs for yt-dlp's own requests,
	// independent of telegram.proxy. They are tried in order and the first one
	// that succeeds is used, so put the preferred proxy first. Accepts a TOML
	// array, a single string, or a comma-separated string. Empty means connect
	// directly.
	Proxies []string `toml:"proxies" mapstructure:"proxies" json:"proxies"`
}
