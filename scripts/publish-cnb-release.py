#!/usr/bin/env python3
"""Publish GitHub Actions artifacts to the configured public CNB distribution repo.

Only CNB_TOKEN is read from CI secrets. The repository URL and branch come from
project/app.json, and CNB Git HTTPS authentication always uses username `cnb`.
The private source repository is never pushed to CNB.
"""

from __future__ import annotations

import hashlib
import json
import os
import re
import stat
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

API_ENDPOINT = "https://api.cnb.cool"
CNB_USERNAME = "cnb"
TOKEN = os.getenv("CNB_TOKEN", "").strip()
CONFIG_PATH = Path("project/app.json")
SOURCE_TAG = os.getenv("GITHUB_REF_NAME", "").strip()
RELEASE_DIR = Path("release")

SEMVER_RE = re.compile(
    r"^(0|[1-9][0-9]*)\."
    r"(0|[1-9][0-9]*)\."
    r"(0|[1-9][0-9]*)"
    r"(?:-([0-9A-Za-z.-]+))?"
    r"(?:\+([0-9A-Za-z.-]+))?$"
)


class CNBError(RuntimeError):
    pass


def fail(message: str) -> None:
    raise CNBError(message)


def load_project() -> dict[str, object]:
    try:
        data = json.loads(CONFIG_PATH.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"cannot read {CONFIG_PATH}: {exc}")
    if not isinstance(data, dict):
        fail(f"{CONFIG_PATH} must contain a JSON object")
    return data


def normalize_repo_url(value: str) -> str:
    return value.strip().rstrip("/")


def repo_slug_from_url(value: str) -> str:
    parsed = urllib.parse.urlparse(value)
    if parsed.scheme != "https" or parsed.netloc.lower() != "cnb.cool":
        fail(f"updateRepositoryURL must be an https://cnb.cool/... URL: {value}")
    slug = parsed.path.strip("/")
    if not slug or "/" not in slug:
        fail(f"CNB repository URL must include group/repository: {value}")
    return slug


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
            return decoded if isinstance(decoded, dict) else {"data": decoded}
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace").strip()
        if allow_not_found and exc.code == 404:
            return None
        detail = f": {body}" if body else ""
        fail(f"CNB API {method} {url} failed with HTTP {exc.code}{detail}")
    except urllib.error.URLError as exc:
        fail(f"CNB API {method} {url} failed: {exc.reason}")


def get_or_create_release(repo_slug: str, branch: str, release_tag: str, app: str, version: str) -> str:
    repo_path = urllib.parse.quote(repo_slug, safe="/")
    tag_path = urllib.parse.quote(release_tag, safe="")
    existing = api_request(
        "GET",
        api_url(f"{repo_path}/-/releases/tags/{tag_path}"),
        allow_not_found=True,
    )
    if existing:
        release_id = str(existing.get("id", "")).strip()
        if release_id:
            print(f"CNB Release {release_tag} already exists: {release_id}")
            return release_id

    prerelease = "-" in version
    created = api_request(
        "POST",
        api_url(f"{repo_path}/-/releases"),
        {
            "tag_name": release_tag,
            "target_commitish": branch,
            "name": f"{app} v{version}",
            "body": f"Distribution artifacts for {app} v{version}.",
            "draft": False,
            "prerelease": prerelease,
            # The repository is shared by multiple applications, so each app
            # owns its own latest.json instead of using repository-wide latest.
            "make_latest": "false",
        },
    )
    release_id = str((created or {}).get("id", "")).strip()
    if not release_id:
        fail(f"CNB created Release {release_tag} but did not return an id")
    print(f"Created CNB Release {release_tag}: {release_id}")
    return release_id


