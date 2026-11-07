package accounts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

type xboxLiveProperties struct {
	AuthMethod string `json:"AuthMethod"`
	SiteName   string `json:"SiteName"`
	RpsTicket  string `json:"RpsTicket"`
}

type xboxLiveRequest struct {
	Properties   xboxLiveProperties `json:"Properties"`
	RelyingParty string             `json:"RelyingParty"`
	TokenType    string             `json:"TokenType"`
}

type xstsProperties struct {
	SandboxID  string   `json:"SandboxId"`
	UserTokens []string `json:"UserTokens"`
}

type xstsRequest struct {
	Properties   xstsProperties `json:"Properties"`
	RelyingParty string         `json:"RelyingParty"`
	TokenType    string         `json:"TokenType"`
}

type xboxDisplayClaims struct {
	XUI []xboxXUI `json:"xui"`
}

type xboxXUI struct {
	UHS string `json:"uhs"`
}

type xboxAuthResponse struct {
	Token         string            `json:"Token"`
	DisplayClaims xboxDisplayClaims `json:"DisplayClaims"`
	XErr          json.Number       `json:"XErr"`
	Message       string            `json:"Message"`
}

type minecraftLoginRequest struct {
	IdentityToken string `json:"identityToken"`
}

type minecraftLoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Username    string `json:"username"`
	Error       string `json:"error"`
}

type minecraftEntitlementItem struct {
	Name string `json:"name"`
}

type minecraftEntitlements struct {
	Items []minecraftEntitlementItem `json:"items"`
}

type minecraftProfileAsset struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	URL     string `json:"url"`
	Variant string `json:"variant"`
	Alias   string `json:"alias"`
}

type minecraftProfile struct {
	ID    string                  `json:"id"`
	Name  string                  `json:"name"`
	Skins []minecraftProfileAsset `json:"skins"`
	Capes []minecraftProfileAsset `json:"capes"`
	Error string                  `json:"error"`
}

// completeAuthentication runs the full Xbox Live → XSTS → Minecraft services →
// entitlement → profile chain and returns a resolved account with credentials
// persisted in the operating-system credential store.
func (provider *MicrosoftProvider) completeAuthentication(ctx context.Context, existing models.Account, oauth microsoftOAuthToken) (models.Account, error) {
	if err := ctx.Err(); err != nil {
		return models.Account{}, err
	}
	xblToken, _, err := provider.xboxLiveAuthenticate(ctx, oauth.AccessToken)
	if err != nil {
		return models.Account{}, err
	}
	xstsToken, uhs, err := provider.xstsAuthorize(ctx, xblToken)
	if err != nil {
		return models.Account{}, err
	}
	minecraftToken, minecraftExpires, err := provider.minecraftLogin(ctx, uhs, xstsToken)
	if err != nil {
		return models.Account{}, err
	}
	if err := provider.checkEntitlement(ctx, minecraftToken); err != nil {
		return models.Account{}, err
	}
	profile, err := provider.fetchProfile(ctx, minecraftToken)
	if err != nil {
		return models.Account{}, err
	}
	accountID := existing.ID
	if accountID == "" {
		accountID, err = newAccountID()
		if err != nil {
			return models.Account{}, fmt.Errorf("generate account ID")
		}
	}
	now := time.Now().UTC()
	account := models.Account{
		ID: accountID, Name: profile.Name, UUID: strings.ToLower(profile.ID), DisplayName: profile.Name,
		Profile:       models.GameProfile{ID: strings.ToLower(profile.ID), Name: profile.Name},
		Type:          models.AccountMicrosoft,
		CredentialRef: microsoftCredentialKey(accountID),
		XUID:          uhs, UserType: "msa",
		CreatedAt: existing.CreatedAt, UpdatedAt: now,
	}
	if account.CreatedAt.IsZero() {
		account.CreatedAt = now
	}
	session := microsoftSession{
		AccessToken:          oauth.AccessToken,
		RefreshToken:         oauth.RefreshToken,
		ExpiresAt:            oauth.ExpiresAt,
		MinecraftAccessToken: minecraftToken,
		MinecraftExpiresAt:   minecraftExpires,
		XUID:                 uhs,
	}
	if err := provider.storeSession(ctx, microsoftCredentialKey(accountID), session); err != nil {
		return models.Account{}, err
	}
	account.Session = models.AccountSession{AccessToken: minecraftToken, ExpiresAt: minecraftExpires}
	return account, nil
}

