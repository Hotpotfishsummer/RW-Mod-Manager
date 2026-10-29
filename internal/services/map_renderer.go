package services

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// GID 翻转位标志 (Tiled 规范)
const (
	FlipH   uint32 = 0x80000000
	FlipV   uint32 = 0x40000000
	FlipD   uint32 = 0x20000000
	MaskGID uint32 = 0x1FFFFFFF
)

// 24 支队伍官方配色表 (与 preferences.ini 及官方 draw_spawn_points 对齐)
var TeamBadgeColors = map[int]color.RGBA{
	1:  {0, 255, 0, 255},     // 绿色
	2:  {208, 32, 19, 255},   // 红色
	3:  {4, 99, 243, 255},    // 蓝色
	4:  {255, 255, 64, 255},  // 黄色
	5:  {0, 255, 255, 255},   // 青色
	6:  {208, 248, 247, 255}, // 浅青白
	7:  {180, 180, 180, 255}, // 灰色
	8:  {255, 0, 234, 255},   // 粉色
	9:  {255, 127, 24, 255},  // 橙色
	10: {147, 104, 196, 255}, // 紫色
	11: {173, 255, 47, 255},  // 黄绿
	12: {0, 180, 180, 255},   // 深青
	13: {138, 43, 226, 255},  // 蓝紫
	14: {255, 20, 147, 255},  // 深粉
	15: {205, 133, 63, 255},  // 赭石
	16: {255, 215, 0, 255},   // 金黄
	17: {30, 144, 255, 255},  // 宝石蓝
	18: {238, 130, 238, 255}, // 罗兰紫
	19: {50, 205, 50, 255},   // 酸橙绿
	20: {220, 20, 60, 255},   // 猩红
	21: {255, 127, 80, 255},  // 珊瑚红
	22: {106, 90, 205, 255},  // 瓦蓝
	23: {255, 140, 0, 255},   // 深橙
	24: {224, 224, 224, 255}, // 银白
}

var layerPrio = map[string]int{
	"ground":  0,
	"ground2": 1,
	"items":   2,
	"units":   3,
	"objects": 4,
}

var (
	reTeamTag        = regexp.MustCompile(`(?i)\(t:(-?[0-9]+)\)|\bt:([0-9]+)\b`)
	reSpawnName      = regexp.MustCompile(`(?i)(?:spawn|player|p)[_\s\-]?([0-9]+)\b`)
	reTrailingDigits = regexp.MustCompile(`^(.*[a-zA-Z_])\d+(\.[a-zA-Z0-9]+)$`)
	reCopySuffix     = regexp.MustCompile(`(?i)\s*-\s*(?:copie|copy|副本)(?:\s*\(\d+\))?(\.[a-zA-Z0-9]+)$`)
)

// TMX XML 数据映射
type TmxMapDoc struct {
	XMLName      xml.Name         `xml:"map"`
	Width        int              `xml:"width,attr"`
	Height       int              `xml:"height,attr"`
	TileWidth    int              `xml:"tilewidth,attr"`
	TileHeight   int              `xml:"tileheight,attr"`
	Properties   []TmxProp        `xml:"properties>property"`
	Tilesets     []TmxTilesetRef  `xml:"tileset"`
	Layers       []TmxLayerDoc    `xml:"layer"`
	ObjectGroups []TmxObjGroupDoc `xml:"objectgroup"`
}

type TmxProp struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
	Text  string `xml:",chardata"`
}

func (p TmxProp) Val() string {
	if p.Value != "" {
		return p.Value
	}
	return strings.TrimSpace(p.Text)
}

type TmxTilesetRef struct {
	FirstGID   int         `xml:"firstgid,attr"`
	Source     string      `xml:"source,attr"`
	Name       string      `xml:"name,attr"`
	TileWidth  int         `xml:"tilewidth,attr"`
	TileHeight int         `xml:"tileheight,attr"`
	Columns    int         `xml:"columns,attr"`
	TileCount  int         `xml:"tilecount,attr"`
	Image      *TmxImgRef  `xml:"image"`
	Properties []TmxProp   `xml:"properties>property"`
	Tiles      []TmxTileEl `xml:"tile"`
}

type TmxImgRef struct {
	Source string `xml:"source,attr"`
	Trans  string `xml:"trans,attr"`
}

type TmxTileEl struct {
	ID         int       `xml:"id,attr"`
	Properties []TmxProp `xml:"properties>property"`
}

type TmxLayerDoc struct {
	Name   string     `xml:"name,attr"`
	Width  int        `xml:"width,attr"`
	Height int        `xml:"height,attr"`
	Data   TmxDataDoc `xml:"data"`
}

type TmxDataDoc struct {
	Encoding    string `xml:"encoding,attr"`
	Compression string `xml:"compression,attr"`
	Raw         string `xml:",chardata"`
}

type TmxObjGroupDoc struct {
	Name    string      `xml:"name,attr"`
	Objects []TmxObjDoc `xml:"object"`
}

type TmxObjDoc struct {
	ID         int       `xml:"id,attr"`
	Name       string    `xml:"name,attr"`
	Type       string    `xml:"type,attr"`
	X          float64   `xml:"x,attr"`
	Y          float64   `xml:"y,attr"`
	Width      float64   `xml:"width,attr"`
	Height     float64   `xml:"height,attr"`
	Properties []TmxProp `xml:"properties>property"`
}

