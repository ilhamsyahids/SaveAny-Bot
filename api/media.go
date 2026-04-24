package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	"github.com/krau/ffmpeg-go"
	ytdlp "github.com/lrstanley/go-ytdlp"
)

var (
	telegramMessageContextResolver = getMessageWithContext
	directMediaDurationExtractor   = extractDirectMediaDuration
	ytdlpMediaDurationExtractor    = extractYTDLPMediaDuration
)

func extractMediaDuration(ctx context.Context, url string) (*MediaDurationResponse, error) {
	if isValidMessageLink(url) {
		return extractTelegramMediaDuration(ctx, url)
	}

	if resp, err := directMediaDurationExtractor(ctx, url); err == nil {
		return resp, nil
	}

	return ytdlpMediaDurationExtractor(ctx, url)
}

func extractTelegramMediaDuration(ctx context.Context, link string) (*MediaDurationResponse, error) {
	chatID, msgID, err := ParseMessageLink(ctx, link)
	if err != nil {
		return nil, fmt.Errorf("failed to parse telegram link: %w", err)
	}

	msgCtx, err := telegramMessageContextResolver(ctx, chatID, msgID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve telegram message: %w", err)
	}

	duration, err := extractTelegramMessageDuration(msgCtx.Message)
	if err != nil {
		return nil, err
	}

	return &MediaDurationResponse{
		URL:             link,
		DurationSeconds: duration,
	}, nil
}

func extractTelegramMessageDuration(msg *tg.Message) (float64, error) {
	if msg == nil {
		return 0, fmt.Errorf("telegram message is nil")
	}

	if msg.Media == nil {
		return 0, fmt.Errorf("telegram message has no media")
	}

	media := msg.Media

	documentMedia, ok := media.(*tg.MessageMediaDocument)
	if !ok || documentMedia == nil || documentMedia.Document == nil {
		return 0, fmt.Errorf("telegram media does not contain a document")
	}

	document, ok := documentMedia.Document.AsNotEmpty()
	if !ok {
		return 0, fmt.Errorf("telegram document is empty")
	}

	for _, attribute := range document.Attributes {
		switch attr := attribute.(type) {
		case *tg.DocumentAttributeVideo:
			if attr.Duration > 0 {
				return float64(attr.Duration), nil
			}
		case *tg.DocumentAttributeAudio:
			if attr.Duration > 0 {
				return float64(attr.Duration), nil
			}
		}
	}

	return 0, fmt.Errorf("duration is not available for telegram media")
}

func extractDirectMediaDuration(ctx context.Context, url string) (*MediaDurationResponse, error) {
	probeResult, err := ffmpeg.ProbeWithTimeout(
		url,
		10*time.Second,
		ffmpeg.KwArgs{
			"show_entries": "format=duration",
			"of":           "json",
			"v":            "error",
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to probe direct media: %w", err)
	}

	var data struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal([]byte(probeResult), &data); err != nil {
		return nil, fmt.Errorf("failed to parse direct media metadata: %w", err)
	}

	var duration float64
	if _, err := fmt.Sscanf(strings.TrimSpace(data.Format.Duration), "%f", &duration); err != nil || duration <= 0 {
		return nil, fmt.Errorf("duration is not available for direct media")
	}

	return &MediaDurationResponse{
		URL:             url,
		DurationSeconds: duration,
	}, nil
}

func extractYTDLPMediaDuration(ctx context.Context, url string) (*MediaDurationResponse, error) {
	result, err := ytdlp.New().DumpSingleJSON().Run(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect media: %w", err)
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to inspect media: %s", result.Stderr)
	}

	var e *ytdlp.ExtractedInfo
	infoList := make([]*ytdlp.ExtractedInfo, 0)

	for _, log := range result.OutputLogs {
		e, err = parseYTDLPExtractedInfoLog(log.Line, log.JSON)
		if err != nil {
			continue
		}
		if e == nil {
			continue
		}

		infoList = append(infoList, e)
	}

	if len(infoList) == 0 || infoList[0] == nil {
		return nil, fmt.Errorf("no media metadata found for url")
	}

	if infoList[0].Duration == nil {
		return nil, fmt.Errorf("duration is not available for url")
	}

	return &MediaDurationResponse{
		URL:             url,
		DurationSeconds: *infoList[0].Duration,
	}, nil
}

func parseYTDLPExtractedInfoLog(line string, rawJSON *json.RawMessage) (*ytdlp.ExtractedInfo, error) {
	if rawJSON != nil && len(*rawJSON) > 0 {
		return parseYTDLPExtractedInfoBytes(*rawJSON)
	}

	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil, nil
	}

	info, lineErr := parseYTDLPExtractedInfoBytes(json.RawMessage([]byte(trimmed)))
	if info != nil {
		return info, nil
	}

	start := strings.IndexByte(trimmed, '{')
	end := strings.LastIndexByte(trimmed, '}')
	if start == -1 {
		return nil, nil
	}
	if end == -1 || start >= end {
		return nil, lineErr
	}

	info, err := parseYTDLPExtractedInfoBytes(json.RawMessage([]byte(trimmed[start : end+1])))
	if err != nil {
		return nil, err
	}
	if info != nil {
		return info, nil
	}

	return nil, lineErr
}

func parseYTDLPExtractedInfoBytes(data json.RawMessage) (*ytdlp.ExtractedInfo, error) {
	e, err := ytdlp.ParseExtractedInfo(&data)
	if err != nil {
		return nil, err
	}
	if e.Type == "" {
		return nil, nil
	}
	return e, nil
}
