#!/usr/bin/env python3
"""Собирает иконки Norka из геометрии NorkaIcon.vue.

Вариант A («вырезка») — иконка приложения: macOS/Linux с полями под squircle,
Windows .ico без полей (мелкие размеры — упрощённые глаза).
Вариант C («минимал») — трей Windows/Linux и монохромный шаблон строки меню macOS.
Цвета глаз те же, что в трее сейчас: зелёный / янтарь / красный / закрытые серые.

Запуск из корня репозитория:

    pip install -r tools/icons/requirements.txt
    python3 tools/icons/generate.py
    python3 tools/icons/generate.py --sheet /tmp/icon-sheet.png

Повторный запуск не масштабирует Lottie подмигивания второй раз.
"""

from __future__ import annotations

import argparse
import io
import json
import re
import struct
import sys
from pathlib import Path

import resvg_py
from PIL import Image, ImageFilter

ROOT = Path(__file__).resolve().parents[2]
VUE = ROOT / 'frontend/src/components/norka/NorkaIcon.vue'
EYE_GREEN = '#3DDC84'
EYE = {
    'connected': '#3DDC84',
    'connecting': '#F5C542',
    'error': '#FF5A5F',
    'stopped': '#8A9099',
}
THEME = {
    'dark': dict(bg='#1E2127', arch='#F2EFEA', pupil='#101215', lid='#2B3631', glow=0.55, shade=0.28),
    'light': dict(bg='#F6F4F0', arch='#1E2127', pupil='#14171B', lid='#A3C4B0', glow=0.22, shade=0.20),
}
# Мелкий размер (favicon / трей / 16–48 px): глаза крупнее, зрачок толще, без бликов.
SMALL = {
    'left': dict(lid='M-85.5 -186L85.5 -186L85.5 -18C28.5 -15.33 -28.5 -21.33 -85.5 -24L-85.5 -186Z'),
    'right': dict(lid='M85.5 -24C28.5 -21.33 -28.5 -15.33 -85.5 -18L-85.5 -186L85.5 -186L85.5 -24Z'),
    'pupil': 'M-5 -17.25C-0.95 -17.25 6.25 -7.69 6.25 4C6.25 15.69 -0.95 25.25 -5 25.25C-9.05 25.25 -16.25 15.69 -16.25 4C-16.25 -7.69 -9.05 -17.25 -5 -17.25Z',
}
# Вариант C, утверждённый вместе с вырезкой: голова и «поля» норки.
HEAD = 'M118 436L118 318C118 222 178 166 256 166C334 166 394 222 394 318L394 436Z'
EDGE = (
    'M64 466C132 424 190 404 256 404C322 404 380 424 448 466'
    'C398 480 330 476 256 476C182 476 114 480 64 466Z'
)
# Светлая кромка норы: на тёмной панели (Windows 11, GNOME/KDE) заливка #1E2127
# пропадает, и без ободка остаются только глаза и светлый холмик.
# 32 в координатах 512 после scale(1.18) — около 1 px на 16 и 2 px на 32.
HOLE_RIM = '#F2EFEA'
HOLE_RIM_WIDTH = 32
MARGIN = 0.88


def load_geometry():
    text = VUE.read_text(encoding='utf-8')
    match = re.search(r'const GEOMETRY = (\{.*?\n\}) as const', text, re.S)
    if not match:
        sys.exit('generate.py: в NorkaIcon.vue нет const GEOMETRY')
    geom = json.loads(match.group(1))
    hole = re.search(r"const HOLE_A = '([^']+)'", text)
    scale = re.search(r'const CUTOUT_SCALE = ([0-9.]+)', text)
    if not hole or not scale:
        sys.exit('generate.py: в NorkaIcon.vue нет HOLE_A / CUTOUT_SCALE')
    return geom, hole.group(1), float(scale.group(1))


GEOM, HOLE_A, CUTOUT_SCALE = load_geometry()
ARCH_A = GEOM['A']['arch']


def wrap(defs: str, body: str) -> str:
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><defs>{defs}</defs>{body}</svg>'


