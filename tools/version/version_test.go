// +build tools

package main

import (
	"testing"
)

// These tests are based exactly on how the Makefile operated previously.

func TestGiteaVersionPriority(t *testing.T) {
	version := GetVersion("v1.0.0", "v1.0.1", "v1.0.2")
	if "v1.0.0" != version {
		t.Errorf("GITEA_VERSION should take priority over DRONE_VERSION or VERSION %s", version)
	}
}

func TestDroneVersionPriority(t *testing.T) {
	version := GetVersion("", "v1.0.1", "v1.0.2")
	if "v1.0.2" != version {
		t.Errorf("VERSION should take priority over DRONE_VERSION %s", version)
	}
}

func TestEnvVersionPriority(t *testing.T) {
	version := GetVersion("", "v1.0.1", "")
	if "1.0.1" != version {
		t.Errorf("DRONE_VERSION should be used when neither GITEA_VERSION nor VERSION are set: %s", version)
	}
}

func TestGitDescribePriority(t *testing.T) {
	version := GetVersion("", "", "")
	if len(version) == 0 {
		t.Errorf("`git describe` should be used for version when none of GITEA_VERSION, DRONE_VERSION, nor VERSION are set: %s", version)
	}
}
