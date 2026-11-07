package core

import (
	"regexp"
	"strings"
)

var sensitiveAssignment = regexp.MustCompile(`(?i)(access[_-]?token|refresh[_-]?token|client[_-]?secret|api[_-]?key|access[_-]?key|private[_-]?key|password|authorization|credential|cookie|bearer)(\s*[=:]\s*|\s+)(?:bearer\s+)?(?:"[^"]*"|'[^']*'|[^\s,;]+)`)

func RedactText(value string) string {
	return sensitiveAssignment.ReplaceAllString(value, "$1$2[REDACTED]")
}

func RedactArguments(arguments []string) []string {
	result := append([]string(nil), arguments...)
	for index, argument := range result {
		lower := strings.ToLower(argument)
		if isSensitiveName(lower) {
			if separator := strings.IndexAny(argument, "=:"); separator >= 0 {
				result[index] = argument[:separator+1] + "[REDACTED]"
			} else {
				result[index] = argument
				if index+1 < len(result) {
					result[index+1] = "[REDACTED]"
				}
			}
			continue
		}
		result[index] = RedactText(argument)
	}
	return result
}

func RedactEnvironment(environment map[string]string) map[string]string {
	result := make(map[string]string, len(environment))
	for key, value := range environment {
		if isSensitiveName(strings.ToLower(key)) {
			result[key] = "[REDACTED]"
		} else {
			result[key] = RedactText(value)
		}
	}
	return result
}

func isSensitiveName(value string) bool {
	for _, marker := range []string{
		"token", "secret", "password", "authorization", "credential",
		"api_key", "api-key", "apikey", "access_key", "access-key", "accesskey",
		"private_key", "private-key", "privatekey", "cookie", "bearer",
	} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}
