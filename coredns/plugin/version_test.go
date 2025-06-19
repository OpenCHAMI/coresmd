package plugin

import (
	"testing"

	"github.com/OpenCHAMI/coresmd/internal/version"
)

func TestVersionIntegration(t *testing.T) {
	// Test that version information is available
	if version.Version == "" {
		t.Log("Version is empty (this is normal for development builds)")
	} else {
		t.Logf("Version: %s", version.Version)
	}

	if version.GitState == "" {
		t.Log("GitState is empty (this is normal for development builds)")
	} else {
		t.Logf("GitState: %s", version.GitState)
	}

	if version.BuildTime == "" {
		t.Log("BuildTime is empty (this is normal for development builds)")
	} else {
		t.Logf("BuildTime: %s", version.BuildTime)
	}

	// Test that VersionInfo map is populated
	if len(version.VersionInfo) == 0 {
		t.Error("VersionInfo map should not be empty")
	}

	// Check that expected keys exist in VersionInfo
	expectedKeys := []string{"version", "state", "build_timestamp", "go_version"}
	for _, key := range expectedKeys {
		if _, exists := version.VersionInfo[key]; !exists {
			t.Errorf("Expected key '%s' not found in VersionInfo", key)
		}
	}
}