def eyes(uid: str, theme: dict, eye: str, small: bool) -> tuple[str, str]:
    defs, body = [], []
    glow = theme['glow']
    for item in GEOM['A']['eyes']:
        side = item['side']
        lid = SMALL[side]['lid'] if small else item['lid']
        defs.append(
            f'<radialGradient id="{uid}-{side}-glow" cx="0.5" cy="0.5" r="0.5">'
            f'<stop offset="0" stop-color="{eye}" stop-opacity="{glow}"/>'
            f'<stop offset="0.45" stop-color="{eye}" stop-opacity="{round(glow * 0.45, 4)}"/>'
            f'<stop offset="1" stop-color="{eye}" stop-opacity="0"/></radialGradient>'
        )
        defs.append(
            f'<mask id="{uid}-{side}-vis" maskUnits="userSpaceOnUse" x="-120" y="-120" width="240" height="240">'
            f'<path d="{item["almond"]}" fill="#fff"/><path d="{lid}" fill="#000"/></mask>'
        )
        if small:
            body.append(
                f'<g transform="translate({item["cx"]} {item["cy"]}) scale(1.12 1.4)">'
                f'<g mask="url(#{uid}-{side}-vis)">'
                f'<path d="{item["almond"]}" fill="{eye}"/>'
                f'<path d="{SMALL["pupil"]}" fill="{theme["pupil"]}"/></g></g>'
            )
        else:
            body.append(
                f'<g transform="translate({item["cx"]} {item["cy"]})">'
                f'<ellipse cx="0" cy="2" rx="{GEOM["A"]["glow"]["rx"]}" ry="{GEOM["A"]["glow"]["ry"]}" fill="url(#{uid}-{side}-glow)"/>'
                f'<path d="{item["almond"]}" fill="{theme["lid"]}"/>'
                f'<g mask="url(#{uid}-{side}-vis)"><path d="{item["almond"]}" fill="{eye}"/>'
                f'<path d="{item["shade"]}" fill="#000" fill-opacity="{theme["shade"]}"/>'
                f'<path d="{item["pupil"]}" fill="{theme["pupil"]}"/>'
                f'<path d="{item["hl1"]}" fill="#fff"/>'
                f'<path d="{item["hl2"]}" fill="#fff" fill-opacity="0.75"/></g></g>'
            )
    return ''.join(defs), ''.join(body)


def closed_eyes(color: str) -> str:
    parts = []
    for item in GEOM['A']['eyes']:
        parts.append(
            f'<g transform="translate({item["cx"]} {item["cy"]}) scale(1.12 1.4)">'
            f'<path d="{item["closed"]}" fill="none" stroke="{color}" stroke-width="8" '
            f'stroke-linecap="round" stroke-linejoin="round"/></g>'
        )
    return ''.join(parts)


def cutout_transform() -> str:
    shift = round(256 * (1 - CUTOUT_SCALE), 4)
    return f'translate({shift} {shift}) scale({CUTOUT_SCALE})'


def variant_a(mode: str, small: bool = False, part: str = 'all') -> str:
    """mode: os | app-dark | app-light. part: all | hole | arch | eyes."""
    theme = THEME['light' if mode == 'app-light' else 'dark']
    defs, eye_body = eyes('va', theme, EYE_GREEN, small)
    if mode == 'app-light':
        stroke, width = 'none', 3
    elif small:
        stroke, width = 'rgba(0,0,0,0.45)', 14
    else:
        stroke, width = 'rgba(0,0,0,0.18)', 3
    hole = f'<path d="{HOLE_A}" fill="{theme["bg"]}"/>'
    arch = f'<path d="{ARCH_A}" fill="{theme["arch"]}" stroke="{stroke}" stroke-width="{width}"/>'
    chunks = []
    if part in ('all', 'hole'):
        chunks.append(hole)
    if part in ('all', 'arch'):
        chunks.append(arch)
    if part in ('all', 'eyes'):
        chunks.append(eye_body)
    if part == 'hole':
        defs = ''
    if part == 'arch':
        defs = ''
    return wrap(defs, f'<g transform="{cutout_transform()}">{"".join(chunks)}</g>')


def variant_c(eye: str, closed: bool = False) -> str:
    theme = THEME['dark']
    if closed:
        defs, eye_body = '', closed_eyes(eye)
    else:
        defs, eye_body = eyes('vc', theme, eye, small=True)
    body = (
        f'<g transform="translate(256 259) scale(1.18) translate(-256 -326)">'
        f'<path d="{HEAD}" fill="{theme["bg"]}" stroke="{HOLE_RIM}" stroke-width="{HOLE_RIM_WIDTH}" '
        f'stroke-linejoin="round"/>'
        f'{eye_body}'
        f'<path d="{EDGE}" fill="{theme["arch"]}" stroke="{theme["arch"]}" stroke-width="24" '
        f'stroke-linejoin="round"/></g>'
    )
    return wrap(defs, body)


