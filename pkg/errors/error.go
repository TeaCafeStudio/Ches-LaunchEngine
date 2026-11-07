package errors

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Code string

const (
	CodeJavaNotFound                   Code = "JAVA_NOT_FOUND"
	CodeJavaVersionIncompatible        Code = "JAVA_VERSION_INCOMPATIBLE"
	CodeVersionJSONInvalid             Code = "VERSION_JSON_INVALID"
	CodeVersionInheritanceCycle        Code = "VERSION_INHERITANCE_CYCLE"
	CodeLibraryDownloadFailed          Code = "LIBRARY_DOWNLOAD_FAILED"
	CodeAssetDownloadFailed            Code = "ASSET_DOWNLOAD_FAILED"
	CodeHashMismatch                   Code = "HASH_MISMATCH"
	CodeAuthenticationFailed           Code = "AUTHENTICATION_FAILED"
	CodeProfileNotFound                Code = "PROFILE_NOT_FOUND"
	CodeArgumentResolutionFailed       Code = "ARGUMENT_RESOLUTION_FAILED"
	CodeProcessStartFailed             Code = "PROCESS_START_FAILED"
	CodeLoaderInstallFailed            Code = "LOADER_INSTALL_FAILED"
	CodeUnsupportedPlatform            Code = "UNSUPPORTED_PLATFORM"
	CodeInvalidGameDirectory           Code = "INVALID_GAME_DIRECTORY"
	CodeInvalidVersionDirectory        Code = "INVALID_VERSION_DIRECTORY"
	CodeAccountProviderNotFound        Code = "ACCOUNT_PROVIDER_NOT_FOUND"
	CodeAccountNotFound                Code = "ACCOUNT_NOT_FOUND"
	CodeAccountTypeUnsupported         Code = "ACCOUNT_TYPE_UNSUPPORTED"
	CodeSessionInvalid                 Code = "SESSION_INVALID"
	CodeSessionRefreshFailed           Code = "SESSION_REFRESH_FAILED"
	CodeProfileUnavailable             Code = "PROFILE_UNAVAILABLE"
	CodeAuthlibInjectorUnavailable     Code = "AUTHLIB_INJECTOR_UNAVAILABLE"
	CodeAuthlibInjectorDownloadFailed  Code = "AUTHLIB_INJECTOR_DOWNLOAD_FAILED"
	CodeAuthlibInjectorIntegrityFailed Code = "AUTHLIB_INJECTOR_INTEGRITY_FAILED"
	CodeLaunchArgumentBuildFailed      Code = "LAUNCH_ARGUMENT_BUILD_FAILED"
	CodeProcessNotFound                Code = "PROCESS_NOT_FOUND"
	CodeTaskNotFound                   Code = "TASK_NOT_FOUND"
	CodeDownloadCancelled              Code = "DOWNLOAD_CANCELLED"
	CodeInvalidConfiguration           Code = "INVALID_CONFIGURATION"
	CodeAPIAuthenticationFailed        Code = "API_AUTHENTICATION_FAILED"
	CodeOAuthAuthorizationFailed       Code = "OAUTH_AUTHORIZATION_FAILED"
	CodeOAuthTokenRefreshFailed        Code = "OAUTH_TOKEN_REFRESH_FAILED"
	CodeXboxAuthenticationFailed       Code = "XBOX_AUTHENTICATION_FAILED"
	CodeXSTSAuthorizationFailed        Code = "XSTS_AUTHORIZATION_FAILED"
	CodeMinecraftLoginFailed           Code = "MINECRAFT_LOGIN_FAILED"
	CodeMinecraftEntitlementMissing    Code = "MINECRAFT_ENTITLEMENT_MISSING"
)

type Error struct {
	Code      Code              `json:"code"`
	Message   string            `json:"message"`
	Details   map[string]string `json:"details,omitempty"`
	Cause     error             `json:"-"`
	Timestamp time.Time         `json:"timestamp"`
}

func New(code Code, message string) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Timestamp: time.Now().UTC(),
	}
}

func Wrap(code Code, message string, cause error) *Error {
	err := New(code, message)
	err.Cause = cause
	return err
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *Error) WithDetails(details map[string]string) *Error {
	if e == nil {
		return nil
	}
	e.Details = make(map[string]string, len(details))
	for key, value := range details {
		if sensitiveKey(key) {
			e.Details[key] = "[REDACTED]"
			continue
		}
		e.Details[key] = value
	}
	return e
}

func (e *Error) MarshalJSON() ([]byte, error) {
	type publicError struct {
		Code      Code              `json:"code"`
		Message   string            `json:"message"`
		Details   map[string]string `json:"details,omitempty"`
		Timestamp time.Time         `json:"timestamp"`
	}
	details := make(map[string]string, len(e.Details))
	for key, value := range e.Details {
		if sensitiveKey(key) {
			details[key] = "[REDACTED]"
			continue
		}
		details[key] = value
	}
	return json.Marshal(publicError{
		Code:      e.Code,
		Message:   e.Message,
		Details:   details,
		Timestamp: e.Timestamp,
	})
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(key)
	for _, marker := range []string{
		"token", "secret", "password", "authorization", "credential",
		"api_key", "api-key", "apikey", "access_key", "access-key", "accesskey",
		"private_key", "private-key", "privatekey", "cookie", "bearer",
	} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}
