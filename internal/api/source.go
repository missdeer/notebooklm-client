package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func AddURLSource(ctx context.Context, call RpcCaller, notebookID, url string) (sourceID, title string, err error) {
	raw, err := call(ctx, rpc.AddSource,
		[]any{
			[]any{[]any{nil, nil, []any{url}, nil, nil, nil, nil, nil, nil, nil, 1}},
			notebookID,
			copySlice(rpc.PlatformWeb),
			[]any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}},
		},
		"/notebook/"+notebookID)
	if err != nil {
		return "", "", fmt.Errorf("add url source: %w", err)
	}
	sourceID, title = parser.ParseAddSource(raw)
	return sourceID, title, nil
}

func AddTextSource(ctx context.Context, call RpcCaller, notebookID, title, content string) (sourceID, titleOut string, err error) {
	raw, err := call(ctx, rpc.AddSource,
		[]any{
			[]any{[]any{nil, []any{title, content}, nil, 2, nil, nil, nil, nil, nil, nil, 1}},
			notebookID,
			copySlice(rpc.PlatformWeb),
			[]any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}},
		},
		"/notebook/"+notebookID)
	if err != nil {
		return "", "", fmt.Errorf("add text source: %w", err)
	}
	sourceID, titleOut = parser.ParseAddSource(raw)
	return sourceID, titleOut, nil
}

type FileUploadDeps struct {
	Session    types.NotebookRpcSession
	Proxy      string
	HTTPClient *http.Client
}

func AddFileSource(ctx context.Context, call RpcCaller, deps FileUploadDeps, notebookID, filePath string) (sourceID, title string, err error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", "", fmt.Errorf("add file source: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", "", fmt.Errorf("add file source: %w", err)
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("not a file: %s", absPath)
	}
	fileName := filepath.Base(absPath)
	fileSize := info.Size()

	raw, err := call(ctx, rpc.AddSourceFile,
		[]any{
			[]any{[]any{fileName}},
			notebookID,
			copySlice(rpc.PlatformWeb),
			[]any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}},
		},
		"/notebook/"+notebookID)
	if err != nil {
		return "", "", fmt.Errorf("add file source register: %w", err)
	}
	sourceID, _ = parser.ParseAddSource(raw)
	if sourceID == "" {
		return "", "", fmt.Errorf("failed to register file source — no sourceId returned")
	}

	fileData, err := os.ReadFile(absPath)
	if err != nil {
		return "", "", fmt.Errorf("add file source: read file: %w", err)
	}

	if err := scottyUpload(ctx, deps, notebookID, fileName, sourceID, fileSize, fileData); err != nil {
		return "", "", fmt.Errorf("add file source: upload: %w", err)
	}

	return sourceID, fileName, nil
}

func scottyUpload(ctx context.Context, deps FileUploadDeps, notebookID, fileName, sourceID string, fileSize int64, fileData []byte) error {
	client := deps.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	baseHeaders := map[string]string{
		"Accept":           "*/*",
		"Cookie":           deps.Session.Cookies,
		"Origin":           "https://notebooklm.google.com",
		"Referer":          "https://notebooklm.google.com/",
		"User-Agent":       deps.Session.UserAgent,
		"x-goog-authuser":  "0",
	}

	initBody, _ := json.Marshal(map[string]string{
		"PROJECT_ID":  notebookID,
		"SOURCE_NAME": fileName,
		"SOURCE_ID":   sourceID,
	})

	initReq, err := http.NewRequestWithContext(ctx, "POST", rpc.UploadURL+"?authuser=0", strings.NewReader(string(initBody)))
	if err != nil {
		return err
	}
	for k, v := range baseHeaders {
		initReq.Header.Set(k, v)
	}
	initReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	initReq.Header.Set("x-goog-upload-command", "start")
	initReq.Header.Set("x-goog-upload-header-content-length", fmt.Sprintf("%d", fileSize))
	initReq.Header.Set("x-goog-upload-protocol", "resumable")

	initResp, err := client.Do(initReq)
	if err != nil {
		return fmt.Errorf("upload init: %w", err)
	}
	io.ReadAll(initResp.Body)
	initResp.Body.Close()

	uploadURL := initResp.Header.Get("X-Goog-Upload-Url")
	if uploadURL == "" {
		return fmt.Errorf("upload session initiation failed (HTTP %d): no x-goog-upload-url", initResp.StatusCode)
	}

	uploadReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, strings.NewReader(string(fileData)))
	if err != nil {
		return err
	}
	for k, v := range baseHeaders {
		uploadReq.Header.Set(k, v)
	}
	uploadReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	uploadReq.Header.Set("x-goog-upload-command", "upload, finalize")
	uploadReq.Header.Set("x-goog-upload-offset", "0")

	uploadResp, err := client.Do(uploadReq)
	if err != nil {
		return fmt.Errorf("upload bytes: %w", err)
	}
	body, _ := io.ReadAll(uploadResp.Body)
	uploadResp.Body.Close()

	if uploadResp.StatusCode < 200 || uploadResp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return fmt.Errorf("file upload failed (HTTP %d): %s", uploadResp.StatusCode, snippet)
	}
	return nil
}

func DeleteSource(ctx context.Context, call RpcCaller, sourceID string) error {
	_, err := call(ctx, rpc.DeleteSource, []any{[]any{[]any{sourceID}}, copySlice(rpc.PlatformWeb)}, "")
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	return nil
}

func GetSourceSummary(ctx context.Context, call RpcCaller, sourceID string) (string, error) {
	raw, err := call(ctx, rpc.GetSourceSummary, []any{[]any{[]any{[]any{sourceID}}}}, "")
	if err != nil {
		return "", fmt.Errorf("get source summary: %w", err)
	}
	_, summary := parser.ParseSourceSummary(raw)
	return summary, nil
}

func RenameSource(ctx context.Context, call RpcCaller, notebookID, sourceID, newTitle string) error {
	_, err := call(ctx, rpc.UpdateSource,
		[]any{nil, []any{sourceID}, []any{[]any{[]any{newTitle}}}},
		"/notebook/"+notebookID)
	if err != nil {
		return fmt.Errorf("rename source: %w", err)
	}
	return nil
}

func RefreshSourceData(ctx context.Context, call RpcCaller, notebookID, sourceID string) error {
	_, err := call(ctx, rpc.RefreshSource,
		[]any{nil, []any{sourceID}, copySlice(rpc.PlatformWeb)},
		"/notebook/"+notebookID)
	if err != nil {
		return fmt.Errorf("refresh source: %w", err)
	}
	return nil
}