// LoadedTileset 内存已切片图集
type LoadedTileset struct {
	FirstGID   int
	Name       string
	TileWidth  int
	TileHeight int
	Columns    int
	TileCount  int
	Tiles      []image.Image
	TileTeam   []int
	TileCC     []bool
}

// FileResolver 统一文件搜索抽象
type FileResolver interface {
	ReadFile(rel string) ([]byte, error)
	FindImage(src string) (image.Image, string, error)
}

// MapRenderer 纯 Go 实现的地图高清缩略图渲染器
type MapRenderer struct {
	gameDir     string
	searchRoots []string
	fontOnce    sync.Once
	fontFace    font.Face
}

func NewMapRenderer(gameDir string) *MapRenderer {
	r := &MapRenderer{
		gameDir: gameDir,
	}
	r.initSearchRoots()
	return r
}

func (r *MapRenderer) initSearchRoots() {
	roots := []string{r.gameDir}
	for _, base := range []string{
		r.gameDir,
		filepath.Join(r.gameDir, "assets"),
		filepath.Join(r.gameDir, "assets", "tilesets"),
		filepath.Join(r.gameDir, "mods"),
		filepath.Join(r.gameDir, "mods", "maps"),
	} {
		for _, sub := range []string{
			"",
			"bitmaps",
			"tilesets",
			"tilesets/bitmaps",
			"tilesets/ridges",
			"tilesets/ridges/bitmaps",
			"tilesets/terrain",
			"tilesets/terrain/bitmaps",
			"maps",
			"maps/bitmaps",
			"maps/ridges",
			"maps/ridges/bitmaps",
			"maps/terrain",
			"maps/terrain/bitmaps",
			"ridges",
			"ridges/bitmaps",
			"terrain",
			"terrain/bitmaps",
		} {
			p := filepath.Clean(filepath.Join(base, sub))
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				found := false
				for _, exist := range roots {
					if strings.EqualFold(exist, p) {
						found = true
						break
					}
				}
				if !found {
					roots = append(roots, p)
				}
			}
		}
	}
	r.searchRoots = roots
}

func (r *MapRenderer) getFontFace(fontSize float64) font.Face {
	r.fontOnce.Do(func() {
		candidates := []string{
			filepath.Join(r.gameDir, "font", "NotoSans-Bold.ttf"),
			filepath.Join(r.gameDir, "font", "Roboto-Bold.ttf"),
			`C:\Windows\Fonts\arialbd.ttf`,
			`C:\Windows\Fonts\segoeuib.ttf`,
			`C:\Windows\Fonts\msyhbd.ttc`,
		}
		for _, path := range candidates {
			if data, err := os.ReadFile(path); err == nil {
				if parsed, err := opentype.Parse(data); err == nil {
					if face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
						Size:    16,
						DPI:     72,
						Hinting: font.HintingFull,
					}); err == nil {
						r.fontFace = face
						return
					}
				}
			}
		}
		r.fontFace = basicfont.Face7x13
	})
	if r.fontFace == nil {
		return basicfont.Face7x13
	}
	return r.fontFace
}

// isImageMostlyEmpty 检测图像是否几乎全黑或全透明 (例如地面图层全部缺失，仅有少量稀疏点)
func isImageMostlyEmpty(img image.Image) bool {
	if img == nil {
		return true
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return true
	}

	totalSampled := 0
	nonEmptyCount := 0

	// 步长采样以保持极高性能
	step := 3
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			totalSampled++
			c := img.At(x, y)
			r, g, b, a := c.RGBA()
			uA := a >> 8
			if uA > 20 {
				brightness := ((r >> 8)*299 + (g >> 8)*587 + (b >> 8)*114) / 1000
				if brightness > 15 {
					nonEmptyCount++
				}
			}
		}
	}

	if totalSampled == 0 {
		return true
	}

	ratio := float64(nonEmptyCount) / float64(totalSampled)
	return ratio < 0.12
}

// RenderTMXBytes 将 TMX 二进制数据渲染为 PNG 字节流 (便捷接口)
func (r *MapRenderer) RenderTMXBytes(tmxBytes []byte, tmxName string, resolver FileResolver, maxDim int) ([]byte, error) {
	return r.RenderTMXBytesWithFallback(tmxBytes, tmxName, resolver, maxDim, nil)
}

