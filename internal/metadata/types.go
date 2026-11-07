package metadata

import "encoding/json"

const ManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

type VersionManifest struct {
	Latest   LatestVersion   `json:"latest"`
	Versions []ManifestEntry `json:"versions"`
}

type LatestVersion struct {
	Release  string `json:"release"`
	Snapshot string `json:"snapshot"`
}

type ManifestEntry struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	Time        string `json:"time"`
	ReleaseTime string `json:"releaseTime"`
	SHA1        string `json:"sha1"`
	Compliance  int    `json:"complianceLevel"`
}

type VersionMetadata struct {
	ID                     string              `json:"id"`
	Type                   string              `json:"type"`
	MainClass              string              `json:"mainClass"`
	InheritsFrom           string              `json:"inheritsFrom,omitempty"`
	ReleaseTime            string              `json:"releaseTime,omitempty"`
	Time                   string              `json:"time,omitempty"`
	Assets                 string              `json:"assets,omitempty"`
	AssetIndex             *AssetIndex         `json:"assetIndex,omitempty"`
	Libraries              []Library           `json:"libraries,omitempty"`
	Downloads              map[string]Artifact `json:"downloads,omitempty"`
	JavaVersion            *JavaVersion        `json:"javaVersion,omitempty"`
	Arguments              *Arguments          `json:"arguments,omitempty"`
	MinecraftArguments     string              `json:"minecraftArguments,omitempty"`
	Logging                *Logging            `json:"logging,omitempty"`
	MinimumLauncherVersion int                 `json:"minimumLauncherVersion,omitempty"`
}

type AssetIndex struct {
	ID        string `json:"id"`
	SHA1      string `json:"sha1"`
	Size      int64  `json:"size"`
	TotalSize int64  `json:"totalSize"`
	URL       string `json:"url"`
}

type AssetIndexData struct {
	Objects        map[string]AssetObject `json:"objects"`
	Virtual        bool                   `json:"virtual,omitempty"`
	MapToResources bool                   `json:"map_to_resources,omitempty"`
}

type AssetObject struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

type JavaVersion struct {
	Component    string `json:"component"`
	MajorVersion int    `json:"majorVersion"`
}

type Arguments struct {
	JVM  []json.RawMessage `json:"jvm"`
	Game []json.RawMessage `json:"game"`
}

type ArgumentObject struct {
	Rules []Rule          `json:"rules,omitempty"`
	Value json.RawMessage `json:"value"`
}

type Rule struct {
	Action   string          `json:"action"`
	OS       *OSRule         `json:"os,omitempty"`
	Features map[string]bool `json:"features,omitempty"`
}

type OSRule struct {
	Name    string `json:"name,omitempty"`
	Arch    string `json:"arch,omitempty"`
	Version string `json:"version,omitempty"`
}

type Library struct {
	Name      string            `json:"name"`
	Downloads *LibraryDownloads `json:"downloads,omitempty"`
	Rules     []Rule            `json:"rules,omitempty"`
	Natives   map[string]string `json:"natives,omitempty"`
	Extract   *NativeExtract    `json:"extract,omitempty"`
	URL       string            `json:"url,omitempty"`
}

type LibraryDownloads struct {
	Artifact    *Artifact           `json:"artifact,omitempty"`
	Classifiers map[string]Artifact `json:"classifiers,omitempty"`
}

type Artifact struct {
	Path   string `json:"path"`
	SHA1   string `json:"sha1,omitempty"`
	Size   int64  `json:"size,omitempty"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256,omitempty"`
}

type NativeExtract struct {
	Exclude []string `json:"exclude,omitempty"`
}

type Logging struct {
	Client *LoggingClient `json:"client,omitempty"`
}

type LoggingClient struct {
	Argument string   `json:"argument"`
	File     Artifact `json:"file"`
	Type     string   `json:"type"`
}
