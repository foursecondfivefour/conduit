#!/usr/bin/env python3
"""Bake a bitmap font for `conduit-tui` from a TrueType file.

The GUI crate renders text itself, with no font library at runtime, so the glyph
tables have to exist as Rust source. This script is the reproducible way to make
them: it reads a TTF, rasterises printable ASCII into fixed 5x13 cells, and emits
`src/font.rs`.

    python3 tools/bake_font.py /usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf

Baked bitmaps are derived from DejaVu Sans Mono, which is distributed under a
permissive licence derived from the Bitstream Vera Fonts licence. Only raster
output is embedded here, not the font file.
"""

from __future__ import annotations

import struct
import sys
from dataclasses import dataclass

CELL_W = 6
CELL_H = 13
PIXEL_SIZE = 13.0
SUPERSAMPLE = 4
FIRST = 32
LAST = 126

# Printable ASCII plus the punctuation and symbols the UI actually uses.
EXTRA = [
    0x00B7,  # middle dot
    0x00AB,  # left guillemet
    0x00BB,  # right guillemet
    0x2013,  # en dash
    0x2014,  # em dash
    0x2018,  # left single quote
    0x2019,  # right single quote
    0x201C,  # left double quote
    0x201D,  # right double quote
    0x2022,  # bullet
    0x2026,  # ellipsis
    0x2190,  # left arrow
    0x2192,  # right arrow
    0x2191,  # up arrow
    0x2193,  # down arrow
    0x25B8,  # small right triangle (collapsed section)
    0x25BE,  # small down triangle (expanded section)
    0x2713,  # check mark
]

CHARSET = sorted(set(list(range(FIRST, LAST + 1)) + EXTRA))


@dataclass
class Glyph:
    points: list[tuple[float, float, bool]]  # x, y, on_curve
    contours: list[int]  # end point indices


class TrueType:
    def __init__(self, path: str) -> None:
        self.data = open(path, "rb").read()
        (num_tables,) = struct.unpack_from(">H", self.data, 4)
        self.tables: dict[str, tuple[int, int]] = {}
        for i in range(num_tables):
            tag, _checksum, offset, length = struct.unpack_from(">4sIII", self.data, 12 + i * 16)
            self.tables[tag.decode("latin-1")] = (offset, length)

        head = self._table("head")
        (units_per_em,) = struct.unpack_from(">H", self.data, head + 18)
        (index_to_loc_format,) = struct.unpack_from(">h", self.data, head + 50)
        self.units_per_em = units_per_em
        self.loc_format = index_to_loc_format

        maxp = self._table("maxp")
        (self.num_glyphs,) = struct.unpack_from(">H", self.data, maxp + 4)

        hhea = self._table("hhea")
        self.ascent, self.descent = struct.unpack_from(">hh", self.data, hhea + 4)
        (self.num_hmetrics,) = struct.unpack_from(">H", self.data, hhea + 34)

        self.cmap = self._read_cmap()
        self.loca = self._read_loca()
        self.advances = self._read_hmtx()

    def _table(self, tag: str) -> int:
        return self.tables[tag][0]

    def _read_cmap(self) -> dict[int, int]:
        base = self._table("cmap")
        (count,) = struct.unpack_from(">H", self.data, base + 2)
        best = None
        for i in range(count):
            platform, encoding, offset = struct.unpack_from(">HHI", self.data, base + 4 + i * 8)
            if (platform, encoding) in ((3, 1), (0, 3), (0, 4)):
                best = base + offset
        if best is None:
            raise SystemExit("no unicode cmap subtable")
        (fmt,) = struct.unpack_from(">H", self.data, best)
        if fmt != 4:
            raise SystemExit(f"unsupported cmap format {fmt}")
        (seg_x2,) = struct.unpack_from(">H", self.data, best + 6)
        segs = seg_x2 // 2
        ends = struct.unpack_from(f">{segs}H", self.data, best + 14)
        starts = struct.unpack_from(f">{segs}H", self.data, best + 16 + seg_x2)
        deltas = struct.unpack_from(f">{segs}h", self.data, best + 16 + seg_x2 * 2)
        range_offset_pos = best + 16 + seg_x2 * 3
        range_offsets = struct.unpack_from(f">{segs}H", self.data, range_offset_pos)
        mapping: dict[int, int] = {}
        for i in range(segs):
            for code in range(starts[i], ends[i] + 1):
                if code == 0xFFFF:
                    continue
                if range_offsets[i] == 0:
                    gid = (code + deltas[i]) & 0xFFFF
                else:
                    pos = range_offset_pos + i * 2 + range_offsets[i] + (code - starts[i]) * 2
                    (gid,) = struct.unpack_from(">H", self.data, pos)
                    if gid:
                        gid = (gid + deltas[i]) & 0xFFFF
                if gid:
                    mapping[code] = gid
        return mapping

    def _read_loca(self) -> list[int]:
        base = self._table("loca")
        if self.loc_format == 0:
            values = struct.unpack_from(f">{self.num_glyphs + 1}H", self.data, base)
            return [v * 2 for v in values]
        return list(struct.unpack_from(f">{self.num_glyphs + 1}I", self.data, base))

    def _read_hmtx(self) -> list[int]:
        base = self._table("hmtx")
        advances = []
        for i in range(self.num_glyphs):
            index = min(i, self.num_hmetrics - 1)
            (advance,) = struct.unpack_from(">H", self.data, base + index * 4)
            advances.append(advance)
        return advances

    def glyph(self, gid: int) -> Glyph | None:
        if gid >= self.num_glyphs:
            return None
        start = self._table("glyf") + self.loca[gid]
        end = self._table("glyf") + self.loca[gid + 1]
        if end <= start:
            return Glyph([], [])
        (num_contours,) = struct.unpack_from(">h", self.data, start)
        if num_contours < 0:
            return None  # composite glyph: not needed for ASCII in a mono face
        pos = start + 10
        ends = struct.unpack_from(f">{num_contours}H", self.data, pos)
        pos += num_contours * 2
        (instruction_len,) = struct.unpack_from(">H", self.data, pos)
        pos += 2 + instruction_len
        point_count = ends[-1] + 1 if ends else 0

        flags: list[int] = []
        while len(flags) < point_count:
            (flag,) = struct.unpack_from(">B", self.data, pos)
            pos += 1
            flags.append(flag)
            if flag & 0x08:
                (repeat,) = struct.unpack_from(">B", self.data, pos)
                pos += 1
                flags.extend([flag] * repeat)
        flags = flags[:point_count]

        xs: list[int] = []
        value = 0
        for flag in flags:
            if flag & 0x02:
                (delta,) = struct.unpack_from(">B", self.data, pos)
                pos += 1
                value += delta if flag & 0x10 else -delta
            elif not flag & 0x10:
                (delta,) = struct.unpack_from(">h", self.data, pos)
                pos += 2
                value += delta
            xs.append(value)

        ys: list[int] = []
        value = 0
        for flag in flags:
            if flag & 0x04:
                (delta,) = struct.unpack_from(">B", self.data, pos)
                pos += 1
                value += delta if flag & 0x20 else -delta
            elif not flag & 0x20:
                (delta,) = struct.unpack_from(">h", self.data, pos)
                pos += 2
                value += delta
            ys.append(value)

        points = [(xs[i], ys[i], bool(flags[i] & 0x01)) for i in range(point_count)]
        return Glyph(points, list(ends))


