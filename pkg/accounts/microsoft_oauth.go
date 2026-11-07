package accounts

import (
	"context"
	"net/url"
	"strings"
	"time"

	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

const microsoftOAuthExpiryBuffer = 5 * time.Minute

type microsoftDeviceCodeResponse struct {
	DeviceCode       string `json:"device_code"`
	UserCode         string `json:"user_code"`
	VerificationURI  string `json:"verification_uri"`
	ExpiresIn        int    `json:"expires_in"`
	Interval         int    `json:"interval"`
	Message          string `json:"message"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type microsoftTokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// StartDeviceCode requests an OAuth device code and returns the verification
// URL and user code that the user must enter in their browser.
func (provider *MicrosoftProvider) StartDeviceCode(ctx context.Context) (models.DeviceCode, error) {
	if err := provider.requireClientID(); err != nil {
		return models.DeviceCode{}, err
	}
	if err := ctx.Err(); err != nil {
		return models.DeviceCode{}, err
	}
	form := url.Values{}
	form.Set("client_id", provider.clientID)
	form.Set("scope", microsoftScope)
	status, body, err := provider.doForm(ctx, provider.endpoints.OAuthDeviceCode, form, "")
	if err != nil {
		return models.DeviceCode{}, err
	}
	defer zeroBytes(body)
	if status != 200 {
		return models.DeviceCode{}, microsoftStatusError("OAuth device code endpoint", status)
	}
	var response microsoftDeviceCodeResponse
	if err := decodeMicrosoftJSON(body, &response); err != nil {
		return models.DeviceCode{}, cleerrors.Wrap(cleerrors.CodeAuthenticationFailed, "OAuth device code endpoint returned an invalid response", err)
	}
	if response.Error != "" {
		return models.DeviceCode{}, microsoftOAuthError(response.Error, response.ErrorDescription)
	}
	if strings.TrimSpace(response.DeviceCode) == "" || strings.TrimSpace(response.UserCode) == "" ||
		strings.TrimSpace(response.VerificationURI) == "" {
		return models.DeviceCode{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "OAuth device code endpoint returned an incomplete response")
	}
	interval := response.Interval
	if interval <= 0 {
		interval = 5
	}
	expiresIn := response.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 900
	}
	return models.DeviceCode{
		VerificationURL: response.VerificationURI,
		UserCode:        response.UserCode,
		DeviceCode:      response.DeviceCode,
		Interval:        interval,
		ExpiresIn:       expiresIn,
		Message:         response.Message,
	}, nil
}

// AuthenticateDeviceCode polls the token endpoint until the user completes
// authorization (or it fails, expires, or is cancelled), then runs the full
// Xbox/XSTS/Minecraft chain and returns a resolved account with credentials
// stored in the operating-system credential store.
func (provider *MicrosoftProvider) AuthenticateDeviceCode(ctx context.Context, code models.DeviceCode) (models.Account, error) {
	if err := provider.requireClientID(); err != nil {
		return models.Account{}, err
	}
	if strings.TrimSpace(code.DeviceCode) == "" {
		return models.Account{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "device code is missing")
	}
	oauth, err := provider.pollDeviceCode(ctx, code)
	if err != nil {
		return models.Account{}, err
	}
	return provider.completeAuthentication(ctx, models.Account{}, oauth)
}

func (provider *MicrosoftProvider) pollDeviceCode(ctx context.Context, code models.DeviceCode) (microsoftOAuthToken, error) {
	interval := code.Interval
	if interval <= 0 {
		interval = 5
	}
	deadline := time.Now().Add(time.Duration(code.ExpiresIn) * time.Second)
	for {
		if err := ctx.Err(); err != nil {
			return microsoftOAuthToken{}, err
		}
		if time.Now().After(deadline) {
			return microsoftOAuthToken{}, cleerrors.New(cleerrors.CodeOAuthAuthorizationFailed, "device code expired before authorization completed")
		}
		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("client_id", provider.clientID)
		form.Set("device_code", code.DeviceCode)
		status, body, err := provider.doForm(ctx, provider.endpoints.OAuthToken, form, "")
		if err != nil {
			if ctx.Err() != nil {
				return microsoftOAuthToken{}, ctx.Err()
			}
			return microsoftOAuthToken{}, err
		}
		var response microsoftTokenResponse
		decodeErr := decodeMicrosoftJSON(body, &response)
		zeroBytes(body)
		if status == 200 && decodeErr == nil && strings.TrimSpace(response.AccessToken) != "" {
			return microsoftOAuthTokenFromResponse(response), nil
		}
		if decodeErr != nil && status == 200 {
			return microsoftOAuthToken{}, cleerrors.Wrap(cleerrors.CodeAuthenticationFailed, "OAuth token endpoint returned an invalid response", decodeErr)
		}
		switch response.Error {
		case "authorization_pending":
			if !sleepContext(ctx, time.Duration(interval)*time.Second) {
				return microsoftOAuthToken{}, ctx.Err()
			}
			continue
		case "slow_down":
			interval += 5
			if !sleepContext(ctx, time.Duration(interval)*time.Second) {
				return microsoftOAuthToken{}, ctx.Err()
			}
			continue
		case "authorization_declined", "access_denied":
			return microsoftOAuthToken{}, cleerrors.New(cleerrors.CodeOAuthAuthorizationFailed, "Microsoft authorization was declined")
		case "expired_token":
			return microsoftOAuthToken{}, cleerrors.New(cleerrors.CodeOAuthAuthorizationFailed, "device code expired")
		case "bad_verification_code":
			return microsoftOAuthToken{}, cleerrors.New(cleerrors.CodeOAuthAuthorizationFailed, "device code was rejected")
		case "":
			return microsoftOAuthToken{}, microsoftStatusError("OAuth token endpoint", status)
		default:
			return microsoftOAuthToken{}, microsoftOAuthError(response.Error, response.ErrorDescription)
		}
	}
}

func microsoftOAuthTokenFromResponse(response microsoftTokenResponse) microsoftOAuthToken {
	return microsoftOAuthToken{
		AccessToken:  response.AccessToken,
		RefreshToken: response.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(response.ExpiresIn) * time.Second),
		TokenType:    response.TokenType,
		Scope:        response.Scope,
	}
}

type microsoftOAuthToken struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	TokenType    string
	Scope        string
}

func (token microsoftOAuthToken) valid() bool {
	return strings.TrimSpace(token.AccessToken) != "" && token.ExpiresAt.After(time.Now().Add(microsoftOAuthExpiryBuffer))
}

func (provider *MicrosoftProvider) refreshOAuth(ctx context.Context, session microsoftSession) (microsoftOAuthToken, error) {
	if strings.TrimSpace(session.RefreshToken) == "" {
		return microsoftOAuthToken{}, cleerrors.New(cleerrors.CodeOAuthTokenRefreshFailed, "Microsoft account has no refresh token")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", provider.clientID)
	form.Set("scope", microsoftScope)
	form.Set("refresh_token", session.RefreshToken)
	status, body, err := provider.doForm(ctx, provider.endpoints.OAuthToken, form, "")
	if err != nil {
		return microsoftOAuthToken{}, err
	}
	defer zeroBytes(body)
	if status != 200 {
		return microsoftOAuthToken{}, cleerrors.Wrap(cleerrors.CodeOAuthTokenRefreshFailed, "OAuth token refresh was rejected", microsoftStatusError("OAuth token endpoint", status))
	}
	var response microsoftTokenResponse
	if err := decodeMicrosoftJSON(body, &response); err != nil {
		return microsoftOAuthToken{}, cleerrors.Wrap(cleerrors.CodeOAuthTokenRefreshFailed, "OAuth token endpoint returned an invalid response", err)
	}
	if response.Error != "" {
		return microsoftOAuthToken{}, cleerrors.Wrap(cleerrors.CodeOAuthTokenRefreshFailed, "OAuth token refresh failed", microsoftOAuthError(response.Error, response.ErrorDescription))
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		return microsoftOAuthToken{}, cleerrors.New(cleerrors.CodeOAuthTokenRefreshFailed, "OAuth token endpoint returned an empty access token")
	}
	token := microsoftOAuthTokenFromResponse(response)
	if token.RefreshToken == "" {
		// The token endpoint may not rotate the refresh token.
		token.RefreshToken = session.RefreshToken
	}
	return token, nil
}

// ensureOAuthToken returns a valid OAuth token, refreshing when the stored
// access token is missing or within the expiry buffer.
func (provider *MicrosoftProvider) ensureOAuthToken(ctx context.Context, session microsoftSession) (microsoftOAuthToken, error) {
	oauth := microsoftOAuthToken{
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
		ExpiresAt:    session.ExpiresAt,
	}
	if oauth.valid() {
		return oauth, nil
	}
	return provider.refreshOAuth(ctx, session)
}

func microsoftOAuthError(code, description string) error {
	if len(code) > 64 || strings.ContainsAny(code, " \t\r\n") {
		code = "invalid_request"
	}
	message := "Microsoft OAuth request failed (" + code + ")"
	if description != "" && len(description) <= 256 {
		message += ": " + description
	}
	return cleerrors.New(cleerrors.CodeOAuthAuthorizationFailed, message)
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
