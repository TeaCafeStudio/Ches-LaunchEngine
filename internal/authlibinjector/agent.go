package authlibinjector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/download"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
)

const (
	version        = "1.2.8"
	artifactURL    = "https://authlib-injector.yushi.moe/artifact/56/authlib-injector-1.2.8.jar"
	artifactSHA256 = "9c7f4343e6c82034958ffb48c14a2cb0c85928be7283103ce17da00c6d5a7b10"
	maxArtifact    = 8 << 20
)

func Resolve(ctx context.Context, gameDirectory string, offline bool) (string, error) {
	relativePath := filepath.Join("libraries", "org", "authlib-injector", version,
		"authlib-injector-"+version+".jar")
	path, err := download.ResolveUnderRoot(gameDirectory, relativePath)
	if err != nil {
		return "", cleerrors.Wrap(cleerrors.CodeAuthlibInjectorUnavailable, "resolve Authlib Injector cache path", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", cleerrors.Wrap(cleerrors.CodeAuthlibInjectorUnavailable, "create Authlib Injector cache directory", err)
	}
	if valid, err := verify(path, artifactSHA256); err != nil {
		return "", cleerrors.Wrap(cleerrors.CodeAuthlibInjectorUnavailable, "inspect Authlib Injector cache", err)
	} else if valid {
		return path, nil
	}
	if offline {
		return "", cleerrors.New(cleerrors.CodeAuthlibInjectorUnavailable, "a verified Authlib Injector is not available in the local cache while offline mode is enabled")
	}
	if err := fetch(ctx, path, artifactURL, artifactSHA256, secureClient()); err != nil {
		return "", err
	}
	return path, nil
}

func secureClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func fetch(ctx context.Context, path, sourceURL, expectedSHA256 string, client *http.Client) error {
	if !strings.HasPrefix(sourceURL, "https://authlib-injector.yushi.moe/") ||
		len(expectedSHA256) != sha256.Size*2 {
		return cleerrors.New(cleerrors.CodeAuthlibInjectorDownloadFailed, "Authlib Injector source or integrity metadata is invalid")
	}
	expected, err := hex.DecodeString(expectedSHA256)
	if err != nil {
		return cleerrors.New(cleerrors.CodeAuthlibInjectorDownloadFailed, "Authlib Injector integrity metadata is invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return cleerrors.New(cleerrors.CodeAuthlibInjectorDownloadFailed, "could not prepare Authlib Injector download")
	}
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return cleerrors.New(cleerrors.CodeAuthlibInjectorDownloadFailed, "could not download Authlib Injector from its official distribution service")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return cleerrors.New(cleerrors.CodeAuthlibInjectorDownloadFailed, fmt.Sprintf("official Authlib Injector service returned HTTP %d", response.StatusCode))
	}
	if response.ContentLength > maxArtifact {
		return cleerrors.New(cleerrors.CodeAuthlibInjectorDownloadFailed, "Authlib Injector artifact exceeds the permitted size")
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".authlib-injector-*.tmp")
	if err != nil {
		return cleerrors.Wrap(cleerrors.CodeAuthlibInjectorUnavailable, "create Authlib Injector cache file", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(temp, hasher), io.LimitReader(response.Body, maxArtifact+1))
	if err != nil {
		_ = temp.Close()
		return cleerrors.New(cleerrors.CodeAuthlibInjectorDownloadFailed, "could not write Authlib Injector artifact")
	}
	if written > maxArtifact {
		_ = temp.Close()
		return cleerrors.New(cleerrors.CodeAuthlibInjectorDownloadFailed, "Authlib Injector artifact exceeds the permitted size")
	}
	if !equalBytes(hasher.Sum(nil), expected) {
		_ = temp.Close()
		return cleerrors.New(cleerrors.CodeAuthlibInjectorIntegrityFailed, "Authlib Injector SHA-256 verification failed")
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return cleerrors.Wrap(cleerrors.CodeAuthlibInjectorUnavailable, "sync Authlib Injector cache file", err)
	}
	if err := temp.Close(); err != nil {
		return cleerrors.Wrap(cleerrors.CodeAuthlibInjectorUnavailable, "close Authlib Injector cache file", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return cleerrors.Wrap(cleerrors.CodeAuthlibInjectorUnavailable, "install Authlib Injector in cache", err)
	}
	return nil
}

func verify(path, expectedSHA256 string) (bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, errors.New("cached Authlib Injector is not a regular file")
	}
	hasher := sha256.New()
	written, err := io.Copy(hasher, io.LimitReader(file, maxArtifact+1))
	if err != nil {
		return false, err
	}
	if written > maxArtifact {
		return false, nil
	}
	expected, err := hex.DecodeString(expectedSHA256)
	if err != nil {
		return false, err
	}
	return equalBytes(hasher.Sum(nil), expected), nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}