// RenderTMXBytesWithFallback 将 TMX 二进制数据渲染为 PNG 字节流，并在渲染图块缺失/黑屏时智能回退至已有底图
func (r *MapRenderer) RenderTMXBytesWithFallback(tmxBytes []byte, tmxName string, resolver FileResolver, maxDim int, existingBytes []byte) ([]byte, error) {
	var doc TmxMapDoc
	if err := xml.Unmarshal(tmxBytes, &doc); err != nil {
		return nil, fmt.Errorf("TMX XML 解析失败: %w", err)
	}
	if doc.Width <= 0 || doc.Height <= 0 {
		return nil, fmt.Errorf("无效地图尺寸: %dx%d", doc.Width, doc.Height)
	}

	mapTW := doc.TileWidth
	if mapTW <= 0 {
		mapTW = 32
	}
	mapTH := doc.TileHeight
	if mapTH <= 0 {
		mapTH = 32
	}

	fullW := doc.Width * mapTW
	fullH := doc.Height * mapTH

	// 计算最终缩略图尺寸 (限制在 maxDim 范围以内)
	targetW := fullW
	targetH := fullH
	if max(fullW, fullH) > maxDim {
		ratio := float64(maxDim) / float64(max(fullW, fullH))
		targetW = max(1, int(math.Round(float64(fullW)*ratio)))
		targetH = max(1, int(math.Round(float64(fullH)*ratio)))
	}

	// 载入所有图集
	tilesets := r.loadTilesets(&doc, resolver)

	// 收集并排序视觉图层
	type layerEntry struct {
		prio  int
		index int
		name  string
		doc   *TmxLayerDoc
	}
	var entries []layerEntry
	for i := range doc.Layers {
		lay := &doc.Layers[i]
		low := strings.ToLower(strings.TrimSpace(lay.Name))
		if low == "set" {
			continue // 忽略 automapping 原图层
		}
		p, ok := layerPrio[low]
		if !ok {
			p = 5
		}
		entries = append(entries, layerEntry{prio: p, index: i, name: lay.Name, doc: lay})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].prio != entries[j].prio {
			return entries[i].prio < entries[j].prio
		}
		return entries[i].index < entries[j].index
	})

	// 为防止巨大地图占用过多内存，如果全图超出 4000x4000 则中间使用缩放画布
	scaleCap := 4000.0
	scale := 1.0
	if float64(max(fullW, fullH)) > scaleCap {
		scale = scaleCap / float64(max(fullW, fullH))
	}
	midTW := mapTW
	midTH := mapTH
	if scale != 1.0 {
		midTW = max(1, int(math.Round(float64(mapTW)*scale)))
		midTH = max(1, int(math.Round(float64(mapTH)*scale)))
	}
	midW := doc.Width * midTW
	midH := doc.Height * midTH

	canvas := image.NewRGBA(image.Rect(0, 0, midW, midH))

	// 翻转图像缓存
	flipCache := make(map[string]image.Image)

	for _, entry := range entries {
		gids, err := decodeTmxData(entry.doc.Data, doc.Width, doc.Height)
		if err != nil {
			continue
		}
		for y := 0; y < doc.Height; y++ {
			for x := 0; x < doc.Width; x++ {
				gid := gids[y*doc.Width+x]
				if gid == 0 {
					continue
				}
				ts, local, flags := resolveGID(tilesets, gid)
				if ts == nil || local < 0 || local >= len(ts.Tiles) || ts.Tiles[local] == nil {
					continue
				}
				tileImg := ts.Tiles[local]
				if flags != 0 {
					cacheKey := fmt.Sprintf("%d_%d_%d", ts.FirstGID, local, flags)
					if cached, ok := flipCache[cacheKey]; ok {
						tileImg = cached
					} else {
						flipped := applyFlip(tileImg, flags)
						flipCache[cacheKey] = flipped
						tileImg = flipped
					}
				}

				destRect := image.Rect(x*midTW, y*midTH, (x+1)*midTW, (y+1)*midTH)
				draw.Draw(canvas, destRect, tileImg, tileImg.Bounds().Min, draw.Over)
			}
		}
	}

	// 检查渲染出的地图内容是否有效 (防止图集缺失造成黑屏)
	isCanvasEmpty := isImageMostlyEmpty(canvas)
	var baseImg image.Image = canvas

	// 如果新渲染画面缺失地表呈黑屏，且调用方传入了已有底图，则验证原图是否可用
	if isCanvasEmpty && len(existingBytes) > 0 {
		if origImg, _, err := image.Decode(bytes.NewReader(existingBytes)); err == nil {
			if !isImageMostlyEmpty(origImg) {
				// 成功恢复原底图！以原底图为基准绘制，避免破坏用户的已有预览
				baseImg = origImg
			}
		}
	}

	// 最终目标缩略图画布
	finalImg := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	// 填充纯黑背景
	draw.Draw(finalImg, finalImg.Bounds(), image.NewUniform(color.RGBA{0, 0, 0, 255}), image.Point{}, draw.Src)
	// 高质量缩放
	draw.BiLinear.Scale(finalImg, finalImg.Bounds(), baseImg, baseImg.Bounds(), draw.Over, nil)

	// 出生点坐标检测与标记绘制
	positions := r.detectSpawnPositions(&doc, tilesets, fullW, fullH, targetW, targetH)
	if len(positions) > 0 {
		face := r.getFontFace(16)
		for tid, pt := range positions {
			drawSpawnBadge(finalImg, int(math.Round(pt.X)), int(math.Round(pt.Y)), tid, face)
		}
	}

	// 编码为 PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, finalImg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type PointF struct {
	X, Y float64
}

