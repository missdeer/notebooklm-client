package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/download"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/session"
	"github.com/missdeer/notebooklm-client/internal/transport"
	"github.com/missdeer/notebooklm-client/internal/types"
	"github.com/missdeer/notebooklm-client/internal/util"
)

type TransportMode string

const (
	TransportBrowser TransportMode = "browser"
	TransportHTTP    TransportMode = "http"
	TransportAuto    TransportMode = "auto"
	TransportCurl    TransportMode = "curl"
)

type ConnectOptions struct {
	Transport   TransportMode
	SessionPath string
	Session     *types.NotebookRpcSession
	Proxy       string
	ProfileDir  string
	Headless    bool
	ChromePath  string
}

type NotebookClient struct {
	transport     transport.Transport
	transportMode TransportMode
	proxy         string
	reqCounter    int
	rpcOverrides  map[string]string
}

func New() *NotebookClient {
	return &NotebookClient{reqCounter: 100000}
}

func (c *NotebookClient) Connect(ctx context.Context, opts ConnectOptions) error {
	c.transportMode = opts.Transport
	c.proxy = opts.Proxy

	if c.transportMode == "" || c.transportMode == TransportAuto {
		c.transportMode = TransportHTTP
	}

	if c.transportMode == TransportBrowser {
		return c.connectBrowser(ctx, opts)
	}

	return c.connectHeadless(ctx, opts)
}

func (c *NotebookClient) connectBrowser(ctx context.Context, opts ConnectOptions) error {
	rod := transport.NewRodTransport(transport.RodTransportOptions{
		ProfileDir: opts.ProfileDir,
		ChromePath: opts.ChromePath,
		Headless:   opts.Headless,
		Proxy:      opts.Proxy,
	})
	if err := rod.Init(ctx); err != nil {
		return fmt.Errorf("browser connect: %w", err)
	}
	c.transport = rod
	c.rpcOverrides = rpc.LoadRpcIDOverrides()
	return nil
}

func (c *NotebookClient) connectHeadless(ctx context.Context, opts ConnectOptions) error {
	sess := opts.Session

	if sess == nil {
		if authJSON := os.Getenv("NOTEBOOKLM_AUTH_JSON"); authJSON != "" {
			var s types.NotebookRpcSession
			if err := json.Unmarshal([]byte(authJSON), &s); err != nil {
				return types.NewSessionError("NOTEBOOKLM_AUTH_JSON contains invalid JSON", err)
			}
			sess = &s
		}
	}

	if sess == nil {
		loaded, err := session.Load(opts.SessionPath)
		if err != nil {
			return fmt.Errorf("load session: %w", err)
		}
		sess = loaded
	}

	if sess == nil {
		return types.NewSessionError(
			"No session available. Run `export-session` to log in, or set NOTEBOOKLM_AUTH_JSON env var.", nil)
	}

	sessionPath := opts.SessionPath
	proxyURL := opts.Proxy
	onSessionExpired := func(ctx context.Context) (*types.NotebookRpcSession, error) {
		log.Println("NotebookLM: Token expired, auto-refreshing...")
		refreshed, err := session.RefreshTokens(ctx, *sess, nil, sessionPath)
		if err != nil {
			fromDisk, loadErr := session.Load(sessionPath)
			if loadErr == nil && fromDisk != nil {
				return fromDisk, nil
			}
			return nil, types.NewSessionError(
				"Session expired and auto-refresh failed. Re-run `export-session`.", err)
		}
		sess = refreshed
		return refreshed, nil
	}
	_ = proxyURL

	t, err := transport.NewUTLSTransport(transport.UTLSTransportOptions{
		Session:          *sess,
		Proxy:            opts.Proxy,
		OnSessionExpired: onSessionExpired,
	})
	if err != nil {
		return fmt.Errorf("create transport: %w", err)
	}

	c.transport = t
	c.rpcOverrides = rpc.LoadRpcIDOverrides()

	blPreview := sess.BL
	if len(blPreview) > 40 {
		blPreview = blPreview[:40]
	}
	log.Printf("NotebookLM: Connected via utls (bl=%s...)", blPreview)
	return nil
}

func (c *NotebookClient) Disconnect() error {
	if c.transport != nil {
		err := c.transport.Close()
		c.transport = nil
		return err
	}
	return nil
}

