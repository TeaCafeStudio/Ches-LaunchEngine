package arguments

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/metadata"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/accounts"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

type Options struct {
	Version  metadata.VersionMetadata
	Install  InstallPaths
	Request  models.LaunchRequest
	Platform metadata.RuleContext
}

type InstallPaths struct {
	GameDirectory    string
	VersionDirectory string
	ClientPath       string
	AssetsRoot       string
	AssetIndex       string
	NativesPath      string
	LogConfig        string
	Classpath        []string
}

var yggdrasilProfileIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
var yggdrasilProfileNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)
var microsoftXUIDPattern = regexp.MustCompile(`^[0-9A-Fa-f]{16}$`)

func Build(options Options) ([]string, error) {
	request := options.Request
	accountType := request.Account.Type
	if accountType == "" {
		accountType = models.AccountOffline
	}
	username := request.Account.Name
	if username == "" {
		username = request.Account.DisplayName
	}
	if username == "" {
		username = request.Account.Profile.Name
	}
	uuid := request.Account.UUID
	if uuid == "" {
		uuid = request.Account.Profile.ID
	}
	accessToken := "0"
	userType := "legacy"
	xuid := ""
	switch accountType {
	case models.AccountOffline:
		if err := accounts.ValidateOfflineIdentity(username, uuid); err != nil {
			return nil, cleerrors.Wrap(cleerrors.CodeAuthenticationFailed, "invalid offline account", err)
		}
	case models.AccountYggdrasil:
		uuid = strings.ToLower(strings.ReplaceAll(uuid, "-", ""))
		profileUUID := strings.ToLower(strings.ReplaceAll(request.Account.Profile.ID, "-", ""))
		if !request.Account.Session.ExpiresAt.IsZero() && time.Now().After(request.Account.Session.ExpiresAt) {
			return nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil account session has expired")
		}
		if !yggdrasilProfileIDPattern.MatchString(uuid) ||
			!yggdrasilProfileNamePattern.MatchString(username) ||
			uuid != profileUUID || username != request.Account.Profile.Name ||
			request.Account.Session.AccessToken == "" {
			return nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil account profile or session is invalid")
		}
		accessToken = request.Account.Session.AccessToken
		if request.Account.UserType != "" {
			userType = request.Account.UserType
		}
	case models.AccountMicrosoft:
		uuid = strings.ToLower(strings.ReplaceAll(uuid, "-", ""))
		profileUUID := strings.ToLower(strings.ReplaceAll(request.Account.Profile.ID, "-", ""))
		if !request.Account.Session.ExpiresAt.IsZero() && time.Now().After(request.Account.Session.ExpiresAt) {
			return nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Microsoft account session has expired")
		}
		if !yggdrasilProfileIDPattern.MatchString(uuid) ||
			!yggdrasilProfileNamePattern.MatchString(username) ||
			uuid != profileUUID || username != request.Account.Profile.Name ||
			request.Account.Session.AccessToken == "" {
			return nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Microsoft account profile or session is invalid")
		}
		if request.Account.XUID != "" && !microsoftXUIDPattern.MatchString(request.Account.XUID) {
			return nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Microsoft account has an invalid Xbox user hash")
		}
		accessToken = request.Account.Session.AccessToken
		userType = "msa"
		xuid = request.Account.XUID
	default:
		return nil, cleerrors.New(cleerrors.CodeAccountProviderNotFound, "the argument builder only accepts accounts resolved by a supported provider")
	}
	mode := request.ArgumentMode
	if mode == "" || mode == models.ArgumentModeAuto {
		mode = models.ArgumentModeAuto
	}

	var jvmArgs, gameArgs []string
	switch mode {
	case models.ArgumentModeAuto, models.ArgumentModeModern:
		if options.Version.Arguments != nil {
			var err error
			jvmArgs, err = expandArgumentList(options.Version.Arguments.JVM, options.Platform)
			if err != nil {
				return nil, cleerrors.Wrap(cleerrors.CodeArgumentResolutionFailed, "resolve JVM arguments", err)
			}
			gameArgs, err = expandArgumentList(options.Version.Arguments.Game, options.Platform)
			if err != nil {
				return nil, cleerrors.Wrap(cleerrors.CodeArgumentResolutionFailed, "resolve game arguments", err)
			}
		} else if mode == models.ArgumentModeModern {
			return nil, cleerrors.New(cleerrors.CodeArgumentResolutionFailed, "modern arguments are not present in this version metadata")
		}
		if options.Version.Arguments == nil && mode == models.ArgumentModeAuto {
			gameArgs = legacyArguments(options.Version)
		}
	case models.ArgumentModeLegacy:
		if options.Version.MinecraftArguments == "" {
			return nil, cleerrors.New(cleerrors.CodeArgumentResolutionFailed, "legacy minecraftArguments are not present in this version metadata")
		}
		gameArgs = legacyArguments(options.Version)
	default:
		return nil, cleerrors.New(cleerrors.CodeArgumentResolutionFailed, "unsupported argument mode")
	}

	values, err := argumentValues(options, username, uuid, accessToken, userType, xuid)
	if err != nil {
		return nil, cleerrors.Wrap(cleerrors.CodeArgumentResolutionFailed, "resolve account argument values", err)
	}
	jvmArgs, err = substitute(jvmArgs, values)
	if err != nil {
		return nil, cleerrors.Wrap(cleerrors.CodeArgumentResolutionFailed, "resolve JVM argument placeholders", err)
	}
	gameArgs, err = substitute(gameArgs, values)
	if err != nil {
		return nil, cleerrors.Wrap(cleerrors.CodeArgumentResolutionFailed, "resolve game argument placeholders", err)
	}

	if !hasArgumentPrefix(jvmArgs, "-Djava.library.path=") {
		jvmArgs = append(jvmArgs, "-Djava.library.path="+options.Install.NativesPath)
	}
	if !hasClassPath(jvmArgs) {
		jvmArgs = append(jvmArgs, "-cp", strings.Join(options.Install.Classpath, string(os.PathListSeparator)))
	}
	jvmArgs = append(jvmArgs, request.JavaArguments...)
	gameArgs = append(gameArgs, request.GameArguments...)

	all := make([]string, 0, len(jvmArgs)+1+len(gameArgs))
	all = append(all, jvmArgs...)
	all = append(all, options.Version.MainClass)
	all = append(all, gameArgs...)
	return all, nil
}

