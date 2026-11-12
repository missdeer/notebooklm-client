package download

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

const shardSize int64 = 512 * 1024
const rangeBlockSize int64 = 256 * 1024
const minimumStolenSize int64 = 64 * 1024

var errRangeIgnored = errors.New("server ignored Range request")

func shardCount(size int64) int {
	if size <= shardSize {
		return 1
	}
	return int(min(int64(10), 1+(size-1)/shardSize))
}

// Offsets are half-open. next advances when a request is reserved, so stealing
// never touches an in-flight request, even if that request has to be retried.
type downloadShard struct{ next, end int64 }
type shardScheduler struct {
	mu     sync.Mutex
	shards []downloadShard
}

func newShardScheduler(size int64, workers int) *shardScheduler {
	s := &shardScheduler{shards: make([]downloadShard, workers)}
	for i := range s.shards {
		s.shards[i] = downloadShard{int64(i) * (size / int64(workers)), int64(i+1) * (size / int64(workers))}
	}
	s.shards[workers-1].end = size
	return s
}

func (s *shardScheduler) claim(worker int) (start, end int64, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	self := &s.shards[worker]
	if self.next == self.end {
		victim, most := -1, int64(0)
		for i, shard := range s.shards {
			if remaining := shard.end - shard.next; remaining > most {
				victim, most = i, remaining
			}
		}
		// Avoid an explosion of tiny requests as the file approaches completion.
		if most < 2*minimumStolenSize {
			return 0, 0, false
		}
		other := &s.shards[victim]
		self.next, self.end = other.next+most/2, other.end
		other.end = self.next
	}
	start, end = self.next, self.next+min(self.end-self.next, rangeBlockSize)
	self.next = end
	return start, end, start < end
}

// parseContentRange accepts only a concrete, satisfiable single byte range.
func parseContentRange(value string) (start, end, total int64, err error) {
	err = fmt.Errorf("invalid Content-Range %q", value)
	if !strings.HasPrefix(value, "bytes ") {
		return
	}
	span, size, ok := strings.Cut(strings.TrimPrefix(value, "bytes "), "/")
	if !ok {
		return
	}
	lo, hi, ok := strings.Cut(span, "-")
	if !ok {
		return
	}
	var e1, e2, e3 error
	start, e1 = strconv.ParseInt(lo, 10, 64)
	end, e2 = strconv.ParseInt(hi, 10, 64)
	total, e3 = strconv.ParseInt(size, 10, 64)
	if e1 == nil && e2 == nil && e3 == nil && start >= 0 && end >= start && total > end {
		err = nil
	}
	return
}

func downloadContent(ctx context.Context, client *http.Client, sess types.NotebookRpcSession, u, output string) error {
	resp, err := openDownloadResponse(ctx, client, sess, u, "bytes=0-0", "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	file, err := os.CreateTemp(filepath.Dir(output), "."+filepath.Base(output)+".*.part")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if resp.StatusCode == http.StatusPartialContent {
		start, end, total, err := parseContentRange(resp.Header.Get("Content-Range"))
		if err != nil {
			return err
		}
		if start != 0 || end != 0 {
			return fmt.Errorf("range probe returned unexpected offsets")
		}
		probe, err := io.ReadAll(io.LimitReader(resp.Body, 2))
		resp.Body.Close()
		if err != nil || len(probe) != 1 {
			return fmt.Errorf("range probe did not return exactly one byte")
		}
		validator := resp.Header.Get("ETag")
		if validator == "" || strings.HasPrefix(validator, "W/") {
			validator = resp.Header.Get("Last-Modified")
		}
		if workers := shardCount(total); workers > 1 {
			log.Printf("Gemini Notebook: Downloading %d bytes with %d workers", total, workers)
			err = downloadShards(ctx, client, sess, u, file, total, workers, validator)
			if err != nil && !errors.Is(err, errRangeIgnored) {
				return err
			}
			if err == nil {
				return finishDownload(file, output)
			}
			log.Print("Gemini Notebook: Server ignored Range; restarting as a single download")
		}
		// Small files and servers that stop honoring Range use one full response.
		if err := file.Truncate(0); err != nil {
			return err
		}
		resp, err = openDownloadResponse(ctx, client, sess, u, "", "")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected full download, got HTTP %d", resp.StatusCode)
	}
	n, err := io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("download read: %w", err)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return io.ErrUnexpectedEOF
	}
	return finishDownload(file, output)
}

func finishDownload(file *os.File, output string) error {
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), output)
}

