package vanilla

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/download"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/metadata"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
)

type InstallResult struct {
	ClientPath  string
	AssetsRoot  string
	AssetIndex  string
	NativesPath string
	Classpath   []string
	LogConfig   string
}

func installFiles(ctx context.Context, downloader *download.Manager, version metadata.VersionMetadata, gameDirectory, versionDirectory string, ruleContext metadata.RuleContext, concurrency, retries int, offline bool) (InstallResult, error) {
	return installFilesWithProgress(ctx, downloader, version, gameDirectory, versionDirectory, ruleContext, concurrency, retries, offline, nil)
}

func installFilesWithProgress(ctx context.Context, downloader *download.Manager, version metadata.VersionMetadata, gameDirectory, versionDirectory string, ruleContext metadata.RuleContext, concurrency, retries int, offline bool, progress func(download.Progress)) (InstallResult, error) {
	librariesDirectory, err := download.ResolveUnderRoot(gameDirectory, "libraries")
	if err != nil {
		return InstallResult{}, cleerrors.Wrap(cleerrors.CodeInvalidGameDirectory, "resolve libraries directory", err)
	}
	if err := os.MkdirAll(librariesDirectory, 0755); err != nil {
		return InstallResult{}, cleerrors.Wrap(cleerrors.CodeInvalidGameDirectory, "create libraries directory", err)
	}
	assetsDirectory, err := download.ResolveUnderRoot(gameDirectory, "assets")
	if err != nil {
		return InstallResult{}, cleerrors.Wrap(cleerrors.CodeInvalidGameDirectory, "resolve assets directory", err)
	}
	if err := os.MkdirAll(assetsDirectory, 0755); err != nil {
		return InstallResult{}, cleerrors.Wrap(cleerrors.CodeInvalidGameDirectory, "create assets directory", err)
	}
	if err := os.MkdirAll(versionDirectory, 0755); err != nil {
		return InstallResult{}, cleerrors.Wrap(cleerrors.CodeInvalidVersionDirectory, "create version directory", err)
	}
	result := InstallResult{
		AssetsRoot: filepath.Join(gameDirectory, "assets"),
		AssetIndex: version.Assets,
	}
	requests := make(map[string]download.Request)
	assetMetadataRequests := make(map[string]download.Request)
	libraries := make([]libraryFile, 0, len(version.Libraries))
	for _, library := range version.Libraries {
		if !ruleContext.Allows(library.Rules) {
			continue
		}
		if artifact := libraryArtifact(library); artifact != nil {
			file, request, err := artifactFile(gameDirectory, library, artifact)
			if err != nil {
				return InstallResult{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "resolve library artifact path", err)
			}
			requests[file] = request
			libraries = append(libraries, libraryFile{library: library, path: file, native: library.IsNative(filepath.ToSlash(file))})
		}
		for _, artifact := range nativeArtifacts(library, ruleContext) {
			file, request, err := artifactFile(gameDirectory, library, &artifact)
			if err != nil {
				return InstallResult{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "resolve native library path", err)
			}
			requests[file] = request
			libraries = append(libraries, libraryFile{library: library, path: file, native: true})
		}
	}

	if client, ok := version.Downloads["client"]; ok {
		clientPath, err := download.ResolveUnderRoot(versionDirectory, version.ID+".jar")
		if err != nil {
			return InstallResult{}, cleerrors.Wrap(cleerrors.CodeInvalidVersionDirectory, "resolve client JAR path", err)
		}
		requests[clientPath] = download.Request{
			URL: client.URL, Path: clientPath, SHA1: client.SHA1, SHA256: client.SHA256, Size: client.Size,
		}
		result.ClientPath = clientPath
	} else {
		return InstallResult{}, cleerrors.New(cleerrors.CodeVersionJSONInvalid, "version metadata does not include the Minecraft client download")
	}

	if version.AssetIndex != nil {
		indexPath, err := download.ResolveUnderRoot(gameDirectory, filepath.Join("assets", "indexes", version.AssetIndex.ID+".json"))
		if err != nil {
			return InstallResult{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "resolve asset index path", err)
		}
		assetMetadataRequests[indexPath] = download.Request{
			URL: version.AssetIndex.URL, Path: indexPath,
			SHA1: version.AssetIndex.SHA1, Size: version.AssetIndex.Size,
		}
		result.AssetIndex = version.AssetIndex.ID
	}
	if version.Logging != nil && version.Logging.Client != nil {
		logConfig := version.Logging.Client.File
		if logConfig.Path != "" {
			path, err := download.ResolveUnderRoot(gameDirectory, filepath.Join("assets", logConfig.Path))
			if err != nil {
				return InstallResult{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "resolve logging configuration path", err)
			}
			assetMetadataRequests[path] = download.Request{
				URL: logConfig.URL, Path: path, SHA1: logConfig.SHA1,
				SHA256: logConfig.SHA256, Size: logConfig.Size,
			}
			result.LogConfig = path
		}
	}

	requestList := make([]download.Request, 0, len(requests))
	for _, request := range requests {
		requestList = append(requestList, request)
	}
	if err := downloader.DownloadManyWithSettingsAndProgress(ctx, requestList, concurrency, retries, offline, progress); err != nil {
		if ctx.Err() != nil {
			return InstallResult{}, ctx.Err()
		}
		return InstallResult{}, cleerrors.Wrap(cleerrors.CodeLibraryDownloadFailed, "download Minecraft client or libraries", err)
	}
	assetMetadataList := make([]download.Request, 0, len(assetMetadataRequests))
	for _, request := range assetMetadataRequests {
		assetMetadataList = append(assetMetadataList, request)
	}
	if err := downloader.DownloadManyWithSettingsAndProgress(ctx, assetMetadataList, concurrency, retries, offline, progress); err != nil {
		if ctx.Err() != nil {
			return InstallResult{}, ctx.Err()
		}
		return InstallResult{}, cleerrors.Wrap(cleerrors.CodeAssetDownloadFailed, "download asset index or logging configuration", err)
	}

	if version.AssetIndex != nil {
		assetIndexPath, err := download.ResolveUnderRoot(gameDirectory, filepath.Join("assets", "indexes", version.AssetIndex.ID+".json"))
		if err != nil {
			return InstallResult{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "resolve asset index path", err)
		}
		result, err = installAssets(ctx, downloader, version, assetIndexPath, gameDirectory, result, concurrency, retries, offline, progress)
		if err != nil {
			return InstallResult{}, err
		}
	} else if version.Assets != "" {
		result.AssetIndex = version.Assets
	}

	classpath, nativesPath, err := prepareClasspath(libraries, versionDirectory)
	if err != nil {
		return InstallResult{}, err
	}
	result.Classpath = append(classpath, result.ClientPath)
	result.NativesPath = nativesPath
	return result, nil
}

