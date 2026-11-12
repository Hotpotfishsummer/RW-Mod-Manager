package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveMod(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rw_test_archive_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	unitsDir := filepath.Join(tempDir, "mods", "units")
	_ = os.MkdirAll(unitsDir, 0755)

	modFile := filepath.Join(unitsDir, "my_cool_mod.rwmod")
	_ = os.WriteFile(modFile, []byte("fake rwmod zip data"), 0644)

	prefPath := filepath.Join(tempDir, "preferences.ini")
	_ = os.WriteFile(prefPath, []byte("modSettings:CoolMod|AABBCC112233|enabled\r\n"), 0644)

	prefSvc := NewPrefService()
	modSvc := NewModService(prefSvc)

	// 1. 测试安全归档 (移入 _archived/units)
	err = modSvc.ArchiveMod(tempDir, "my_cool_mod.rwmod", false)
	if err != nil {
		t.Fatalf("ArchiveMod 安全归档失败: %v", err)
	}

	if _, err := os.Stat(modFile); !os.IsNotExist(err) {
		t.Errorf("归档后原始目录中的模组文件应已被移除")
	}

	archivedFile := filepath.Join(tempDir, "mods", "_archived", "units", "my_cool_mod.rwmod")
	if _, err := os.Stat(archivedFile); err != nil {
		t.Errorf("归档目录下未找到归档后的模组文件: %v", err)
	}

	// 2. 重新放置文件测试彻底删除
	_ = os.WriteFile(modFile, []byte("recreated data"), 0644)
	err = modSvc.ArchiveMod(tempDir, "my_cool_mod.rwmod", true)
	if err != nil {
		t.Fatalf("ArchiveMod 彻底删除失败: %v", err)
	}

	if _, err := os.Stat(modFile); !os.IsNotExist(err) {
		t.Errorf("彻底删除后原始模组文件依然存在")
	}
}

func TestArchiveMap(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rw_test_map_archive_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mapsDir := filepath.Join(tempDir, "mods", "maps")
	_ = os.MkdirAll(mapsDir, 0755)

	tmxFile := filepath.Join(mapsDir, "battle_1917.tmx")
	thumbFile := filepath.Join(mapsDir, "battle_1917_map.png")
	_ = os.WriteFile(tmxFile, []byte("<map width=\"50\" height=\"50\"></map>"), 0644)
	_ = os.WriteFile(thumbFile, []byte("fake png bytes"), 0644)

	prefSvc := NewPrefService()
	mapSvc := NewMapService(prefSvc)

	// 1. 测试安全归档独立地图及对应出生点预览图
	err = mapSvc.ArchiveMap(tempDir, "battle_1917.tmx", false)
	if err != nil {
		t.Fatalf("ArchiveMap 安全归档失败: %v", err)
	}

	if _, err := os.Stat(tmxFile); !os.IsNotExist(err) {
		t.Errorf("归档后原始目录中的地图文件应已被移除")
	}
	if _, err := os.Stat(thumbFile); !os.IsNotExist(err) {
		t.Errorf("归档后原始目录中的预览图文件应已被移除")
	}

	archivedTmx := filepath.Join(tempDir, "mods", "_archived", "maps", "battle_1917.tmx")
	archivedThumb := filepath.Join(tempDir, "mods", "_archived", "maps", "battle_1917_map.png")
	if _, err := os.Stat(archivedTmx); err != nil {
		t.Errorf("归档目录下未找到地图文件: %v", err)
	}
	if _, err := os.Stat(archivedThumb); err != nil {
		t.Errorf("归档目录下未找到预览图文件: %v", err)
	}

	// 2. 测试地图包归档
	packFile := filepath.Join(mapsDir, "super_pack.rwmod")
	_ = os.WriteFile(packFile, []byte("fake rwmod pack bytes"), 0644)

	err = mapSvc.ArchiveMap(tempDir, "super_pack.rwmod", true)
	if err != nil {
		t.Fatalf("ArchiveMap 彻底删除地图包失败: %v", err)
	}
	if _, err := os.Stat(packFile); !os.IsNotExist(err) {
		t.Errorf("彻底删除后原始地图包文件依然存在")
	}
}
