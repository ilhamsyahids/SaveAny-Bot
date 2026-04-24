package api

import (
	"context"
	"fmt"

	ytdlp "github.com/lrstanley/go-ytdlp"
)

func extractMediaDuration(ctx context.Context, url string) (*MediaDurationResponse, error) {
	result, err := ytdlp.New().DumpSingleJSON().Simulate().Run(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect media: %w", err)
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to inspect media: %s", result.Stderr)
	}

	infoList, err := result.GetExtractedInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to parse media metadata: %w", err)
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