func legacyArguments(version metadata.VersionMetadata) []string {
	return strings.Fields(version.MinecraftArguments)
}

func expandArgumentList(rawArguments []json.RawMessage, platform metadata.RuleContext) ([]string, error) {
	var result []string
	for _, raw := range rawArguments {
		var legacyValue string
		if err := json.Unmarshal(raw, &legacyValue); err == nil {
			result = append(result, legacyValue)
			continue
		}
		var argument metadata.ArgumentObject
		if err := json.Unmarshal(raw, &argument); err != nil {
			return nil, fmt.Errorf("invalid argument metadata: %w", err)
		}
		if len(argument.Rules) > 0 && !platform.Allows(argument.Rules) {
			continue
		}
		var values []string
		if err := json.Unmarshal(argument.Value, &values); err == nil {
			result = append(result, values...)
			continue
		}
		if err := json.Unmarshal(argument.Value, &legacyValue); err != nil {
			return nil, fmt.Errorf("argument value must be a string or string array: %w", err)
		}
		result = append(result, legacyValue)
	}
	return result, nil
}

var placeholderPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func substitute(args []string, values map[string]string) ([]string, error) {
	result := make([]string, len(args))
	for index, argument := range args {
		var missing string
		result[index] = placeholderPattern.ReplaceAllStringFunc(argument, func(match string) string {
			name := placeholderPattern.FindStringSubmatch(match)[1]
			value, ok := values[name]
			if !ok {
				missing = name
				return match
			}
			return value
		})
		if missing != "" {
			return nil, fmt.Errorf("unsupported placeholder ${%s}", missing)
		}
	}
	return result, nil
}

func argumentValues(options Options, username, uuid, accessToken, userType, xuid string) (map[string]string, error) {
	version := options.Version
	request := options.Request
	if uuid == "" {
		uuid = OfflineUUID(username)
	}
	versionType := version.Type
	if versionType == "" {
		versionType = "release"
	}
	width, height := request.Window.Width, request.Window.Height
	if width <= 0 {
		width = 854
	}
	if height <= 0 {
		height = 480
	}
	values := map[string]string{
		"auth_player_name":  username,
		"version_name":      version.ID,
		"game_directory":    request.GameDirectory,
		"assets_root":       options.Install.AssetsRoot,
		"assets_index_name": options.Install.AssetIndex,
		"auth_uuid":         strings.ReplaceAll(uuid, "-", ""),
		"auth_access_token": accessToken,
		"clientid":          "",
		"auth_xuid":         xuid,
		"user_type":         userType,
		"version_type":      versionType,
		"natives_directory": options.Install.NativesPath,
		"launcher_name":     "ChesLaunchEngine",
		"launcher_version":  "dev",
		"classpath":         strings.Join(options.Install.Classpath, string(os.PathListSeparator)),
		"library_directory": filepath.Join(request.GameDirectory, "libraries"),
		"resolution_width":  fmt.Sprint(width),
		"resolution_height": fmt.Sprint(height),
		"user_properties":   "{}",
		"auth_session":      "0",
		"path":              options.Install.LogConfig,
	}
	if request.Account.Type == models.AccountYggdrasil {
		userProperties, err := marshalUserProperties(request.Account.Profile.Properties)
		if err != nil {
			return nil, err
		}
		values["auth_session"] = "token:" + accessToken + ":" + strings.ReplaceAll(uuid, "-", "")
		values["user_properties"] = userProperties
	}
	return values, nil
}

func marshalUserProperties(properties []models.ProfileProperty) (string, error) {
	if len(properties) == 0 {
		return "{}", nil
	}
	type propertyValue struct {
		Name      string `json:"name"`
		Value     string `json:"value"`
		Signature string `json:"signature,omitempty"`
	}
	grouped := make(map[string][]propertyValue, len(properties))
	for _, property := range properties {
		grouped[property.Name] = append(grouped[property.Name], propertyValue{
			Name: property.Name, Value: property.Value, Signature: property.Signature,
		})
	}
	encoded, err := json.Marshal(grouped)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func hasClassPath(arguments []string) bool {
	for _, argument := range arguments {
		if argument == "-cp" || argument == "-classpath" || argument == "--class-path" || strings.HasPrefix(argument, "-Djava.class.path=") {
			return true
		}
	}
	return false
}

func hasArgumentPrefix(arguments []string, prefix string) bool {
	for _, argument := range arguments {
		if strings.HasPrefix(argument, prefix) {
			return true
		}
	}
	return false
}

func OfflineUUID(username string) string {
	return accounts.OfflineUUID(username)
}