func (c *NotebookClient) EnsureConnected() error {
	if c.transport == nil {
		return types.NewSessionError("NotebookLM client not connected", nil)
	}
	return nil
}

func (c *NotebookClient) GetRpcSession() *types.NotebookRpcSession {
	if c.transport == nil {
		return nil
	}
	s := c.transport.GetSession()
	return &s
}

func (c *NotebookClient) GetProxy() string { return c.proxy }
func (c *NotebookClient) GetTransportMode() TransportMode { return c.transportMode }

func (c *NotebookClient) resolveRpcID(staticID string) string {
	return rpc.ResolveRpcID(staticID, c.rpcOverrides)
}

func (c *NotebookClient) CallBatchExecute(ctx context.Context, rpcID string, payload []any, sourcePath string) (string, error) {
	if err := c.EnsureConnected(); err != nil {
		return "", err
	}

	resolvedID := c.resolveRpcID(rpcID)
	sess := c.transport.GetSession()

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	fReq, err := json.Marshal([]any{[]any{[]any{resolvedID, string(payloadJSON), nil, "generic"}}})
	if err != nil {
		return "", fmt.Errorf("marshal f.req: %w", err)
	}

	c.reqCounter += util.JitteredIncrement(100000, 0.3)

	qp := map[string]string{
		"rpcids":      resolvedID,
		"source-path": sourcePath,
		"bl":          sess.BL,
		"rt":          "c",
		"_reqid":      fmt.Sprintf("%d", c.reqCounter),
	}
	if sess.Language != "" {
		qp["hl"] = sess.Language
	}
	if sess.FSID != "" {
		qp["f.sid"] = sess.FSID
	}

	return c.transport.Execute(ctx, transport.Request{
		URL:         rpc.BatchExecuteURL,
		QueryParams: qp,
		Body: map[string]string{
			"f.req": string(fReq),
			"at":    sess.AT,
		},
	})
}

func (c *NotebookClient) CallChatStream(ctx context.Context, notebookID, message string, sourceIDs []string) (string, error) {
	if err := c.EnsureConnected(); err != nil {
		return "", err
	}

	sess := c.transport.GetSession()
	sidsTriple := make([]any, len(sourceIDs))
	for i, id := range sourceIDs {
		sidsTriple[i] = []any{[]any{id}}
	}

	payload := []any{
		[]any{message, 0, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, sidsTriple},
		notebookID,
		nil,
		rpc.DefaultUserConfig,
	}
	payloadJSON, _ := json.Marshal(payload)

	body := url.Values{}
	body.Set("f.req", string(payloadJSON))
	body.Set("at", sess.AT)

	c.reqCounter += util.JitteredIncrement(100000, 0.3)
	qp := map[string]string{
		"bl":     sess.BL,
		"rt":     "c",
		"_reqid": fmt.Sprintf("%d", c.reqCounter),
	}
	if sess.Language != "" {
		qp["hl"] = sess.Language
	}
	if sess.FSID != "" {
		qp["f.sid"] = sess.FSID
	}

	return c.transport.Execute(ctx, transport.Request{
		URL:         rpc.ChatStreamURL,
		QueryParams: qp,
		Body: map[string]string{
			"f.req": string(payloadJSON),
			"at":    sess.AT,
		},
	})
}

// rpcCaller returns an api.RpcCaller bound to this client.
func (c *NotebookClient) rpcCaller() api.RpcCaller {
	return c.CallBatchExecute
}

func (c *NotebookClient) chatCaller() api.ChatStreamCaller {
	return c.CallChatStream
}

// Delegated API methods

func (c *NotebookClient) CreateNotebook(ctx context.Context) (string, error) {
	return api.CreateNotebook(ctx, c.rpcCaller())
}

func (c *NotebookClient) ListNotebooks(ctx context.Context) ([]types.NotebookInfo, error) {
	return api.ListNotebooks(ctx, c.rpcCaller())
}

func (c *NotebookClient) GetNotebookDetail(ctx context.Context, notebookID string) (string, []types.SourceInfo, error) {
	return api.GetNotebookDetail(ctx, c.rpcCaller(), notebookID)
}

func (c *NotebookClient) DeleteNotebook(ctx context.Context, notebookID string) error {
	return api.DeleteNotebook(ctx, c.rpcCaller(), notebookID)
}

