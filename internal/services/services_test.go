package services

import (
	"path/filepath"
	"testing"
)

func TestGameServices(t *testing.T) {
	gameDir := `E:\Rusted Warfare\Rusted Warfare`

	gameSvc := NewGameService()
	if !gameSvc.ValidateGameDir(gameDir) {
		t.Fatalf("ValidateGameDir failed for %s", gameDir)
	}

	prefSvc := NewPrefService()
	statusMap, err := prefSvc.ReadModSettings(gameDir)
	if err != nil {
		t.Fatalf("ReadModSettings error: %v", err)
	}
	t.Logf("Read %d mod settings from preferences.ini", len(statusMap))

	modSvc := NewModService(prefSvc)
	mods, err := modSvc.ScanMods(gameDir)
	if err != nil {
		t.Fatalf("ScanMods error: %v", err)
	}
	t.Logf("Successfully scanned %d mods in %s", len(mods), filepath.Join(gameDir, "mods", "units"))

	enabledCount := 0
	hasThumbCount := 0
	for _, m := range mods {
		if m.Enabled {
			enabledCount++
		}
		if m.Thumbnail != "" {
			hasThumbCount++
		}
	}
	t.Logf("Mods stats: %d enabled, %d with thumbnails extracted", enabledCount, hasThumbCount)

	mapSvc := NewMapService(prefSvc)
	maps, err := mapSvc.ScanMaps(gameDir)
	if err != nil {
		t.Fatalf("ScanMaps error: %v", err)
	}
	t.Logf("Successfully scanned %d maps in %s", len(maps), filepath.Join(gameDir, "mods", "maps"))
}
