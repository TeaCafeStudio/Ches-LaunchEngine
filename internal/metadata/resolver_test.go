package metadata

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/download"
)

func TestResolverMergesInheritedMetadataAndSupportsOfflineCache(t *testing.T) {
	parent := VersionMetadata{
		ID: "parent", Type: "release", MainClass: "example.Parent",
		Libraries: []Library{{Name: "org.example:shared:1", Downloads: &LibraryDownloads{
			Artifact: &Artifact{Path: "org/example/shared/1/shared-1.jar"},
		}}},
		Arguments: &Arguments{Game: []json.RawMessage{json.RawMessage(`"--parent"`)}},
	}
	child := VersionMetadata{
		ID: "child", Type: "release", MainClass: "example.Child", InheritsFrom: "parent",
		Libraries: []Library{{Name: "org.example:shared:2", Downloads: &LibraryDownloads{
			Artifact: &Artifact{Path: "org/example/shared/2/shared-2.jar"},
		}}},
		Arguments: &Arguments{Game: []json.RawMessage{json.RawMessage(`"--child"`)}},
	}
	parentData, _ := json.Marshal(parent)
	childData, _ := json.Marshal(child)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/manifest":
			_ = json.NewEncoder(writer).Encode(VersionManifest{Versions: []ManifestEntry{
				{ID: "parent", URL: serverURL(request, "/parent"), SHA1: hashSHA1(parentData)},
				{ID: "child", URL: serverURL(request, "/child"), SHA1: hashSHA1(childData)},
			}})
		case "/parent":
			_, _ = writer.Write(parentData)
		case "/child":
			_, _ = writer.Write(childData)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	manager, err := download.New(download.Options{Concurrency: 2, Retries: 0, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := NewResolver(ResolverOptions{Downloader: manager, ManifestURL: server.URL + "/manifest"})
	if err != nil {
		t.Fatal(err)
	}
	gameDirectory := t.TempDir()
	got, err := resolver.Resolve(context.Background(), gameDirectory, "child", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.MainClass != child.MainClass || len(got.Libraries) != 1 || got.Libraries[0].Name != "org.example:shared:2" {
		t.Fatalf("incorrect inherited metadata merge: %+v", got)
	}
	if len(got.Arguments.Game) != 2 {
		t.Fatalf("merged game arguments = %d, want parent and child arguments", len(got.Arguments.Game))
	}

	server.Close()
	cached, err := resolver.Resolve(context.Background(), gameDirectory, "child", true)
	if err != nil {
		t.Fatalf("offline cached resolve failed: %v", err)
	}
	if cached.ID != "child" || cached.MainClass != child.MainClass {
		t.Fatalf("offline metadata = %+v", cached)
	}
}

func TestResolverDetectsInheritanceCycle(t *testing.T) {
	first := VersionMetadata{ID: "first", MainClass: "example.Main", InheritsFrom: "second"}
	second := VersionMetadata{ID: "second", MainClass: "example.Main", InheritsFrom: "first"}
	firstData, _ := json.Marshal(first)
	secondData, _ := json.Marshal(second)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/manifest":
			_ = json.NewEncoder(writer).Encode(VersionManifest{Versions: []ManifestEntry{
				{ID: "first", URL: server.URL + "/first", SHA1: hashSHA1(firstData)},
				{ID: "second", URL: server.URL + "/second", SHA1: hashSHA1(secondData)},
			}})
		case "/first":
			_, _ = writer.Write(firstData)
		case "/second":
			_, _ = writer.Write(secondData)
		}
	}))
	defer server.Close()
	manager, err := download.New(download.Options{Concurrency: 1, Retries: 0})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := NewResolver(ResolverOptions{Downloader: manager, ManifestURL: server.URL + "/manifest"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.Resolve(context.Background(), t.TempDir(), "first", false)
	if err == nil || !containsCode(err.Error(), "VERSION_INHERITANCE_CYCLE") {
		t.Fatalf("Resolve() error = %v, want inheritance cycle", err)
	}
}

func TestResolveRejectsUnsafeVersionID(t *testing.T) {
	manager, _ := download.New(download.Options{Concurrency: 1})
	resolver, _ := NewResolver(ResolverOptions{Downloader: manager})
	_, err := resolver.Resolve(context.Background(), t.TempDir(), "../outside", true)
	if err == nil {
		t.Fatal("Resolve() accepted a path-like version ID")
	}
}

func hashSHA1(data []byte) string {
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}

func serverURL(request *http.Request, path string) string {
	return "http://" + request.Host + path
}

func containsCode(message, code string) bool {
	return strings.Contains(message, code)
}