func downloadShards(ctx context.Context, client *http.Client, sess types.NotebookRpcSession, u string, file *os.File, total int64, workers int, validator string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := file.Truncate(total); err != nil {
		return err
	}
	scheduler := newShardScheduler(total, workers)
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error
	for worker := 0; worker < workers; worker++ {
		wg.Go(func() {
			for ctx.Err() == nil {
				start, end, ok := scheduler.claim(worker)
				if !ok {
					return
				}
				if err := downloadRange(ctx, client, sess, u, file, start, end, total, validator); err != nil {
					once.Do(func() { firstErr = err; cancel() })
					return
				}
			}
		})
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

func downloadRange(ctx context.Context, client *http.Client, sess types.NotebookRpcSession, u string, file *os.File, start, end, total int64, validator string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		var retry bool
		retry, lastErr = downloadRangeAttempt(ctx, client, sess, u, file, start, end, total, validator)
		if lastErr == nil || !retry {
			return lastErr
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return fmt.Errorf("range download failed: %w", lastErr)
}

func downloadRangeAttempt(ctx context.Context, client *http.Client, sess types.NotebookRpcSession, u string, file *os.File, start, end, total int64, validator string) (retry bool, err error) {
	resp, err := openDownloadResponse(ctx, client, sess, u, fmt.Sprintf("bytes=%d-%d", start, end-1), validator)
	if err != nil {
		return errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF), err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return false, errRangeIgnored
	}
	lo, hi, size, err := parseContentRange(resp.Header.Get("Content-Range"))
	if err != nil || resp.StatusCode != http.StatusPartialContent || lo != start || hi != end-1 || size != total {
		return false, fmt.Errorf("range response does not match requested bytes %d-%d/%d", start, end-1, total)
	}
	if tag := resp.Header.Get("ETag"); validator != "" && tag != "" && strings.HasPrefix(validator, "\"") && tag != validator {
		return false, fmt.Errorf("artifact changed during download")
	}
	if _, err := io.CopyN(io.NewOffsetWriter(file, start), resp.Body, end-start); err != nil {
		return true, err
	}
	var extra [1]byte
	if n, err := io.ReadFull(resp.Body, extra[:]); n != 0 || err != io.EOF {
		return false, fmt.Errorf("range response contains extra data or has no EOF")
	}
	return false, nil
}

func openDownloadResponse(ctx context.Context, client *http.Client, sess types.NotebookRpcSession, u, byteRange, validator string) (*http.Response, error) {
	for attempt := 1; attempt <= 10; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		ua := sess.UserAgent
		if ua == "" {
			ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("Referer", rpc.SessionBaseURL(sess.BaseHost)+"/")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-User", "?1")
		if byteRange != "" {
			req.Header.Set("Range", byteRange)
		}
		if validator != "" {
			req.Header.Set("If-Range", validator)
		}
		resp, err := client.Do(req)
		if err == nil {
			reader := bufio.NewReader(resp.Body)
			preview, peekErr := reader.Peek(512)
			if peekErr != nil && peekErr != io.EOF {
				resp.Body.Close()
				return nil, peekErr
			}
			looksHTML := resp.StatusCode != http.StatusPartialContent && isHTMLResponse(string(preview))
			if isAuthFailure(string(preview)) && looksHTML {
				resp.Body.Close()
				return nil, fmt.Errorf("download auth failure: login page returned (cookies may be expired)")
			}
			if resp.StatusCode != 404 && !looksHTML {
				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
					resp.Body.Close()
					return nil, fmt.Errorf("download HTTP %d", resp.StatusCode)
				}
				if encoding := resp.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
					resp.Body.Close()
					return nil, fmt.Errorf("unexpected download encoding %s", encoding)
				}
				resp.Body = &bufferedDownloadBody{Reader: reader, Closer: resp.Body}
				return resp, nil
			}
			status := resp.StatusCode
			resp.Body.Close()
			if status != 404 && status != http.StatusOK {
				return nil, fmt.Errorf("download HTTP %d", status)
			}
			err = fmt.Errorf("CDN not ready (HTTP %d)", status)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt == 10 {
			return nil, fmt.Errorf("download failed after %d attempts: %w", attempt, err)
		}
		log.Printf("Gemini Notebook: Download not ready (attempt %d/10), retrying...", attempt)
		if err := waitDownloadRetry(ctx, attempt); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("download failed")
}

type bufferedDownloadBody struct {
	io.Reader
	io.Closer
}