def flatten(glyph: Glyph, scale: float) -> list[list[tuple[float, float]]]:
    """Convert TrueType quadratic contours into polylines in pixel space."""
    contours: list[list[tuple[float, float]]] = []
    begin = 0
    for end in glyph.contours:
        pts = glyph.points[begin : end + 1]
        begin = end + 1
        if not pts:
            continue
        on_curve = [(p[0] * scale, -p[1] * scale, p[2]) for p in pts]
        # Rotate so the list starts on an on-curve point; if there is none,
        # synthesise one midway between the last and first off-curve points.
        start_index = next((i for i, p in enumerate(on_curve) if p[2]), None)
        if start_index is None:
            first, last = on_curve[0], on_curve[-1]
            on_curve.insert(0, ((first[0] + last[0]) / 2, (first[1] + last[1]) / 2, True))
            start_index = 0
        points = on_curve[start_index:] + on_curve[:start_index]

        polyline: list[tuple[float, float]] = [(points[0][0], points[0][1])]
        cursor = (points[0][0], points[0][1])
        control: tuple[float, float] | None = None
        for x, y, on in points[1:] + [points[0]]:
            if on:
                if control is None:
                    polyline.append((x, y))
                else:
                    polyline.extend(quad(cursor, control, (x, y)))
                    control = None
                cursor = (x, y)
            else:
                if control is not None:
                    mid = ((control[0] + x) / 2, (control[1] + y) / 2)
                    polyline.extend(quad(cursor, control, mid))
                    cursor = mid
                control = (x, y)
        contours.append(polyline)
    return contours


def quad(
    p0: tuple[float, float],
    p1: tuple[float, float],
    p2: tuple[float, float],
    steps: int = 8,
) -> list[tuple[float, float]]:
    out = []
    for i in range(1, steps + 1):
        t = i / steps
        a = (1 - t) ** 2
        b = 2 * (1 - t) * t
        c = t * t
        out.append((a * p0[0] + b * p1[0] + c * p2[0], a * p0[1] + b * p1[1] + c * p2[1]))
    return out