// detectSpawnPositions 精准解析地图出生点坐标 (支持对象层与图块层)
func (r *MapRenderer) detectSpawnPositions(doc *TmxMapDoc, tilesets []*LoadedTileset, fullW, fullH, thumbW, thumbH int) map[int]PointF {
	positions := make(map[int]PointF)
	sx := float64(thumbW) / float64(fullW)
	sy := float64(thumbH) / float64(fullH)

	// 阶段 1: 扫描所有 <objectgroup> 对象层
	for _, og := range doc.ObjectGroups {
		for _, obj := range og.Objects {
			props := make(map[string]string)
			for _, p := range obj.Properties {
				props[strings.ToLower(p.Name)] = p.Val()
			}
			oname := strings.TrimSpace(obj.Name)
			otype := strings.TrimSpace(obj.Type)
			lowName := strings.ToLower(oname)
			lowType := strings.ToLower(otype)

			isCC := strings.EqualFold(props["unit"], "commandcenter") ||
				strings.Contains(lowName, "commandcenter") ||
				strings.Contains(lowType, "commandcenter") ||
				strings.HasPrefix(lowName, "spawn") ||
				strings.HasPrefix(lowName, "factionselector")

			teamID := -1
			if teamStr, ok := props["team"]; ok {
				if t, err := strconv.Atoi(teamStr); err == nil && t >= 0 {
					teamID = t + 1 // 0-based team -> 1-based player
				}
			}
			if teamID == -1 {
				if m := reTeamTag.FindStringSubmatch(oname); len(m) > 1 {
					targetStr := m[1]
					if targetStr == "" && len(m) > 2 {
						targetStr = m[2]
					}
					if t, err := strconv.Atoi(targetStr); err == nil && t >= 0 {
						teamID = t + 1
					}
				}
			}
			if teamID == -1 {
				if m := reSpawnName.FindStringSubmatch(oname); len(m) > 1 {
					if t, err := strconv.Atoi(m[1]); err == nil && t >= 1 && t <= 24 {
						teamID = t
					}
				}
			}

			if isCC && teamID > 0 {
				cx := obj.X + obj.Width/2.0
				cy := obj.Y + obj.Height/2.0
				positions[teamID] = PointF{X: cx * sx, Y: cy * sy}
			}
		}
	}

	// 阶段 2: 扫描全图所有图块层中的指挥中心图块 (Units/Items/Triggers 等图层)
	tileCCCells := make(map[int][]PointF)
	for _, lay := range doc.Layers {
		gids, err := decodeTmxData(lay.Data, doc.Width, doc.Height)
		if err != nil {
			continue
		}
		for y := 0; y < doc.Height; y++ {
			for x := 0; x < doc.Width; x++ {
				gid := gids[y*doc.Width+x]
				if gid == 0 {
					continue
				}
				ts, local, _ := resolveGID(tilesets, gid)
				if ts != nil && local < len(ts.TileCC) && ts.TileCC[local] && ts.TileTeam[local] >= 0 {
					tid := ts.TileTeam[local] + 1 // 0-based 队伍映射到 1-based 玩家
					tileCCCells[tid] = append(tileCCCells[tid], PointF{X: float64(x), Y: float64(y)})
				}
			}
		}
	}

	mapTW := float64(doc.TileWidth)
	mapTH := float64(doc.TileHeight)
	for tid, cells := range tileCCCells {
		if _, exists := positions[tid]; !exists && len(cells) > 0 {
			var sumX, sumY float64
			for _, c := range cells {
				sumX += c.X
				sumY += c.Y
			}
			avgX := sumX/float64(len(cells)) + 0.5
			avgY := sumY/float64(len(cells)) + 0.5
			wx := avgX * mapTW
			wy := avgY * mapTH
			positions[tid] = PointF{X: wx * sx, Y: wy * sy}
		}
	}

	return positions
}