type libraryFile struct {
	library metadata.Library
	path    string
	native  bool
}

func libraryArtifact(library metadata.Library) *metadata.Artifact {
	if library.Downloads != nil && library.Downloads.Artifact != nil {
		return library.Downloads.Artifact
	}
	return nil
}

func nativeArtifacts(library metadata.Library, platform metadata.RuleContext) []metadata.Artifact {
	if library.Downloads == nil {
		return nil
	}
	if classifier, ok := library.Natives[platform.OS]; ok {
		arch := "32"
		if strings.Contains(platform.Arch, "64") || platform.Arch == "aarch64" {
			arch = "64"
		}
		classifier = strings.ReplaceAll(classifier, "${arch}", arch)
		if artifact, ok := library.Downloads.Classifiers[classifier]; ok {
			return []metadata.Artifact{artifact}
		}
	}
	return nil
}

func artifactFile(gameDirectory string, library metadata.Library, artifact *metadata.Artifact) (string, download.Request, error) {
	relative := artifact.Path
	if relative == "" {
		var ok bool
		relative, ok = library.ArtifactPath()
		if !ok {
			return "", download.Request{}, fmt.Errorf("library %q has no downloadable artifact path", library.Name)
		}
	}
	path, err := download.ResolveUnderRoot(gameDirectory, filepath.Join("libraries", relative))
	if err != nil {
		return "", download.Request{}, err
	}
	url := artifact.URL
	if url == "" {
		url = library.ArtifactURL(filepath.ToSlash(relative))
	}
	return path, download.Request{
		URL: url, Path: path, SHA1: artifact.SHA1, SHA256: artifact.SHA256, Size: artifact.Size,
	}, nil
}

