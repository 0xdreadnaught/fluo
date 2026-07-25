"""winshot.py -- window-bound screenshot for validating fluo rendering.

Captures a single window's client area via PrintWindow (PW_RENDERFULLCONTENT),
NOT a display grab. Falls back to a screen-DC copy clipped to the window's
client rect only if PrintWindow yields an all-black image (some GL swapchains).
Pure stdlib (ctypes + zlib PNG writer) -- no pip dependencies.

Usage:
    python tools/winshot.py --list                 # list visible window titles
    python tools/winshot.py "title substring" out.png
"""
import ctypes
import ctypes.wintypes as wt
import struct
import sys
import zlib

u32 = ctypes.windll.user32
g32 = ctypes.windll.gdi32

# Per-monitor DPI awareness so rects are in physical pixels.
try:
    u32.SetProcessDpiAwarenessContext(ctypes.c_void_p(-4))
except Exception:
    u32.SetProcessDPIAware()

SRCCOPY = 0x00CC0020
PW_RENDERFULLCONTENT = 2
BI_RGB = 0
DIB_RGB_COLORS = 0


class BITMAPINFOHEADER(ctypes.Structure):
    _fields_ = [
        ("biSize", wt.DWORD), ("biWidth", wt.LONG), ("biHeight", wt.LONG),
        ("biPlanes", wt.WORD), ("biBitCount", wt.WORD),
        ("biCompression", wt.DWORD), ("biSizeImage", wt.DWORD),
        ("biXPelsPerMeter", wt.LONG), ("biYPelsPerMeter", wt.LONG),
        ("biClrUsed", wt.DWORD), ("biClrImportant", wt.DWORD),
    ]


def list_windows(sub=None):
    out = []

    @ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)
    def cb(hwnd, _):
        if u32.IsWindowVisible(hwnd):
            n = u32.GetWindowTextLengthW(hwnd)
            if n:
                buf = ctypes.create_unicode_buffer(n + 1)
                u32.GetWindowTextW(hwnd, buf, n + 1)
                out.append((hwnd, buf.value))
        return True

    u32.EnumWindows(cb, 0)
    if sub is None:
        return out
    return [(h, t) for h, t in out if sub.lower() in t.lower()]


def _dib_pixels(dc, bmp, w, h):
    """Read a 32-bit top-down BGRA pixel buffer out of a GDI bitmap."""
    bi = BITMAPINFOHEADER()
    bi.biSize = ctypes.sizeof(BITMAPINFOHEADER)
    bi.biWidth, bi.biHeight = w, -h  # negative = top-down rows
    bi.biPlanes, bi.biBitCount, bi.biCompression = 1, 32, BI_RGB
    buf = ctypes.create_string_buffer(w * h * 4)
    got = g32.GetDIBits(dc, bmp, 0, h, buf, ctypes.byref(bi), DIB_RGB_COLORS)
    if got != h:
        raise OSError(f"GetDIBits returned {got}, want {h}")
    return bytearray(buf.raw)


def capture(hwnd):
    """Returns (w, h, rgba_bytes) of the window's client area."""
    wr, cr, origin = wt.RECT(), wt.RECT(), wt.POINT(0, 0)
    u32.GetWindowRect(hwnd, ctypes.byref(wr))
    u32.GetClientRect(hwnd, ctypes.byref(cr))
    u32.ClientToScreen(hwnd, ctypes.byref(origin))
    ww, wh = wr.right - wr.left, wr.bottom - wr.top
    cw, ch = cr.right, cr.bottom
    ox, oy = origin.x - wr.left, origin.y - wr.top
    if cw <= 0 or ch <= 0:
        raise OSError("window has no client area (minimized?)")

    wdc = u32.GetWindowDC(hwnd)
    mdc = g32.CreateCompatibleDC(wdc)
    bmp = g32.CreateCompatibleBitmap(wdc, ww, wh)
    g32.SelectObject(mdc, bmp)
    ok = u32.PrintWindow(hwnd, mdc, PW_RENDERFULLCONTENT)
    px = _dib_pixels(mdc, bmp, ww, wh) if ok else None
    g32.DeleteObject(bmp)
    g32.DeleteDC(mdc)
    u32.ReleaseDC(hwnd, wdc)

    def crop_client(pixels, stride_w):
        out = bytearray(cw * ch * 4)
        for y in range(ch):
            s = ((y + oy) * stride_w + ox) * 4
            d = y * cw * 4
            out[d:d + cw * 4] = pixels[s:s + cw * 4]
        return out

    client = crop_client(px, ww) if px else None
    if client is None or not any(client):  # PrintWindow failed or all-black GL surface
        sdc = u32.GetDC(0)
        mdc = g32.CreateCompatibleDC(sdc)
        bmp = g32.CreateCompatibleBitmap(sdc, cw, ch)
        g32.SelectObject(mdc, bmp)
        g32.BitBlt(mdc, 0, 0, cw, ch, sdc, origin.x, origin.y, SRCCOPY)
        client = _dib_pixels(mdc, bmp, cw, ch)
        g32.DeleteObject(bmp)
        g32.DeleteDC(mdc)
        u32.ReleaseDC(0, sdc)

    # BGRA -> RGBA, force opaque alpha (GDI alpha is unreliable)
    for i in range(0, len(client), 4):
        client[i], client[i + 2] = client[i + 2], client[i]
        client[i + 3] = 255
    return cw, ch, bytes(client)


def write_png(path, w, h, rgba):
    def chunk(tag, data):
        return (struct.pack(">I", len(data)) + tag + data
                + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF))

    raw = b"".join(b"\x00" + rgba[y * w * 4:(y + 1) * w * 4] for y in range(h))
    with open(path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n")
        f.write(chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 6, 0, 0, 0)))
        f.write(chunk(b"IDAT", zlib.compress(raw, 6)))
        f.write(chunk(b"IEND", b""))


def main():
    if len(sys.argv) == 2 and sys.argv[1] == "--list":
        for hwnd, title in list_windows():
            safe = title.encode(sys.stdout.encoding or "utf-8", "replace").decode(
                sys.stdout.encoding or "utf-8")
            print(f"{hwnd:>10}  {safe}")
        return 0
    if len(sys.argv) != 3:
        print(__doc__)
        return 2
    matches = list_windows(sys.argv[1])
    if not matches:
        print(f"no visible window matching {sys.argv[1]!r}", file=sys.stderr)
        return 1
    hwnd, title = matches[0]
    w, h, rgba = capture(hwnd)
    write_png(sys.argv[2], w, h, rgba)
    print(f"captured {title!r} client area {w}x{h} -> {sys.argv[2]}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
