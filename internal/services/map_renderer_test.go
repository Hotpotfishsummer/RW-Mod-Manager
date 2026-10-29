package services

import (
	"bytes"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMapRendererNative(t *testing.T) {
	gameDir := `E:\Rusted Warfare\Rusted Warfare`
	if _, err := os.Stat(gameDir); err != nil {
		t.Skip("Game dir not found, skipping renderer test")
	}

	renderer := NewMapRenderer(gameDir)

	// 1. 测试独立地图渲染 (带对象层出生点)
	tmxStandalone := filepath.Join(gameDir, "mods", "maps", "1942 燃烧的东亚(10p)syjh byRW世界地图社.tmx")
	if _, err := os.Stat(tmxStandalone); err == nil {
		pngBytes, err := renderer.RenderStandalone(tmxStandalone, 500)
		if err != nil {
			t.Fatalf("RenderStandalone failed: %v", err)
		}
		img, format, err := image.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("Decode rendered PNG failed: %v", err)
		}
		t.Logf("Standalone map rendered successfully: format=%s, size=%dx%d, bytes=%d",
			format, img.Bounds().Dx(), img.Bounds().Dy(), len(pngBytes))
	}

	// 2. 测试 .rwmod 压缩包内部单个地图渲染与回写
	packPath := filepath.Join(gameDir, "mods", "units", "经典地图包1.rwmod")
	if _, err := os.Stat(packPath); err == nil {
		pngBytes, err := renderer.RenderPackEntry(packPath, "[p2]Joust (2p).tmx", 500)
		if err != nil {
			t.Fatalf("RenderPackEntry failed: %v", err)
		}
		img, format, err := image.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("Decode pack rendered PNG failed: %v", err)
		}
		t.Logf("Pack map rendered & patched successfully: format=%s, size=%dx%d, bytes=%d",
			format, img.Bounds().Dx(), img.Bounds().Dy(), len(pngBytes))
	}

	// 3. 测试 MapService.GenerateThumbnail 方法调用
	prefSvc := NewPrefService()
	mapSvc := NewMapService(prefSvc)
	b64, err := mapSvc.GenerateThumbnail(gameDir, packPath+":[p10] Napali Warfare (10p).tmx")
	if err != nil {
		t.Fatalf("MapService.GenerateThumbnail failed: %v", err)
	}
	if !strings.HasPrefix(b64, "data:image/png;base64,") {
		t.Fatalf("Invalid base64 prefix: %s", b64[:30])
	}
	// 4. 测试 WW1 模组地图渲染与回写
	ww1Path := filepath.Join(gameDir, "mods", "units", "WW1ToendallWarsv1.5.rwmod")
	if _, err := os.Stat(ww1Path); err == nil {
		e1 := "WW1 To end all Wars v1.5/maps/(ww1)_(4p)_Entrenched_village.tmx"
		png1, err := renderer.RenderPackEntry(ww1Path, e1, 500)
		if err != nil {
			t.Fatalf("RenderPackEntry e1 failed: %v", err)
		}
		img1, _, _ := image.Decode(bytes.NewReader(png1))
		t.Logf("WW1 Entrenched_village rendered & patched: %dx%d, bytes=%d",
			img1.Bounds().Dx(), img1.Bounds().Dy(), len(png1))

		e2 := "WW1 To end all Wars v1.5/maps/(ww1)_(8p)_Kaiserschlacht-2nd_Battle_of_the_Marne.tmx"
		png2, err := renderer.RenderPackEntry(ww1Path, e2, 500)
		if err != nil {
			t.Fatalf("RenderPackEntry e2 failed: %v", err)
		}
		img2, _, _ := image.Decode(bytes.NewReader(png2))
		t.Logf("WW1 Kaiserschlacht rendered & patched: %dx%d, bytes=%d",
			img2.Bounds().Dx(), img2.Bounds().Dy(), len(png2))
	}
}
