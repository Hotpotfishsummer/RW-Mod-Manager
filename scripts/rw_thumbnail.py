#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
rw_thumbnail.py - 生成《铁锈战争》(Rusted Warfare) 自定义地图缩略图

核心特性:
  1. 出生点精准解析:
     - 支持 <objectgroup> 对象层 (commandCenter / factionSelector / spawnPoint / t:N 命名)
     - 支持 Units 等图块层 (准确支持 0-based 队伍索引映射为 1-based 玩家标号)
     - 优雅容错: 即使图集图片缺失，仍能无损解析出生点属性并绘制标记
  2. 出生点高对比度标记绘制 (黑底半透明圆 + 队伍高光描边 + 队伍色数字，支持多达 24 支队伍)
  3. 支持对 .rwmod 压缩包内单张地图进行精准重绘与热替换更新
  4. batch 与 patch 命令支持 --force 全量强制重绘，覆盖未带标记的历史缩略图
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
from PIL import Image, ImageDraw, ImageFont

# 游戏根目录判定
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

# 队伍颜色表: (前景色/显示色, 特征像素参考色)
# 队伍 1-10 严格匹配官方 preferences.ini teamColors
# 队伍 11-24 提供高辨识度互补配色
TEAM_COLORS = {
    1:  ((0, 255, 0),     (0, 255, 0)),        # 绿色
    2:  ((208, 32, 19),   (255, 0, 0)),        # 红色
    3:  ((4, 99, 243),    (50, 100, 150)),     # 蓝色
    4:  ((255, 255, 64),  (200, 200, 0)),      # 黄色
    5:  ((0, 255, 255),   (230, 170, 100)),    # 青色
    6:  ((208, 248, 247), (93, 218, 218)),     # 浅白/浅青
    7:  ((180, 180, 180), (184, 106, 205)),    # 灰色
    8:  ((255, 0, 234),   (106, 108, 205)),    # 粉色
    9:  ((255, 127, 24),  (205, 132, 30)),     # 橙色
    10: ((147, 104, 196), (120, 0, 176)),      # 紫色
    11: ((173, 255, 47),  (173, 255, 47)),     # 黄绿
    12: ((0, 180, 180),   (0, 180, 180)),      # 深青
    13: ((138, 43, 226),  (138, 43, 226)),     # 蓝紫
    14: ((255, 20, 147),  (255, 20, 147)),     # 深粉
    15: ((205, 133, 63),  (205, 133, 63)),     # 赭石
    16: ((255, 215, 0),   (255, 215, 0)),      # 金黄
    17: ((30, 144, 255),  (30, 144, 255)),     # 宝石蓝
    18: ((238, 130, 238), (238, 130, 238)),    # 罗兰紫
    19: ((50, 205, 50),   (50, 205, 50)),      # 酸橙绿
    20: ((220, 20, 60),   (220, 20, 60)),      # 猩红
    21: ((255, 127, 80),  (255, 127, 80)),     # 珊瑚红
    22: ((106, 90, 205),  (106, 90, 205)),     # 瓦蓝
    23: ((255, 140, 0),   (255, 140, 0)),      # 深橙
    24: ((224, 224, 224), (224, 224, 224)),    # 银白
}

_FONT_CANDIDATES = [
    os.path.join(GAME_ROOT, "font", "NotoSans-Bold.ttf"),
    os.path.join(GAME_ROOT, "font", "Roboto-Bold.ttf"),
    r"C:\Windows\Fonts\arialbd.ttf",
    r"C:\Windows\Fonts\segoeuib.ttf",
    r"C:\Windows\Fonts\msyhbd.ttc",
    r"C:\Windows\Fonts\simhei.ttf",
]

LAYER_ORDER = {"ground": 0, "ground2": 1, "items": 2, "units": 3, "objects": 4}


