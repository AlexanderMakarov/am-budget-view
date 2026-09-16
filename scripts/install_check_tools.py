#!/usr/bin/env python3
"""Install checksum-verified, pinned check tools locally (Python >= 3.12)."""

import hashlib
import json
import platform
import shutil
import tarfile
import tempfile
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
TOOLS = ROOT / ".tools"


def fetch(url):
    with urllib.request.urlopen(url, timeout=120) as response:
        return response.read()


def install(url, checksum, archive_root, destination):
    if destination.is_dir():
        return
    print(f"Installing {destination.name}...", flush=True)
    content = fetch(url)
    if hashlib.sha256(content).hexdigest() != checksum:
        raise RuntimeError(f"Checksum mismatch: {url}")
    with tempfile.TemporaryDirectory(dir=TOOLS) as temporary:
        archive = Path(temporary) / "download.tar.gz"
        archive.write_bytes(content)
        with tarfile.open(archive) as source:
            source.extractall(temporary, filter="data")
        shutil.move(str(Path(temporary) / archive_root), destination)


def main():
    system = platform.system().lower()
    arch = {"x86_64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(platform.machine())
    if system not in ("linux", "darwin") or not arch:
        raise SystemExit("Check-tool setup supports Linux/macOS amd64/arm64; use WSL on Windows.")
    TOOLS.mkdir(exist_ok=True)
    go = (ROOT / ".go-version").read_text().strip()
    lint = (ROOT / ".golangci-version").read_text().strip()
    go_dir = TOOLS / f"go-{go}"
    if not go_dir.is_dir():
        releases = json.loads(fetch("https://go.dev/dl/?mode=json&include=all"))
        filename = f"go{go}.{system}-{arch}.tar.gz"
        checksum = next(f["sha256"] for r in releases for f in r["files"]
                        if f["filename"] == filename)
        install(f"https://go.dev/dl/{filename}", checksum, "go", go_dir)
    lint_dir = TOOLS / f"golangci-lint-{lint}"
    if not lint_dir.is_dir():
        base = f"https://github.com/golangci/golangci-lint/releases/download/v{lint}"
        name = f"golangci-lint-{lint}-{system}-{arch}"
        checksums = fetch(f"{base}/golangci-lint-{lint}-checksums.txt").decode()
        checksum = next(line.split()[0] for line in checksums.splitlines()
                        if line.split()[-1] == f"{name}.tar.gz")
        install(f"{base}/{name}.tar.gz", checksum, name, lint_dir)
    print("Check tools installed in .tools/. Run make check.")


if __name__ == "__main__":
    main()