def coverage(contours: list[list[tuple[float, float]]], width: int, height: int) -> list[list[int]]:
    """Supersampled nonzero-winding coverage for one cell."""
    grid = [[0] * width for _ in range(height)]
    if not contours:
        return grid
    step = 1.0 / SUPERSAMPLE
    offset = step / 2
    for py in range(height):
        for px in range(width):
            hits = 0
            for sy in range(SUPERSAMPLE):
                y = py + offset + sy * step
                for sx in range(SUPERSAMPLE):
                    x = px + offset + sx * step
                    winding = 0
                    for poly in contours:
                        count = len(poly)
                        for i in range(count):
                            x0, y0 = poly[i]
                            x1, y1 = poly[(i + 1) % count]
                            if y0 <= y < y1:
                                if (x1 - x0) * (y - y0) - (x - x0) * (y1 - y0) > 0:
                                    winding += 1
                            elif y1 <= y < y0:
                                if (x1 - x0) * (y - y0) - (x - x0) * (y1 - y0) < 0:
                                    winding -= 1
                    if winding != 0:
                        hits += 1
            grid[py][px] = round(255 * hits / (SUPERSAMPLE * SUPERSAMPLE))
    return grid


def main() -> None:
    path = sys.argv[1] if len(sys.argv) > 1 else "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"
    font = TrueType(path)
    scale = PIXEL_SIZE / font.units_per_em
    # Place the baseline so ascenders fit in the cell.
    ascent_px = round(font.ascent * scale)
    baseline = min(ascent_px, CELL_H - 2)

    rows: list[str] = []
    advances: list[int] = []
    codes: list[int] = []
    for code in CHARSET:
        gid = font.cmap.get(code)
        advance = font.advances[gid] if gid is not None else font.units_per_em // 2
        advances.append(round(advance * scale))
        codes.append(code)
        glyph = font.glyph(gid) if gid is not None else None
        if glyph is None:
            rows.append(",".join(["0"] * (CELL_W * CELL_H)))
            continue
        contours = flatten(glyph, scale)
        shifted = [[(x, y + baseline) for x, y in poly] for poly in contours]
        grid = coverage(shifted, CELL_W, CELL_H)
        flat = [v for row in grid for v in row]
        rows.append(",".join(str(v) for v in flat))

    cell = CELL_W * CELL_H
    out = [
        "//! Bitmap font baked by `tools/bake_font.py` — do not edit by hand.",
        "//!",
        f"//! Source face: {path.rsplit('/', 1)[-1]} (DejaVu family, permissive",
        "//! Bitstream Vera derived licence). Only raster output is embedded here.",
        "",
        "/// Width of one glyph cell, in pixels.",
        f"pub const CELL_WIDTH: u16 = {CELL_W};",
        "",
        "/// Height of one glyph cell, in pixels.",
        f"pub const CELL_HEIGHT: u16 = {CELL_H};",
        "",
        "/// Advance of every glyph, in pixels (the source face is monospaced).",
        f"pub const ADVANCE: u16 = {max(advances)};",
        "",
        "/// Baseline offset inside a cell, in pixels.",
        f"pub const BASELINE: u16 = {baseline};",
        "",
        "/// Bytes of coverage per glyph.",
        f"pub const CELL_BYTES: usize = {cell};",
        "",
        "/// Code points covered by [`COVERAGE`], in ascending order.",
        f"pub static CODES: [u32; {len(codes)}] = [",
    ]
    line = "    "
    for code in codes:
        line += f"0x{code:04X}, "
        if len(line) > 92:
            out.append(line.rstrip())
            line = "    "
    if line.strip():
        out.append(line.rstrip())
    out.append("];")
    out.append("")
    out.append("/// 8-bit coverage, one cell per entry of [`CODES`], row-major.")
    out.append(f"pub static COVERAGE: [[u8; CELL_BYTES]; {len(codes)}] = [")
    for code, row in zip(codes, rows):
        out.append(f"    [{row}], // U+{code:04X}")
    out.append("];")
    out.append("")
    out.append("/// Coverage for one character, or `None` when the font has no glyph.")
    out.append("#[must_use]")
    out.append("pub fn coverage(ch: char) -> Option<&'static [u8; CELL_BYTES]> {")
    out.append("    let code = u32::from(ch);")
    out.append("    let index = CODES.binary_search(&code).ok()?;")
    out.append("    COVERAGE.get(index)")
    out.append("}")
    out.append("")
    open("src/font.rs", "w").write("\n".join(out) + "\n")
    missing = [c for c in CHARSET if c not in font.cmap]
    if missing:
        print(f"warning: no glyph for {[hex(c) for c in missing]}", file=sys.stderr)
    print(f"wrote src/font.rs: {len(codes)} glyphs, {CELL_W}x{CELL_H}, baseline {baseline}")


if __name__ == "__main__":
    main()
