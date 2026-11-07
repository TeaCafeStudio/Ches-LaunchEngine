package core

import (
	"reflect"
	"testing"
)

func TestRedactArgumentsAndEnvironment(t *testing.T) {
	arguments := RedactArguments([]string{
		"--accessToken", "access-secret",
		"-DclientSecret=client-secret",
		"--******",
	})
	for _, secret := range []string{"access-secret", "client-secret", "pass-secret"} {
		for _, argument := range arguments {
			if argument == secret {
				t.Fatalf("sensitive argument was not redacted: %v", arguments)
			}
		}
	}
	if !reflect.DeepEqual(arguments, []string{
		"--accessToken", "[REDACTED]", "-DclientSecret=[REDACTED]", "--******",
	}) {
		t.Fatalf("redacted arguments = %v", arguments)
	}
	environment := RedactEnvironment(map[string]string{
		"REFRESH_TOKEN": "refresh-secret", "MESSAGE": "******",
		"AUTHORIZATION": "******",
	})
	if environment["REFRESH_TOKEN"] != "[REDACTED]" || environment["MESSAGE"] != "******" {
		t.Fatalf("redacted environment = %v", environment)
	}
	if environment["AUTHORIZATION"] != "[REDACTED]" {
		t.Fatalf("authorization header was not redacted: %v", environment)
	}
	if text := RedactText("Authorization: ******"); text != "Authorization: [REDACTED]" {
		t.Fatalf("redacted authorization text = %q", text)
	}
}

func TestRedactLegacySessionArgument(t *testing.T) {
	const secret = "session-secret"
	got := RedactArguments([]string{"token:" + secret + ":123456781234123412341234567890ab"})
	if len(got) != 1 || got[0] != "token:[REDACTED]" {
		t.Fatalf("RedactArguments() = %v, want token redacted", got)
	}
}
