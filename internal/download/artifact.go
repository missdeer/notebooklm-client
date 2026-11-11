package download

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

// SaveArtifact saves an existing artifact in a supported native format.
// An empty format selects the default; this does not transcode media.
func SaveArtifact(ctx context.Context, call api.RpcCaller, getHTML func(context.Context, string) (string, error), download DownloadFn, artifact types.ArtifactInfo, outputDir, format string) (string, error) {
	if artifact.StatusCode != nil && *artifact.StatusCode != 3 {
		// Audio can expose final media while still reporting status 2.
		if !(artifact.Type == rpc.ArtifactAudio && *artifact.StatusCode == 2 && (artifact.DownloadURL != "" || artifact.StreamURL != "")) {
			return "", fmt.Errorf("artifact %s is not ready for download (status %d)", artifact.ID, *artifact.StatusCode)
		}
	}
	if outputDir == "" {
		outputDir = "."
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "auto" {
		format = ""
	}
	defaults := map[int]string{
		rpc.ArtifactAudio: "m4a", rpc.ArtifactVideo: "mp4", rpc.ArtifactReport: "md",
		rpc.ArtifactQuiz: "html", rpc.ArtifactInfographic: "png", rpc.ArtifactDataTable: "csv",
	}
	if format == "" {
		format = defaults[artifact.Type]
	}
	if artifact.Type != rpc.ArtifactSlideDeck {
		want, ok := defaults[artifact.Type]
		if !ok {
			return "", fmt.Errorf("download is not supported for artifact type %d", artifact.Type)
		}
		if format != want {
			return "", fmt.Errorf("artifact type %d supports --format %s; conversion to %s is not supported", artifact.Type, want, format)
		}
	} else if format != "" && format != "pptx" && format != "pdf" {
		return "", fmt.Errorf("slide decks support --format pptx or pdf")
	}

	switch artifact.Type {
	case rpc.ArtifactReport:
		return SaveReport(ctx, call, artifact.ID, outputDir)
	case rpc.ArtifactQuiz:
		return SaveQuizHTML(ctx, getHTML, artifact.ID, outputDir, "interactive")
	case rpc.ArtifactDataTable:
		return SaveDataTable(ctx, call, artifact.ID, outputDir)
	case rpc.ArtifactAudio:
		u := artifact.DownloadURL
		if u == "" {
			u = artifact.StreamURL
		}
		return saveArtifactURL(ctx, download, u, outputDir, "audio", format)
	}

	meta, err := GetArtifactMetadata(ctx, call, artifact.ID)
	if err != nil {
		return "", err
	}
	if len(meta) == 0 || meta[0] != artifact.ID {
		return "", fmt.Errorf("metadata missing or mismatched for artifact %s", artifact.ID)
	}
	var u, prefix string
	switch artifact.Type {
	case rpc.ArtifactSlideDeck:
		pdf, pptx := slideDeckURLs(meta)
		if format == "" {
			format = "pptx"
			if pptx == "" {
				format = "pdf"
			}
		}
		u, prefix = pptx, "slides"
		if format == "pdf" {
			u = pdf
		}
	case rpc.ArtifactVideo:
		u, prefix = metadataString(meta, 8, 3), "video"
		if u == "" {
			u = artifact.DownloadURL
		}
		if u == "" {
			u = artifact.StreamURL
		}
	case rpc.ArtifactInfographic:
		u, prefix = metadataString(meta, 14, 2, 0, 1, 0), "infographic"
	}
	return saveArtifactURL(ctx, download, u, outputDir, prefix, format)
}

func metadataString(data any, path ...int) string {
	for _, index := range path {
		arr, ok := data.([]any)
		if !ok || index < 0 || index >= len(arr) {
			return ""
		}
		data = arr[index]
	}
	s, _ := data.(string)
	return s
}

func saveArtifactURL(ctx context.Context, download DownloadFn, u, outputDir, prefix, format string) (string, error) {
	if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		return "", fmt.Errorf("no %s download URL available for this artifact", format)
	}
	return download(ctx, u, outputDir, fmt.Sprintf("%s_%d.%s", prefix, time.Now().UnixMilli(), format))
}
