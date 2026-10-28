#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
rw_thumbnail.py - 生成《铁锈战争》(Rusted Warfare) 自定义地图缩略图

原理(经 game-lib.jar 反编译确认):
  com.corrodinggames.rts.appFramework.c.c(String):
      return path.replace(".tmx", "") + "_map.png";
  com.corrodinggames.rts.java.d.b:
      "Map thumbnail is too large. Size:(W,H) (max:500 pixels)"
  com.corrodinggames.rts.gameFramework.j:
      支持 <property name="embedded_png"> 内嵌图集(base64)

用法:
  python rw_thumbnail.py render <map.tmx> [-o out.png] [--max 500]
  python rw_thumbnail.py batch  <目录> [--max 500] [--force] [--dry-run]
  python rw_thumbnail.py patch  <pack.rwmod> [...] [--max 500]
"""

import argparse
import base64
import io
import os
import re
import shutil
import sys
import tempfile
import zipfile
import zlib
import xml.etree.ElementTree as ET

import numpy as np
from PIL import Image

# 游戏根目录(本脚本位于 <root>/tools/ 下)
# 游戏根目录(优先从环境变量或自动判定有效铁锈目录读取)
candidate_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
if os.environ.get("RW_GAME_ROOT") and os.path.exists(os.environ.get("RW_GAME_ROOT")):
    GAME_ROOT = os.environ.get("RW_GAME_ROOT")
elif os.path.exists(os.path.join(candidate_root, "game-lib.jar")):
    GAME_ROOT = candidate_root
else:
    GAME_ROOT = r"E:\Rusted Warfare\Rusted Warfare"

FLIP_H = 0x80000000
FLIP_V = 0x40000000
FLIP_D = 0x20000000

# 队伍颜色: (显示色, 地图上指挥中心精灵的颜色)
# 与 preferences.ini 的 teamColors 一致(来自官方 thumbnail_tool/draw_spawn_points.py)
TEAM_COLORS = {
    1: ((0, 255, 0), (0, 255, 0)),
    2: ((255, 0, 0), (255, 0, 0)),
    3: ((0, 0, 255), (50, 100, 150)),
    4: ((255, 255, 0), (200, 200, 0)),
    5: ((0, 255, 255), (230, 170, 100)),
    6: ((255, 255, 255), (93, 218, 218)),
    7: ((0, 0, 0), (184, 106, 205)),
    8: ((255, 0, 255), (106, 108, 205)),
    9: ((255, 127, 24), (205, 132, 30)),
    10: ((147, 104, 196), (120, 0, 176)),
}

_FONT_CANDIDATES = [
    os.path.join(GAME_ROOT, "font", "NotoSans-Bold.ttf"),
    os.path.join(GAME_ROOT, "font", "Roboto-Bold.ttf"),
    r"C:\Windows\Fonts\arialbd.ttf",
    r"C:\Windows\Fonts\segoeuib.ttf",
]

# 渲染优先级:数字越小越先画(在下面)
LAYER_ORDER = {"ground": 0, "ground2": 1, "items": 2, "units": 3, "objects": 4}

# 游戏根目录(本脚本位于 <root>/tools/ 下)


def _search_roots():
    """外部图集的候选搜索根,按由近及远排列。"""
    roots = []
    for base in (GAME_ROOT, os.path.join(GAME_ROOT, "assets"), os.path.join(GAME_ROOT, "mods")):
        for sub in ("", "tilesets", "tilesets/bitmaps", "tilesets/ridges", "tilesets/terrain",
                    "maps", "maps/bitmaps", "maps/terrain", "maps/terrain/bitmaps"):
            p = os.path.normpath(os.path.join(base, sub)) if sub else base
            if os.path.isdir(p) and p not in roots:
                roots.append(p)
    return roots


SEARCH_ROOTS = _search_roots()


def _apply_transparency(img, trans):
    """Tiled 的 trans 属性指定一个应视为透明的颜色键。"""
    # 调色板 PNG:先按内建 transparency 转 RGBA
    if img.mode == "P":
        img = img.convert("RGBA") if "transparency" not in img.info else Image.open(
            img.filename or img
        ).convert("RGBA")
    if not trans:
        return img
    try:
        rgb = tuple(int(trans[i:i + 2], 16) for i in (0, 2, 4))
    except Exception:
        return img
    if img.mode != "RGBA":
        img = img.convert("RGBA")
    arr = np.array(img)
    mask = np.all(arr[:, :, :3] == np.array(rgb, dtype=np.uint8), axis=-1)
    arr[mask, 3] = 0
    return Image.fromarray(arr, "RGBA")


def _find_file(rel, tmx_dir, zip_reader=None):
    """按多种规则解析外部文件(.tsx / .png)路径,返回 bytes 或 None。"""
    rel = rel.replace("\\", "/")
    bases = [tmx_dir]
    d = tmx_dir
    for _ in range(3):
        d = os.path.dirname(d.rstrip("/"))
        if not d:
            break
        bases.append(d)
    bases.extend(SEARCH_ROOTS)
    cands = []
    for b in bases:
        cands.append(os.path.normpath(os.path.join(b, rel)))
        if "/" in rel:
            cands.append(os.path.normpath(os.path.join(b, rel.split("/", 1)[1])))
    cands.append(os.path.normpath(os.path.join(tmx_dir, os.path.basename(rel))))

    if zip_reader is not None:
        names = zip_reader.namelist()
        for c in cands:
            rr = c.replace("\\", "/").lstrip("./")
            for n in names:
                if n == rr or n.endswith("/" + rr):
                    return zip_reader.read(n), os.path.dirname(n).replace("\\", "/")
        # zip 内没有 -> 回退到游戏安装目录(模组常引用内置 assets/tilesets)
        for c in cands:
            if os.path.isfile(c):
                with open(c, "rb") as fh:
                    return fh.read(), os.path.dirname(c).replace("\\", "/")
        return None

    for c in cands:
        if os.path.isfile(c):
            with open(c, "rb") as fh:
                return fh.read(), os.path.dirname(c).replace("\\", "/")
    return None


def _find_external_image(src, tmx_dir, zip_reader=None):
    """按多种规则解析外部图集路径。"""
    src = src.replace("\\", "/")
    candidates = []
    bases = [tmx_dir]
    d = tmx_dir
    for _ in range(3):
        d = os.path.dirname(d.rstrip("/"))
        if not d or d == bases[0]:
            break
        bases.append(d)
    bases.extend(SEARCH_ROOTS)
    for b in bases:
        candidates.append(os.path.normpath(os.path.join(b, src)))
        if "/" in src:
            candidates.append(os.path.normpath(os.path.join(b, src.split("/", 1)[1])))
    candidates.append(os.path.normpath(os.path.join(tmx_dir, os.path.basename(src))))

    if zip_reader is not None:
        names = zip_reader.namelist()
        for c in candidates:
            rel = c.replace("\\", "/").lstrip("./")
            for n in names:
                if n == rel or n.endswith("/" + rel):
                    return Image.open(io.BytesIO(zip_reader.read(n))).convert("RGBA")
        base = os.path.basename(src)
        for n in names:
            if os.path.basename(n).lower() == base.lower():
                return Image.open(io.BytesIO(zip_reader.read(n))).convert("RGBA")
        # zip 内没有 -> 回退到游戏安装目录
        for c in candidates:
            if os.path.isfile(c):
                return Image.open(c).convert("RGBA")
        return None

    for c in candidates:
        if os.path.isfile(c):
            return Image.open(c).convert("RGBA")
    return None


def _prop_text(el):
    """property 可能是 value 属性,也可能是元素正文(RW 导出用后者装 base64)。"""
    v = el.get("value")
    if v is not None:
        return v
    return el.text or ""


def _decode_layer(data_el, w, h):
    txt = data_el.text or ""
    raw = base64.b64decode(re.sub(r"\s+", "", txt))
    comp = data_el.get("compression")
    if comp == "zlib":
        raw = zlib.decompress(raw)
    elif comp == "gzip":
        import gzip

        raw = gzip.decompress(raw)
    elif comp:
        raise ValueError("unsupported layer compression: %r" % comp)
    n = w * h
    if len(raw) < n * 4:
        raise ValueError("layer data too short: %d < %d" % (len(raw), n * 4))
    return np.frombuffer(raw[: n * 4], dtype="<u4").reshape(h, w)


class Tileset(object):
    __slots__ = ("firstgid", "name", "tw", "th", "cols", "tiles", "tile_team", "tile_cc")

    def __init__(self, firstgid, name, tw, th, cols, tiles, tile_team=None, tile_cc=None):
        self.firstgid = firstgid
        self.name = name
        self.tw = tw
        self.th = th
        self.cols = cols
        self.tiles = tiles
        n = len(tiles)
        self.tile_team = tile_team if tile_team is not None else [-1] * n
        self.tile_cc = tile_cc if tile_cc is not None else [False] * n


def _open_image_from_spec(ts, tmx_dir, zip_reader=None):
    """从 embedded_png 或外部 image source 载入 tileset 位图。"""
    props = {}
    for pr in ts.findall("./properties/property"):
        props[pr.get("name")] = _prop_text(pr)

    if "embedded_png" in props:
        raw = re.sub(r"\s+", "", props["embedded_png"])
        try:
            img = Image.open(io.BytesIO(base64.b64decode(raw))).convert("RGBA")
            ie0 = ts.find("image")
            trans = ie0.get("trans") if ie0 is not None else None
            return _apply_transparency(img, trans)
        except Exception as exc:
            print("    ! embedded_png 解码失败(%s): %s" % (ts.get("name"), exc))

    ie = ts.find("image")
    if ie is None or not ie.get("source"):
        return None
    img = _find_external_image(ie.get("source"), tmx_dir, zip_reader)
    if img is None:
        print("    ! 缺少外部图集: %s" % ie.get("source"))
        return None
    return _apply_transparency(img, ie.get("trans"))


def _load_tilesets(root, tmx_dir, zip_reader=None):
    map_tw = int(root.get("tilewidth") or 32)
    map_th = int(root.get("tileheight") or 32)
    out = []
    for ts in root.findall("tileset"):
        firstgid = int(ts.get("firstgid") or 1)
        src = ts.get("source")
        img_base = tmx_dir
        if src:
            # <tileset firstgid="N" source="terrain/Long Grass.tsx" /> 外部引用
            got = _find_file(src, tmx_dir, zip_reader)
            if got is None:
                print("    ! 缺少图集定义文件: %s" % src)
                continue
            tsx, img_base = got
            try:
                ts = ET.fromstring(tsx)
            except Exception as exc:
                print("    ! 图集定义解析失败(%s): %s" % (src, exc))
                continue
            ts.set("firstgid", str(firstgid))
            if not ts.get("name"):
                ts.set("name", src)
        img = _open_image_from_spec(ts, img_base, zip_reader)
        if img is None:
            continue
        tw = int(ts.get("tilewidth") or map_tw)
        th = int(ts.get("tileheight") or map_th)
        if tw <= 0 or th <= 0:
            continue
        cols = ts.get("columns")
        cols = int(cols) if cols else max(1, img.width // tw)
        rows = max(1, img.height // th)
        tiles = []
        for local in range(cols * rows):
            x = (local % cols) * tw
            y = (local // cols) * th
            if x + tw > img.width or y + th > img.height:
                tiles.append(None)
                continue
            tiles.append(img.crop((x, y, x + tw, y + th)))
        # 读取 <tile> 上的 team / unit 属性(用于定位各队指挥中心)
        tile_team = [-1] * len(tiles)
        tile_cc = [False] * len(tiles)
        for tile in ts.findall("tile"):
            try:
                tid = int(tile.get("id"))
            except (TypeError, ValueError):
                continue
            if tid < 0 or tid >= len(tiles):
                continue
            for pr in tile.findall("./properties/property"):
                pn = pr.get("name")
                pv = pr.get("value") or ""
                if pn == "team":
                    try:
                        tile_team[tid] = int(pv)
                    except ValueError:
                        pass
                elif pn == "unit" and pv.lower() == "commandcenter":
                    tile_cc[tid] = True
        out.append(
            Tileset(int(ts.get("firstgid") or 1), ts.get("name") or "", tw, th, cols, tiles,
                    tile_team, tile_cc)
        )
    out.sort(key=lambda t: t.firstgid)
    return out


def _resolve_gid(tilesets, gid):
    """gid -> (tileset, local_id);用 Tiled 语义:取 firstgid 最大且 <= gid 的图集。"""
    if gid == 0:
        return None
    flags = gid & (FLIP_H | FLIP_V | FLIP_D)
    base = gid & ~(FLIP_H | FLIP_V | FLIP_D)
    cand = None
    for ts in tilesets:
        if ts.firstgid <= base:
            cand = ts
        else:
            break
    if cand is None:
        return None
    local = base - cand.firstgid
    if local < 0 or local >= len(cand.tiles) or cand.tiles[local] is None:
        return None
    return cand, local, flags


def _apply_flip(img, flags):
    if flags & FLIP_H:
        img = img.transpose(Image.FLIP_LEFT_RIGHT)
    if flags & FLIP_V:
        img = img.transpose(Image.FLIP_TOP_BOTTOM)
    if flags & FLIP_D:
        img = img.transpose(Image.Transpose.ROTATE_90)
    return img


def render_tmx_bytes(tmx_bytes, tmx_name="<memory>", zip_reader=None, max_dim=500,
                     scale_cap=8000, verbose=True, exclude_layers=(), detect_units=False,
                     player_count=0):
    """
    渲染 TMX 字节流为缩略图 PIL 图像。
    exclude_layers: 需要跳过的图层名(小写)
    detect_units:   True 时在缩放前检测各队出生点,并换算到最终图像坐标
    返回 (image, positions)
    """
    root = ET.fromstring(tmx_bytes)
    w = int(root.get("width") or 0)
    h = int(root.get("height") or 0)
    if w <= 0 or h <= 0:
        raise ValueError("bad map size %sx%s" % (w, h))
    map_tw = int(root.get("tilewidth") or 32)
    map_th = int(root.get("tileheight") or 32)

    tmx_dir = os.path.dirname(tmx_name).replace("\\", "/")
    tilesets = _load_tilesets(root, tmx_dir, zip_reader)

    props = {}
    for pr in root.findall("./properties/property"):
        props[pr.get("name")] = _prop_text(pr)
    try:
        fill_gid = int(props.get("fill", "0"))
    except ValueError:
        fill_gid = 0

    skip = set(x.lower() for x in exclude_layers)
    # 收集图层,按 RW 的视觉顺序排
    layers = []
    for lay in root.findall("layer"):
        name = (lay.get("name") or "").strip()
        low = name.lower()
        if low == "set":
            continue  # automapping 源图层,游戏本身忽略
        if low in skip:
            continue
        prio = LAYER_ORDER.get(low, 5)
        layers.append((prio, len(layers), name, lay))
    layers.sort(key=lambda x: (x[0], x[1]))

    # 中间画布尺寸上限,避免超大地图吃内存
    full_w, full_h = w * map_tw, h * map_th
    scale = 1.0
    if max(full_w, full_h) > scale_cap:
        scale = scale_cap / float(max(full_w, full_h))

    if scale != 1.0:
        tw = max(1, int(round(map_tw * scale)))
        th = max(1, int(round(map_th * scale)))
    else:
        tw, th = map_tw, map_th

    canvas = Image.new("RGBA", (w * tw, h * th), (0, 0, 0, 0))

    flip_cache = {}

    def tile_image(ts, local, flags):
        if not flags:
            return ts.tiles[local]
        key = (id(ts), local, flags)
        img = flip_cache.get(key)
        if img is None:
            img = _apply_flip(ts.tiles[local], flags)
            if scale != 1.0:
                img = img.resize((tw, th), Image.LANCZOS)
            flip_cache[key] = img
        return img

    # 预先把所有用到的 tileset 预缩放(仅在 scale != 1 时)
    if scale != 1.0:
        for ts in tilesets:
            ts.tiles = [
                t.resize((tw, th), Image.LANCZOS) if t is not None else None for t in ts.tiles
            ]

    cc_cells = {}
    for prio, _seq, name, lay in layers:
        data_el = lay.find("data")
        if data_el is None:
            continue
        try:
            gids = _decode_layer(data_el, w, h)
        except Exception as exc:
            print("    ! 图层 %r 解码失败: %s" % (name, exc))
            continue
        rows = gids.tolist()
        for y in range(h):
            row = rows[y]
            for x in range(w):
                gid = row[x]
                if gid == 0:
                    if prio <= LAYER_ORDER["ground2"] and fill_gid:
                        gid = fill_gid
                    else:
                        continue
                res = _resolve_gid(tilesets, gid)
                if res is None:
                    continue
                ts, local, flags = res
                if detect_units and ts.tile_cc[local] and ts.tile_team[local] > 0:
                    cc_cells.setdefault(ts.tile_team[local], []).append((x, y))
                img = ts.tiles[local]
                if flags:
                    img = tile_image(ts, local, flags)
                if ts.tw != tw or ts.th != th:
                    img = img.resize((tw, th), Image.LANCZOS)
                canvas.paste(img, (x * tw, y * th), img)

    # 出生点检测必须在缩放前做(缩放会破坏精确颜色)
    positions = {}
    if detect_units and player_count > 0:
        positions = _detect_team_positions(canvas, player_count, cc_cells, tw, th)

    # 缩放到 max_dim 以内
    cw, ch = canvas.size
    if max(cw, ch) > max_dim:
        r = max_dim / float(max(cw, ch))
        canvas = canvas.resize((max(1, int(round(cw * r))), max(1, int(round(ch * r)))),
                               Image.LANCZOS)
        # 坐标换算到最终图像
        positions = dict((tid, (px * r, py * r)) for tid, (px, py) in positions.items())

    # 展平到不透明背景(RW 用 PNGImageData 加载,RGB 最稳)
    bg = Image.new("RGB", canvas.size, (0, 0, 0))
    bg.paste(canvas, (0, 0), canvas)
    return bg, positions


def _read_tmx_bytes(path):
    with open(path, "rb") as fh:
        return fh.read()


def _detect_team_positions(unit_img, player_count, cc_cells=None, tw=1, th=1):
    """
    在"只有 Units 图层"的渲染图上,按队伍颜色找出各队指挥中心位置。
    算法复刻官方 thumbnail_tool/draw_spawn_points.py(用 numpy 向量化)。
    cc_cells: 由图集属性(unit=commandCenter / team=N)定位到的候选格,
              用于把颜色搜索限制在指挥中心附近,避免误匹配。
    返回 {team_id: (x, y)},坐标为 unit_img 的像素坐标。
    """
    arr = np.array(unit_img)
    if arr.ndim != 3 or arr.shape[2] != 4:
        return {}
    mask = arr[:, :, 3] != 0
    if not mask.any():
        return {}
    ys, xs = np.nonzero(mask)
    y0, y1, x0, x1 = ys.min(), ys.max() + 1, xs.min(), xs.max() + 1
    sub = arr[y0:y1, x0:x1]
    submask = sub[:, :, 3] != 0
    rgb = sub[:, :, :3].astype(np.int16)
    out = {}
    for tid in range(1, min(player_count, 10) + 1):
        _disp, target = TEAM_COLORS[tid]
        # 限制搜索区域到该队指挥中心附近
        region = None
        if cc_cells and tid in cc_cells:
            H, W = sub.shape[:2]
            region = np.zeros((H, W), dtype=bool)
            for (cx, cy) in cc_cells[tid]:
                rx0 = max(0, int(cx * tw - tw) - x0)
                rx1 = min(W, int(cx * tw + tw * 2) - x0)
                ry0 = max(0, int(cy * th - th) - y0)
                ry1 = min(H, int(cy * th + th * 2) - y0)
                if rx1 > rx0 and ry1 > ry0:
                    region[ry0:ry1, rx0:rx1] = True
            if not region.any():
                region = None
        diff = np.abs(rgb - np.array(target, dtype=np.int16)).sum(axis=2)
        diff = np.where(submask, diff, 1 << 30)
        if region is not None:
            diff = np.where(region, diff, 1 << 30)
        best = int(diff.min())
        if best >= 500:  # 与官方脚本一致的接受阈值
            continue
        bys, bxs = np.nonzero(diff == best)
        out[tid] = (float(bxs.mean()) + x0, float(bys.mean()) + y0)
    return out


def _load_font(size):
    from PIL import ImageFont

    for p in _FONT_CANDIDATES:
        if os.path.isfile(p):
            try:
                return ImageFont.truetype(p, size)
            except Exception:
                continue
    return ImageFont.load_default()


def _draw_team_markers(map_img, positions, unit_size):
    """在缩略图上画出生点编号(黑色半透明圆 + 队伍色数字)。"""
    if not positions:
        return map_img
    from PIL import ImageDraw

    W, H = map_img.size
    sx = W / float(unit_size[0])
    sy = H / float(unit_size[1])
    layer = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    draw = ImageDraw.Draw(layer)
    font = _load_font(18)
    eSize = 11
    for tid, (px, py) in positions.items():
        disp = TEAM_COLORS.get(tid, ((255, 255, 255), (255, 255, 255)))[0]
        tx, ty = px * sx, py * sy
        draw.ellipse((tx - eSize, ty - eSize, tx + eSize, ty + eSize), fill=(0, 0, 0, 100))
        txt = str(tid)
        try:
            l, t, r, b = draw.textbbox((0, 0), txt, font=font)
            tw, th = r - l, b - t
        except Exception:
            tw = th = 10
        draw.text((tx - tw / 2.0, ty - th / 2.0 - 3), txt, fill=disp + (255,), font=font)
    base = map_img.convert("RGBA")
    base.paste(layer, (0, 0), layer)
    return base.convert("RGB")


def _player_count_from_name(path):
    """支持官方 [pN] 规范以及社区常见的 (10p)、（12P混战）、(p4)、10人 等命名。"""
    name = os.path.basename(path)
    # 1. 官方规范 [p2] 或 [2p]
    m = re.search(r'\[(?:.*;)?p?([0-9]+)p?\]', name, re.IGNORECASE)
    if m:
        try:
            val = int(m.group(1))
            if 1 <= val <= 24: return val
        except: pass
    # 2. 括号内的 (10p), （12P混战）, (p4)
    m = re.search(r'[\(（](?:p)?([0-9]+)\s*p?[^\)）]*[\)）]', name, re.IGNORECASE)
    if m:
        try:
            val = int(m.group(1))
            if 1 <= val <= 24: return val
        except: pass
    # 3. 匹配人数字符，如 10人
    m = re.search(r'([0-9]+)\s*人', name)
    if m:
        try:
            val = int(m.group(1))
            if 1 <= val <= 24: return val
        except: pass
    # 4. 匹配结尾或带下划线的数字+p，如 23p混战, _2p_
    m = re.search(r'(?:[_\s\-])([0-9]{1,2})\s*p\b', name, re.IGNORECASE)
    if m:
        try:
            val = int(m.group(1))
            if 1 <= val <= 24: return val
        except: pass
    m = re.search(r'([0-9]{1,2})\s*p(?:混战|图)?', name, re.IGNORECASE)
    if m:
        try:
            val = int(m.group(1))
            if 1 <= val <= 24: return val
        except: pass
    # 默认探查前10支队伍的指挥中心
    return 10


def render_map(tmx_bytes, tmx_name, zip_reader=None, max_dim=500):
    """完整渲染:地图画面 + 出生点编号。返回 PIL 图像。"""
    player_count = _player_count_from_name(tmx_name)
    map_img, _ = render_tmx_bytes(tmx_bytes, tmx_name, zip_reader, max_dim,
                                  exclude_layers=())
    if player_count > 0:
        _u, positions = render_tmx_bytes(
            tmx_bytes, tmx_name, zip_reader, max_dim,
            exclude_layers=("ground", "ground2", "items", "set", "objects"),
            detect_units=True, player_count=player_count)
        if positions:
            map_img = _draw_team_markers(map_img, positions, map_img.size)
    return map_img


def cmd_render(args):
    data = _read_tmx_bytes(args.tmx)
    img = render_map(data, os.path.abspath(args.tmx), None, args.max)
    out = args.out
    if not out:
        base = args.tmx[:-4] if args.tmx.lower().endswith(".tmx") else args.tmx
        out = base + "_map.png"
    img.save(out)
    print("  -> %s  %dx%d  %d bytes" % (out, img.width, img.height, os.path.getsize(out)))


def cmd_batch(args):
    root = args.dir
    if os.path.isfile(root):
        files = [root]
        root = os.path.dirname(root) or "."
    else:
        files = []
        for dirpath, _dirs, names in os.walk(root):
            # rules/ 目录里是地形自动衔接规则集(由 rules.txt 声明),不是可玩地图
            if os.path.basename(dirpath).lower() == "rules":
                continue
            for n in names:
                if n.lower().endswith(".tmx"):
                    files.append(os.path.join(dirpath, n))
    files.sort()
    made = skipped = failed = 0
    for f in files:
        base = f[:-4]
        out = base + "_map.png"
        if os.path.exists(out) and not args.force:
            print("  = 已有缩略图,跳过: %s" % os.path.basename(out))
            skipped += 1
            continue
        if args.dry_run:
            print("  * 将生成: %s" % out)
            made += 1
            continue
        try:
            img = render_map(_read_tmx_bytes(f), os.path.abspath(f), None, args.max)
            img.save(out)
            print("  + %s  %dx%d" % (os.path.basename(out), img.width, img.height))
            made += 1
        except Exception as exc:
            print("  ! 失败 %s: %s" % (os.path.basename(f), exc))
            failed += 1
    print("\n完成: 新建 %d, 跳过 %d, 失败 %d" % (made, skipped, failed))
    return 1 if failed else 0


def cmd_patch(args):
    """把缺失的 _map.png 补进 .rwmod(zip)包内部。"""
    for pack in args.packs:
        print("== %s" % os.path.basename(pack))
        with zipfile.ZipFile(pack) as z:
            names = z.namelist()
            tmxs = [n for n in names if n.lower().endswith(".tmx")]
            have = set(n.lower() for n in names)
            todo = []
            for n in tmxs:
                want = n[:-4] + "_map.png"
                if want.lower() not in have:
                    todo.append((n, want))
            if not todo:
                print("  所有地图都已有缩略图")
                continue
            if args.dry_run:
                for n, want in todo:
                    print("  * 将注入: %s" % want)
                continue
            add = {}
            with zipfile.ZipFile(pack) as z:
                for n, want in todo:
                    try:
                        img = render_map(z.read(n), n, z, args.max)
                        buf = io.BytesIO()
                        img.save(buf, format="PNG")
                        add[want] = buf.getvalue()
                        print("  + %s  %dx%d  %d bytes"
                              % (want, img.width, img.height, len(add[want])))
                    except Exception as exc:
                        print("  ! 渲染失败 %s: %s" % (n, exc))
            if not add:
                continue
            tmp_fd, tmp_path = tempfile.mkstemp(suffix=".rwmod",
                                               dir=os.path.dirname(os.path.abspath(pack)))
            os.close(tmp_fd)
            try:
                with zipfile.ZipFile(pack) as zin, \
                        zipfile.ZipFile(tmp_path, "w", zipfile.ZIP_DEFLATED) as zout:
                    for item in zin.infolist():
                        data = zin.read(item.filename)
                        zi = zipfile.ZipInfo(item.filename, date_time=item.date_time)
                        zi.compress_type = item.compress_type
                        zi.external_attr = item.external_attr
                        zout.writestr(zi, data)
                    for name, data in add.items():
                        zout.writestr(name, data)
                shutil.move(tmp_path, pack)
                print("  已写回 %s(+%d 个缩略图)" % (os.path.basename(pack), len(add)))
            finally:
                if os.path.exists(tmp_path):
                    os.remove(tmp_path)
    return 0


def cmd_fixnames(args):
    """
    修复缩略图文件名与地图名不严格匹配的情况。
    游戏的 zip 查找是 ZipFile.getEntry() + String.equals(),严格区分大小写,
    所以 <地图>_map.PNG / 前缀写错的 缩略图在游戏里是找不到的。
    """
    for pack in args.packs:
        print("== %s" % os.path.basename(pack))
        with zipfile.ZipFile(pack) as z:
            names = set(z.namelist())
            lowmap = {}
            for n in names:
                lowmap.setdefault(n.lower(), n)
            copy_jobs = []   # (源条目, 目标条目)
            render_jobs = [] # (tmx条目, 目标条目)
            for n in sorted(names):
                if not n.lower().endswith(".tmx"):
                    continue
                want = n[:-4] + "_map.png"
                if want in names:
                    continue
                alt = lowmap.get(want.lower())
                if alt:
                    copy_jobs.append((alt, want))
                else:
                    render_jobs.append((n, want))
        if not copy_jobs and not render_jobs:
            print("  所有地图的缩略图文件名均匹配")
            continue
        if args.dry_run:
            for s, t in copy_jobs:
                print("  * 将复制 %r -> %r" % (s, t))
            for s, t in render_jobs:
                print("  * 将渲染 %r -> %r" % (s, t))
            continue

        add = {}
        with zipfile.ZipFile(pack) as z:
            for src, want in copy_jobs:
                add[want] = z.read(src)
                print("  + 复制 %s" % want)
            for src, want in render_jobs:
                try:
                    img = render_map(z.read(src), src, z, args.max)
                    buf = io.BytesIO()
                    img.save(buf, format="PNG")
                    add[want] = buf.getvalue()
                    print("  + 渲染 %s  %dx%d" % (want, img.width, img.height))
                except Exception as exc:
                    print("  ! 渲染失败 %s: %s" % (src, exc))
        if not add:
            continue
        tmp_fd, tmp_path = tempfile.mkstemp(suffix=".rwmod",
                                           dir=os.path.dirname(os.path.abspath(pack)))
        os.close(tmp_fd)
        try:
            with zipfile.ZipFile(pack) as zin, \
                    zipfile.ZipFile(tmp_path, "w", zipfile.ZIP_DEFLATED) as zout:
                for item in zin.infolist():
                    data = zin.read(item.filename)
                    zi = zipfile.ZipInfo(item.filename, date_time=item.date_time)
                    zi.compress_type = item.compress_type
                    zi.external_attr = item.external_attr
                    zout.writestr(zi, data)
                for name, data in add.items():
                    zout.writestr(name, data)
            shutil.move(tmp_path, pack)
            print("  已写回 (+%d)" % len(add))
        finally:
            if os.path.exists(tmp_path):
                os.remove(tmp_path)
    return 0


def main():
    ap = argparse.ArgumentParser(description="Rusted Warfare 地图缩略图生成器")
    sub = ap.add_subparsers(dest="cmd", required=True)

    r = sub.add_parser("render", help="渲染单个 .tmx")
    r.add_argument("tmx")
    r.add_argument("-o", "--out", default=None)
    r.add_argument("--max", type=int, default=500)
    r.set_defaults(func=cmd_render)

    b = sub.add_parser("batch", help="批量处理目录")
    b.add_argument("dir")
    b.add_argument("--max", type=int, default=500)
    b.add_argument("--force", action="store_true")
    b.add_argument("--dry-run", action="store_true")
    b.set_defaults(func=cmd_batch)

    p = sub.add_parser("patch", help="把缩略图注入 .rwmod 包")
    p.add_argument("packs", nargs="+")
    p.add_argument("--max", type=int, default=500)
    p.add_argument("--dry-run", action="store_true")
    p.set_defaults(func=cmd_patch)

    fx = sub.add_parser("fixnames", help="修复 rwmod 内缩略图文件名大小写/前缀不匹配")
    fx.add_argument("packs", nargs="+")
    fx.add_argument("--max", type=int, default=500)
    fx.add_argument("--dry-run", action="store_true")
    fx.set_defaults(func=cmd_fixnames)

    args = ap.parse_args()
    return args.func(args) or 0


if __name__ == "__main__":
    sys.exit(main())
