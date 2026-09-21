#!/usr/bin/env python3
"""Record a short session with the real hood binary and write
docs/demo.gif and docs/demo.cast.

Needs Pillow and a built binary (make build). The session runs in a temp
directory with a fixed identity and fixed dates, so the output is the same
every time. Each command runs under a pty so hood prints its colors.
"""
import json
import os
import pty
import re
import select
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parent.parent
BINARY = ROOT / "bin" / "hood"
COLS, ROWS = 100, 30
FONT_PATH = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"
FONT_SIZE = 15
BG, FG = (24, 25, 38), (205, 214, 244)
COLORS = {
    30: (69, 71, 90), 31: (243, 139, 168), 32: (166, 227, 161), 33: (249, 226, 175),
    34: (137, 180, 250), 35: (203, 166, 247), 36: (148, 226, 213), 37: FG,
}

# (shell command, file edit to make before it, pause in seconds afterwards)
STEPS = [
    ("hood init", None, 1.2),
    ("hood config user.name 'Ada Lovelace'", None, 0.3),
    ("hood config user.email ada@example.com", None, 0.6),
    ("hood --xray add hello.txt", ("hello.txt", "hello\n"), 2.8),
    ("hood --xray commit -m 'first commit'", None, 3.2),
    ("hood switch -c feature", None, 1.6),
    ("hood --xray commit -am 'say hello twice'", ("hello.txt", "hello\nhello again\n"), 3.2),
    ("hood switch main", None, 1.4),
    ("hood add notes.txt", ("notes.txt", "remember this\n"), 0.6),
    ("hood commit -m 'add notes'", None, 1.2),
    ("hood --xray merge feature", None, 3.2),
    ("hood graph", None, 3.5),
]
ENV = {
    "GIT_AUTHOR_NAME": "Ada Lovelace", "GIT_AUTHOR_EMAIL": "ada@example.com",
    "GIT_COMMITTER_NAME": "Ada Lovelace", "GIT_COMMITTER_EMAIL": "ada@example.com",
}


def run_in_pty(cmd, cwd, env):
    """Run a shell command with a pty and return everything it printed."""
    master, slave = pty.openpty()
    proc = subprocess.Popen(["sh", "-c", cmd], cwd=cwd, env=env, stdin=slave, stdout=slave, stderr=slave)
    os.close(slave)
    out = b""
    while True:
        ready, _, _ = select.select([master], [], [], 0.2)
        if ready:
            try:
                chunk = os.read(master, 4096)
            except OSError:
                break
            if not chunk:
                break
            out += chunk
        elif proc.poll() is not None:
            break
    proc.wait()
    os.close(master)
    return out.decode().replace("\r\n", "\n")


SGR = re.compile(r"\x1b\[([0-9;]*)m")


def parse_ansi(text):
    """Turn text with SGR codes into lines of (char, color, bold, dim) cells."""
    lines, line = [], []
    color, bold, dim = FG, False, False
    pos = 0
    for m in SGR.finditer(text):
        for ch in text[pos:m.start()]:
            if ch == "\n":
                lines.append(line)
                line = []
            else:
                line.append((ch, color, bold, dim))
        pos = m.end()
        for code in (int(c or 0) for c in m.group(1).split(";")):
            if code == 0:
                color, bold, dim = FG, False, False
            elif code == 1:
                bold = True
            elif code == 2:
                dim = True
            elif code in COLORS:
                color = COLORS[code]
    for ch in text[pos:]:
        if ch == "\n":
            lines.append(line)
            line = []
        else:
            line.append((ch, color, bold, dim))
    if line:
        lines.append(line)
    return lines


class Screen:
    def __init__(self):
        self.font = ImageFont.truetype(FONT_PATH, FONT_SIZE)
        self.cw = int(self.font.getlength("M"))
        self.ch = FONT_SIZE + 5
        self.width, self.height = COLS * self.cw + 24, ROWS * self.ch + 24
        self.lines = []  # each line is a list of cells

    def add(self, cells_lines):
        self.lines.extend(cells_lines)
        self.lines = self.lines[-ROWS:]

    def frame(self, partial=None):
        img = Image.new("RGB", (self.width, self.height), BG)
        draw = ImageDraw.Draw(img)
        shown = self.lines[-(ROWS - (1 if partial is not None else 0)):]
        if partial is not None:
            shown = shown + [partial]
        for y, line in enumerate(shown):
            for x, (ch, color, bold, dim) in enumerate(line[:COLS]):
                c = tuple(int(v * 0.6) for v in color) if dim else color
                draw.text((12 + x * self.cw, 12 + y * self.ch), ch, font=self.font, fill=c)
        return img


def prompt_cells(cmd):
    green, white = COLORS[32], FG
    cells = [("$", green, True, False), (" ", white, False, False)]
    return cells + [(ch, white, False, False) for ch in cmd]


def main():
    if not BINARY.exists():
        sys.exit("build the binary first: make build")
    work = Path(tempfile.mkdtemp(prefix="hood-demo-"))
    env = dict(os.environ, **ENV, TERM="xterm-256color", HOOD_HOME=str(work / ".home"))
    env["PATH"] = f"{BINARY.parent}:{env['PATH']}"
    for k in ("NO_COLOR", "HOOD_XRAY"):
        env.pop(k, None)
    env["GIT_AUTHOR_DATE"] = env["GIT_COMMITTER_DATE"] = "1767225600 +0100"

    screen = Screen()
    frames, durations, cast = [], [], []
    clock = 0.0

    def add_frame(partial, ms):
        frames.append(screen.frame(partial))
        durations.append(ms)

    def cast_out(text, advance):
        nonlocal clock
        clock += advance
        cast.append([round(clock, 3), "o", text])

    add_frame(None, 600)
    for cmd, edit, pause in STEPS:
        if edit:
            (work / edit[0]).write_text(edit[1])
        typed = []
        for ch in cmd:
            typed.append(ch)
            add_frame(prompt_cells("".join(typed)) + [("█", FG, False, False)], 45)
            cast_out(ch if len(typed) > 1 else "$ " + ch, 0.045)
        output = run_in_pty(cmd, work, env)
        screen.add([prompt_cells(cmd)])
        screen.add(parse_ansi(output))
        cast_out("\r\n" + output.replace("\n", "\r\n"), 0.3)
        add_frame(None, int(pause * 1000))
        cast_out("", pause)

    out = ROOT / "docs"
    out.mkdir(exist_ok=True)
    frames[0].save(out / "demo.gif", save_all=True, append_images=frames[1:], duration=durations,
                   loop=0, optimize=True)
    header = {"version": 2, "width": COLS, "height": ROWS, "timestamp": 1767225600,
              "env": {"TERM": "xterm-256color", "SHELL": "/bin/sh"}, "title": "hood demo"}
    with open(out / "demo.cast", "w") as f:
        f.write(json.dumps(header) + "\n")
        for event in cast:
            f.write(json.dumps(event) + "\n")
    shutil.rmtree(work, ignore_errors=True)
    print("wrote", out / "demo.gif", "and", out / "demo.cast")


if __name__ == "__main__":
    main()
