package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildInviteWelcomeURL(t *testing.T) {
	t.Setenv("APP_URL", "https://preview.example.com")

	url := buildInviteWelcomeURL("invite-token-123")
	assert.Equal(t, "https://preview.example.com/welcome/invite?invite_token=invite-token-123", url)
}

func TestBuildInviteWelcomeURLEncodesToken(t *testing.T) {
	t.Setenv("APP_URL", "https://preview.example.com")

	url := buildInviteWelcomeURL("token with spaces")
	assert.Equal(t, "https://preview.example.com/welcome/invite?invite_token=token+with+spaces", url)
}
