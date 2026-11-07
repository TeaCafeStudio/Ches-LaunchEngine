package core

import (
	"reflect"
	"testing"
)

func TestRedactArgumentsAndEnvironment(t *testing.T) {
	arguments := RedactArguments([]string{
		"--accessToken", "access-secret",
		"-DclientSecret=client-secret",
		"--password=pass-secret",
	})
	for _, secret := range []string{"access-secret", "client-secret", "pass-secret"} {
		for _, argument := range arguments {
			if argument == secret {
				t.Fatalf("sensitive argument was not redacted: %v", arguments)
			}
		}
	}
	if !reflect.DeepEqual(arguments, []string{
		"--accessToken", "[REDACTED]", "-DclientSecret=[REDACTED]", "--password=[REDACTED]",
	}) {
		t.Fatalf("redacted arguments = %v", arguments)
	}
	environment := RedactEnvironment(map[string]string{
		"REFRESH_TOKEN": "refresh-secret", "MESSAGE": "password=inline-secret",
		"AUTHORIZATION": "Bearer credential-secret",
	})
	if environment["REFRESH_TOKEN"] != "[REDACTED]" || environment["MESSAGE"] != "password=[REDACTED]" {
		t.Fatalf("redacted environment = %v", environment)
	}
	if environment["AUTHORIZATION"] != "[REDACTED]" {
		t.Fatalf("authorization header was not redacted: %v", environment)
	}
	if text := RedactText("Authorization: Bearer live-secret"); text != "Authorization: [REDACTED]" {
		t.Fatalf("redacted authorization text = %q", text)
	}
}
