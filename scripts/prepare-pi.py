#!/usr/bin/env python3
"""Prepare a verified, target-specific standalone Pi payload for go:embed."""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import subprocess
import tarfile
import time
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parent.parent


def digest(path):
    checksum = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            checksum.update(chunk)
    return checksum.hexdigest()


def download(spec, cache):
    dest = cache / spec["url"].rsplit("/", 1)[1]
    if dest.exists() and digest(dest) == spec["sha256"]:
        return dest
    partial = dest.with_suffix(dest.suffix + ".part")
    for attempt in range(8):
        start = partial.stat().st_size if partial.exists() else 0
        headers = {"User-Agent": "MyWebScrcpy-build"}
        if start:
            headers["Range"] = f"bytes={start}-"
        print(f"[Pi] 下载 {dest.name}，已接收 {start} bytes", flush=True)
        try:
            with urllib.request.urlopen(urllib.request.Request(spec["url"], headers=headers), timeout=60) as response:
                if start and response.status != 206:
                    start = 0  # Server ignored Range: replace rather than append.
                if start and not response.headers.get("Content-Range", "").startswith(f"bytes {start}-"):
                    raise RuntimeError("下载服务返回了错误的字节范围")
                length = response.headers.get("Content-Length")
                with partial.open("ab" if start else "wb") as output:
                    shutil.copyfileobj(response, output)
                if length and partial.stat().st_size != start + int(length):
                    raise RuntimeError(f"依赖下载未完成: {dest.name}")
            if digest(partial) != spec["sha256"]:
                partial.unlink()
                raise RuntimeError(f"SHA-256 校验失败: {dest.name}")
            partial.replace(dest)
            return dest
        except Exception as error:
            if attempt == 7:
                raise RuntimeError(f"无法取得锁定依赖 {dest.name}: {error}") from error
            time.sleep(min(attempt + 1, 5))
    raise RuntimeError("下载失败")


def members(archive):
    if archive.name.endswith(".zip"):
        with zipfile.ZipFile(archive) as source:
            for entry in source.infolist():
                if not entry.is_dir():
                    yield entry.filename, source.read(entry), (entry.external_attr >> 16) & 0o777
    else:
        with tarfile.open(archive, "r:gz") as source:
            for entry in source:
                if entry.isfile():
                    yield entry.name, source.extractfile(entry).read(), entry.mode
                elif not entry.isdir():
                    raise RuntimeError(f"上游包包含非普通文件: {entry.name}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goos", default=os.environ.get("GOOS") or subprocess.check_output(["go", "env", "GOHOSTOS"], text=True).strip())
    parser.add_argument("--goarch", default=os.environ.get("GOARCH") or subprocess.check_output(["go", "env", "GOHOSTARCH"], text=True).strip())
    parser.add_argument("--cache", type=Path, default=ROOT / ".cache" / "pi")
    args = parser.parse_args()
    lock = json.loads((ROOT / "scripts/pi-runtime.lock.json").read_text())
    target = f"{args.goos}/{args.goarch}"
    if target not in lock["targets"]:
        parser.error(f"没有锁定的 Pi 独立包: {target}")
    args.cache.mkdir(parents=True, exist_ok=True)
    payload = ROOT / "internal/pibundle/payload"
    output = payload / "runtime.zip"
    temporary = payload / "runtime.zip.tmp"
    suffix = ".exe" if args.goos == "windows" else ""
    entries = {}
    with zipfile.ZipFile(temporary, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=6) as bundle:
        def write(name, data, mode):
            path = PurePosixPath(name)
            if path.is_absolute() or ".." in path.parts or "\\" in name or name in entries:
                raise RuntimeError(f"无效或重复的依赖路径: {name}")
            info = zipfile.ZipInfo(name, (2026, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = (0o100000 | mode) << 16
            bundle.writestr(info, data)
            entries[name] = {"sha256": hashlib.sha256(data).hexdigest(), "mode": mode}

        for name, spec in lock["targets"][target].items():
            archive = download(spec, args.cache)
            for path, data, mode in members(archive):
                if name == "pi":
                    # Official Unix archives have pi/; Windows archives start at root.
                    relative = path if suffix else path.removeprefix("pi/")
                    write(relative, data, 0o755 if relative == "pi" + suffix else 0o644)
                elif PurePosixPath(path).name == name + suffix:
                    write(f"bin/{name}{suffix}", data, 0o755)
                else:
                    write(f"third-party/{name}/{path}", data, 0o644)
        write("third-party/PI-LICENSE", (ROOT / "internal/pibundle/PI-LICENSE").read_bytes(), 0o644)
        for required in ["pi" + suffix, "package.json", "bin/fd" + suffix, "bin/rg" + suffix]:
            if required not in entries:
                raise RuntimeError(f"上游依赖缺少必需文件: {required}")
    manifest = {"version": lock["version"], "goos": args.goos, "goarch": args.goarch,
                "sha256": digest(temporary), "files": entries}
    temporary.replace(output)
    (payload / "manifest.json").write_text(json.dumps(manifest, sort_keys=True, indent=2) + "\n")
    print(f"[Pi] 已准备 {target} Pi {lock['version']}，{output.stat().st_size / 1024 / 1024:.1f} MiB")


if __name__ == "__main__":
    main()
