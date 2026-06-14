package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

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

type chatHistoryEntry = []any // [string, nil, int]

type chatState struct {
	threadID    string
	history     []chatHistoryEntry
	turnCounter int
}

type NotebookClient struct {
	transport     transport.Transport
	transportMode TransportMode
	proxy         string
	reqCounter    int
	rpcOverrides  map[string]string

	chatMu     sync.Mutex
	chatStates map[string]*chatState
	chatLocks  map[string]*sync.Mutex
}

func New() *NotebookClient {
	return &NotebookClient{
		reqCounter: 100000,
		chatStates: make(map[string]*chatState),
		chatLocks:  make(map[string]*sync.Mutex),
	}
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
	// Use a uTLS-fingerprinted client for refresh: Google guards the dashboard
	// HTML endpoint with cookie-to-TLS fingerprint binding, so plain net/http
	// gets 302'd to accounts.google.com/CookieMismatch even with valid cookies.
	proxyClient := transport.NewUTLSHTTPClient(opts.Proxy)
	onSessionExpired := func(ctx context.Context) (*types.NotebookRpcSession, error) {
		log.Println("NotebookLM: Token expired, auto-refreshing...")
		refreshed, err := session.RefreshTokens(ctx, *sess, proxyClient, sessionPath)
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
	return c.callChatStreamWithState(ctx, c.getChatState(notebookID), notebookID, message, sourceIDs)
}

func (c *NotebookClient) callChatStreamWithState(ctx context.Context, state *chatState, notebookID, message string, sourceIDs []string) (string, error) {
	if err := c.EnsureConnected(); err != nil {
		return "", err
	}

	sess := c.transport.GetSession()
	sidsTriple := make([]any, len(sourceIDs))
	for i, id := range sourceIDs {
		sidsTriple[i] = []any{[]any{id}}
	}

	// The web UI sends `null` for the first turn on a fresh session and an
	// accumulating array thereafter. Sending `[]` instead of `null` for the
	// first turn causes the server to accept the message but not write it to
	// the notebook's visible chat thread on follow-ups.
	var history any
	if len(state.history) > 0 {
		h := make([]any, len(state.history))
		for i, e := range state.history {
			h[i] = e
		}
		history = h
	} else {
		history = nil
	}

	var threadIDField any
	if state.threadID != "" {
		threadIDField = state.threadID
	}

	innerPayload := []any{
		sidsTriple,
		message,
		history,
		[]any{2, nil, []any{1}, []any{1}},
		threadIDField,
		nil,
		nil,
		notebookID,
		state.turnCounter + 1,
	}
	innerJSON, _ := json.Marshal(innerPayload)
	fReq, _ := json.Marshal([]any{nil, string(innerJSON)})

	c.reqCounter += util.JitteredIncrement(100000, 0.3)
	hl := sess.Language
	if hl == "" {
		hl = "en"
	}
	qp := map[string]string{
		"bl":     sess.BL,
		"hl":     hl,
		"rt":     "c",
		"_reqid": fmt.Sprintf("%d", c.reqCounter),
	}
	if sess.FSID != "" {
		qp["f.sid"] = sess.FSID
	}

	return c.transport.Execute(ctx, transport.Request{
		URL:         rpc.ChatStreamURL,
		QueryParams: qp,
		Body: map[string]string{
			"f.req": string(fReq),
			"at":    sess.AT,
		},
	})
}

func (c *NotebookClient) getChatState(notebookID string) *chatState {
	c.chatMu.Lock()
	defer c.chatMu.Unlock()
	if s, ok := c.chatStates[notebookID]; ok {
		return s
	}
	s := &chatState{}
	c.chatStates[notebookID] = s
	return s
}

func (c *NotebookClient) bindChatThread(notebookID, threadID string) {
	if threadID == "" {
		return
	}
	state := c.getChatState(notebookID)
	c.chatMu.Lock()
	defer c.chatMu.Unlock()
	if state.threadID == threadID {
		return
	}
	state.threadID = threadID
	state.history = nil
	state.turnCounter = 0
}

// ensureChatThread resolves the notebook's default chat thread before sending.
// NotebookLM auto-allocates one default thread per notebook on creation, so
// hPTbtc should succeed. Empty result is best-effort — fall back to letting
// the server allocate a thread on first chat (matches pre-fix behaviour).
func (c *NotebookClient) ensureChatThread(ctx context.Context, notebookID string) (*chatState, error) {
	state := c.getChatState(notebookID)
	c.chatMu.Lock()
	if state.threadID != "" {
		c.chatMu.Unlock()
		return state, nil
	}
	c.chatMu.Unlock()

	threads, err := api.ListChatThreads(ctx, c.rpcCaller(), notebookID)
	if err != nil {
		return state, err
	}

	c.chatMu.Lock()
	defer c.chatMu.Unlock()
	if state.threadID == "" && len(threads) > 0 && threads[0] != "" {
		state.threadID = threads[0]
	}
	return state, nil
}

func (c *NotebookClient) recordChatTurn(state *chatState, message, replyText, replyThreadID string) {
	c.chatMu.Lock()
	defer c.chatMu.Unlock()
	if replyThreadID != "" && state.threadID == "" {
		state.threadID = replyThreadID
	}
	// Newest-first; assistant reply precedes user prompt within each turn —
	// matches the layout the web UI sends back on follow-ups.
	state.history = append([]chatHistoryEntry{{message, nil, 1}}, state.history...)
	if replyText != "" {
		state.history = append([]chatHistoryEntry{{replyText, nil, 2}}, state.history...)
	}
	state.turnCounter++
}

// chatLock returns (and lazily creates) the per-notebook serialization lock.
// Chat calls within the same notebook must run sequentially because history
// accumulates and the server expects monotonic turn counters.
func (c *NotebookClient) chatLock(notebookID string) *sync.Mutex {
	c.chatMu.Lock()
	defer c.chatMu.Unlock()
	if m, ok := c.chatLocks[notebookID]; ok {
		return m
	}
	m := &sync.Mutex{}
	c.chatLocks[notebookID] = m
	return m
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
	notebookID, threadID, err := api.CreateNotebook(ctx, c.rpcCaller())
	if err != nil {
		return "", err
	}
	if threadID != "" {
		c.bindChatThread(notebookID, threadID)
	}
	return notebookID, nil
}

// CreateNotebookFull returns both the notebook ID and the auto-allocated
// default chat thread ID. Prefer this when you need the thread up front.
func (c *NotebookClient) CreateNotebookFull(ctx context.Context) (notebookID, threadID string, err error) {
	notebookID, threadID, err = api.CreateNotebook(ctx, c.rpcCaller())
	if err != nil {
		return "", "", err
	}
	if threadID != "" {
		c.bindChatThread(notebookID, threadID)
	}
	return notebookID, threadID, nil
}

// ListChatThreads enumerates chat threads bound to a notebook. The first entry
// is the default thread the web UI uses; SendChat/SendChatWithCitations resolve
// it automatically, so call this only when enumerating or seeding a custom one.
func (c *NotebookClient) ListChatThreads(ctx context.Context, notebookID string) ([]string, error) {
	return api.ListChatThreads(ctx, c.rpcCaller(), notebookID)
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
		HTTPClient: transport.NewProxyHTTPClient(c.proxy),
	}, notebookID, filePath)
}

