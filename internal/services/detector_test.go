package services

import (
	"path/filepath"
	"testing"
)

func TestGameDirDetection(t *testing.T) {
	svc := NewGameService()
	dirs := svc.DetectAllGameDirs()

	t.Logf("Detected %d game directories:", len(dirs))
	foundValid := false
	for i, d := range dirs {
		t.Logf("  [%d] Path: %s | Source: %s (%s) | Valid: %v | Version: %s | Mods: %d | Maps: %d | Active: %v",
			i, d.Path, d.Source, d.SourceName, d.IsValid, d.Version, d.ModCount, d.MapCount, d.IsActive)
		if d.IsValid {
			foundValid = true
		}
	}

	if !foundValid {
		t.Log("Note: No valid Rusted Warfare installation detected on this environment (expected if clean machine).")
	}

	known := svc.GetKnownGameDirs()
	t.Logf("Known game directories: %d", len(known))

	// Test switching to an existing valid dir if found
	for _, d := range dirs {
		if d.IsValid {
			err := svc.SwitchGameDir(d.Path)
			if err != nil {
				t.Fatalf("SwitchGameDir failed: %v", err)
			}
			if filepath.Clean(svc.Config.GameDir) != filepath.Clean(d.Path) {
				t.Fatalf("Expected GameDir %s, got %s", d.Path, svc.Config.GameDir)
			}
			t.Logf("Successfully switched GameDir to %s", d.Path)
			break
		}
	}
}
