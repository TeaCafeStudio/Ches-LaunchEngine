package vanilla

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/download"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/metadata"
)

func TestInstallFilesDownloadsAssetsLibrariesAndExtractsNatives(t *testing.T) {
	nativeJar := zipBytes(t, map[string]string{
		"META-INF/signature.SF": "signature",
		"native/game.dll":       "native payload",
	})
	libraryJar := []byte("library jar")
	clientJar := []byte("client jar")
	asset := []byte("asset data")
	assetHash := sha1Hex(asset)
	indexData, err := json.Marshal(metadata.AssetIndexData{
		Objects: map[string]metadata.AssetObject{"icons/icon.png": {Hash: assetHash, Size: int64(len(asset))}},
	})
	if err != nil {
		t.Fatal(err)
	}

	files := map[string][]byte{
		"/native.jar": nativeJar, "/library.jar": libraryJar,
		"/client.jar": clientJar, "/asset-index.json": indexData, "/asset": asset,
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, ok := files[request.URL.Path]
		if request.URL.Path == "/"+assetHash[:2]+"/"+assetHash {
			data, ok = asset, true
		}
		if !ok {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write(data)
	}))
	defer server.Close()

	gameDirectory := t.TempDir()
	versionDirectory := filepath.Join(gameDirectory, "versions", "test")
	version := metadata.VersionMetadata{
		ID: "test", Assets: "test-assets",
		Downloads: map[string]metadata.Artifact{
			"client": {URL: server.URL + "/client.jar", SHA1: sha1Hex(clientJar), Size: int64(len(clientJar))},
		},
		AssetIndex: &metadata.AssetIndex{
			ID: "test-assets", URL: server.URL + "/asset-index.json",
			SHA1: sha1Hex(indexData), Size: int64(len(indexData)),
		},
		Libraries: []metadata.Library{
			{
				Name: "org.example:library:1.0",
				Downloads: &metadata.LibraryDownloads{Artifact: &metadata.Artifact{
					Path: "org/example/library/1.0/library-1.0.jar",
					URL:  server.URL + "/library.jar", SHA1: sha1Hex(libraryJar), Size: int64(len(libraryJar)),
				}},
			},
			{
				Name:  "org.example:native:1.0:natives-windows",
				Rules: []metadata.Rule{{Action: "allow", OS: &metadata.OSRule{Name: "windows"}}},
				Downloads: &metadata.LibraryDownloads{Artifact: &metadata.Artifact{
					Path: "org/example/native/1.0/native-1.0-natives-windows.jar",
					URL:  server.URL + "/native.jar", SHA1: sha1Hex(nativeJar), Size: int64(len(nativeJar)),
				}},
			},
		},
	}
	transport := rewriteTransport{target: server.URL, base: http.DefaultTransport}
	manager, err := download.New(download.Options{
		Concurrency: 3, Retries: 0, Timeout: time.Second,
		Client: &http.Client{Transport: transport, Timeout: time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := installFiles(context.Background(), manager, version, gameDirectory, versionDirectory,
		metadata.RuleContext{OS: "windows", Arch: "x86_64"}, 3, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Classpath) != 2 || !strings.HasSuffix(result.Classpath[0], "library-1.0.jar") ||
		!strings.HasSuffix(result.Classpath[1], "test.jar") {
		t.Fatalf("unexpected classpath: %v", result.Classpath)
	}
	extracted, err := os.ReadFile(filepath.Join(result.NativesPath, "native", "game.dll"))
	if err != nil {
		t.Fatal(err)
	}
	if string(extracted) != "native payload" {
		t.Fatalf("extracted native = %q", extracted)
	}
	if _, err := os.Stat(filepath.Join(result.NativesPath, "META-INF", "signature.SF")); !os.IsNotExist(err) {
		t.Fatalf("signature file was extracted, stat error: %v", err)
	}
	downloadedAsset, err := os.ReadFile(filepath.Join(gameDirectory, "assets", "objects", assetHash[:2], assetHash))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(downloadedAsset, asset) {
		t.Fatalf("downloaded asset = %q, want %q", downloadedAsset, asset)
	}
}

type rewriteTransport struct {
	target string
	base   http.RoundTripper
}

func (transport rewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Host == "resources.download.minecraft.net" {
		target, err := url.Parse(transport.target)
		if err != nil {
			return nil, err
		}
		copy := request.Clone(request.Context())
		copy.URL.Scheme = target.Scheme
		copy.URL.Host = target.Host
		copy.Host = target.Host
		return transport.base.RoundTrip(copy)
	}
	return transport.base.RoundTrip(request)
}

func TestExtractNativeRejectsZipSlip(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "unsafe.jar")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../escaped.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("unsafe"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(t.TempDir(), "natives")
	if err := os.MkdirAll(destination, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractNative(archivePath, destination, nil); err == nil {
		t.Fatal("extractNative() accepted a Zip Slip entry")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(destination), "escaped.txt")); !os.IsNotExist(err) {
		t.Fatalf("Zip Slip wrote outside destination: %v", err)
	}
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func sha1Hex(data []byte) string {
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}
