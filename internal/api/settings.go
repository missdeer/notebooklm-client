package api

import (
	"context"
	"fmt"

	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func GetOutputLanguage(ctx context.Context, call RpcCaller) (string, error) {
	raw, err := call(ctx, rpc.GetAccountInfo,
		[]any{nil, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}},
		"/")
	if err != nil {
		return "", fmt.Errorf("get output language: %w", err)
	}
	envelopes := parser.ParseEnvelopes(raw)
	if len(envelopes) == 0 {
		return "", nil
	}
	result := envelopes[0]
	outer, ok := result[0].([]any)
	if !ok || len(outer) < 3 {
		return "", nil
	}
	settings, ok := outer[2].([]any)
	if !ok || len(settings) < 5 {
		return "", nil
	}
	langArr, ok := settings[4].([]any)
	if !ok || len(langArr) < 1 {
		return "", nil
	}
	lang, _ := langArr[0].(string)
	return lang, nil
}

func SetOutputLanguage(ctx context.Context, call RpcCaller, language string) error {
	_, err := call(ctx, rpc.SetUserSettings,
		[]any{[]any{[]any{nil, []any{[]any{nil, nil, nil, nil, []any{language}}}}}},
		"/")
	if err != nil {
		return fmt.Errorf("set output language: %w", err)
	}
	return nil
}

func GetStudioConfig(ctx context.Context, call RpcCaller, notebookID string) (types.StudioConfig, error) {
	raw, err := call(ctx, rpc.GetStudioConfig,
		[]any{copySlice(rpc.DefaultUserConfig), notebookID},
		"/notebook/"+notebookID)
	if err != nil {
		return types.StudioConfig{}, fmt.Errorf("get studio config: %w", err)
	}
	return parser.ParseStudioConfig(raw), nil
}

func GetAccountInfo(ctx context.Context, call RpcCaller) (types.AccountInfo, error) {
	raw, err := call(ctx, rpc.GetAccountInfo,
		[]any{copySlice(rpc.DefaultUserConfig)},
		"/")
	if err != nil {
		return types.AccountInfo{}, fmt.Errorf("get account info: %w", err)
	}
	return parser.ParseAccountInfo(raw), nil
}
