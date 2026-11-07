package metadata

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/download"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
)

type Resolver struct {
	download    *download.Manager
	client      *http.Client
	manifestURL string
}

type ResolverOptions struct {
	Downloader  *download.Manager
	HTTPClient  *http.Client
	ManifestURL string
}

func NewResolver(options ResolverOptions) (*Resolver, error) {
	if options.Downloader == nil {
		return nil, fmt.Errorf("metadata resolver requires a download manager")
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if options.ManifestURL == "" {
		options.ManifestURL = ManifestURL
	}
	return &Resolver{
		download:    options.Downloader,
		client:      options.HTTPClient,
		manifestURL: options.ManifestURL,
	}, nil
}

func (resolver *Resolver) Resolve(ctx context.Context, gameDirectory, versionID string, offline bool) (VersionMetadata, error) {
	return resolver.ResolveAt(ctx, gameDirectory, filepath.Join(gameDirectory, "versions", versionID), versionID, offline)
}

func (resolver *Resolver) ListVersions(ctx context.Context, gameDirectory string, offline bool) (VersionManifest, error) {
	return resolver.loadManifest(ctx, gameDirectory, offline)
}

func (resolver *Resolver) ResolveAt(ctx context.Context, gameDirectory, versionDirectory, versionID string, offline bool) (VersionMetadata, error) {
	if !validVersionID(versionID) {
		return VersionMetadata{}, cleerrors.New(cleerrors.CodeVersionJSONInvalid, "version ID contains invalid path characters")
	}
	if err := os.MkdirAll(versionDirectory, 0755); err != nil {
		return VersionMetadata{}, cleerrors.Wrap(cleerrors.CodeInvalidVersionDirectory, "create version metadata directory", err)
	}
	manifest, err := resolver.loadManifest(ctx, gameDirectory, offline)
	if err != nil {
		return VersionMetadata{}, err
	}

	entries := make(map[string]ManifestEntry, len(manifest.Versions))
	for _, entry := range manifest.Versions {
		entries[entry.ID] = entry
	}
	resolved := make(map[string]VersionMetadata)
	visiting := make(map[string]bool)
	return resolver.resolveVersion(ctx, gameDirectory, filepath.Dir(versionDirectory), versionID, versionDirectory, versionID, offline, entries, visiting, resolved)
}

func (resolver *Resolver) resolveVersion(ctx context.Context, gameDirectory, versionsDirectory, explicitVersionID, explicitVersionDirectory, versionID string, offline bool, entries map[string]ManifestEntry, visiting map[string]bool, resolved map[string]VersionMetadata) (VersionMetadata, error) {
	if version, ok := resolved[versionID]; ok {
		return version, nil
	}
	if visiting[versionID] {
		return VersionMetadata{}, cleerrors.New(cleerrors.CodeVersionInheritanceCycle, "version metadata contains an inheritance cycle")
	}
	visiting[versionID] = true
	defer delete(visiting, versionID)

	entry, inManifest := entries[versionID]
	versionDirectory := filepath.Join(versionsDirectory, versionID)
	if versionID == explicitVersionID {
		versionDirectory = explicitVersionDirectory
	}
	var versionPath string
	var err error
	if versionID == explicitVersionID {
		versionPath, err = download.ResolveUnderRoot(versionDirectory, versionID+".json")
	} else {
		versionPath, err = download.ResolveUnderRoot(versionsDirectory, filepath.Join(versionID, versionID+".json"))
	}
	if err != nil {
		return VersionMetadata{}, cleerrors.Wrap(cleerrors.CodeInvalidVersionDirectory, "resolve version metadata path", err)
	}
	var version VersionMetadata
	cacheValid := false
	if data, err := os.ReadFile(versionPath); err == nil {
		if !inManifest || verifySHA1(data, entry.SHA1) {
			if json.Unmarshal(data, &version) == nil {
				cacheValid = true
			}
		}
	}

	if !cacheValid {
		if offline {
			return VersionMetadata{}, cleerrors.New(cleerrors.CodeVersionJSONInvalid, "version metadata is not available in the offline cache")
		}
		if !inManifest {
			return VersionMetadata{}, cleerrors.New(cleerrors.CodeVersionJSONInvalid, "version ID was not found in the official manifest")
		}
		data, err := resolver.fetchJSON(ctx, entry.URL)
		if err != nil {
			return VersionMetadata{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "download version metadata", err)
		}
		if !verifySHA1(data, entry.SHA1) {
			return VersionMetadata{}, cleerrors.New(cleerrors.CodeHashMismatch, "version metadata SHA-1 does not match the official manifest")
		}
		if err := json.Unmarshal(data, &version); err != nil {
			return VersionMetadata{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "parse version metadata", err)
		}
		if err := writeAtomic(versionPath, data); err != nil {
			return VersionMetadata{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "cache version metadata", err)
		}
	}
	if version.ID == "" || version.MainClass == "" {
		return VersionMetadata{}, cleerrors.New(cleerrors.CodeVersionJSONInvalid, "version metadata is missing its ID or main class")
	}
	if version.ID != versionID {
		return VersionMetadata{}, cleerrors.New(cleerrors.CodeVersionJSONInvalid, "version metadata ID does not match the requested ID")
	}

	if version.InheritsFrom != "" {
		if !validVersionID(version.InheritsFrom) {
			return VersionMetadata{}, cleerrors.New(cleerrors.CodeVersionJSONInvalid, "parent version ID contains invalid path characters")
		}
		parent, err := resolver.resolveVersion(ctx, gameDirectory, versionsDirectory, explicitVersionID, explicitVersionDirectory, version.InheritsFrom, offline, entries, visiting, resolved)
		if err != nil {
			return VersionMetadata{}, err
		}
		version = mergeVersion(parent, version)
	}
	resolved[versionID] = version
	return version, nil
}

func (resolver *Resolver) loadManifest(ctx context.Context, gameDirectory string, offline bool) (VersionManifest, error) {
	manifestPath, err := download.ResolveUnderRoot(gameDirectory, filepath.Join("versions", "version_manifest_v2.json"))
	if err != nil {
		return VersionManifest{}, cleerrors.Wrap(cleerrors.CodeInvalidGameDirectory, "resolve version manifest path", err)
	}
	if data, err := os.ReadFile(manifestPath); err == nil {
		var manifest VersionManifest
		if json.Unmarshal(data, &manifest) == nil && len(manifest.Versions) > 0 {
			if offline {
				return manifest, nil
			}
			updated, fetchErr := resolver.fetchJSON(ctx, resolver.manifestURL)
			if fetchErr != nil {
				if ctx.Err() != nil {
					return VersionManifest{}, ctx.Err()
				}
				return manifest, nil
			}
			if json.Unmarshal(updated, &manifest) == nil && len(manifest.Versions) > 0 {
				if err := writeAtomic(manifestPath, updated); err != nil {
					return VersionManifest{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "cache version manifest", err)
				}
				return manifest, nil
			}
		}
	}
	if offline {
		return VersionManifest{}, nil
	}
	data, err := resolver.fetchJSON(ctx, resolver.manifestURL)
	if err != nil {
		return VersionManifest{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "download version manifest", err)
	}
	var manifest VersionManifest
	if err := json.Unmarshal(data, &manifest); err != nil || len(manifest.Versions) == 0 {
		return VersionManifest{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "parse version manifest", err)
	}
	if err := writeAtomic(manifestPath, data); err != nil {
		return VersionManifest{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "cache version manifest", err)
	}
	return manifest, nil
}

func (resolver *Resolver) fetchJSON(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := resolver.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %s", response.Status)
	}
	const maxMetadataSize = 32 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, maxMetadataSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxMetadataSize {
		return nil, fmt.Errorf("metadata document exceeds %d bytes", maxMetadataSize)
	}
	return data, nil
}