def upload_asset(repo_slug: str, release_id: str, asset: Path) -> None:
    repo_path = urllib.parse.quote(repo_slug, safe="/")
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


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def classify_asset(app: str, path: Path) -> dict[str, object] | None:
    if path.name == "SHA256SUMS":
        return None
    match = re.fullmatch(re.escape(app) + r"-(windows|linux|darwin)-([a-z0-9_]+)(.*)", path.name)
    if not match:
        fail(f"unexpected release asset name: {path.name}")

    platform, arch, suffix = match.groups()
    runtime_suffix = {"windows": ".exe", "linux": "", "darwin": ".zip"}[platform]
    installer_suffix = {"windows": "-installer.exe", "linux": ".deb", "darwin": ".pkg"}[platform]
    if suffix == runtime_suffix:
        kind = "runtime"
    elif suffix == installer_suffix:
        kind = "installer"
    else:
        fail(f"unsupported release asset for {platform}/{arch}: {path.name}")

    return {
        "kind": kind,
        "platform": platform,
        "arch": arch,
        "filename": path.name,
        "sha256": sha256_file(path),
        "size": path.stat().st_size,
    }


def build_manifest(app: str, version: str, release_tag: str, assets: list[Path]) -> dict[str, object]:
    artifacts = []
    for asset in assets:
        entry = classify_asset(app, asset)
        if entry is not None:
            artifacts.append(entry)
    if not any(item["kind"] == "runtime" for item in artifacts):
        fail("release contains no runtime update artifacts")
    artifacts.sort(key=lambda item: (str(item["platform"]), str(item["arch"]), str(item["kind"])))
    prerelease = "-" in version
    return {
        "schemaVersion": 1,
        "app": app,
        "version": version,
        "tag": release_tag,
        "channel": "prerelease" if prerelease else "stable",
        "publishedAt": datetime.now(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z"),
        "artifacts": artifacts,
    }


def semver_parts(value: str) -> tuple[int, int, int, list[str]]:
    match = SEMVER_RE.fullmatch(value)
    if not match:
        fail(f"invalid semantic version: {value}")
    prerelease = match.group(4)
    return int(match.group(1)), int(match.group(2)), int(match.group(3)), prerelease.split(".") if prerelease else []


def compare_semver(left: str, right: str) -> int:
    l_major, l_minor, l_patch, l_pre = semver_parts(left)
    r_major, r_minor, r_patch, r_pre = semver_parts(right)
    if (l_major, l_minor, l_patch) != (r_major, r_minor, r_patch):
        return 1 if (l_major, l_minor, l_patch) > (r_major, r_minor, r_patch) else -1
    if not l_pre and not r_pre:
        return 0
    if not l_pre:
        return 1
    if not r_pre:
        return -1
    for left_id, right_id in zip(l_pre, r_pre):
        if left_id == right_id:
            continue
        left_numeric = left_id.isdigit()
        right_numeric = right_id.isdigit()
        if left_numeric and right_numeric:
            return 1 if int(left_id) > int(right_id) else -1
        if left_numeric != right_numeric:
            return -1 if left_numeric else 1
        return 1 if left_id > right_id else -1
    if len(l_pre) == len(r_pre):
        return 0
    return 1 if len(l_pre) > len(r_pre) else -1


def should_update_pointer(path: Path, version: str) -> bool:
    if not path.is_file():
        return True
    try:
        current = json.loads(path.read_text(encoding="utf-8"))
        current_version = str(current.get("version", ""))
    except (OSError, json.JSONDecodeError, AttributeError):
        return True
    return not current_version or compare_semver(version, current_version) >= 0


def git_env(askpass: Path) -> dict[str, str]:
    env = os.environ.copy()
    env["GIT_ASKPASS"] = str(askpass)
    env["GIT_TERMINAL_PROMPT"] = "0"
    env["CNB_TOKEN"] = TOKEN
    return env


def run_git(args: list[str], cwd: Path, env: dict[str, str], *, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=cwd,
        env=env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=check,
    )


def publish_manifest_repo(repo_url: str, branch: str, app: str, source_tag: str, manifest: dict[str, object]) -> None:
    git_url = repo_url + ("" if repo_url.endswith(".git") else ".git")
    version = str(manifest["version"])
    pointer_name = "prerelease.json" if manifest.get("channel") == "prerelease" else "latest.json"
    payload = json.dumps(manifest, ensure_ascii=False, indent=2) + "\n"

    for attempt in range(1, 4):
        with tempfile.TemporaryDirectory(prefix="cnb-publish-") as temp_dir:
            temp = Path(temp_dir)
            askpass = temp / "askpass.sh"
            askpass.write_text(
                "#!/bin/sh\n"
                "case \"$1\" in\n"
                f"  *Username*) printf '%s\\n' '{CNB_USERNAME}' ;;\n"
                "  *) printf '%s\\n' \"$CNB_TOKEN\" ;;\n"
                "esac\n",
                encoding="utf-8",
            )
            askpass.chmod(askpass.stat().st_mode | stat.S_IXUSR)
            env = git_env(askpass)
            checkout = temp / "repo"
            clone = subprocess.run(
                ["git", "clone", "--depth", "1", "--branch", branch, git_url, str(checkout)],
                env=env,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
            )
            if clone.returncode != 0:
                fail(f"unable to clone CNB distribution repository: {clone.stdout.strip()}")

            app_dir = checkout / app
            version_dir = app_dir / source_tag
            version_dir.mkdir(parents=True, exist_ok=True)
            (version_dir / "manifest.json").write_text(payload, encoding="utf-8")

            pointer = app_dir / pointer_name
            if should_update_pointer(pointer, version):
                pointer.write_text(payload, encoding="utf-8")

            run_git(["config", "user.name", "github-actions"], checkout, env)
            run_git(["config", "user.email", "github-actions@users.noreply.github.com"], checkout, env)
            run_git(["add", "--", app], checkout, env)
            diff = run_git(["diff", "--cached", "--quiet"], checkout, env, check=False)
            if diff.returncode == 0:
                print("CNB manifest repository is already up to date")
                return
            if diff.returncode != 1:
                fail(f"git diff failed: {diff.stdout.strip()}")

            run_git(["commit", "-m", f"release({app}): publish {source_tag}"], checkout, env)
            push = run_git(["push", "origin", f"HEAD:{branch}"], checkout, env, check=False)
            if push.returncode == 0:
                print(f"Published manifest: {app}/{source_tag}/manifest.json")
                print(f"Updated pointer when applicable: {app}/{pointer_name}")
                return
            if attempt == 3:
                fail(f"unable to push CNB manifests after {attempt} attempts: {push.stdout.strip()}")
            print(f"CNB manifest push raced with another publisher; retrying ({attempt}/3)")
            time.sleep(attempt)


def main() -> int:
    if not TOKEN:
        fail("CNB_TOKEN is required (use a CNB access token with repository write permission)")
    if not SOURCE_TAG.startswith("v") or not SEMVER_RE.fullmatch(SOURCE_TAG[1:]):
        fail(f"GITHUB_REF_NAME must be a semantic-version tag such as v1.2.3 (got {SOURCE_TAG!r})")
    if not RELEASE_DIR.is_dir():
        fail(f"release directory not found: {RELEASE_DIR}")

    project = load_project()
    app = str(project.get("binaryName", "")).strip()
    repo_url = normalize_repo_url(str(project.get("updateRepositoryURL", "")))
    branch = str(project.get("updateBranch", "")).strip() or "main"
    if not app or not repo_url:
        fail("project/app.json must define binaryName and updateRepositoryURL")

    repo_slug = repo_slug_from_url(repo_url)
    version = SOURCE_TAG[1:]
    release_tag = f"{app}-{SOURCE_TAG}"
    assets = sorted(path for path in RELEASE_DIR.iterdir() if path.is_file())
    if not assets:
        fail(f"no release assets found in {RELEASE_DIR}")
    if not any(path.name == "SHA256SUMS" for path in assets):
        fail("SHA256SUMS is required in the release directory")

    manifest = build_manifest(app, version, release_tag, assets)
    release_id = get_or_create_release(repo_slug, branch, release_tag, app, version)
    for asset in assets:
        upload_asset(repo_slug, release_id, asset)

    # Publish metadata only after every binary attachment is available.
    publish_manifest_repo(repo_url, branch, app, SOURCE_TAG, manifest)
    print(f"Published {len(assets)} assets to CNB Release {release_tag}")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except CNBError as exc:
        print(f"error: {exc}", file=sys.stderr)
        sys.exit(1)
    except subprocess.CalledProcessError as exc:
        output = exc.stdout.strip() if isinstance(exc.stdout, str) else ""
        if output:
            print(output, file=sys.stderr)
        print(f"error: command failed with exit code {exc.returncode}", file=sys.stderr)
        sys.exit(exc.returncode or 1)
