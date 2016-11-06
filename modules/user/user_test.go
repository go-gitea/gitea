package user

import (
	"os"
	"testing"
)

func TestCurrentUsername(t *testing.T) {
	os.Setenv("USER", "")
	os.Setenv("USERNAME", "foobar")

	user := CurrentUsername()
	if user != "foobar" {
		t.Errorf("expected foobar as user, got: %s", user)
	}

	os.Setenv("USER", "gitea")
	user = CurrentUsername()
	if user != "gitea" {
		t.Errorf("expected gitea as user, got: %s", user)
	}
}