func (r *MapRenderer) loadTilesets(doc *TmxMapDoc, resolver FileResolver) []*LoadedTileset {
	var out []*LoadedTileset
	for _, tsRef := range doc.Tilesets {
		curTs := tsRef
		// 如果是外部引用 <tileset source="...tsx"/>
		if curTs.Source != "" && resolver != nil {
			if data, err := resolver.ReadFile(curTs.Source); err == nil {
				var extTs TmxTilesetRef
				if err := xml.Unmarshal(data, &extTs); err == nil {
					extTs.FirstGID = curTs.FirstGID
					if extTs.Name == "" {
						extTs.Name = curTs.Source
					}
					curTs = extTs
				}
			}
		}

		tw := curTs.TileWidth
		if tw <= 0 {
			tw = doc.TileWidth
		}
		th := curTs.TileHeight
		if th <= 0 {
			th = doc.TileHeight
		}

		var tiles []image.Image
		tileCount := curTs.TileCount
		cols := curTs.Columns

		// 载入图集位图 (支持外部文件与 embedded_png 内嵌 Base64)
		var atlasImg image.Image
		var transHex string

		if resolver != nil && curTs.Image != nil && curTs.Image.Source != "" {
			if img, trans, err := resolver.FindImage(curTs.Image.Source); err == nil && img != nil {
				atlasImg = img
				transHex = trans
			}
		}

		// 若未从文件找到，检查图集属性或图块属性中的 embedded_png 内嵌 Base64 图集
		if atlasImg == nil {
			var embeddedB64 string
			for _, p := range curTs.Properties {
				if strings.EqualFold(p.Name, "embedded_png") && p.Val() != "" {
					embeddedB64 = p.Val()
					break
				}
			}
			if embeddedB64 == "" {
				for _, t := range curTs.Tiles {
					for _, p := range t.Properties {
						if strings.EqualFold(p.Name, "embedded_png") && p.Val() != "" {
							embeddedB64 = p.Val()
							break
						}
					}
					if embeddedB64 != "" {
						break
					}
				}
			}
			if embeddedB64 != "" {
				cleanB64 := strings.Map(func(r rune) rune {
					if unicode.IsSpace(r) {
						return -1
					}
					return r
				}, embeddedB64)
				if raw, err := base64.StdEncoding.DecodeString(cleanB64); err == nil {
					if dec, _, err := image.Decode(bytes.NewReader(raw)); err == nil {
						atlasImg = dec
					}
				}
			}
		}

		if atlasImg != nil {
			if curTs.Image != nil && curTs.Image.Trans != "" {
				transHex = curTs.Image.Trans
			}
			imgRGBA := applyTransparency(atlasImg, transHex)
			b := imgRGBA.Bounds()
			if cols <= 0 {
				cols = max(1, b.Dx()/tw)
			}
			rows := max(1, b.Dy()/th)
			for local := 0; local < cols*rows; local++ {
				tx := (local % cols) * tw
				ty := (local / cols) * th
				if tx+tw > b.Dx() || ty+th > b.Dy() {
					tiles = append(tiles, nil)
				} else {
					sub := imgRGBA.SubImage(image.Rect(b.Min.X+tx, b.Min.Y+ty, b.Min.X+tx+tw, b.Min.Y+ty+th))
					tiles = append(tiles, sub)
				}
			}
		}

		if len(tiles) == 0 {
			if tileCount <= 0 {
				tileCount = 200
			}
			tiles = make([]image.Image, tileCount)
		}

		tileTeam := make([]int, len(tiles))
		tileCC := make([]bool, len(tiles))
		for i := range tileTeam {
			tileTeam[i] = -1
		}

		for _, tileEl := range curTs.Tiles {
			tid := tileEl.ID
			if tid < 0 {
				continue
			}
			if tid >= len(tiles) {
				diff := tid - len(tiles) + 1
				for k := 0; k < diff; k++ {
					tiles = append(tiles, nil)
					tileTeam = append(tileTeam, -1)
					tileCC = append(tileCC, false)
				}
			}
			for _, p := range tileEl.Properties {
				name := strings.ToLower(p.Name)
				val := p.Val()
				if name == "team" {
					if t, err := strconv.Atoi(val); err == nil {
						tileTeam[tid] = t
					}
				} else if name == "unit" && strings.EqualFold(val, "commandcenter") {
					tileCC[tid] = true
				}
			}
		}

		out = append(out, &LoadedTileset{
			FirstGID:   curTs.FirstGID,
			Name:       curTs.Name,
			TileWidth:  tw,
			TileHeight: th,
			Columns:    cols,
			TileCount:  len(tiles),
			Tiles:      tiles,
			TileTeam:   tileTeam,
			TileCC:     tileCC,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].FirstGID < out[j].FirstGID
	})
	return out
}

func resolveGID(tilesets []*LoadedTileset, gid uint32) (*LoadedTileset, int, uint32) {
	if gid == 0 {
		return nil, 0, 0
	}
	flags := gid & (FlipH | FlipV | FlipD)
	base := int(gid & MaskGID)

	var cand *LoadedTileset
	for _, ts := range tilesets {
		if ts.FirstGID <= base {
			cand = ts
		} else {
			break
		}
	}
	if cand == nil {
		return nil, 0, 0
	}
	local := base - cand.FirstGID
	if local < 0 || local >= len(cand.Tiles) {
		return nil, 0, 0
	}
	return cand, local, flags
}

func decodeTmxData(data TmxDataDoc, w, h int) ([]uint32, error) {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, data.Raw)

	raw, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return nil, err
	}

	var reader io.Reader = bytes.NewReader(raw)
	if data.Compression == "zlib" {
		zr, err := zlib.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		reader = zr
	} else if data.Compression == "gzip" {
		gr, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		reader = gr
	}

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	expected := w * h * 4
	if len(decompressed) < expected {
		return nil, fmt.Errorf("图层数据长度不足: %d < %d", len(decompressed), expected)
	}

	gids := make([]uint32, w*h)
	for i := 0; i < w*h; i++ {
		gids[i] = binary.LittleEndian.Uint32(decompressed[i*4 : (i+1)*4])
	}
	return gids, nil
}

