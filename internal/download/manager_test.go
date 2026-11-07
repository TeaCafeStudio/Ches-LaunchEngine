package download

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadVerifiesAndCachesFile(t *testing.T) {
	payload := []byte("verified payload")
	sum := sha1.Sum(payload)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	manager, err := New(Options{Concurrency: 2, Retries: 0, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "cache", "artifact.jar")
	request := Request{
		URL: server.URL, Path: path, SHA1: hex.EncodeToString(sum[:]), Size: int64(len(payload)),
	}
	if err := manager.Download(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Fatalf("downloaded %q, want %q", data, payload)
	}
	if err := manager.Download(context.Background(), request); err != nil {
		t.Fatalf("verified cache was not reused: %v", err)
	}
}

func TestDownloadReportsProgressAndCacheCompletion(t *testing.T) {
	payload := []byte(strings.Repeat("progress", 4096))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	manager, err := New(Options{Concurrency: 1, Retries: 0, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{URL: server.URL, Path: filepath.Join(t.TempDir(), "asset"), Size: int64(len(payload))}
	var progress []Progress
	collect := func(event Progress) { progress = append(progress, event) }
	if err := manager.DownloadManyWithSettingsAndProgress(context.Background(), []Request{request}, 1, 0, false, collect); err != nil {
		t.Fatal(err)
	}
	if len(progress) < 2 || !progress[len(progress)-1].Completed {
		t.Fatalf("download progress did not include completion: %+v", progress)
	}
	if err := manager.DownloadManyWithSettingsAndProgress(context.Background(), []Request{request}, 1, 0, false, collect); err != nil {
		t.Fatal(err)
	}
	last := progress[len(progress)-1]
	if !last.Completed || !last.Cached {
		t.Fatalf("cached download progress = %+v", last)
	}
}

func TestDownloadDoesNotReplaceFileOnHashFailure(t *testing.T) {
	oldData := []byte("previous file")
	newData := []byte("untrusted response")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(newData)
	}))
	defer server.Close()
	manager, err := New(Options{Concurrency: 1, Retries: 0, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "artifact.jar")
	if err := os.WriteFile(path, oldData, 0600); err != nil {
		t.Fatal(err)
	}
	request := Request{URL: server.URL, Path: path, SHA1: strings.Repeat("0", 40)}
	if err := manager.Download(context.Background(), request); err == nil {
		t.Fatal("Download() succeeded with an incorrect SHA-1")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(oldData) {
		t.Fatalf("failed download replaced cached file with %q", got)
	}
}

func TestDownloadManyHonorsOfflineModeAndCancellation(t *testing.T) {
	manager, err := New(Options{Concurrency: 2, Retries: 0, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	err = manager.DownloadManyWithOptions(context.Background(), []Request{{
		URL: "https://example.invalid/file", Path: filepath.Join(t.TempDir(), "missing"),
	}}, 1, true)
	if err == nil || !strings.Contains(err.Error(), "offline mode") {
		t.Fatalf("offline download error = %v, want offline cache miss", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := manager.DownloadMany(ctx, []Request{{URL: "https://example.invalid/file", Path: "unused"}}, 1); err == nil {
		t.Fatal("DownloadMany() ignored context cancellation")
	}
}

func TestResolveUnderRootRejectsTraversalAndSymlinks(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveUnderRoot(root, "..\\escape"); err == nil {
		t.Fatal("ResolveUnderRoot() accepted a traversal path")
	}
	link := filepath.Join(root, "linked")
	target := t.TempDir()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if _, err := ResolveUnderRoot(root, filepath.Join("linked", "file")); err == nil {
		t.Fatal("ResolveUnderRoot() accepted a symlink parent")
	}
	targetFile := filepath.Join(target, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target"), 0600); err != nil {
		t.Fatal(err)
	}
	leafLink := filepath.Join(root, "leaf-link")
	if err := os.Symlink(targetFile, leafLink); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if _, err := ResolveUnderRoot(root, "leaf-link"); err == nil {
		t.Fatal("ResolveUnderRoot() accepted a symlink destination")
	}
}
