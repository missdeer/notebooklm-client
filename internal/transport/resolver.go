package transport

import (
	"context"

	"github.com/missdeer/notebooklm-client/internal/types"
)

type TransportTier string

const (
	TierCurl TransportTier = "curl"
	TierUTLS TransportTier = "utls"
)

var TierLabels = map[TransportTier]string{
	TierCurl: "curl-impersonate (100% Chrome fingerprint)",
	TierUTLS: "utls (99% Chrome fingerprint)",
}

type TransportFactoryOptions struct {
	Session          types.NotebookRpcSession
	CurlBinaryPath   string
	Proxy            string
	OnSessionExpired func(context.Context) (*types.NotebookRpcSession, error)
}

func DetectBestTier(curlPath string) TransportTier {
	if CurlIsAvailable(curlPath) {
		return TierCurl
	}
	return TierUTLS
}

func CreateTransport(ctx context.Context, tier TransportTier, opts TransportFactoryOptions) (Transport, error) {
	switch tier {
	case TierCurl:
		return NewCurlTransport(CurlTransportOptions{
			Session:          opts.Session,
			BinaryPath:       opts.CurlBinaryPath,
			Proxy:            opts.Proxy,
			OnSessionExpired: opts.OnSessionExpired,
		})
	default:
		return NewUTLSTransport(UTLSTransportOptions{
			Session:          opts.Session,
			Proxy:            opts.Proxy,
			OnSessionExpired: opts.OnSessionExpired,
		})
	}
}
