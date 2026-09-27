#!/usr/bin/env python3
"""Render the installer's terminal UI to PNG screenshots for the README.

Runs each scene of tools/screenshots under a pseudo-terminal (so the UI takes
its full-colour path), replays the output through a terminal emulator (pyte)
and screenshots the final screen with headless Chromium (Playwright).

    pip install pyte playwright
    python3 tools/screenshots/render.py docs/images

Set CHROMIUM to a Chromium binary if Playwright can't find one, and VERSION
to the installer version shown in the banner (default 2.2.0).
"""
import html
import os
import subprocess
import sys
import tempfile

import pyte
from playwright.sync_api import sync_playwright

COLS = 104
# scene: (rows, seconds to run before capturing; None runs to the end).
# "progress" catches the install scene mid-download to show the live bar.
SCENES = {"start": (44, None), "review": (20, None), "progress": (12, 1.3),
          "install": (30, None), "finish": (16, None)}

# Terminal palette (Catppuccin-like dark theme) for the 16 ANSI colours.
PALETTE = {
    "default": "#cdd6f4", "black": "#45475a", "red": "#f38ba8", "green": "#a6e3a1",
    "brown": "#f9e2af", "yellow": "#f9e2af", "blue": "#89b4fa", "magenta": "#f5c2e7",
    "cyan": "#94e2d5", "white": "#bac2de", "brightblack": "#7f849c",
}
BG = "#1e1e2e"


def colour(value, default):
    if value == "default":
        return default
    if value in PALETTE:
        return PALETTE[value]
    if len(value) == 6 and all(c in "0123456789abcdefABCDEF" for c in value):
        return "#" + value
    return default


def capture(binary, scene, rows, stop):
    run = "install" if scene == "progress" else scene
    cmd = f"stty cols {COLS} rows {rows}; {binary} {run}"
    if stop:
        cmd = f"stty cols {COLS} rows {rows}; timeout {stop} {binary} {run}; true"
    env = dict(os.environ, COLORTERM="truecolor", TERM="xterm-256color")
    out = subprocess.run(["script", "-qec", cmd, "/dev/null"], env=env,
                         stdout=subprocess.PIPE, check=True).stdout
    screen = pyte.Screen(COLS, rows)
    stream = pyte.ByteStream(screen)
    stream.feed(out)
    return screen


def to_html(screen):
    lines = []
    last = max((y for y in range(screen.lines)
                if "".join(screen.buffer[y][x].data for x in range(screen.columns)).strip()), default=0)
    for y in range(last + 1):
        row = screen.buffer[y]
        cells = []
        for x in range(screen.columns):
            ch = row[x]
            fg = colour(ch.fg, PALETTE["default"])
            bg = colour(ch.bg, BG)
            if ch.bold and ch.fg in PALETTE and ch.fg not in ("default", "black"):
                fg = PALETTE[ch.fg]
            if ch.bold and ch.fg == "black":
                fg = PALETTE["brightblack"]
            if ch.data == "█":
                cells.append(f'<span class="c" style="background:{fg}"> </span>')
                continue
            if ch.data == "▀":
                # Draw half blocks as two exact halves so they tile without gaps.
                cells.append(f'<span class="c" style="background:linear-gradient({fg} 50%,{bg} 50%)"> </span>')
                continue
            style = f"color:{fg}"
            if bg != BG:
                style += f";background:{bg}"
            if ch.bold:
                style += ";font-weight:700"
            cells.append(f'<span class="c" style="{style}">{html.escape(ch.data)}</span>')
        lines.append('<div class="l">' + "".join(cells) + "</div>")
    return f"""<!doctype html><meta charset="utf-8"><style>
body{{margin:0;background:transparent;padding:24px}}
.w{{display:inline-block;background:{BG};border-radius:10px;box-shadow:0 12px 40px #0008;overflow:hidden}}
.t{{height:34px;background:#181825;display:flex;align-items:center;padding:0 14px;gap:8px}}
.d{{width:12px;height:12px;border-radius:50%}}
.n{{flex:1;text-align:center;color:#7f849c;font:13px sans-serif;margin-right:52px}}
.s{{padding:14px 18px 18px;font:15px/1 'DejaVu Sans Mono',monospace}}
.l{{height:19px;white-space:pre;display:flex}}
.c{{display:inline-block;width:9.04px;height:19px;line-height:19px;text-align:center}}
</style><div class="w"><div class="t"><span class="d" style="background:#f38ba8"></span><span class="d" style="background:#f9e2af"></span><span class="d" style="background:#a6e3a1"></span><span class="n">player@linux: ~</span></div><div class="s">{''.join(lines)}</div></div>"""


def main():
    out_dir = sys.argv[1] if len(sys.argv) > 1 else "docs/images"
    os.makedirs(out_dir, exist_ok=True)
    version = os.environ.get("VERSION", "2.2.0")
    binary = os.path.join(tempfile.mkdtemp(), "screenshots")
    subprocess.run(["go", "build", "-o", binary, "-ldflags",
                    f"-X bellum-installer/pkg/config.InstallerVersion={version}",
                    "./tools/screenshots"], check=True)
    with sync_playwright() as p:
        browser = p.chromium.launch(executable_path=os.environ.get("CHROMIUM") or None)
        page = browser.new_page(device_scale_factor=2)
        for scene, (rows, stop) in SCENES.items():
            page.set_content(to_html(capture(binary, scene, rows, stop)))
            path = os.path.join(out_dir, f"install-{scene}.png")
            page.locator(".w").screenshot(path=path, omit_background=True)
            print(path)
        browser.close()


if __name__ == "__main__":
    main()
