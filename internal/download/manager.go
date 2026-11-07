package download

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Options struct {
	Concurrency int
	Retries     int
	Timeout     time.Duration
	Client      *http.Client
	Offline     bool
}

type Manager struct {
	client  *http.Client
	retries int
	offline bool
}

type Request struct {
	URL    string
	Path   string
	SHA1   string
	SHA256 string
	Size   int64
}

type Progress struct {
	Path      string
	Received  int64
	Total     int64
	Cached    bool
	Completed bool
}

func New(options Options) (*Manager, error) {
	if options.Concurrency < 1 || options.Concurrency > 64 {
		return nil, fmt.Errorf("download concurrency must be between 1 and 64")
	}
	if options.Retries < 0 || options.Retries > 10 {
		return nil, fmt.Errorf("download retries must be between 0 and 10")
	}
	if options.Timeout <= 0 {
		options.Timeout = 30 * time.Second
	}
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: options.Timeout}
	}
	return &Manager{client: client, retries: options.Retries, offline: options.Offline}, nil
}

func (manager *Manager) Download(ctx context.Context, request Request) error {
	return manager.download(ctx, request, manager.offline, manager.retries, nil)
}

func (manager *Manager) download(ctx context.Context, request Request, offline bool, retries int, progress func(Progress)) error {
	if err := validateRequest(request); err != nil {
		return err
	}
	if valid, err := verifyFile(request); err == nil && valid {
		if progress != nil {
			progress(Progress{Path: request.Path, Received: request.Size, Total: request.Size, Cached: true, Completed: true})
		}
		return nil
	}
	if offline {
		return fmt.Errorf("required file is missing or invalid in offline mode: %s", request.Path)
	}

	var lastErr error
	if progress != nil {
		progress(Progress{Path: request.Path, Total: request.Size})
	}
	for attempt := 0; attempt <= retries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := manager.downloadOnce(ctx, request, progress); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < retries {
			timer := time.NewTimer(time.Duration(attempt+1) * 250 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return fmt.Errorf("download %s: %w", request.URL, lastErr)
}

func (manager *Manager) DownloadMany(ctx context.Context, requests []Request, concurrency int) error {
	return manager.DownloadManyWithSettings(ctx, requests, concurrency, manager.retries, manager.offline)
}

func (manager *Manager) DownloadManyWithOptions(ctx context.Context, requests []Request, concurrency int, offline bool) error {
	return manager.DownloadManyWithSettings(ctx, requests, concurrency, manager.retries, offline)
}

func (manager *Manager) DownloadManyWithSettings(ctx context.Context, requests []Request, concurrency, retries int, offline bool) error {
	return manager.DownloadManyWithSettingsAndProgress(ctx, requests, concurrency, retries, offline, nil)
}

func (manager *Manager) DownloadManyWithSettingsAndProgress(ctx context.Context, requests []Request, concurrency, retries int, offline bool, progress func(Progress)) error {
	if concurrency < 1 || concurrency > 64 {
		return fmt.Errorf("download concurrency must be between 1 and 64")
	}
	if retries < 0 || retries > 10 {
		return fmt.Errorf("download retries must be between 0 and 10")
	}
	if len(requests) == 0 {
		return nil
	}

	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan Request)
	errorsFound := make(chan error, 1)
	var workers sync.WaitGroup
	reportProgress := progress
	if progress != nil {
		var progressMu sync.Mutex
		reportProgress = func(event Progress) {
			progressMu.Lock()
			defer progressMu.Unlock()
			progress(event)
		}
	}
	workerCount := concurrency
	if len(requests) < workerCount {
		workerCount = len(requests)
	}

	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for request := range jobs {
				if err := manager.download(workCtx, request, offline, retries, reportProgress); err != nil {
					select {
					case errorsFound <- err:
						cancel()
					default:
					}
					return
				}
			}
		}()
	}
	workersDone := make(chan struct{})
	go func() {
		workers.Wait()
		close(workersDone)
	}()

	for _, request := range requests {
		select {
		case <-workCtx.Done():
			close(jobs)
			<-workersDone
			select {
			case err := <-errorsFound:
				return err
			default:
			}
			return workCtx.Err()
		case jobs <- request:
		}
	}
	close(jobs)

	for {
		select {
		case err := <-errorsFound:
			<-workersDone
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-workersDone:
			return nil
		}
	}
}

