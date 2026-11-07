package authlibinjector

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
)

func TestResolveRequiresVerifiedCachedAgentInOfflineMode(t *testing.T) {
	root := t.TempDir()
	path, err := Resolve(context.Background(), root, true)
	if path != "" {
		t.Fatalf("Resolve() path = %q, want empty", path)
	}
	var engineError *cleerrors.Error
	if !errors.As(err, &engineError) || engineError.Code != cleerrors.CodeAuthlibInjectorUnavailable {
		t.Fatalf("Resolve() error = %v, want AUTHLIB_INJECTOR_UNAVAILABLE", err)
	}
}

func TestVerifyRejectsCorruptCachedAgent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.jar")
	if err := os.WriteFile(path, []byte("not a jar"), 0600); err != nil {
		t.Fatal(err)
	}
	valid, err := verify(path, artifactSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("verify() accepted a corrupt cached artifact")
	}
}

func TestSecureClientDoesNotFollowRedirects(t *testing.T) {
	var redirected atomic.Bool
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Store(true)
	}))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL, http.StatusFound)
	}))
	defer origin.Close()

	response, err := secureClient().Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusFound)
	}
	if redirected.Load() {
		t.Fatal("secure client followed a redirect")
	}
}

func TestFetchRejectsNonOfficialArtifactSource(t *testing.T) {
	err := fetch(context.Background(), filepath.Join(t.TempDir(), "agent.jar"),
		"https://untrusted.example/agent.jar", artifactSHA256, secureClient())
	var engineError *cleerrors.Error
	if !errors.As(err, &engineError) || engineError.Code != cleerrors.CodeAuthlibInjectorDownloadFailed {
		t.Fatalf("fetch() error = %v, want AUTHLIB_INJECTOR_DOWNLOAD_FAILED", err)
	}
}