func applyFlip(src image.Image, flags uint32) image.Image {
	if flags == 0 || src == nil {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	flipH := (flags & FlipH) != 0
	flipV := (flags & FlipV) != 0
	flipD := (flags & FlipD) != 0

	outW, outH := w, h
	if flipD {
		outW, outH = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := x
			sy := y
			if flipH {
				sx = w - 1 - sx
			}
			if flipV {
				sy = h - 1 - sy
			}
			dx, dy := sx, sy
			if flipD {
				dx, dy = sy, sx
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func applyTransparency(img image.Image, transHex string) *image.RGBA {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	var transR, transG, transB uint8
	hasTrans := false
	if len(transHex) == 6 {
		if r, err := strconv.ParseUint(transHex[0:2], 16, 8); err == nil {
			if g, err := strconv.ParseUint(transHex[2:4], 16, 8); err == nil {
				if b, err := strconv.ParseUint(transHex[4:6], 16, 8); err == nil {
					transR = uint8(r)
					transG = uint8(g)
					transB = uint8(b)
					hasTrans = true
				}
			}
		}
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r, g, b, a := c.RGBA()
			uR := uint8(r >> 8)
			uG := uint8(g >> 8)
			uB := uint8(b >> 8)
			uA := uint8(a >> 8)
			if hasTrans && uR == transR && uG == transG && uB == transB {
				uA = 0
			}
			rgba.SetRGBA(x, y, color.RGBA{R: uR, G: uG, B: uB, A: uA})
		}
	}
	return rgba
}

func drawSpawnBadge(dst *image.RGBA, cx, cy int, tid int, fFace font.Face) {
	dispColor, ok := TeamBadgeColors[tid]
	if !ok {
		dispColor = color.RGBA{255, 255, 255, 255}
	}

	w := dst.Bounds().Dx()
	h := dst.Bounds().Dy()
	eSize := int(float64(min(w, h)) * 0.026)
	if eSize < 9 {
		eSize = 9
	}
	if eSize > 14 {
		eSize = 14
	}

	// 1. 黑色半透明底圆与白色微光描边
	drawCircle(dst, cx, cy, eSize, color.RGBA{0, 0, 0, 160}, color.RGBA{255, 255, 255, 140})

	// 2. 居中绘制加粗队伍色数字
	txt := strconv.Itoa(tid)
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(dispColor),
		Face: fFace,
	}
	advance := d.MeasureString(txt)
	textW := advance.Round()
	metrics := fFace.Metrics()
	textH := metrics.Ascent.Round()

	tx := cx - textW/2
	ty := cy + textH/2 - 2

	// 阴影
	shadowDrawer := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.RGBA{0, 0, 0, 220}),
		Face: fFace,
		Dot:  fixed.P(tx+1, ty+1),
	}
	shadowDrawer.DrawString(txt)

	d.Dot = fixed.P(tx, ty)
	d.DrawString(txt)
}

func drawCircle(dst *image.RGBA, cx, cy, r int, fill, border color.RGBA) {
	minX := max(0, cx-r-1)
	maxX := min(dst.Bounds().Max.X, cx+r+2)
	minY := max(0, cy-r-1)
	maxY := min(dst.Bounds().Max.Y, cy+r+2)

	r2 := float64(r * r)
	innerR2 := float64((r - 1) * (r - 1))
	outerR2 := float64((r + 1) * (r + 1))

	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			dx := float64(x - cx)
			dy := float64(y - cy)
			d2 := dx*dx + dy*dy
			if d2 <= innerR2 {
				blendPixel(dst, x, y, fill)
			} else if d2 <= r2 {
				blendPixel(dst, x, y, fill)
				blendPixel(dst, x, y, border)
			} else if d2 <= outerR2 {
				factor := (outerR2 - d2) / (outerR2 - r2)
				b := border
				b.A = uint8(float64(border.A) * factor)
				blendPixel(dst, x, y, b)
			}
		}
	}
}

func blendPixel(dst *image.RGBA, x, y int, src color.RGBA) {
	if src.A == 0 {
		return
	}
	off := dst.PixOffset(x, y)
	if src.A == 255 {
		dst.Pix[off] = src.R
		dst.Pix[off+1] = src.G
		dst.Pix[off+2] = src.B
		dst.Pix[off+3] = 255
		return
	}
	sa := uint32(src.A)
	da := 255 - sa
	dst.Pix[off] = uint8((uint32(src.R)*sa + uint32(dst.Pix[off])*da) / 255)
	dst.Pix[off+1] = uint8((uint32(src.G)*sa + uint32(dst.Pix[off+1])*da) / 255)
	dst.Pix[off+2] = uint8((uint32(src.B)*sa + uint32(dst.Pix[off+2])*da) / 255)
	dst.Pix[off+3] = 255
}

// buildPathCandidates 生成针对松散命名与编辑器后缀的文件名候选集合
func buildPathCandidates(rel string) []string {
	var candidates []string
	seen := make(map[string]bool)

	add := func(s string) {
		s = filepath.ToSlash(strings.TrimSpace(s))
		if s == "" || seen[strings.ToLower(s)] {
			return
		}
		seen[strings.ToLower(s)] = true
		candidates = append(candidates, s)
	}

	norm := filepath.ToSlash(rel)
	add(norm)

	clean := filepath.ToSlash(filepath.Clean(norm))
	clean = strings.TrimPrefix(clean, "../")
	clean = strings.TrimPrefix(clean, "./")
	add(clean)

	base := filepath.Base(clean)
	add(base)

	// 去除末尾数字后缀 (如 dirt2longgrass_flat1.png -> dirt2longgrass_flat.png)
	if m := reTrailingDigits.FindStringSubmatch(base); len(m) > 2 {
		noDigitBase := m[1] + m[2]
		dir := filepath.Dir(clean)
		if dir != "." && dir != "/" && dir != "" {
			add(filepath.ToSlash(filepath.Join(dir, noDigitBase)))
		}
		add(noDigitBase)
	}

	// 去除副本后缀 (如 farms - Copie.png -> farms.png)
	if m := reCopySuffix.FindStringSubmatch(base); len(m) > 1 {
		idx := reCopySuffix.FindStringIndex(base)
		if idx != nil && idx[0] > 0 {
			noCopyBase := base[:idx[0]] + m[1]
			dir := filepath.Dir(clean)
			if dir != "." && dir != "/" && dir != "" {
				add(filepath.ToSlash(filepath.Join(dir, noCopyBase)))
			}
			add(noCopyBase)
		}
	}

	// 路径切片级候选 (如 maps/terrain/bitmaps/dirt.png -> terrain/bitmaps/dirt.png, bitmaps/dirt.png)
	parts := strings.Split(clean, "/")
	for i := 1; i < len(parts); i++ {
		sub := strings.Join(parts[i:], "/")
		add(sub)
		if m := reTrailingDigits.FindStringSubmatch(parts[len(parts)-1]); len(m) > 2 {
			subParts := make([]string, len(parts[i:]))
			copy(subParts, parts[i:])
			subParts[len(subParts)-1] = m[1] + m[2]
			add(strings.Join(subParts, "/"))
		}
	}

	return candidates
}