func (provider *MicrosoftProvider) xboxLiveAuthenticate(ctx context.Context, oauthAccessToken string) (string, string, error) {
	payload := xboxLiveRequest{
		Properties: xboxLiveProperties{
			AuthMethod: "RPS",
			SiteName:   "user.auth.xboxlive.com",
			RpsTicket:  "d=" + oauthAccessToken,
		},
		RelyingParty: "http://auth.xboxlive.com",
		TokenType:    "JWT",
	}
	status, body, err := provider.doJSON(ctx, http.MethodPost, provider.endpoints.XboxLive, payload, "")
	if err != nil {
		return "", "", err
	}
	defer zeroBytes(body)
	if status != 200 {
		return "", "", cleerrors.Wrap(cleerrors.CodeXboxAuthenticationFailed, "Xbox Live user authentication failed", microsoftStatusError("Xbox Live", status))
	}
	var response xboxAuthResponse
	if err := decodeMicrosoftJSON(body, &response); err != nil {
		return "", "", cleerrors.Wrap(cleerrors.CodeXboxAuthenticationFailed, "Xbox Live returned an invalid response", err)
	}
	if strings.TrimSpace(response.Token) == "" {
		return "", "", cleerrors.New(cleerrors.CodeXboxAuthenticationFailed, "Xbox Live did not return a user token")
	}
	uhs := firstXboxUHS(response)
	if !microsoftXUIDPattern.MatchString(uhs) {
		return "", "", cleerrors.New(cleerrors.CodeXboxAuthenticationFailed, "Xbox Live returned a missing or invalid user hash")
	}
	return response.Token, uhs, nil
}

func (provider *MicrosoftProvider) xstsAuthorize(ctx context.Context, xblToken string) (string, string, error) {
	payload := xstsRequest{
		Properties: xstsProperties{
			SandboxID:  "RETAIL",
			UserTokens: []string{xblToken},
		},
		RelyingParty: "rp://api.minecraftservices.com/",
		TokenType:    "JWT",
	}
	status, body, err := provider.doJSON(ctx, http.MethodPost, provider.endpoints.XSTS, payload, "")
	if err != nil {
		return "", "", err
	}
	defer zeroBytes(body)
	if status != 200 {
		return "", "", cleerrors.Wrap(cleerrors.CodeXSTSAuthorizationFailed, "XSTS authorization failed", microsoftStatusError("XSTS", status))
	}
	var response xboxAuthResponse
	if err := decodeMicrosoftJSON(body, &response); err != nil {
		return "", "", cleerrors.Wrap(cleerrors.CodeXSTSAuthorizationFailed, "XSTS returned an invalid response", err)
	}
	if response.XErr.String() != "" && response.XErr.String() != "0" {
		return "", "", xstsXErrError(response.XErr.String())
	}
	if strings.TrimSpace(response.Token) == "" {
		return "", "", cleerrors.New(cleerrors.CodeXSTSAuthorizationFailed, "XSTS did not return an authorization token")
	}
	uhs := firstXboxUHS(response)
	if !microsoftXUIDPattern.MatchString(uhs) {
		return "", "", cleerrors.New(cleerrors.CodeXSTSAuthorizationFailed, "XSTS returned a missing or invalid user hash")
	}
	return response.Token, uhs, nil
}

func (provider *MicrosoftProvider) minecraftLogin(ctx context.Context, uhs, xstsToken string) (string, time.Time, error) {
	payload := minecraftLoginRequest{IdentityToken: "XBL3.0 x=" + uhs + ";" + xstsToken}
	status, body, err := provider.doJSON(ctx, http.MethodPost, provider.endpoints.MinecraftLogin, payload, "")
	if err != nil {
		return "", time.Time{}, err
	}
	defer zeroBytes(body)
	if status != 200 {
		return "", time.Time{}, cleerrors.Wrap(cleerrors.CodeMinecraftLoginFailed, "Minecraft services login failed", microsoftStatusError("Minecraft services", status))
	}
	var response minecraftLoginResponse
	if err := decodeMicrosoftJSON(body, &response); err != nil {
		return "", time.Time{}, cleerrors.Wrap(cleerrors.CodeMinecraftLoginFailed, "Minecraft services returned an invalid response", err)
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		return "", time.Time{}, cleerrors.New(cleerrors.CodeMinecraftLoginFailed, "Minecraft services did not return an access token")
	}
	expiresIn := response.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 86400
	}
	return response.AccessToken, time.Now().Add(time.Duration(expiresIn) * time.Second), nil
}

