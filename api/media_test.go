package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gotd/td/tg"
)

func TestParseYTDLPExtractedInfoLog(t *testing.T) {
	jsonLine := json.RawMessage(`{"id":"abc123","duration":12.5,"_type":"video"}`)

	tests := []struct {
		name         string
		line         string
		rawJSON      *json.RawMessage
		wantDuration float64
		wantNil      bool
		wantErr      bool
	}{
		{
			name:         "raw json field",
			rawJSON:      &jsonLine,
			wantDuration: 12.5,
		},
		{
			name:         "line is json",
			line:         string(jsonLine),
			wantDuration: 12.5,
		},
		{
			name:         "line contains json payload",
			line:         "DEBUG yt-dlp output => " + string(jsonLine),
			wantDuration: 12.5,
		},
		{
			name:    "non json line ignored",
			line:    "[youtube] Extracting URL: https://youtube.com/watch?v=abc123",
			wantNil: true,
		},
		{
			name:    "bad json does not parse",
			line:    `{"id":"abc123","duration":`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parseYTDLPExtractedInfoLog(tt.line, tt.rawJSON)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if info != nil {
					t.Fatalf("expected nil info, got %+v", info)
				}
				return
			}
			if info == nil || info.Duration == nil {
				t.Fatalf("expected extracted info with duration, got %+v", info)
			}
			if *info.Duration != tt.wantDuration {
				t.Fatalf("expected duration %.1f, got %.1f", tt.wantDuration, *info.Duration)
			}
		})
	}
}

func TestExtractTelegramMessageDuration(t *testing.T) {
	tests := []struct {
		name         string
		message      *tg.Message
		wantDuration float64
	}{
		{
			name: "video duration",
			message: &tg.Message{Media: &tg.MessageMediaDocument{Document: &tg.Document{Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeVideo{Duration: 42},
			}}}},
			wantDuration: 42,
		},
		{
			name: "audio duration",
			message: &tg.Message{Media: &tg.MessageMediaDocument{Document: &tg.Document{Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeAudio{Duration: 95},
			}}}},
			wantDuration: 95,
		},
		{
			name:         "missing duration",
			message:      &tg.Message{Media: &tg.MessageMediaDocument{Document: &tg.Document{}}},
			wantDuration: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTelegramDuration(tt.message)
			if got != tt.wantDuration {
				t.Fatalf("expected duration %.1f, got %.1f", tt.wantDuration, got)
			}
		})
	}
}

func TestExtractMediaMetadataDispatch(t *testing.T) {
	origTelegramResolver := telegramMessageContextResolver
	origDirectExtractor := directMediaMetadataExtractor
	origYTDLPExtractor := ytdlpMediaMetadataExtractor
	defer func() {
		telegramMessageContextResolver = origTelegramResolver
		directMediaMetadataExtractor = origDirectExtractor
		ytdlpMediaMetadataExtractor = origYTDLPExtractor
	}()

	t.Run("telegram link uses telegram metadata", func(t *testing.T) {
		directCalled := false
		ytdlpCalled := false
		telegramMessageContextResolver = func(ctx context.Context, chatID int64, msgID int) (*MessageContext, error) {
			return &MessageContext{Message: &tg.Message{Media: &tg.MessageMediaDocument{Document: &tg.Document{Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeVideo{Duration: 33},
			}}}}}, nil
		}
		directMediaMetadataExtractor = func(ctx context.Context, url string) (*MediaMetadataResponse, error) {
			directCalled = true
			return nil, errors.New("should not be called")
		}
		ytdlpMediaMetadataExtractor = func(ctx context.Context, url string) (*MediaMetadataResponse, error) {
			ytdlpCalled = true
			return nil, errors.New("should not be called")
		}

		resp, err := extractMediaMetadata(context.Background(), "https://t.me/c/123456789/123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.DurationSeconds != 33 {
			t.Fatalf("expected duration 33, got %.1f", resp.DurationSeconds)
		}
		if directCalled {
			t.Fatal("direct extractor should not be called for telegram links")
		}
		if ytdlpCalled {
			t.Fatal("ytdlp extractor should not be called for telegram links")
		}
	})

	t.Run("direct probe succeeds before ytdlp", func(t *testing.T) {
		directMediaMetadataExtractor = func(ctx context.Context, url string) (*MediaMetadataResponse, error) {
			return &MediaMetadataResponse{URL: url, DurationSeconds: 12.5}, nil
		}
		ytdlpMediaMetadataExtractor = func(ctx context.Context, url string) (*MediaMetadataResponse, error) {
			return nil, errors.New("should not be called")
		}

		resp, err := extractMediaMetadata(context.Background(), "https://cdn.example.com/audio.mp3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.DurationSeconds != 12.5 {
			t.Fatalf("expected duration 12.5, got %.1f", resp.DurationSeconds)
		}
	})

	t.Run("falls back to ytdlp after direct probe failure", func(t *testing.T) {
		directMediaMetadataExtractor = func(ctx context.Context, url string) (*MediaMetadataResponse, error) {
			return nil, errors.New("probe failed")
		}
		ytdlpMediaMetadataExtractor = func(ctx context.Context, url string) (*MediaMetadataResponse, error) {
			return &MediaMetadataResponse{URL: url, DurationSeconds: 88}, nil
		}

		resp, err := extractMediaMetadata(context.Background(), "https://example.com/watch?v=abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.DurationSeconds != 88 {
			t.Fatalf("expected duration 88, got %.1f", resp.DurationSeconds)
		}
	})
}