func installAssets(ctx context.Context, downloader *download.Manager, version metadata.VersionMetadata, indexPath, gameDirectory string, result InstallResult, concurrency, retries int, offline bool, progress func(download.Progress)) (InstallResult, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return InstallResult{}, cleerrors.Wrap(cleerrors.CodeAssetDownloadFailed, "read asset index", err)
	}
	var index metadata.AssetIndexData
	if err := json.Unmarshal(data, &index); err != nil {
		return InstallResult{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "parse asset index", err)
	}

	requests := make(map[string]download.Request, len(index.Objects))
	assetPaths := make(map[string]string, len(index.Objects))
	for name, object := range index.Objects {
		if len(object.Hash) < 2 {
			return InstallResult{}, cleerrors.New(cleerrors.CodeVersionJSONInvalid, "asset index contains an invalid object hash")
		}
		relative := filepath.Join("objects", object.Hash[:2], object.Hash)
		path, err := download.ResolveUnderRoot(gameDirectory, filepath.Join("assets", relative))
		if err != nil {
			return InstallResult{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "resolve asset path", err)
		}
		requests[path] = download.Request{
			URL:  "https://resources.download.minecraft.net/" + object.Hash[:2] + "/" + object.Hash,
			Path: path, SHA1: object.Hash, Size: object.Size,
		}
		assetPaths[name] = path
	}
	requestList := make([]download.Request, 0, len(requests))
	for _, request := range requests {
		requestList = append(requestList, request)
	}
	if err := downloader.DownloadManyWithSettingsAndProgress(ctx, requestList, concurrency, retries, offline, progress); err != nil {
		if ctx.Err() != nil {
			return InstallResult{}, ctx.Err()
		}
		return InstallResult{}, cleerrors.Wrap(cleerrors.CodeAssetDownloadFailed, "download Minecraft assets", err)
	}

	if index.Virtual || index.MapToResources {
		destinationRelative := filepath.Join("assets", "virtual", version.Assets)
		if index.MapToResources {
			destinationRelative = "resources"
		}
		destinationRoot, err := download.ResolveUnderRoot(gameDirectory, destinationRelative)
		if err != nil {
			return InstallResult{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "resolve virtual asset directory", err)
		}
		if err := os.MkdirAll(destinationRoot, 0755); err != nil {
			return InstallResult{}, cleerrors.Wrap(cleerrors.CodeInvalidGameDirectory, "create virtual asset directory", err)
		}
		for name, source := range assetPaths {
			destination, err := download.ResolveUnderRoot(destinationRoot, name)
			if err != nil {
				return InstallResult{}, cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "resolve virtual asset path", err)
			}
			if err := copyAsset(source, destination); err != nil {
				return InstallResult{}, cleerrors.Wrap(cleerrors.CodeAssetDownloadFailed, "materialize virtual asset", err)
			}
		}
	}
	return result, nil
}

func copyAsset(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(destination); err == nil {
		return nil
	}
	if err := os.Link(source, destination); err == nil {
		return nil
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(destination)
		return copyErr
	}
	return closeErr
}

func prepareClasspath(libraries []libraryFile, versionDirectory string) ([]string, string, error) {
	classpath := make([]string, 0, len(libraries))
	seen := make(map[string]bool)
	nativesPath, err := download.ResolveUnderRoot(versionDirectory, "natives")
	if err != nil {
		return nil, "", cleerrors.Wrap(cleerrors.CodeInvalidVersionDirectory, "resolve native libraries directory", err)
	}
	if err := os.MkdirAll(nativesPath, 0755); err != nil {
		return nil, "", cleerrors.Wrap(cleerrors.CodeInvalidVersionDirectory, "create native libraries directory", err)
	}
	for _, file := range libraries {
		if file.native {
			if err := extractNative(file.path, nativesPath, file.library.Extract); err != nil {
				return nil, "", cleerrors.Wrap(cleerrors.CodeVersionJSONInvalid, "extract native library", err)
			}
			continue
		}
		key := libraryConflictKey(file.library)
		if key == "" {
			key = file.path
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		classpath = append(classpath, file.path)
	}
	return classpath, nativesPath, nil
}

func libraryConflictKey(library metadata.Library) string {
	parts := strings.Split(library.Name, ":")
	if len(parts) < 3 {
		return ""
	}
	classifier := ""
	if len(parts) > 3 {
		classifier = parts[3]
	}
	return parts[0] + ":" + parts[1] + ":" + classifier
}

func extractNative(archivePath, destination string, options *metadata.NativeExtract) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	excludes := []string{"META-INF/"}
	if options != nil {
		excludes = append(excludes, options.Exclude...)
	}
	root, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	for _, entry := range reader.File {
		name := filepath.Clean(filepath.FromSlash(entry.Name))
		if filepath.IsAbs(name) || filepath.VolumeName(name) != "" || name == "." || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("native archive contains an unsafe path")
		}
		excluded := false
		for _, pattern := range excludes {
			normalizedPattern := strings.TrimSuffix(filepath.ToSlash(pattern), "/")
			matched, _ := filepath.Match(filepath.FromSlash(normalizedPattern), filepath.FromSlash(name))
			if matched || strings.HasPrefix(filepath.ToSlash(name), normalizedPattern+"/") {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("native archive contains a symbolic link")
		}
		target, err := download.ResolveUnderRoot(root, name)
		if err != nil {
			return fmt.Errorf("native archive contains an unsafe path: %w", err)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if entry.UncompressedSize64 > 256<<20 {
			return fmt.Errorf("native archive entry exceeds 256 MiB")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		file, err := os.CreateTemp(filepath.Dir(target), ".cle-native-*")
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(file, io.LimitReader(source, 256<<20))
		sourceErr := source.Close()
		fileErr := file.Close()
		if copyErr != nil {
			os.Remove(file.Name())
			return copyErr
		}
		if sourceErr != nil {
			os.Remove(file.Name())
			return sourceErr
		}
		if fileErr != nil {
			os.Remove(file.Name())
			return fileErr
		}
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			os.Remove(file.Name())
			return err
		}
		if err := os.Rename(file.Name(), target); err != nil {
			os.Remove(file.Name())
			return err
		}
	}
	return nil
}