func (manager *Manager) downloadOnce(ctx context.Context, request Request, progress func(Progress)) error {
	if err := os.MkdirAll(filepath.Dir(request.Path), 0755); err != nil {
		return fmt.Errorf("create download directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(request.Path), ".cle-download-*")
	if err != nil {
		return fmt.Errorf("create temporary download file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, request.URL, nil)
	if err != nil {
		temp.Close()
		return fmt.Errorf("create download request: %w", err)
	}
	response, err := manager.client.Do(httpRequest)
	if err != nil {
		temp.Close()
		return fmt.Errorf("request download: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		temp.Close()
		return fmt.Errorf("unexpected HTTP status %s", response.Status)
	}

	hashers := make([]hash.Hash, 0, 2)
	if request.SHA1 != "" {
		hashers = append(hashers, sha1.New())
	}
	if request.SHA256 != "" {
		hashers = append(hashers, sha256.New())
	}
	writers := []io.Writer{temp}
	for _, hasher := range hashers {
		writers = append(writers, hasher)
	}
	var written int64
	var copyErr error
	reader := io.Reader(response.Body)
	if progress != nil {
		reader = &progressReader{
			reader: response.Body,
			total:  request.Size,
			report: func(received int64) {
				progress(Progress{Path: request.Path, Received: received, Total: request.Size})
			},
		}
	}
	written, copyErr = io.Copy(io.MultiWriter(writers...), reader)
	bodyErr := response.Body.Close()
	if copyErr != nil {
		temp.Close()
		return fmt.Errorf("write downloaded file: %w", copyErr)
	}
	if bodyErr != nil {
		temp.Close()
		return fmt.Errorf("close download response: %w", bodyErr)
	}
	if request.Size > 0 && written != request.Size {
		temp.Close()
		return fmt.Errorf("download size mismatch: got %d bytes, expected %d", written, request.Size)
	}

	hashIndex := 0
	if request.SHA1 != "" {
		actual := hex.EncodeToString(hashers[hashIndex].Sum(nil))
		hashIndex++
		if !strings.EqualFold(actual, request.SHA1) {
			temp.Close()
			return fmt.Errorf("SHA-1 mismatch: got %s, expected %s", actual, request.SHA1)
		}
	}
	if request.SHA256 != "" {
		actual := hex.EncodeToString(hashers[hashIndex].Sum(nil))
		if !strings.EqualFold(actual, request.SHA256) {
			temp.Close()
			return fmt.Errorf("SHA-256 mismatch: got %s, expected %s", actual, request.SHA256)
		}
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync downloaded file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close downloaded file: %w", err)
	}
	if err := os.Rename(tempPath, request.Path); err != nil {
		return fmt.Errorf("install downloaded file: %w", err)
	}
	if progress != nil {
		progress(Progress{Path: request.Path, Received: written, Total: request.Size, Completed: true})
	}
	return nil
}

type progressReader struct {
	reader    io.Reader
	total     int64
	received  int64
	lastEvent time.Time
	report    func(int64)
}

func (reader *progressReader) Read(buffer []byte) (int, error) {
	n, err := reader.reader.Read(buffer)
	reader.received += int64(n)
	if n > 0 && (reader.received == reader.total || time.Since(reader.lastEvent) >= 200*time.Millisecond) {
		reader.lastEvent = time.Now()
		reader.report(reader.received)
	}
	return n, err
}

func validateRequest(request Request) error {
	if request.URL == "" || request.Path == "" {
		return errors.New("download URL and destination path are required")
	}
	if request.SHA1 != "" && len(request.SHA1) != sha1.Size*2 {
		return errors.New("SHA-1 must contain 40 hexadecimal characters")
	}
	if request.SHA256 != "" && len(request.SHA256) != sha256.Size*2 {
		return errors.New("SHA-256 must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(request.SHA1); err != nil && request.SHA1 != "" {
		return errors.New("SHA-1 contains invalid hexadecimal characters")
	}
	if _, err := hex.DecodeString(request.SHA256); err != nil && request.SHA256 != "" {
		return errors.New("SHA-256 contains invalid hexadecimal characters")
	}
	return nil
}

func verifyFile(request Request) (bool, error) {
	file, err := os.Open(request.Path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if request.Size > 0 && info.Size() != request.Size {
		return false, nil
	}
	if request.SHA1 == "" && request.SHA256 == "" {
		return true, nil
	}
	hashers := make([]hash.Hash, 0, 2)
	if request.SHA1 != "" {
		hashers = append(hashers, sha1.New())
	}
	if request.SHA256 != "" {
		hashers = append(hashers, sha256.New())
	}
	writers := make([]io.Writer, len(hashers))
	for index, hasher := range hashers {
		writers[index] = hasher
	}
	if _, err := io.Copy(io.MultiWriter(writers...), file); err != nil {
		return false, err
	}
	index := 0
	if request.SHA1 != "" {
		if !strings.EqualFold(hex.EncodeToString(hashers[index].Sum(nil)), request.SHA1) {
			return false, nil
		}
		index++
	}
	if request.SHA256 != "" && !strings.EqualFold(hex.EncodeToString(hashers[index].Sum(nil)), request.SHA256) {
		return false, nil
	}
	return true, nil
}