func (provider *MicrosoftProvider) checkEntitlement(ctx context.Context, minecraftToken string) error {
	status, body, err := provider.do(ctx, http.MethodGet, provider.endpoints.MinecraftEntitlements, nil, "application/json", minecraftToken)
	if err != nil {
		return err
	}
	defer zeroBytes(body)
	if status != 200 {
		return cleerrors.Wrap(cleerrors.CodeMinecraftEntitlementMissing, "Minecraft entitlement check failed", microsoftStatusError("Minecraft entitlement service", status))
	}
	var response minecraftEntitlements
	if err := decodeMicrosoftJSON(body, &response); err != nil {
		return cleerrors.Wrap(cleerrors.CodeMinecraftEntitlementMissing, "Minecraft entitlement service returned an invalid response", err)
	}
	for _, item := range response.Items {
		switch item.Name {
		case "game_minecraft", "product_minecraft":
			return nil
		}
	}
	return cleerrors.New(cleerrors.CodeMinecraftEntitlementMissing, "this Microsoft account does not own Minecraft: Java Edition")
}

func (provider *MicrosoftProvider) fetchProfile(ctx context.Context, minecraftToken string) (minecraftProfile, error) {
	status, body, err := provider.do(ctx, http.MethodGet, provider.endpoints.MinecraftProfile, nil, "application/json", minecraftToken)
	if err != nil {
		return minecraftProfile{}, err
	}
	defer zeroBytes(body)
	if status == http.StatusNotFound {
		return minecraftProfile{}, cleerrors.New(cleerrors.CodeProfileNotFound, "this Microsoft account has no Minecraft: Java Edition profile")
	}
	if status != 200 {
		return minecraftProfile{}, cleerrors.Wrap(cleerrors.CodeProfileNotFound, "Minecraft profile request failed", microsoftStatusError("Minecraft profile service", status))
	}
	var profile minecraftProfile
	if err := decodeMicrosoftJSON(body, &profile); err != nil {
		return minecraftProfile{}, cleerrors.Wrap(cleerrors.CodeProfileNotFound, "Minecraft profile service returned an invalid response", err)
	}
	if !microsoftProfileIDPattern.MatchString(strings.ToLower(profile.ID)) || !microsoftProfileNamePattern.MatchString(profile.Name) {
		return minecraftProfile{}, cleerrors.New(cleerrors.CodeProfileNotFound, "Minecraft profile service returned an invalid profile")
	}
	return profile, nil
}

func firstXboxUHS(response xboxAuthResponse) string {
	if len(response.DisplayClaims.XUI) == 0 {
		return ""
	}
	return response.DisplayClaims.XUI[0].UHS
}

func xstsXErrError(code string) error {
	switch code {
	case "2148916233":
		return cleerrors.New(cleerrors.CodeXSTSAuthorizationFailed, "this Microsoft account does not have an Xbox Live account")
	case "2148916235":
		return cleerrors.New(cleerrors.CodeXSTSAuthorizationFailed, "Xbox Live is not available in this account's region")
	case "2148916236":
		return cleerrors.New(cleerrors.CodeXSTSAuthorizationFailed, "this account requires adult verification on Xbox")
	case "2148916237":
		return cleerrors.New(cleerrors.CodeXSTSAuthorizationFailed, "this is a child account and must be added to a Microsoft family")
	case "2148916238":
		return cleerrors.New(cleerrors.CodeXSTSAuthorizationFailed, "this child account cannot authorize Xbox Live")
	default:
		return cleerrors.New(cleerrors.CodeXSTSAuthorizationFailed, "XSTS authorization was rejected (XErr "+code+")")
	}
}