def template_c() -> str:
    """Монохромный чёрный силуэт: альфа несёт форму, система красит строку меню."""
    eyes_cut = ''.join(
        f'<g transform="translate({item["cx"]} {item["cy"]}) scale(1.12 1.4)">'
        f'<path d="{item["almond"]}" fill="#000"/></g>'
        for item in GEOM['A']['eyes']
    )
    pupils = ''.join(
        f'<g transform="translate({item["cx"]} {item["cy"]}) scale(1.12 1.4)">'
        f'<path d="{SMALL["pupil"]}" fill="#000"/></g>'
        for item in GEOM['A']['eyes']
    )
    defs = (
        f'<mask id="m"><rect x="-200" y="-200" width="900" height="900" fill="#fff"/>{eyes_cut}'
        f'<path d="{EDGE}" fill="#000" stroke="#000" stroke-width="70" stroke-linejoin="round"/></mask>'
    )
    body = (
        '<g transform="translate(256 259) scale(1.18) translate(-256 -326)">'
        f'<path d="{HEAD}" fill="#000" mask="url(#m)"/>{pupils}'
        f'<path d="{EDGE}" fill="#000" stroke="#000" stroke-width="24" stroke-linejoin="round"/></g>'
    )
    return wrap(defs, body)


def render(svg: str, size: int) -> Image.Image:
    png = resvg_py.svg_to_bytes(svg_string=svg, width=size, height=size)
    return Image.open(io.BytesIO(bytes(png))).convert('RGBA')


def shadow(img: Image.Image) -> Image.Image:
    """Лёгкая тень под вырезкой, как в утверждённом мастере Dock."""
    size = img.width
    alpha = img.getchannel('A')
    out = Image.new('RGBA', img.size, (0, 0, 0, 0))
    for blur, dy, opacity in (
        (size * 0.022, size * 0.018, 0.32),
        (max(0.6, size * 0.004), size * 0.003, 0.22),
    ):
        layer = Image.new('RGBA', img.size, (0, 0, 0, 0))
        mask = alpha.filter(ImageFilter.GaussianBlur(blur)).point(lambda value, opacity=opacity: int(value * opacity))
        layer.putalpha(mask)
        out.alpha_composite(layer, (0, int(round(dy))))
    out.alpha_composite(img)
    return out


def plate(svg: str, size: int, shadowed: bool = True) -> Image.Image:
    """Вырезка с полями ~12%, чтобы squircle Dock и серая подложка macOS 26 её не срезали."""
    inner = max(1, int(size * MARGIN))
    art = render(svg, inner)
    canvas = Image.new('RGBA', (size, size), (0, 0, 0, 0))
    offset = (size - inner) // 2
    canvas.alpha_composite(art, (offset, offset))
    return shadow(canvas) if shadowed else canvas