func (c *NotebookClient) RenameNotebook(ctx context.Context, notebookID, newTitle string) error {
	return api.RenameNotebook(ctx, c.rpcCaller(), notebookID, newTitle)
}

func (c *NotebookClient) AddURLSource(ctx context.Context, notebookID, sourceURL string) (string, string, error) {
	return api.AddURLSource(ctx, c.rpcCaller(), notebookID, sourceURL)
}

func (c *NotebookClient) AddTextSource(ctx context.Context, notebookID, title, content string) (string, string, error) {
	return api.AddTextSource(ctx, c.rpcCaller(), notebookID, title, content)
}

func (c *NotebookClient) AddFileSource(ctx context.Context, notebookID, filePath string) (string, string, error) {
	sess := c.transport.GetSession()
	return api.AddFileSource(ctx, c.rpcCaller(), api.FileUploadDeps{
		Session:    sess,
		Proxy:      c.proxy,
		HTTPClient: &http.Client{},
	}, notebookID, filePath)
}

func (c *NotebookClient) DeleteSource(ctx context.Context, sourceID string) error {
	return api.DeleteSource(ctx, c.rpcCaller(), sourceID)
}

func (c *NotebookClient) GetSourceSummary(ctx context.Context, sourceID string) (string, error) {
	return api.GetSourceSummary(ctx, c.rpcCaller(), sourceID)
}

func (c *NotebookClient) GenerateArtifact(ctx context.Context, notebookID string, sourceIDs []string, opts types.ArtifactOption) (string, string, error) {
	sess := c.transport.GetSession()
	lang := sess.Language
	if lang == "" {
		lang = "en"
	}
	return api.GenerateArtifact(ctx, c.rpcCaller(), notebookID, sourceIDs, lang, opts)
}

func (c *NotebookClient) GetArtifacts(ctx context.Context, notebookID string) ([]types.ArtifactInfo, error) {
	return api.GetArtifacts(ctx, c.rpcCaller(), notebookID)
}

func (c *NotebookClient) GetInteractiveHTML(ctx context.Context, artifactID string) (string, error) {
	return api.GetInteractiveHTML(ctx, c.rpcCaller(), artifactID)
}

func (c *NotebookClient) SendChat(ctx context.Context, notebookID, message string, sourceIDs []string) (string, string, error) {
	return api.SendChat(ctx, c.chatCaller(), notebookID, message, sourceIDs)
}

func (c *NotebookClient) GetStudioConfig(ctx context.Context, notebookID string) (types.StudioConfig, error) {
	return api.GetStudioConfig(ctx, c.rpcCaller(), notebookID)
}

func (c *NotebookClient) GetAccountInfo(ctx context.Context) (types.AccountInfo, error) {
	return api.GetAccountInfo(ctx, c.rpcCaller())
}

func (c *NotebookClient) CreateWebSearch(ctx context.Context, notebookID, query string, mode types.ResearchMode) (string, string, error) {
	return api.CreateWebSearch(ctx, c.rpcCaller(), notebookID, query, mode)
}

func (c *NotebookClient) PollResearchResults(ctx context.Context, notebookID string, timeout int) ([]types.ResearchResult, string, error) {
	return api.PollResearchResults(ctx, c.rpcCaller(), notebookID, 0)
}

func (c *NotebookClient) DownloadFile(ctx context.Context, downloadURL, outputDir, filename string) (string, error) {
	sess := c.transport.GetSession()
	return download.DownloadFileHTTP(ctx, download.Deps{
		Session: sess,
		Proxy:   c.proxy,
	}, downloadURL, outputDir, filename)
}

func (c *NotebookClient) MakeDownloadFn() download.DownloadFn {
	return func(ctx context.Context, downloadURL, outputDir, filename string) (string, error) {
		return c.DownloadFile(ctx, downloadURL, outputDir, filename)
	}
}

// ExportSession is for browser transport only.
func (c *NotebookClient) ExportSession(path string) (string, error) {
	if c.transport == nil {
		return "", fmt.Errorf("not connected")
	}
	sess := c.transport.GetSession()
	return session.Save(sess, path)
}

func buildCookieString(jar []types.SessionCookie) string {
	var parts []string
	for _, c := range jar {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}
