package mediautil

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestIsSupported(t *testing.T) {
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