def save_png(img: Image.Image, path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    img.save(path, 'PNG')


def _dib32(img: Image.Image) -> bytes:
    image = img.convert('RGBA')
    width, height = image.size
    header = struct.pack('<IIIHHIIIIII', 40, width, height * 2, 1, 32, 0, width * height * 4, 0, 0, 0, 0)
    raw = image.tobytes()
    xor_rows = bytearray()
    for y in range(height - 1, -1, -1):
        row = raw[y * width * 4:(y + 1) * width * 4]
        for index in range(0, len(row), 4):
            red, green, blue, alpha = row[index:index + 4]
            xor_rows += bytes((blue, green, red, alpha))
    stride = ((width + 31) // 32) * 4
    pixels = image.load()
    mask = bytearray()
    for y in range(height - 1, -1, -1):
        row = bytearray(stride)
        for x in range(width):
            if pixels[x, y][3] < 128:
                row[x // 8] |= 0x80 >> (x % 8)
        mask += row
    return header + bytes(xor_rows) + bytes(mask)


def write_ico(path: Path, images: list[Image.Image]) -> None:
    """ICO: 32-bit BMP (как текущие файлы) и PNG для 256 px, с альфой."""
    ordered = sorted((img.convert('RGBA') for img in images), key=lambda item: item.size[0])
    blobs = []
    for img in ordered:
        if img.size[0] >= 256:
            buf = io.BytesIO()
            img.save(buf, 'PNG')
            blobs.append(buf.getvalue())
        else:
            blobs.append(_dib32(img))
    header = struct.pack('<HHH', 0, 1, len(ordered))
    entries = bytearray()
    offset = 6 + 16 * len(ordered)
    for img, blob in zip(ordered, blobs):
        size = img.size[0]
        width_b = 0 if size >= 256 else size
        height_b = 0 if size >= 256 else size
        entries += struct.pack('<BBBBHHII', width_b, height_b, 0, 0, 1, 32, len(blob), offset)
        offset += len(blob)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(header + bytes(entries) + b''.join(blobs))


def assert_green_eyes(img: Image.Image, label: str) -> None:
    target = (0x3D, 0xDC, 0x84)
    count = 0
    step = 1 if img.width <= 128 else 2
    pixels = img.load()
    for y in range(0, img.height, step):
        for x in range(0, img.width, step):
            red, green, blue, alpha = pixels[x, y]
            if alpha < 200:
                continue
            if abs(red - target[0]) <= 18 and abs(green - target[1]) <= 18 and abs(blue - target[2]) <= 22:
                count += 1
    if count < 8:
        sys.exit(f'generate.py: у {label} нет зелёных миндалевидных глаз ({count} пикселей)')


def assert_template(img: Image.Image) -> None:
    pixels = img.load()
    opaque = 0
    for y in range(0, img.height, 2):
        for x in range(0, img.width, 2):
            red, green, blue, alpha = pixels[x, y]
            if alpha == 0:
                continue
            if abs(red - green) > 2 or abs(green - blue) > 2:
                sys.exit('generate.py: шаблон macOS не монохромный')
            if red > 24 and alpha > 200:
                sys.exit('generate.py: шаблон macOS должен быть чёрным с альфой, не белым')
            if alpha > 200 and red < 8:
                opaque += 1
    if opaque < 100:
        sys.exit('generate.py: у шаблона macOS нет непрозрачного силуэта')


def retarget_wink(path: Path) -> None:
    """Кадр 0 подмигивания совпадает с вырезкой: дырка вместо плитки и тот же масштаб."""
    data = json.loads(path.read_text(encoding='utf-8'))
    if any(layer.get('nm') == 'cutout-scale' for layer in data['layers']):
        return
    scale_pct = round(CUTOUT_SCALE * 100, 4)
    parent = {
        'ddd': 0, 'ind': 100, 'ty': 3, 'nm': 'cutout-scale', 'sr': 1,
        'ks': {
            'o': {'a': 0, 'k': 100},
            'r': {'a': 0, 'k': 0},
            'p': {'a': 0, 'k': [256, 256, 0]},
            'a': {'a': 0, 'k': [0, 0, 0]},
            's': {'a': 0, 'k': [scale_pct, scale_pct, 100]},
        },
        'ao': 0, 'ip': 0, 'op': data.get('op', 180), 'st': 0, 'bm': 0,
    }
    hole = {
        'ty': 'sh', 'nm': 'hole',
        'ks': {'a': 0, 'k': {
            'c': True,
            'v': [[126, 408], [126, 326], [256, 214], [386, 326], [386, 408]],
            'i': [[0, 0], [0, 0], [-74, 0], [0, -67], [0, 0]],
            'o': [[0, 0], [0, -67], [74, 0], [0, 0], [0, 0]],
        }},
    }
    for layer in data['layers']:
        if layer.get('parent') is None and layer.get('ty') in (3, 4):
            pos = layer['ks']['p']
            if pos.get('a') != 0:
                sys.exit(f'generate.py: {path.name} слой {layer.get("nm")} с анимированной позицией')
            x, y, z = pos['k']
            pos['k'] = [x - 256, y - 256, z]
            layer['parent'] = 100
        if layer.get('nm') != 'background':
            continue
        group = layer['shapes'][0]
        group['it'][0] = hole
        group['nm'] = 'hole'
    data['layers'].insert(0, parent)
    path.write_text(json.dumps(data, ensure_ascii=False, separators=(',', ':')), encoding='utf-8')


def write_sources() -> None:
    out = ROOT / 'tools/icons/masters'
    out.mkdir(parents=True, exist_ok=True)
    (out / 'A-os.svg').write_text(variant_a('os'), encoding='utf-8')
    (out / 'A-os-small.svg').write_text(variant_a('os', small=True), encoding='utf-8')
    (out / 'A-app-dark.svg').write_text(variant_a('app-dark'), encoding='utf-8')
    (out / 'A-app-light.svg').write_text(variant_a('app-light'), encoding='utf-8')
    (out / 'C-macos-template.svg').write_text(template_c(), encoding='utf-8')
    (out / 'C-tray.svg').write_text(variant_c(EYE_GREEN), encoding='utf-8')


def build_assets() -> None:
    detailed = variant_a('os')
    small = variant_a('os', small=True)
    appicon = plate(detailed, 1024)
    assert_green_eyes(appicon.resize((256, 256), Image.Resampling.LANCZOS), 'appicon')
    save_png(appicon, ROOT / 'build/appicon.png')

    win_sizes = (16, 20, 24, 32, 40, 48, 64, 96, 128, 256)
    win_images = []
    for size in win_sizes:
        svg = small if size <= 48 else detailed
        win_images.append(render(svg, size))
    assert_green_eyes(win_images[win_sizes.index(32)], 'windows 32')
    write_ico(ROOT / 'build/windows/icon.ico', win_images)

    iconset = ROOT / 'build/darwin/norka.iconset'
    # имя файла → сторона в пикселях. Мелкие — упрощённые глаза.
    named = {
        'icon_16x16.png': 16,
        'icon_16x16@2x.png': 32,
        'icon_32x32.png': 32,
        'icon_32x32@2x.png': 64,
        'icon_128x128.png': 128,
        'icon_128x128@2x.png': 256,
        'icon_256x256.png': 256,
        'icon_256x256@2x.png': 512,
        'icon_512x512.png': 512,
        'icon_512x512@2x.png': 1024,
    }
    for name, size in named.items():
        if size == 1024:
            image = appicon
        else:
            svg = small if size <= 48 else detailed
            image = plate(svg, size)
        save_png(image, iconset / name)

    linux_sizes = (16, 32, 48, 64, 128, 256, 512)
    hicolor = ROOT / 'build/linux/hicolor'
    for size in linux_sizes:
        svg = small if size <= 48 else detailed
        image = plate(svg, size)
        save_png(image, hicolor / f'{size}x{size}/apps/norka.png')

    favicon_svg = variant_a('os', small=True)
    favicon_path = ROOT / 'frontend/public/favicon.svg'
    favicon_path.write_text(
        favicon_svg.replace(
            '<svg ',
            '<svg id="norka-cutout" ',
            1,
        ),
        encoding='utf-8',
    )
    write_ico(
        ROOT / 'frontend/public/favicon.ico',
        [render(small, size) for size in (16, 32, 48)],
    )

    layers = ROOT / 'build/darwin/AppIcon.icon/Assets'
    for part in ('hole', 'arch', 'eyes'):
        save_png(plate(variant_a('os', part=part), 1024, shadowed=False), layers / f'{part}.png')
    icon_json = {
        'fill': {'solid': 'extended-srgb:0,0,0,0'},
        'groups': [{
            'name': 'Norka',
            'blur-material': 0,
            'lighting': 'combined',
            'shadow': {'kind': 'neutral', 'opacity': 0.35},
            'specular': False,
            'translucency': {'enabled': False, 'value': 0},
            'layers': [
                {'name': 'Hole', 'image-name': 'hole.png', 'glass': False},
                {'name': 'Arch', 'image-name': 'arch.png', 'glass': False},
                {'name': 'Eyes', 'image-name': 'eyes.png', 'glass': False},
            ],
        }],
        'supported-platforms': {'squares': ['macOS']},
    }
    (ROOT / 'build/darwin/AppIcon.icon/icon.json').write_text(
        json.dumps(icon_json, indent=2) + '\n',
        encoding='utf-8',
    )

    template = render(template_c(), 512)
    assert_template(template)
    save_png(template, ROOT / 'build/macos-systray.png')

    for status, color in EYE.items():
        svg = variant_c(color, closed=(status == 'stopped'))
        frames = []
        for size in (16, 20, 24, 32):
            image = render(svg, size)
            save_png(image, ROOT / f'build/tray/tray-{status}-{size}.png')
            frames.append(image)
        if status == 'connected':
            assert_green_eyes(frames[-1], 'tray-connected-32')
        write_ico(ROOT / f'build/tray/tray-{status}.ico', frames)

    for name in ('norka-A-wink.json', 'norka-A-wink-light.json'):
        retarget_wink(ROOT / 'frontend/src/assets/norka' / name)


def icon_sheet(path: Path) -> None:
    """Лист 16/32/128/512/1024 плюс трей и шаблон macOS."""
    from PIL import ImageDraw, ImageFont

    files = {
        16: ROOT / 'build/darwin/norka.iconset/icon_16x16.png',
        32: ROOT / 'build/darwin/norka.iconset/icon_16x16@2x.png',
        128: ROOT / 'build/darwin/norka.iconset/icon_128x128.png',
        512: ROOT / 'build/darwin/norka.iconset/icon_512x512.png',
        1024: ROOT / 'build/appicon.png',
    }
    # Мелкие размеры увеличены ближайшим соседом, чтобы пиксели остались чёткими.
    display = {16: 96, 32: 96, 128: 128, 512: 160, 1024: 160}
    tiles = []
    for size in (16, 32, 128, 512, 1024):
        image = Image.open(files[size]).convert('RGBA')
        shown = image.resize((display[size], display[size]), Image.Resampling.NEAREST if size <= 32 else Image.Resampling.LANCZOS)
        tiles.append((str(size), shown))
    font_path = '/usr/share/fonts/truetype/macos/Inter-Regular.ttf'
    font = ImageFont.truetype(font_path, 14) if Path(font_path).exists() else ImageFont.load_default()

    def checker(size: int) -> Image.Image:
        board = Image.new('RGBA', (size, size))
        pixels = board.load()
        for y in range(size):
            for x in range(size):
                pixels[x, y] = (255, 255, 255, 255) if (x // 8 + y // 8) % 2 == 0 else (214, 214, 218, 255)
        return board

    pad, label_h = 28, 22
    row_h = max(tile.height for _, tile in tiles) + label_h + 8
    tray_icons = []
    for status in EYE:
        image = Image.open(ROOT / f'build/tray/tray-{status}-32.png').convert('RGBA')
        tray_icons.append(image.resize((64, 64), Image.Resampling.NEAREST))
    template = Image.open(ROOT / 'build/macos-systray.png').convert('RGBA').resize((64, 64), Image.Resampling.NEAREST)
    strip_h = 64
    width = pad + sum(tile.width + pad for _, tile in tiles)
    height = pad + row_h + 16 + (18 + strip_h + 12) * 2 + 18 + strip_h + pad
    sheet = Image.new('RGB', (width, height), (244, 244, 245))
    draw = ImageDraw.Draw(sheet)
    x = pad
    for label, tile in tiles:
        draw.text((x, pad), f'{label} px', fill=(24, 24, 27), font=font)
        cell = checker(tile.width)
        cell.alpha_composite(tile)
        sheet.paste(cell.convert('RGB'), (x, pad + label_h))
        x += tile.width + pad
    y = pad + row_h + 16
    captions = (
        ('трей C · светлая панель', (246, 246, 248)),
        ('трей C · тёмная панель', (32, 32, 36)),
    )
    for caption, bg in captions:
        draw.text((pad, y), caption, fill=(82, 82, 91), font=font)
        y += 18
        strip = Image.new('RGB', (width - pad * 2, strip_h), bg)
        sheet.paste(strip, (pad, y))
        x = pad + 12
        for image in tray_icons:
            cell = Image.new('RGBA', image.size, bg + (255,))
            cell.alpha_composite(image)
            sheet.paste(cell.convert('RGB'), (x, y))
            x += 72
        y += strip_h + 12
    draw.text((pad, y), 'macOS template', fill=(82, 82, 91), font=font)
    y += 18
    x = pad
    for bg in ((246, 246, 248), (38, 38, 42)):
        bar = Image.new('RGBA', (64, 64), bg + (255,))
        bar.alpha_composite(template)
        sheet.paste(bar.convert('RGB'), (x, y))
        x += 72
    path.parent.mkdir(parents=True, exist_ok=True)
    sheet.save(path, 'PNG')


def main() -> None:
    parser = argparse.ArgumentParser(description='Собрать иконки Norka из SVG')
    parser.add_argument('--sheet', type=Path, help='дополнительно записать лист размеров')
    args = parser.parse_args()
    write_sources()
    build_assets()
    if args.sheet:
        icon_sheet(args.sheet)
        print(f'sheet {args.sheet}')
    print('icons ok')


if __name__ == '__main__':
    main()