def _search_roots():
    """外部图集的候选搜索根路径。"""
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
    """Tiled 的 trans 属性指定一个视为透明的颜色键。"""
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
    """按多种规则解析外部文件(.tsx / .png)路径,返回 (bytes, base_dir) 或 None。"""
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
    """按多种规则解析外部图集位图。"""
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
                    try:
                        return Image.open(io.BytesIO(zip_reader.read(n))).convert("RGBA")
                    except Exception:
                        pass
        base = os.path.basename(src)
        for n in names:
            if os.path.basename(n).lower() == base.lower():
                try:
                    return Image.open(io.BytesIO(zip_reader.read(n))).convert("RGBA")
                except Exception:
                    pass
        for c in candidates:
            if os.path.isfile(c):
                try:
                    return Image.open(c).convert("RGBA")
                except Exception:
                    pass
        return None

    for c in candidates:
        if os.path.isfile(c):
            try:
                return Image.open(c).convert("RGBA")
            except Exception:
                pass
    return None


def _prop_text(el):
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
        except Exception:
            pass

    ie = ts.find("image")
    if ie is None or not ie.get("source"):
        return None
    img = _find_external_image(ie.get("source"), tmx_dir, zip_reader)
    if img is None:
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
            got = _find_file(src, tmx_dir, zip_reader)
            if got is not None:
                tsx, img_base = got
                try:
                    ts = ET.fromstring(tsx)
                    ts.set("firstgid", str(firstgid))
                    if not ts.get("name"):
                        ts.set("name", src)
                except Exception:
                    pass

        img = _open_image_from_spec(ts, img_base, zip_reader)
        tw = int(ts.get("tilewidth") or map_tw)
        th = int(ts.get("tileheight") or map_th)
        tilecount = int(ts.get("tilecount") or 120)
        cols = ts.get("columns")

        if img is not None:
            cols = int(cols) if cols else max(1, img.width // tw)
            rows = max(1, img.height // th)
            tiles = []
            for local in range(cols * rows):
                x = (local % cols) * tw
                y = (local // cols) * th
                if x + tw > img.width or y + th > img.height:
                    tiles.append(None)
                else:
                    tiles.append(img.crop((x, y, x + tw, y + th)))
        else:
            cols = int(cols) if cols else 10
            tiles = [None] * max(tilecount, 200)

        tile_team = [-1] * len(tiles)
        tile_cc = [False] * len(tiles)
        for tile in ts.findall("tile"):
            try:
                tid = int(tile.get("id"))
            except (TypeError, ValueError):
                continue
            if tid < 0:
                continue
            if tid >= len(tiles):
                # 动态扩容
                diff = tid - len(tiles) + 1
                tiles.extend([None] * diff)
                tile_team.extend([-1] * diff)
                tile_cc.extend([False] * diff)
            for pr in tile.findall("./properties/property"):
                pn = pr.get("name")
                pv = _prop_text(pr)
                if pn == "team":
                    try:
                        tile_team[tid] = int(pv)
                    except ValueError:
                        pass
                elif pn == "unit" and pv.lower() == "commandcenter":
                    tile_cc[tid] = True

        out.append(
            Tileset(firstgid, ts.get("name") or "", tw, th, cols, tiles, tile_team, tile_cc)
        )
    out.sort(key=lambda t: t.firstgid)
    return out


def _resolve_gid(tilesets, gid):
    if gid == 0:
        return None
    gid_int = int(gid)
    flags = gid_int & (FLIP_H | FLIP_V | FLIP_D)
    base = gid_int & ~(FLIP_H | FLIP_V | FLIP_D)
    cand = None
    for ts in tilesets:
        if ts.firstgid <= base:
            cand = ts
        else:
            break
    if cand is None:
        return None
    local = base - cand.firstgid
    if local < 0 or local >= len(cand.tiles):
        return None
    return cand, local, flags


def _apply_flip(img, flags):
    if img is None:
        return None
    if flags & FLIP_H:
        img = img.transpose(Image.FLIP_LEFT_RIGHT)
    if flags & FLIP_V:
        img = img.transpose(Image.FLIP_TOP_BOTTOM)
    if flags & FLIP_D:
        img = img.transpose(Image.Transpose.ROTATE_90)
    return img


def detect_spawn_positions(root, tilesets, full_w, full_h, thumb_w, thumb_h, player_count=10):
    """
    全方位解析地图出生点坐标。
    阶段 1: 解析 <objectgroup> 明确声明的对象 (CommandCenter / FactionSelector / Spawns)
    阶段 2: 解析 Units 等图块层中的指挥中心图块 (0-based team 正确映射到 1-based 玩家)
    返回 {player_id: (tx, ty)}，坐标为缩略图中的真实像素坐标。
    """
    positions = {}
    sx = thumb_w / float(full_w)
    sy = thumb_h / float(full_h)

    # 阶段 1: 扫描所有 <objectgroup>
    for og in root.findall("objectgroup"):
        for obj in og.findall("object"):
            props = {}
            for p in obj.findall("./properties/property"):
                props[p.get("name", "")] = _prop_text(p)
            oname = (obj.get("name") or "").strip()
            otype = (obj.get("type") or "").strip()

            is_cc = (
                props.get("unit", "").lower() == "commandcenter"
                or "commandcenter" in oname.lower()
                or "commandcenter" in otype.lower()
                or oname.lower().startswith("spawn")
                or oname.lower().startswith("factionselector")
            )

            team_id = None
            if "team" in props:
                try:
                    t = int(props["team"])
                    if t >= 0:
                        team_id = t + 1
                except ValueError:
                    pass

            if team_id is None:
                m = re.search(r"\(t:(-?[0-9]+)\)", oname, re.IGNORECASE) or re.search(
                    r"\bt:([0-9]+)\b", oname, re.IGNORECASE
                )
                if m:
                    t = int(m.group(1))
                    if t >= 0:
                        team_id = t + 1

            if team_id is None:
                m = re.search(r"(?:spawn|player|p)[_\s\-]?([0-9]+)\b", oname, re.IGNORECASE)
                if m:
                    t = int(m.group(1))
                    if 1 <= t <= 24:
                        team_id = t

            if is_cc and team_id is not None:
                try:
                    ox = float(obj.get("x", 0))
                    oy = float(obj.get("y", 0))
                    ow = float(obj.get("width", 0))
                    oh = float(obj.get("height", 0))
                    cx = ox + ow / 2.0
                    cy = oy + oh / 2.0
                    positions[team_id] = (cx * sx, cy * sy)
                except Exception:
                    pass

    # 阶段 2: 扫描 Units / 图块层
    map_w = int(root.get("width") or 0)
    map_h = int(root.get("height") or 0)
    map_tw = int(root.get("tilewidth") or 32)
    map_th = int(root.get("tileheight") or 32)

    tile_cc_cells = {}
    for lay in root.findall("layer"):
        lname = (lay.get("name") or "").strip().lower()
        if "unit" in lname or lname in ("objects", "triggers"):
            data_el = lay.find("data")
            if data_el is None:
                continue
            try:
                gids = _decode_layer(data_el, map_w, map_h)
            except Exception:
                continue
            rows = gids.tolist()
            for y in range(map_h):
                row = rows[y]
                for x in range(map_w):
                    gid = row[x]
                    if gid == 0:
                        continue
                    res = _resolve_gid(tilesets, gid)
                    if res is None:
                        continue
                    ts, local, flags = res
                    if ts.tile_cc[local] and ts.tile_team[local] >= 0:
                        tid = ts.tile_team[local] + 1
                        tile_cc_cells.setdefault(tid, []).append((x, y))

    # 将图块坐标转换为缩略图坐标
    for tid, cells in tile_cc_cells.items():
        if tid not in positions:
            avg_x = sum(c[0] for c in cells) / float(len(cells)) + 0.5
            avg_y = sum(c[1] for c in cells) / float(len(cells)) + 0.5
            wx = avg_x * map_tw
            wy = avg_y * map_th
            positions[tid] = (wx * sx, wy * sy)

    return positions


def render_tmx_bytes(tmx_bytes, tmx_name="<memory>", zip_reader=None, max_dim=500,
                     scale_cap=8000, exclude_layers=()):
    """渲染 TMX 地图画面。返回 (PIL.Image, positions, full_w, full_h)。"""
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
    layers = []
    for lay in root.findall("layer"):
        name = (lay.get("name") or "").strip()
        low = name.lower()
        if low == "set":
            continue
        if low in skip:
            continue
        prio = LAYER_ORDER.get(low, 5)
        layers.append((prio, len(layers), name, lay))
    layers.sort(key=lambda x: (x[0], x[1]))

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
        tile = ts.tiles[local]
        if tile is None:
            return None
        if not flags:
            return tile
        key = (id(ts), local, flags)
        img = flip_cache.get(key)
        if img is None:
            img = _apply_flip(tile, flags)
            if scale != 1.0 and img is not None:
                img = img.resize((tw, th), Image.LANCZOS)
            flip_cache[key] = img
        return img

    if scale != 1.0:
        for ts in tilesets:
            ts.tiles = [
                t.resize((tw, th), Image.LANCZOS) if t is not None else None for t in ts.tiles
            ]

    for prio, _seq, name, lay in layers:
        data_el = lay.find("data")
        if data_el is None:
            continue
        try:
            gids = _decode_layer(data_el, w, h)
        except Exception:
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
                img = ts.tiles[local]
                if img is None:
                    continue
                if flags:
                    img = tile_image(ts, local, flags)
                    if img is None:
                        continue
                if ts.tw != tw or ts.th != th:
                    img = img.resize((tw, th), Image.LANCZOS)
                canvas.paste(img, (x * tw, y * th), img)

    # 缩放到目标缩略图尺寸
    cw, ch = canvas.size
    final_w, final_h = cw, ch
    if max(cw, ch) > max_dim:
        r = max_dim / float(max(cw, ch))
        final_w = max(1, int(round(cw * r)))
        final_h = max(1, int(round(ch * r)))
        canvas = canvas.resize((final_w, final_h), Image.LANCZOS)

    bg = Image.new("RGB", canvas.size, (0, 0, 0))
    bg.paste(canvas, (0, 0), canvas)

    # 精确获取出生点
    positions = detect_spawn_positions(root, tilesets, full_w, full_h, final_w, final_h)
    return bg, positions, full_w, full_h


def _read_tmx_bytes(path):
    with open(path, "rb") as fh:
        return fh.read()


def _load_font(size):
    for p in _FONT_CANDIDATES:
        if os.path.isfile(p):
            try:
                return ImageFont.truetype(p, size)
            except Exception:
                continue
    return ImageFont.load_default()


def _draw_team_markers(map_img, positions):
    """
    在缩略图上画出生点编号:
    - 黑色半透明圆底
    - 细腻白色/队伍色光晕描边
    - 队伍色高对比度加粗数字
    """
    if not positions:
        return map_img

    W, H = map_img.size
    layer = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    draw = ImageDraw.Draw(layer)

    # 动态根据图像大小计算标号圈半径 (保持视觉适中)
    eSize = max(9, min(14, int(round(min(W, H) * 0.026))))
    fontSize = max(10, int(round(eSize * 1.35)))
    font = _load_font(fontSize)

    for tid in sorted(positions.keys()):
        tx, ty = positions[tid]
        disp = TEAM_COLORS.get(tid, ((255, 255, 255), (255, 255, 255)))[0]

        # 1. 黑色半透明底圆
        draw.ellipse(
            (tx - eSize, ty - eSize, tx + eSize, ty + eSize),
            fill=(0, 0, 0, 160),
            outline=(255, 255, 255, 140),
            width=1,
        )

        # 2. 居中绘制数字
        txt = str(tid)
        try:
            l, t, r, b = draw.textbbox((0, 0), txt, font=font)
            tw, th = r - l, b - t
        except Exception:
            tw, th = 8, 10
        # 阴影与正文
        draw.text((tx - tw / 2.0 + 1, ty - th / 2.0), txt, fill=(0, 0, 0, 200), font=font)
        draw.text((tx - tw / 2.0, ty - th / 2.0 - 1), txt, fill=disp + (255,), font=font)

    base = map_img.convert("RGBA")
    base.paste(layer, (0, 0), layer)
    return base.convert("RGB")


def _player_count_from_name(path):
    """支持官方 [pN] 规范以及社区常见的 (10p)、（12P混战）、(p4)、10人 等命名。"""
    name = os.path.basename(path)
    # 1. 官方规范 [p2] 或 [2p]
    m = re.search(r"\[(?:.*;)?p?([0-9]+)p?\]", name, re.IGNORECASE)
    if m:
        try:
            val = int(m.group(1))
            if 1 <= val <= 24:
                return val
        except Exception:
            pass
    # 2. 括号内的 (10p), （12P混战）, (p4)
    m = re.search(r"[\(（](?:p)?([0-9]+)\s*p?[^\)）]*[\)）]", name, re.IGNORECASE)
    if m:
        try:
            val = int(m.group(1))
            if 1 <= val <= 24:
                return val
        except Exception:
            pass
    # 3. 匹配人数字符，如 10人
    m = re.search(r"([0-9]+)\s*人", name)
    if m:
        try:
            val = int(m.group(1))
            if 1 <= val <= 24:
                return val
        except Exception:
            pass
    # 4. 匹配结尾或带下划线的数字+p，如 23p混战, _2p_
    m = re.search(r"(?:[_\s\-])([0-9]{1,2})\s*p\b", name, re.IGNORECASE)
    if m:
        try:
            val = int(m.group(1))
            if 1 <= val <= 24:
                return val
        except Exception:
            pass
    m = re.search(r"([0-9]{1,2})\s*p(?:混战|图)?", name, re.IGNORECASE)
    if m:
        try:
            val = int(m.group(1))
            if 1 <= val <= 24:
                return val
        except Exception:
            pass
    return 10


def render_map(tmx_bytes, tmx_name, zip_reader=None, max_dim=500):
    """完整渲染: 地图底图 + 精准出生点标号。返回 PIL 图像。"""
    map_img, positions, _w, _h = render_tmx_bytes(
        tmx_bytes, tmx_name, zip_reader, max_dim, exclude_layers=()
    )
    if positions:
        map_img = _draw_team_markers(map_img, positions)
    return map_img


def cmd_render(args):
    target = args.tmx
    out = args.out

    # 支持 pack.rwmod:inner_map.tmx 语法
    if ".rwmod:" in target:
        parts = target.split(".rwmod:", 1)
        pack_path = parts[0] + ".rwmod"
        inner_tmx = parts[1].replace("\\", "/")
        with zipfile.ZipFile(pack_path, "r") as z:
            data = z.read(inner_tmx)
            img = render_map(data, inner_tmx, z, args.max)

        if not out:
            base = inner_tmx[:-4] if inner_tmx.lower().endswith(".tmx") else inner_tmx
            out = base + "_map.png"

        # 如果指定了 --patch-zip，直接更新进压缩包
        if args.patch_zip:
            want_name = (inner_tmx[:-4] if inner_tmx.lower().endswith(".tmx") else inner_tmx) + "_map.png"
            buf = io.BytesIO()
            img.save(buf, format="PNG")
            _update_zip_entries(pack_path, {want_name: buf.getvalue()})
            print("  已同步更新至压缩包: %s -> %s" % (os.path.basename(pack_path), want_name))

        img.save(out)
        print("  -> %s  %dx%d  %d bytes" % (out, img.width, img.height, os.path.getsize(out)))
        return 0

    data = _read_tmx_bytes(target)
    img = render_map(data, os.path.abspath(target), None, args.max)
    if not out:
        base = target[:-4] if target.lower().endswith(".tmx") else target
        out = base + "_map.png"
    img.save(out)
    print("  -> %s  %dx%d  %d bytes" % (out, img.width, img.height, os.path.getsize(out)))
    return 0


def cmd_batch(args):
    root = args.dir
    if os.path.isfile(root):
        files = [root]
        root = os.path.dirname(root) or "."
    else:
        files = []
        for dirpath, _dirs, names in os.walk(root):
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
    print("\n完成: 新建/覆盖 %d, 跳过 %d, 失败 %d" % (made, skipped, failed))
    return 1 if failed else 0


def _update_zip_entries(pack_path, entries_dict):
    """安全热更新 zip/rwmod 内部文件。"""
    tmp_fd, tmp_path = tempfile.mkstemp(suffix=".rwmod", dir=os.path.dirname(os.path.abspath(pack_path)))
    os.close(tmp_fd)
    try:
        with zipfile.ZipFile(pack_path) as zin, zipfile.ZipFile(tmp_path, "w", zipfile.ZIP_DEFLATED) as zout:
            added_lower = set(k.lower() for k in entries_dict.keys())
            for item in zin.infolist():
                if item.filename.lower() in added_lower:
                    continue  # 跳过旧版本，由新 entries 写入
                data = zin.read(item.filename)
                zi = zipfile.ZipInfo(item.filename, date_time=item.date_time)
                zi.compress_type = item.compress_type
                zi.external_attr = item.external_attr
                zout.writestr(zi, data)
            for name, data in entries_dict.items():
                zout.writestr(name, data)
        shutil.move(tmp_path, pack_path)
    finally:
        if os.path.exists(tmp_path):
            os.remove(tmp_path)


def cmd_patch(args):
    """为 .rwmod 包内部地图生成或强制重绘带出生点标记的 _map.png。"""
    for pack in args.packs:
        if not os.path.exists(pack):
            print("! 文件不存在: %s" % pack)
            continue
        print("== %s" % os.path.basename(pack))
        with zipfile.ZipFile(pack) as z:
            names = z.namelist()
            tmxs = [n for n in names if n.lower().endswith(".tmx")]
            have = set(n.lower() for n in names)
            todo = []
            for n in tmxs:
                want = n[:-4] + "_map.png"
                if args.force or (want.lower() not in have):
                    todo.append((n, want))
            if not todo:
                print("  所有地图都已有缩略图且未指定 --force")
                continue
            if args.dry_run:
                for n, want in todo:
                    print("  * 将生成并更新: %s" % want)
                continue
            add = {}
            for n, want in todo:
                try:
                    img = render_map(z.read(n), n, z, args.max)
                    buf = io.BytesIO()
                    img.save(buf, format="PNG")
                    add[want] = buf.getvalue()
                    print("  + %s  %dx%d  %d bytes" % (want, img.width, img.height, len(add[want])))
                except Exception as exc:
                    print("  ! 渲染失败 %s: %s" % (n, exc))
            if not add:
                continue
            _update_zip_entries(pack, add)
            print("  已写回 %s (更新 %d 个缩略图)" % (os.path.basename(pack), len(add)))
    return 0


def cmd_fixnames(args):
    """修复缩略图文件名大小写及缺失。"""
    for pack in args.packs:
        print("== %s" % os.path.basename(pack))
        with zipfile.ZipFile(pack) as z:
            names = set(z.namelist())
            lowmap = {}
            for n in names:
                lowmap.setdefault(n.lower(), n)
            copy_jobs = []
            render_jobs = []
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
        _update_zip_entries(pack, add)
        print("  已写回 (+%d)" % len(add))
    return 0


def main():
    ap = argparse.ArgumentParser(description="Rusted Warfare 地图高清出生点缩略图生成器")
    sub = ap.add_subparsers(dest="cmd", required=True)

    r = sub.add_parser("render", help="渲染单个 .tmx (支持 pack.rwmod:inner.tmx)")
    r.add_argument("tmx")
    r.add_argument("-o", "--out", default=None)
    r.add_argument("--max", type=int, default=500)
    r.add_argument("--patch-zip", action="store_true", help="如果是 zip 内部地图，直接写回 zip")
    r.set_defaults(func=cmd_render)

    b = sub.add_parser("batch", help="批量处理目录")
    b.add_argument("dir")
    b.add_argument("--max", type=int, default=500)
    b.add_argument("--force", action="store_true", help="强制全量重绘覆盖已有缩略图")
    b.add_argument("--dry-run", action="store_true")
    b.set_defaults(func=cmd_batch)

    p = sub.add_parser("patch", help="把缩略图注入/重绘至 .rwmod 包内部")
    p.add_argument("packs", nargs="+")
    p.add_argument("--max", type=int, default=500)
    p.add_argument("--force", action="store_true", help="强制全量重绘替换包内已有缩略图")
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