func (c *NotebookClient) DeleteSource(ctx context.Context, sourceID string) error {
	return api.DeleteSource(ctx, c.rpcCaller(), sourceID)
}

func (c *NotebookClient) GetSourceSummary(ctx context.Context, sourceID string) (string, error) {
	return api.GetSourceSummary(ctx, c.rpcCaller(), sourceID)
}

func (c *NotebookClient) RenameSource(ctx context.Context, notebookID, sourceID, newTitle string) error {
	return api.RenameSource(ctx, c.rpcCaller(), notebookID, sourceID, newTitle)
}

func (c *NotebookClient) RefreshSource(ctx context.Context, notebookID, sourceID string) error {
	return api.RefreshSourceData(ctx, c.rpcCaller(), notebookID, sourceID)
}

func (c *NotebookClient) ListNotes(ctx context.Context, notebookID string) ([]api.Note, error) {
	return api.ListNotes(ctx, c.rpcCaller(), notebookID)
}

func (c *NotebookClient) CreateNote(ctx context.Context, notebookID, title, content string) (string, error) {
	return api.CreateNote(ctx, c.rpcCaller(), notebookID, title, content)
}

func (c *NotebookClient) UpdateNote(ctx context.Context, notebookID, noteID, content, title string) error {
	return api.UpdateNote(ctx, c.rpcCaller(), notebookID, noteID, content, title)
}

func (c *NotebookClient) DeleteNote(ctx context.Context, notebookID, noteID string) error {
	return api.DeleteNote(ctx, c.rpcCaller(), notebookID, noteID)
}

func (c *NotebookClient) GetShareStatus(ctx context.Context, notebookID string) (any, error) {
	return api.GetShareStatus(ctx, c.rpcCaller(), notebookID)
}

func (c *NotebookClient) ShareNotebookPublic(ctx context.Context, notebookID string, isPublic bool) error {
	return api.ShareNotebook(ctx, c.rpcCaller(), notebookID, isPublic)
}

func (c *NotebookClient) ShareNotebookWithUser(ctx context.Context, notebookID, email, permission string, notify bool, message string) error {
	return api.ShareNotebookWithUser(ctx, c.rpcCaller(), notebookID, email, permission, notify, message)
}

func (c *NotebookClient) ImportResearch(ctx context.Context, notebookID, researchID string, results []types.ResearchResult, report string) error {
	return api.ImportResearch(ctx, c.rpcCaller(), notebookID, researchID, results, report)
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
	lock := c.chatLock(notebookID)
	lock.Lock()
	defer lock.Unlock()

	state, _ := c.ensureChatThread(ctx, notebookID)
	caller := func(ctx context.Context, nbID, msg string, sids []string) (string, error) {
		return c.callChatStreamWithState(ctx, state, nbID, msg, sids)
	}
	text, threadID, err := api.SendChat(ctx, caller, notebookID, message, sourceIDs)
	if err != nil {
		return "", "", err
	}
	c.recordChatTurn(state, message, text, threadID)
	return text, threadID, nil
}

func (c *NotebookClient) SendChatWithCitations(ctx context.Context, notebookID, message string, sourceIDs []string) (types.ChatWithCitationsResult, error) {
	lock := c.chatLock(notebookID)
	lock.Lock()
	defer lock.Unlock()

	state, _ := c.ensureChatThread(ctx, notebookID)
	caller := func(ctx context.Context, nbID, msg string, sids []string) (string, error) {
		return c.callChatStreamWithState(ctx, state, nbID, msg, sids)
	}
	result, err := api.SendChatWithCitations(ctx, caller, notebookID, message, sourceIDs)
	if err != nil {
		return types.ChatWithCitationsResult{}, err
	}
	c.recordChatTurn(state, message, result.Text, result.ThreadID)
	return result, nil
}

func (c *NotebookClient) DeleteChatThread(ctx context.Context, threadID string) error {
	return api.DeleteChatThread(ctx, c.rpcCaller(), threadID)
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
		Session:    sess,
		Proxy:      c.proxy,
		HTTPClient: transport.NewProxyHTTPClient(c.proxy),
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
