#!/usr/bin/env python3
"""Publish the already-built GitHub release assets to a CNB Release."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

API_ENDPOINT = os.getenv("CNB_API_ENDPOINT", "https://api.cnb.cool").rstrip("/")
TOKEN = os.getenv("CNB_TOKEN", "").strip()
REPO = os.getenv("CNB_REPO_SLUG", "").strip().strip("/")
TARGET_BRANCH = os.getenv("CNB_TARGET_BRANCH", "").strip() or "main"
TAG = os.getenv("GITHUB_REF_NAME", "").strip()
GITHUB_REPOSITORY = os.getenv("GITHUB_REPOSITORY", "").strip()
RELEASE_DIR = Path(os.getenv("CNB_RELEASE_DIR", "release"))


class CNBError(RuntimeError):
    pass


def fail(message: str) -> None:
    raise CNBError(message)


def api_url(path: str) -> str:
    return f"{API_ENDPOINT}/{path.lstrip('/')}"


def api_request(
    method: str,
    url: str,
    payload: dict[str, object] | None = None,
    *,
    allow_not_found: bool = False,
) -> dict[str, object] | None:
    data = None
    headers = {
        "Accept": "application/json",
        "Authorization": f"Bearer {TOKEN}",
    }
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"

    request = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            body = response.read().decode("utf-8", errors="replace").strip()
            if not body:
                return None
            decoded = json.loads(body)
            if isinstance(decoded, dict):
                return decoded
            return {"data": decoded}
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace").strip()
        if allow_not_found and exc.code == 404:
            return None
        detail = f": {body}" if body else ""
        fail(f"CNB API {method} {url} failed with HTTP {exc.code}{detail}")
    except urllib.error.URLError as exc:
        fail(f"CNB API {method} {url} failed: {exc.reason}")


def get_or_create_release() -> str:
    repo_path = urllib.parse.quote(REPO, safe="/")
    tag_path = urllib.parse.quote(TAG, safe="")
    existing = api_request(
        "GET",
        api_url(f"{repo_path}/-/releases/tags/{tag_path}"),
        allow_not_found=True,
    )
    if existing:
        release_id = str(existing.get("id", "")).strip()
        if release_id:
            print(f"CNB Release {TAG} already exists: {release_id}")
            return release_id

    prerelease = "-" in TAG
    source_url = (
        f"https://github.com/{GITHUB_REPOSITORY}/releases/tag/{TAG}"
        if GITHUB_REPOSITORY
        else "GitHub Release"
    )
    created = api_request(
        "POST",
        api_url(f"{repo_path}/-/releases"),
        {
            "tag_name": TAG,
            "target_commitish": TARGET_BRANCH,
            "name": TAG,
            "body": f"Synchronized from {source_url}",
            "draft": False,
            "prerelease": prerelease,
            "make_latest": "false" if prerelease else "true",
        },
    )
    release_id = str((created or {}).get("id", "")).strip()
    if not release_id:
        fail(f"CNB created Release {TAG} but did not return an id")
    print(f"Created CNB Release {TAG}: {release_id}")
    return release_id


def upload_asset(release_id: str, asset: Path) -> None:
    repo_path = urllib.parse.quote(REPO, safe="/")
    release_path = urllib.parse.quote(release_id, safe="")
    metadata = api_request(
        "POST",
        api_url(f"{repo_path}/-/releases/{release_path}/asset-upload-url"),
        {
            "asset_name": asset.name,
            "size": asset.stat().st_size,
            "overwrite": True,
            "ttl": 0,
        },
    ) or {}

    upload_url = str(metadata.get("upload_url", "")).strip()
    verify_url = str(metadata.get("verify_url", "")).strip()
    if not upload_url or not verify_url:
        fail(f"CNB did not return upload_url/verify_url for {asset.name}")

    print(f"Uploading {asset.name} ({asset.stat().st_size} bytes)")
    subprocess.run(
        [
            "curl",
            "--fail",
            "--silent",
            "--show-error",
            "--request",
            "PUT",
            "--upload-file",
            str(asset),
            upload_url,
        ],
        check=True,
    )

    separator = "&" if "?" in verify_url else "?"
    api_request("POST", f"{verify_url}{separator}ttl=0")
    print(f"Uploaded {asset.name}")


def main() -> int:
    if not TOKEN:
        fail("CNB_TOKEN is required")
    if not REPO:
        fail("CNB_REPO_SLUG is required")
    if not TAG:
        fail("GITHUB_REF_NAME is required")
    if not RELEASE_DIR.is_dir():
        fail(f"Release directory not found: {RELEASE_DIR}")

    assets = sorted(path for path in RELEASE_DIR.iterdir() if path.is_file())
    if not assets:
        fail(f"No release assets found in {RELEASE_DIR}")

    release_id = get_or_create_release()
    for asset in assets:
        upload_asset(release_id, asset)

    print(f"Published {len(assets)} assets to CNB Release {TAG}")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except CNBError as exc:
        print(f"error: {exc}", file=sys.stderr)
        sys.exit(1)
    except subprocess.CalledProcessError as exc:
        print(f"error: asset upload command failed with exit code {exc.returncode}", file=sys.stderr)
        sys.exit(exc.returncode or 1)