// DiskResolver 针对磁盘独立文件的资源解析器
type DiskResolver struct {
	baseDir     string
	searchRoots []string
}

func (d *DiskResolver) ReadFile(rel string) ([]byte, error) {
	candidates := buildPathCandidates(rel)

	// 1. 先在 baseDir 下查找
	for _, cand := range candidates {
		p := filepath.Join(d.baseDir, filepath.FromSlash(cand))
		if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
			return data, nil
		}
	}

	// 2. 在所有 searchRoots 搜索根下查找
	for _, root := range d.searchRoots {
		for _, cand := range candidates {
			p := filepath.Join(root, filepath.FromSlash(cand))
			if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("file not found: %s", rel)
}

func (d *DiskResolver) FindImage(src string) (image.Image, string, error) {
	data, err := d.ReadFile(src)
	if err != nil {
		return nil, "", err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, "", err
}

// ZipResolver 针对 .rwmod 压缩包内条目的资源解析器
type ZipResolver struct {
	zipReader   *zip.Reader
	innerDir    string
	searchRoots []string
}

func (z *ZipResolver) ReadFile(rel string) ([]byte, error) {
	candidates := buildPathCandidates(rel)

	// 1. 先在 zip 内部检索 (忽略大小写与相对层级)
	for _, cand := range candidates {
		candLower := strings.ToLower(cand)
		candBaseLower := strings.ToLower(filepath.Base(cand))
		wantInner := strings.ToLower(filepath.ToSlash(filepath.Join(z.innerDir, cand)))

		for _, f := range z.zipReader.File {
			fLow := strings.ToLower(filepath.ToSlash(f.Name))
			if fLow == wantInner || fLow == candLower || strings.HasSuffix(fLow, "/"+candLower) || filepath.Base(fLow) == candBaseLower {
				rc, err := f.Open()
				if err == nil {
					data, err := io.ReadAll(rc)
					rc.Close()
					if err == nil && len(data) > 0 {
						return data, nil
					}
				}
			}
		}
	}

	// 2. 压缩包内若无，回退至游戏全局磁盘搜索根
	for _, root := range z.searchRoots {
		for _, cand := range candidates {
			p := filepath.Join(root, filepath.FromSlash(cand))
			if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("zip entry not found: %s", rel)
}

func (z *ZipResolver) FindImage(src string) (image.Image, string, error) {
	data, err := z.ReadFile(src)
	if err != nil {
		return nil, "", err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, "", err
}

// RenderStandalone 渲染独立地图并自动在同级目录下写回 _map.png
func (r *MapRenderer) RenderStandalone(tmxPath string, maxDim int) ([]byte, error) {
	tmxData, err := os.ReadFile(tmxPath)
	if err != nil {
		return nil, err
	}
	resolver := &DiskResolver{
		baseDir:     filepath.Dir(tmxPath),
		searchRoots: r.searchRoots,
	}

	base := strings.TrimSuffix(tmxPath, filepath.Ext(tmxPath))
	outPng := base + "_map.png"
	var existingBytes []byte
	if data, err := os.ReadFile(outPng); err == nil {
		existingBytes = data
	} else if data, err := os.ReadFile(base + "_map.PNG"); err == nil {
		existingBytes = data
	}

	pngBytes, err := r.RenderTMXBytesWithFallback(tmxData, filepath.Base(tmxPath), resolver, maxDim, existingBytes)
	if err != nil {
		return nil, err
	}
	_ = os.WriteFile(outPng, pngBytes, 0644)
	return pngBytes, nil
}

// RenderPackEntry 渲染 .rwmod 包内的单个地图并安全热替换写回压缩包内部
func (r *MapRenderer) RenderPackEntry(packPath, entryName string, maxDim int) ([]byte, error) {
	zr, err := zip.OpenReader(packPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	var tmxBytes []byte
	normEntry := filepath.ToSlash(entryName)
	for _, f := range zr.File {
		if filepath.ToSlash(f.Name) == normEntry {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			tmxBytes, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if len(tmxBytes) == 0 {
		return nil, fmt.Errorf("rwmod 中未找到地图条目: %s", entryName)
	}

	base := strings.TrimSuffix(normEntry, filepath.Ext(normEntry))
	wantPngEntry := base + "_map.png"
	wantPngEntryLower := strings.ToLower(wantPngEntry)

	var existingBytes []byte
	for _, f := range zr.File {
		if strings.ToLower(filepath.ToSlash(f.Name)) == wantPngEntryLower {
			if rc, err := f.Open(); err == nil {
				existingBytes, _ = io.ReadAll(rc)
				rc.Close()
			}
			break
		}
	}

	resolver := &ZipResolver{
		zipReader:   &zr.Reader,
		innerDir:    filepath.ToSlash(filepath.Dir(normEntry)),
		searchRoots: r.searchRoots,
	}
	pngBytes, err := r.RenderTMXBytesWithFallback(tmxBytes, filepath.Base(normEntry), resolver, maxDim, existingBytes)
	if err != nil {
		return nil, err
	}

	// 热更新至 .rwmod 包
	zr.Close() // 先释放句柄
	if err := UpdateZipEntry(packPath, wantPngEntry, pngBytes); err != nil {
		return nil, fmt.Errorf("写回 rwmod 压缩包失败: %w", err)
	}

	return pngBytes, nil
}

// BatchRenderStandalone 批量处理独立地图目录 (force=true 时强制覆盖已有预览图)
func (r *MapRenderer) BatchRenderStandalone(mapsDir string, force bool) (int, error) {
	count := 0
	err := filepath.Walk(mapsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".tmx") {
			base := strings.TrimSuffix(path, filepath.Ext(path))
			pngPath := base + "_map.png"
			pngPathUpper := base + "_map.PNG"
			hasPng := false
			if _, err := os.Stat(pngPath); err == nil {
				hasPng = true
			} else if _, err := os.Stat(pngPathUpper); err == nil {
				hasPng = true
			}

			if hasPng && !force {
				return nil
			}

			if _, err := r.RenderStandalone(path, 500); err == nil {
				count++
			}
		}
		return nil
	})
	return count, err
}

// BatchPatchPacks 批量处理 .rwmod 地图包 (force=true 时全量覆盖更新内部已有缩略图)
func (r *MapRenderer) BatchPatchPacks(packPaths []string, force bool) (int, error) {
	total := 0
	for _, packPath := range packPaths {
		zr, err := zip.OpenReader(packPath)
		if err != nil {
			continue
		}

		haveLower := make(map[string]bool)
		var tmxs []string
		for _, f := range zr.File {
			low := strings.ToLower(f.Name)
			haveLower[low] = true
			if strings.HasSuffix(low, ".tmx") {
				tmxs = append(tmxs, f.Name)
			}
		}

		var todo []string
		for _, tmx := range tmxs {
			base := strings.TrimSuffix(tmx, filepath.Ext(tmx))
			wantPng := strings.ToLower(base + "_map.png")
			if force || !haveLower[wantPng] {
				todo = append(todo, tmx)
			}
		}

		if len(todo) == 0 {
			zr.Close()
			continue
		}

		resolver := &ZipResolver{
			zipReader:   &zr.Reader,
			innerDir:    "",
			searchRoots: r.searchRoots,
		}

		newEntries := make(map[string][]byte)
		for _, tmxEntry := range todo {
			var tmxData []byte
			for _, f := range zr.File {
				if f.Name == tmxEntry {
					rc, err := f.Open()
					if err == nil {
						tmxData, _ = io.ReadAll(rc)
						rc.Close()
					}
					break
				}
			}
			if len(tmxData) == 0 {
				continue
			}

			base := strings.TrimSuffix(tmxEntry, filepath.Ext(tmxEntry))
			wantPng := strings.ToLower(base + "_map.png")
			var existingBytes []byte
			for _, f := range zr.File {
				if strings.ToLower(filepath.ToSlash(f.Name)) == wantPng {
					if rc, err := f.Open(); err == nil {
						existingBytes, _ = io.ReadAll(rc)
						rc.Close()
					}
					break
				}
			}

			resolver.innerDir = filepath.ToSlash(filepath.Dir(tmxEntry))
			pngBytes, err := r.RenderTMXBytesWithFallback(tmxData, filepath.Base(tmxEntry), resolver, 500, existingBytes)
			if err == nil {
				newEntries[base+"_map.png"] = pngBytes
			}
		}
		zr.Close()

		if len(newEntries) > 0 {
			if err := UpdateZipEntriesMulti(packPath, newEntries); err == nil {
				total += len(newEntries)
			}
		}
	}
	return total, nil
}

// UpdateZipEntry 安全替换 zip 文件中的单个条目
func UpdateZipEntry(packPath, entryName string, data []byte) error {
	return UpdateZipEntriesMulti(packPath, map[string][]byte{entryName: data})
}

// UpdateZipEntriesMulti 安全替换 zip 文件中的多个条目 (原子替换)
func UpdateZipEntriesMulti(packPath string, entries map[string][]byte) error {
	r, err := zip.OpenReader(packPath)
	if err != nil {
		return err
	}
	defer r.Close()

	tmpFile, err := os.CreateTemp(filepath.Dir(packPath), "patch_*.rwmod")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	w := zip.NewWriter(tmpFile)
	lowAdded := make(map[string]bool)
	for k := range entries {
		lowAdded[strings.ToLower(filepath.ToSlash(k))] = true
	}

	for _, f := range r.File {
		if lowAdded[strings.ToLower(filepath.ToSlash(f.Name))] {
			continue // 跳过旧版本
		}
		header := f.FileHeader
		fw, err := w.CreateHeader(&header)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(fw, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}

	for name, data := range entries {
		header := &zip.FileHeader{
			Name:     filepath.ToSlash(name),
			Method:   zip.Deflate,
			Modified: time.Now(),
		}
		fw, err := w.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := fw.Write(data); err != nil {
			return err
		}
	}

	if err := w.Close(); err != nil {
		return err
	}
	tmpFile.Close()
	r.Close()

	return os.Rename(tmpPath, packPath)
}
