package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/parser"
)

func SaveQuizHTML(ctx context.Context, getHTML func(context.Context, string) (string, error), artifactID, outputDir, prefix string) (string, error) {
	html, err := getHTML(ctx, artifactID)
	if err != nil {
		return "", fmt.Errorf("get interactive html: %w", err)
	}
	if html == "" {
		return "", fmt.Errorf("no HTML content returned for artifact %s", artifactID)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("%s_%d.html", prefix, time.Now().UnixMilli())
	outPath := filepath.Join(outputDir, filename)
	if err := os.WriteFile(outPath, []byte(html), 0o644); err != nil {
		return "", err
	}
	return outPath, nil
}

func SaveReport(ctx context.Context, call api.RpcCaller, artifactID, outputDir string) (string, error) {
	meta, err := PollArtifactMetadata(ctx, call, artifactID, func(m []any) bool {
		if len(m) > 7 {
			if arr, ok := m[7].([]any); ok && len(arr) > 0 {
				if _, ok := arr[0].(string); ok {
					return true
				}
			}
		}
		return false
	}, 30)
	if err != nil {
		return "", fmt.Errorf("save report: %w", err)
	}

	markdown := ""
	if len(meta) > 7 {
		if arr, ok := meta[7].([]any); ok && len(arr) > 0 {
			markdown, _ = arr[0].(string)
		}
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("report_%d.md", time.Now().UnixMilli())
	outPath := filepath.Join(outputDir, filename)
	if err := os.WriteFile(outPath, []byte(markdown), 0o644); err != nil {
		return "", err
	}
	return outPath, nil
}

func SaveSlideDeck(ctx context.Context, call api.RpcCaller, download DownloadFn, artifactID, outputDir string) (pptxPath, pdfPath string, err error) {
	meta, err := PollArtifactMetadata(ctx, call, artifactID, func(m []any) bool {
		if len(m) > 16 {
			if arr, ok := m[16].([]any); ok && len(arr) > 0 {
				return true
			}
		}
		return false
	}, 60)
	if err != nil {
		return "", "", fmt.Errorf("save slide deck: %w", err)
	}

	if len(meta) > 16 {
		if arr, ok := meta[16].([]any); ok {
			for _, item := range arr {
				u, ok := item.(string)
				if !ok {
					if sub, ok := item.([]any); ok && len(sub) > 0 {
						u, _ = sub[0].(string)
					}
				}
				if u == "" {
					continue
				}
				if strings.HasSuffix(u, ".pptx") || strings.Contains(u, "pptx") {
					pptxPath, _ = download(ctx, u, outputDir, fmt.Sprintf("slides_%d.pptx", time.Now().UnixMilli()))
				} else if strings.HasSuffix(u, ".pdf") || strings.Contains(u, "pdf") {
					pdfPath, _ = download(ctx, u, outputDir, fmt.Sprintf("slides_%d.pdf", time.Now().UnixMilli()))
				}
			}
		}
	}
	return pptxPath, pdfPath, nil
}

func SaveInfographic(ctx context.Context, call api.RpcCaller, download DownloadFn, artifactID, outputDir string) (string, error) {
	meta, err := PollArtifactMetadata(ctx, call, artifactID, func(m []any) bool {
		if len(m) > 14 {
			if u, ok := m[14].(string); ok && u != "" {
				return true
			}
			if arr, ok := m[14].([]any); ok && len(arr) > 0 {
				return true
			}
		}
		return false
	}, 60)
	if err != nil {
		return "", fmt.Errorf("save infographic: %w", err)
	}

	var imageURL string
	if len(meta) > 14 {
		if u, ok := meta[14].(string); ok {
			imageURL = u
		} else if arr, ok := meta[14].([]any); ok && len(arr) > 0 {
			imageURL, _ = arr[0].(string)
		}
	}
	if imageURL == "" {
		return "", fmt.Errorf("no infographic image URL found")
	}

	return download(ctx, imageURL, outputDir, fmt.Sprintf("infographic_%d.png", time.Now().UnixMilli()))
}

func SaveDataTable(ctx context.Context, call api.RpcCaller, artifactID, outputDir string) (string, error) {
	meta, err := PollArtifactMetadata(ctx, call, artifactID, IsDataTableReady, 30)
	if err != nil {
		return "", fmt.Errorf("save data table: %w", err)
	}
	csv := ExtractDataTableCSV(meta)
	if csv == "" {
		return "", fmt.Errorf("no CSV data found for data table")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("datatable_%d.csv", time.Now().UnixMilli())
	outPath := filepath.Join(outputDir, filename)
	if err := os.WriteFile(outPath, []byte(csv), 0o644); err != nil {
		return "", err
	}
	return outPath, nil
}

func IsDataTableReady(meta []any) bool {
	if len(meta) <= 18 {
		return false
	}
	if arr, ok := meta[18].([]any); ok && len(arr) > 0 {
		return true
	}
	return false
}

func ExtractDataTableCSV(meta []any) string {
	if len(meta) <= 18 {
		return ""
	}
	rows := ExtractTableRows(meta)
	if len(rows) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				sb.WriteByte(',')
			}
			if strings.ContainsAny(cell, ",\"\n") {
				sb.WriteByte('"')
				sb.WriteString(strings.ReplaceAll(cell, `"`, `""`))
				sb.WriteByte('"')
			} else {
				sb.WriteString(cell)
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func ExtractTableRows(meta []any) [][]string {
	if len(meta) <= 18 {
		return nil
	}
	raw, err := extractTableData(meta[18])
	if err != nil {
		return nil
	}
	return raw
}

func extractTableData(data any) ([][]string, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("not an array")
	}
	_ = parser.ParseEnvelopes // ensure parser package available
	var rows [][]string
	for _, rowData := range arr {
		rowArr, ok := rowData.([]any)
		if !ok {
			continue
		}
		var row []string
		for _, cell := range rowArr {
			switch v := cell.(type) {
			case string:
				row = append(row, v)
			case float64:
				row = append(row, fmt.Sprintf("%g", v))
			default:
				row = append(row, fmt.Sprintf("%v", v))
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}
