package download

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestShardCountBoundaries(t *testing.T) {
	for _, tc := range []struct {
		size    int64
		workers int
	}{
		{0, 1}, {shardSize, 1}, {shardSize + 1, 2}, {2 * shardSize, 2},
		{2*shardSize + 1, 3}, {10 * shardSize, 10}, {30 * shardSize, 10},
	} {
		if got := shardCount(tc.size); got != tc.workers {
			t.Fatalf("%d bytes: %d workers, want %d", tc.size, got, tc.workers)
		}
	}
}

func TestIdleWorkerStealsLargestUnreservedTail(t *testing.T) {
	s := newShardScheduler(6*shardSize, 3)
	// Worker 1 has work in flight. Worker 2 still has its entire shard left.
	s.claim(1)
	_, last, _ := s.claim(0)
	for last < 2*shardSize {
		_, last, _ = s.claim(0)
	}
	start, end, ok := s.claim(0)
	if !ok || start < 4*shardSize || end > 6*shardSize {
		t.Fatalf("did not steal from the largest remaining shard: %d-%d", start, end)
	}
	// Reserved work and stolen work stay disjoint, and all bytes are covered.
	if s.shards[2].end != start || s.shards[1].end != 4*shardSize {
		t.Fatal("steal moved in-flight work or selected the wrong victim")
	}
}

func TestParallelDownloadIntegrityAndConcurrency(t *testing.T) {
	data := bytes.Repeat([]byte("abcdefgh01234567"), int(7*shardSize/16)+1)
	var active, peak atomic.Int32
	var arrivals atomic.Int32
	barrier := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") != "identity" {
			t.Error("range offsets require identity encoding")
		}
		if r.Header.Get("Range") != "bytes=0-0" {
			if r.Header.Get("If-Range") != `"stable"` {
				t.Error("missing range validator")
			}
			current := active.Add(1)
			defer active.Add(-1)
			for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
			}
			if arrivals.Add(1) == 2 {
				close(barrier)
			}
			select {
			case <-barrier:
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("ETag", `"stable"`)
		http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	path, err := DownloadFileHTTP(ctx, Deps{}, server.URL, t.TempDir(), "file.bin")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("assembled file does not match source: %v", err)
	}
	if peak.Load() < 2 || peak.Load() > 8 {
		t.Fatalf("unexpected parallelism %d", peak.Load())
	}
}

func TestRangeIgnoredFallback(t *testing.T) {
	data := bytes.Repeat([]byte("01234567"), int(shardSize/8)+1)
	for _, ignoreProbe := range []bool{true, false} {
		t.Run(fmt.Sprint(ignoreProbe), func(t *testing.T) {
			var full atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !ignoreProbe && r.Header.Get("Range") == "bytes=0-0" {
					http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
					return
				}
				if r.Header.Get("Range") == "" {
					full.Add(1)
				}
				w.Write(data)
			}))
			defer server.Close()
			path, err := DownloadFileHTTP(context.Background(), Deps{}, server.URL, t.TempDir(), "file.bin")
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatal("fallback produced corrupt output")
			}
			if !ignoreProbe && full.Load() != 1 {
				t.Fatal("range fallback did not restart full download")
			}
		})
	}
}

func TestMalformedRangePreservesExistingFile(t *testing.T) {
	data := bytes.Repeat([]byte("x"), int(shardSize)+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "bytes=0-0" {
			http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
			return
		}
		w.Header().Set("Content-Range", "bytes 0-31/32")
		w.WriteHeader(206)
		w.Write(bytes.Repeat([]byte("x"), 32))
	}))
	defer server.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(path, []byte("previous output"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := DownloadFileHTTP(context.Background(), Deps{}, server.URL, dir, "file.bin")
	if err == nil || !strings.Contains(err.Error(), "range response") {
		t.Fatalf("bad range was accepted: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "previous output" {
		t.Fatal("failed download replaced existing output")
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal("failed download left temporary files")
	}
}

func TestParallelDownloadCancellation(t *testing.T) {
	data := bytes.Repeat([]byte("x"), int(shardSize)+1)
	started := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "bytes=0-0" {
			http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
			return
		}
		once.Do(func() { close(started) })
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		select {
		case <-started:
			cancel()
		case <-ctx.Done():
		}
	}()
	dir := t.TempDir()
	_, err := DownloadFileHTTP(ctx, Deps{}, server.URL, dir, "file.bin")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 0 {
		t.Fatal("canceled download left output or temporary files")
	}
}

func TestTruncatedRangeRetriesWithoutHoles(t *testing.T) {
	data := bytes.Repeat([]byte("01234567"), int(shardSize/8)+1)
	var truncated atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "bytes=0-262143" && truncated.CompareAndSwap(false, true) {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-262143/%d", len(data)))
			w.Header().Set("Content-Length", "262144")
			w.WriteHeader(http.StatusPartialContent)
			w.Write(data[:100])
			return
		}
		http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
	}))
	defer server.Close()
	path, err := DownloadFileHTTP(context.Background(), Deps{}, server.URL, t.TempDir(), "file.bin")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if !truncated.Load() || err != nil || !bytes.Equal(got, data) {
		t.Fatal("range retry left missing or duplicate data")
	}
}
