package mediautil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gotd/td/tg"
	"github.com/krau/SaveAny-Bot/config"
)

func TestIsSupported(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configFile, []byte("[telegram]\naudio_video_only = true\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := config.Init(t.Context(), configFile); err != nil {
		t.Fatalf("config init: %v", err)
	}

	tests := []struct {
		name  string
		media tg.MessageMediaClass
		want  bool
	}{
		{
			name: "audio document attribute",
			media: &tg.MessageMediaDocument{Document: &tg.Document{
				MimeType: "application/octet-stream",
				Attributes: []tg.DocumentAttributeClass{
					&tg.DocumentAttributeAudio{},
				},
			}},
			want: true,
		},
		{
			name: "video document attribute",
			media: &tg.MessageMediaDocument{Document: &tg.Document{
				MimeType: "application/octet-stream",
				Attributes: []tg.DocumentAttributeClass{
					&tg.DocumentAttributeVideo{},
				},
			}},
			want: true,
		},
		{
			name: "audio mime type fallback",
			media: &tg.MessageMediaDocument{Document: &tg.Document{
				MimeType: "audio/mpeg",
			}},
			want: true,
		},
		{
			name:  "photo rejected",
			media: &tg.MessageMediaPhoto{},
			want:  false,
		},
		{
			name: "generic document rejected",
			media: &tg.MessageMediaDocument{Document: &tg.Document{
				MimeType: "application/pdf",
				Attributes: []tg.DocumentAttributeClass{
					&tg.DocumentAttributeFilename{FileName: "file.pdf"},
				},
			}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupported(tt.media); got != tt.want {
				t.Fatalf("IsSupported() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSupportedLegacyMode(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configFile, []byte("[telegram]\naudio_video_only = false\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := config.Init(t.Context(), configFile); err != nil {
		t.Fatalf("config init: %v", err)
	}

	if !IsSupported(&tg.MessageMediaPhoto{}) {
		t.Fatal("photo should be supported when audio_video_only is false")
	}

	if !IsSupported(&tg.MessageMediaDocument{Document: &tg.Document{MimeType: "application/pdf"}}) {
		t.Fatal("generic documents should be supported when audio_video_only is false")
	}
}