func mergeVersion(parent, child VersionMetadata) VersionMetadata {
	merged := parent
	merged.ID = child.ID
	merged.InheritsFrom = child.InheritsFrom
	if child.Type != "" {
		merged.Type = child.Type
	}
	if child.MainClass != "" {
		merged.MainClass = child.MainClass
	}
	if child.ReleaseTime != "" {
		merged.ReleaseTime = child.ReleaseTime
	}
	if child.Time != "" {
		merged.Time = child.Time
	}
	if child.Assets != "" {
		merged.Assets = child.Assets
	}
	if child.AssetIndex != nil {
		merged.AssetIndex = child.AssetIndex
	}
	if child.JavaVersion != nil {
		merged.JavaVersion = child.JavaVersion
	}
	if child.Arguments != nil {
		if parent.Arguments == nil {
			merged.Arguments = child.Arguments
		} else {
			merged.Arguments = &Arguments{
				JVM:  append(append([]json.RawMessage(nil), parent.Arguments.JVM...), child.Arguments.JVM...),
				Game: append(append([]json.RawMessage(nil), parent.Arguments.Game...), child.Arguments.Game...),
			}
		}
	}
	if child.MinecraftArguments != "" {
		merged.MinecraftArguments = child.MinecraftArguments
	}
	if child.Logging != nil {
		merged.Logging = child.Logging
	}
	if child.MinimumLauncherVersion != 0 {
		merged.MinimumLauncherVersion = child.MinimumLauncherVersion
	}
	merged.Libraries = mergeLibraries(parent.Libraries, child.Libraries)
	merged.Downloads = make(map[string]Artifact, len(parent.Downloads)+len(child.Downloads))
	for key, artifact := range parent.Downloads {
		merged.Downloads[key] = artifact
	}
	for key, artifact := range child.Downloads {
		merged.Downloads[key] = artifact
	}
	return merged
}

func mergeLibraries(parent, child []Library) []Library {
	merged := make([]Library, 0, len(parent)+len(child))
	indexes := make(map[string]int, len(parent)+len(child))
	for _, library := range append(append([]Library(nil), parent...), child...) {
		key := libraryKey(library)
		if index, exists := indexes[key]; exists {
			merged[index] = library
			continue
		}
		indexes[key] = len(merged)
		merged = append(merged, library)
	}
	return merged
}

func libraryKey(library Library) string {
	parts := strings.Split(library.Name, ":")
	if len(parts) >= 3 {
		classifier := ""
		if len(parts) >= 4 {
			classifier = parts[3]
		}
		return strings.Join([]string{parts[0], parts[1], classifier}, ":")
	}
	if library.Downloads != nil && library.Downloads.Artifact != nil && library.Downloads.Artifact.Path != "" {
		return library.Downloads.Artifact.Path
	}
	return library.Name
}

var validID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func validVersionID(id string) bool {
	return id != "" && validID.MatchString(id) && id != "." && id != ".."
}

func verifySHA1(data []byte, expected string) bool {
	if expected == "" {
		return true
	}
	sum := sha1.Sum(data)
	return strings.EqualFold(hex.EncodeToString(sum[:]), expected)
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".cle-metadata-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
