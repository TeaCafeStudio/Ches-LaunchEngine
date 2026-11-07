package arguments

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/metadata"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

func TestBuildModernArgumentsAppliesRulesAndPreservesPaths(t *testing.T) {
	version := metadata.VersionMetadata{
		ID: "1.21.1", Type: "release", MainClass: "net.minecraft.client.main.Main",
		Arguments: &metadata.Arguments{
			JVM: []json.RawMessage{
				json.RawMessage(`"-Djava.library.path=${natives_directory}"`),
				json.RawMessage(`"-cp"`),
				json.RawMessage(`"${classpath}"`),
			},
			Game: []json.RawMessage{
				json.RawMessage(`"--username"`),
				json.RawMessage(`"${auth_player_name}"`),
				json.RawMessage(`{"rules":[{"action":"allow","features":{"has_custom_resolution":true}}],"value":["--width","${resolution_width}"]}`),
			},
		},
	}
	gameDirectory := filepath.Join(t.TempDir(), "Minecraft User")
	options := Options{
		Version: version,
		Install: InstallPaths{
			GameDirectory: gameDirectory,
			AssetsRoot:    filepath.Join(gameDirectory, "assets"),
			AssetIndex:    "17",
			NativesPath:   filepath.Join(gameDirectory, "natives"),
			Classpath:     []string{"library path.jar", "client.jar"},
		},
		Request: models.LaunchRequest{
			VersionID: "1.21.1", GameDirectory: gameDirectory,
			Account:       models.Account{Name: "Test_Player", Type: models.AccountOffline},
			Window:        models.WindowOptions{Width: 1280, Height: 720},
			JavaArguments: []string{"-Xmx2G"},
		},
		Platform: metadata.RuleContext{OS: "windows", Arch: "x86_64", Features: map[string]bool{"has_custom_resolution": true}},
	}
	args, err := Build(options)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-Djava.library.path=" + filepath.Join(gameDirectory, "natives"),
		"-cp", "library path.jar" + string(os.PathListSeparator) + "client.jar",
		"-Xmx2G", "net.minecraft.client.main.Main",
		"--username", "Test_Player", "--width", "1280",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("Build() args:\n%q\nwant:\n%q", args, want)
	}
}

func TestBuildReportsUnknownPlaceholder(t *testing.T) {
	version := metadata.VersionMetadata{
		ID: "old", MainClass: "example.Main",
		Arguments: &metadata.Arguments{Game: []json.RawMessage{json.RawMessage(`"${unknown_value}"`)}},
	}
	_, err := Build(Options{Version: version, Request: models.LaunchRequest{
		Account: models.Account{Name: "OfflinePlayer", Type: models.AccountOffline},
	}})
	if err == nil {
		t.Fatal("Build() succeeded with an unknown placeholder")
	}
}

func TestBuildRejectsUnsupportedAccountTypesAndInvalidUUIDs(t *testing.T) {
	for _, account := range []models.Account{
		{Name: "Player", Type: models.AccountMicrosoft},
		{Name: "Player", UUID: "invalid", Type: models.AccountOffline},
	} {
		if _, err := Build(Options{Request: models.LaunchRequest{Account: account}}); err == nil {
			t.Fatalf("Build() accepted account %+v", account)
		}
	}
}

func TestOfflineUUIDIsStable(t *testing.T) {
	if got, want := OfflineUUID("Notch"), "b50ad385-829d-3141-a216-7e7d7539ba7f"; got != want {
		t.Fatalf("OfflineUUID(Notch) = %s, want %s", got, want)
	}
}

func TestBuildOfflineAuthenticationArgumentsForModernAndLegacyMetadata(t *testing.T) {
	const uuid = "12345678-1234-1234-1234-123456789abc"
	tests := []struct {
		name    string
		version metadata.VersionMetadata
	}{
		{
			name: "modern",
			version: metadata.VersionMetadata{
				ID: "modern", MainClass: "example.Main",
				Arguments: &metadata.Arguments{Game: []json.RawMessage{
					json.RawMessage(`"--username"`),
					json.RawMessage(`"${auth_player_name}"`),
					json.RawMessage(`"--uuid"`),
					json.RawMessage(`"${auth_uuid}"`),
					json.RawMessage(`"--accessToken"`),
					json.RawMessage(`"${auth_access_token}"`),
					json.RawMessage(`"--userType"`),
					json.RawMessage(`"${user_type}"`),
				}},
			},
		},
		{
			name: "legacy",
			version: metadata.VersionMetadata{
				ID: "legacy", MainClass: "example.Main",
				MinecraftArguments: "--username ${auth_player_name} --uuid ${auth_uuid} --accessToken ${auth_access_token} --userType ${user_type}",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := Build(Options{
				Version: test.version,
				Request: models.LaunchRequest{
					VersionID: test.version.ID,
					Account: models.Account{
						Type:    models.AccountOffline,
						Profile: models.GameProfile{ID: uuid, Name: "Offline_User"},
					},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			want := []string{
				"-Djava.library.path=", "-cp", "", "example.Main",
				"--username", "Offline_User", "--uuid", "12345678123412341234123456789abc",
				"--accessToken", "0", "--userType", "legacy",
			}
			if !reflect.DeepEqual(args, want) {
				t.Fatalf("Build() args:\n%q\nwant:\n%q", args, want)
			}
		})
	}
}

func TestBuildYggdrasilAuthenticationArgumentsAndRejectsInvalidSession(t *testing.T) {
	const profileID = "123456781234123412341234567890ab"
	version := metadata.VersionMetadata{
		ID: "1.21.1", MainClass: "example.Main",
		Arguments: &metadata.Arguments{Game: []json.RawMessage{
			json.RawMessage(`"--username"`),
			json.RawMessage(`"${auth_player_name}"`),
			json.RawMessage(`"--uuid"`),
			json.RawMessage(`"${auth_uuid}"`),
			json.RawMessage(`"--accessToken"`),
			json.RawMessage(`"${auth_access_token}"`),
			json.RawMessage(`"--userType"`),
			json.RawMessage(`"${user_type}"`),
			json.RawMessage(`"--session"`),
			json.RawMessage(`"${auth_session}"`),
			json.RawMessage(`"--user-properties"`),
			json.RawMessage(`"${user_properties}"`),
		}},
	}
	account := models.Account{
		Name: "Online_Player", UUID: profileID, Type: models.AccountYggdrasil, UserType: "legacy",
		Profile: models.GameProfile{
			ID: profileID, Name: "Online_Player",
			Properties: []models.ProfileProperty{{Name: "textures", Value: "texture-data", Signature: "texture-signature"}},
		},
		Session: models.AccountSession{AccessToken: "private-token", ClientToken: "private-client"},
	}
	args, err := Build(Options{Version: version, Request: models.LaunchRequest{Account: account}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-Djava.library.path=", "-cp", "", "example.Main",
		"--username", "Online_Player", "--uuid", profileID,
		"--accessToken", "private-token", "--userType", "legacy",
		"--session", "token:private-token:" + profileID,
		"--user-properties", `{"textures":[{"name":"textures","value":"texture-data","signature":"texture-signature"}]}`,
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("Build() args:\n%q\nwant:\n%q", args, want)
	}

	account.Profile.Name = "Different_Player"
	if _, err := Build(Options{Version: version, Request: models.LaunchRequest{Account: account}}); err == nil {
		t.Fatal("Build() accepted a session whose username differs from the selected profile")
	}
}
