#!/usr/bin/env python3
"""Fake X11 server used to verify `conduit-tui`'s wire-protocol client.

The sandbox has no X server at all (no `DISPLAY`, no `Xvfb`), so the only honest
way to test the client is to speak the protocol back at it. This script:

1. binds a Unix socket at `$FAKE_X_SOCKET` (default `/tmp/.X11-unix/X9`);
2. answers the setup handshake with a plausible screen;
3. records every request the client sends;
4. replays `Expose`, `ButtonPress`, and `ClientMessage(WM_DELETE_WINDOW)`;
5. writes a JSON report (`$FAKE_X_REPORT`, default `/tmp/fake-x-report.json`)
   describing the handshake, the request log, and the pixel payload size.

Exit status is 0 only when the client behaved: setup announced little-endian,
`PutImage` arrived with the expected byte count, and the button press landed
inside the window.
"""

from __future__ import annotations

import json
import os
import struct
import sys
import time


WINDOW_W = 420
WINDOW_H = 560


def u16(value: int) -> bytes:
    return struct.pack("<H", value)


def u32(value: int) -> bytes:
    return struct.pack("<I", value)


def build_setup_reply() -> bytes:
    """A setup reply with one 1920x1080 screen, depth 24, TrueColor."""
    body = bytearray()
    body += b"\x00" * 4  # release number
    body += u32(0x0040_0000)  # resource id base
    body += u32(0x003F_FFFF)  # resource id mask
    body += u32(0)  # motion buffer
    body += u16(0)  # vendor length
    body += u16(65535)  # max request length, words
    body += bytes([1])  # one screen
    body += bytes([1])  # one pixmap format
    body += bytes([0])  # image byte order: LSB
    body += bytes([0])  # bitmap bit order
    body += bytes([32])  # scanline unit
    body += bytes([32])  # scanline pad
    body += bytes([8])  # min keycode
    body += bytes([255])  # max keycode
    body += b"\x00" * 4  # unused
    assert len(body) == 32

    body += bytes([24, 32, 0, 0, 0, 0, 0, 0])  # ZPixmap depth 24

    # Screen.
    body += u32(0x100)  # root
    body += u32(0x20)  # default colormap
    body += u32(0xFFFFFF)  # white
    body += u32(0)  # black
    body += u32(0)  # input masks
    body += u16(1920)
    body += u16(1080)
    body += u16(508)
    body += u16(285)
    body += u16(1)
    body += u16(1)
    body += u32(0x21)  # root visual
    body += bytes([0, 0, 24, 1])  # backing, save-unders, depth, depths
    assert len(body) == 80

    # Depth 24 → one TrueColor visual.
    body += bytes([24, 0])
    body += u16(1)
    body += b"\x00" * 4
    body += u32(0x21)
    body += bytes([4, 8])
    body += u16(256)
    body += u32(0x00FF_0000)
    body += u32(0x0000_FF00)
    body += u32(0x0000_00FF)
    body += u32(0)

    header = bytes([1, 0]) + u16(11) + u16(0) + u16(len(body) // 4)
    return header + bytes(body)


def read_request(stream) -> tuple[int, bytes]:
    head = stream.recv(4)
    if len(head) < 4:
        raise EOFError("client closed the connection")
    opcode = head[0]
    words = u16_value(head[2:4])
    rest = b""
    while len(rest) < (words - 1) * 4:
        chunk = stream.recv((words - 1) * 4 - len(rest))
        if not chunk:
            raise EOFError("client closed mid-request")
        rest += chunk
    return opcode, rest


def u16_value(raw: bytes) -> int:
    return struct.unpack("<H", raw)[0]


def event(kind: int, detail: int, sequence: int, **fields) -> bytes:
    raw = bytearray(32)
    raw[0] = kind
    raw[1] = detail
    struct.pack_into("<H", raw, 2, sequence)
    for offset, value, width in fields.get("words", []):
        struct.pack_into("<H", raw, offset, value & 0xFFFF) if width == 2 else struct.pack_into(
            "<I", raw, offset, value & 0xFFFF_FFFF
        )
    return bytes(raw)


def main() -> int:
    path = os.environ.get("FAKE_X_SOCKET", "/tmp/.X11-unix/X9")
    report_path = os.environ.get("FAKE_X_REPORT", "/tmp/fake-x-report.json")
    os.makedirs(os.path.dirname(path), exist_ok=True)
    if os.path.exists(path):
        os.unlink(path)

    import socket

    server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    server.bind(path)
    server.listen(1)
    server.settimeout(20.0)

    report: dict = {"socket": path, "requests": [], "ok": False, "errors": []}
    print(f"fake X server listening on {path}", flush=True)

    conn, _ = server.accept()
    conn.settimeout(20.0)

    setup = b""
    while len(setup) < 12:
        chunk = conn.recv(12 - len(setup))
        if not chunk:
            report["errors"].append("client closed before setup")
            break
        setup += chunk
    report["setup"] = {
        "byte_order": "little" if setup[:1] == b"l" else "big",
        "major": u16_value(setup[2:4]),
        "minor": u16_value(setup[4:6]),
    }
    if setup[:1] != b"l":
        report["errors"].append("client did not announce little-endian")
    conn.sendall(build_setup_reply())
    print("handshake completed", flush=True)

    conn.sendall(event(12, 0, 1, words=[(4, 0x100, 4), (8, WINDOW_W, 2), (10, WINDOW_H, 2),
                                        (16, WINDOW_W, 2), (18, WINDOW_H, 2)]))
    time.sleep(0.15)
    conn.sendall(event(4, 1, 2, words=[(4, 0x100, 4), (12, 200, 2), (14, 300, 2),
                                       (24, 200, 2), (26, 300, 2)]))

    deadline = time.time() + 12.0
    images = 0
    pending: list[dict] = []
    expect_close = False
    frames_done = False
    while time.time() < deadline and not expect_close:
        try:
            opcode, body = read_request(conn)
        except (EOFError, OSError):
            break
        entry = {"opcode": opcode, "bytes": len(body) + 4}
        if opcode == 16:  # InternAtom → needs a reply
            name_len = u16_value(body[0:2])
            entry["atom"] = body[4:4 + name_len].decode("latin-1")
            reply = bytearray(32)
            reply[0] = 1
            struct.pack_into("<H", reply, 2, 1)
            struct.pack_into("<I", reply, 4, 0)
            struct.pack_into("<I", reply, 8, 0x100 + len(report["requests"]))
            conn.sendall(bytes(reply))
        elif opcode == 1:  # CreateWindow
            entry["width"] = u16_value(body[16:18])
            entry["height"] = u16_value(body[18:20])
            entry["depth"] = body[0]
            window_id = struct.unpack("<I", body[4:8])[0]
            entry["window"] = window_id
        elif opcode == 55:  # CreateGC
            entry["gc"] = struct.unpack("<I", body[0:4])[0]
        elif opcode == 72:  # PutImage
            width = u16_value(body[8:10])
            height = u16_value(body[10:12])
            # body: window, gc, width, height, dst-x, dst-y, left-pad, depth
            dst_y = struct.unpack("<h", body[14:16])[0]
            payload = len(body) - 20
            entry["image"] = {
                "width": width,
                "height": height,
                "dst_y": dst_y,
                "payload": payload,
            }
            images += 1
            pending.append(entry["image"])
            complete = dst_y + height >= WINDOW_H
            if complete and not frames_done:
                frames_done = True
                # Ask for one more frame, then simulate the window manager
                # asking the client to close.
                time.sleep(0.2)
                conn.sendall(event(12, 0, 2))
                time.sleep(0.3)
                expect_close = True
                conn.sendall(event(33, 32, 3, words=[(4, 0x100, 4)]))
                report["close_requested"] = True
        report["requests"].append(entry)

    if report.get("close_requested"):
        # Give the client a moment to act on the close message, then drain until
        # it hangs up rather than closing the socket underneath it.
        conn.settimeout(2.0)
        drained = 0
        try:
            while True:
                chunk = conn.recv(4096)
                if not chunk:
                    break
                drained += len(chunk)
        except OSError:
            pass
        report["post_close_bytes"] = drained

    report["images"] = images
    try:
        conn.close()
    except OSError:
        pass
    server.close()
    os.unlink(path)

    ops = [r["opcode"] for r in report["requests"]]
    if not any(o == 1 for o in ops):
        report["errors"].append("no CreateWindow")
    if images == 0:
        report["errors"].append("no PutImage")
    # Group PutImage requests into frames: a frame ends when the next band's
    # destination y is not below the previous one.
    frames: list[list[dict]] = []
    for image in pending:
        if frames and image["dst_y"] >= frames[-1][-1]["dst_y"] + 1 and not (
            frames[-1][-1]["dst_y"] + frames[-1][-1]["height"] <= image["dst_y"]
        ):
            frames.append([image])
        elif frames and image["dst_y"] == frames[-1][-1]["dst_y"] + frames[-1][-1]["height"]:
            frames[-1].append(image)
        else:
            frames.append([image])
    report["frames"] = [
        {
            "bands": len(bands),
            "covered_height": sum(b["height"] for b in bands),
            "max_payload": max(b["payload"] for b in bands),
        }
        for bands in frames
    ]
    for frame in frames:
        covered = sum(band["height"] for band in frame)
        if covered != WINDOW_H:
            report["errors"].append(f"frame covers {covered} rows, expected {WINDOW_H}")
        if frame[0]["dst_y"] != 0:
            report["errors"].append(f"frame starts at y={frame[0]['dst_y']}, expected 0")
        for band in frame:
            expected = band["width"] * band["height"] * 4
            if band["payload"] != expected:
                report["errors"].append(
                    f"band payload {band['payload']} != {expected}"
                )
            if band["width"] != WINDOW_W:
                report["errors"].append(f"band width {band['width']} unexpected")
    report["ok"] = not report["errors"]
    with open(report_path, "w") as handle:
        json.dump(report, handle, indent=2)
    print(json.dumps({"ok": report["ok"], "images": images, "errors": report["errors"]}), flush=True)
    return 0 if report["ok"] else 1


if __name__ == "__main__":
    sys.exit(main())
