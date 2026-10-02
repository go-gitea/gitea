import argparse
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path


def version_tag(version):
    version = version.removeprefix("v")
    if not re.fullmatch(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-rc\d+)?", version):
        raise ValueError("Version must be a stable or rc version, for example 28.0.1 or 29.0.0-rc0")
    return f"v{version}"


def validate_branch(branch, tag):
    match = re.fullmatch(r"release/v(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?", branch)
    if not match:
        raise ValueError("Select a release/v* branch when running this workflow")
    line = ".".join(part for part in match.groups() if part is not None)
    if not tag.removeprefix("v").startswith(f"{line}."):
        raise ValueError(f"Version {tag} does not belong to {branch}")


def api(path, token, missing_ok=False):
    if not token:
        raise ValueError("A required GitHub API token is missing")
    request = urllib.request.Request(
        f"https://api.github.com/{path}",
        headers={"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        if missing_ok and error.code == 404:
            return None
        raise ValueError(f"GitHub API check failed: HTTP {error.code} for {path}") from error


def authorize(repository, actors, token):
    if not re.fullmatch(r"[\w.-]+/[\w.-]+", repository):
        raise ValueError("Invalid repository")
    for actor in set(actors):
        if not re.fullmatch(r"[\w-]+", actor):
            raise ValueError("Missing or invalid workflow actor")
        permission = api(f"repos/{repository}/collaborators/{actor}/permission", token)
        if permission.get("role_name") not in {"maintain", "admin"}:
            raise ValueError(f"{actor} must have the Maintain or Admin repository role")


def check_version(repository, tag, token):
    for path in (f"repos/{repository}/git/ref/tags/{tag}", f"repos/{repository}/releases/tags/{tag}"):
        if api(path, token, missing_ok=True) is not None:
            raise ValueError(f"Version {tag} already exists as a remote tag or GitHub release")


def git(*args):
    return subprocess.check_output(["git", *args], text=True).strip()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["prepare", "preview", "check"])
    args = parser.parse_args()
    tag = version_tag(os.environ["RELEASE_VERSION"])
    branch = os.environ["GITHUB_REF_NAME"]
    if os.environ["GITHUB_REF_TYPE"] != "branch":
        raise ValueError("This workflow must run on a release branch")
    validate_branch(branch, tag)
    repository = os.environ["GITHUB_REPOSITORY"]
    authorize(repository, [os.environ["GITHUB_ACTOR"], os.environ["GITHUB_TRIGGERING_ACTOR"]],
              os.environ.get("GITHUB_TOKEN", ""))
    check_version(repository, tag, os.environ.get("GITHUB_TOKEN", ""))
    commit = os.environ["GITHUB_SHA"]
    if args.command != "check" and git("rev-parse", "HEAD") != commit:
        raise ValueError("Checkout does not match the workflow's selected commit")
    remote = git("ls-remote", "origin", f"refs/heads/{branch}").split()
    if not remote or remote[0] != commit:
        raise ValueError("The release branch has moved; start a new workflow run")
    if args.command == "prepare":
        candidates = [value for value in git("tag", "--merged", commit).splitlines()
                      if re.fullmatch(r"v\d+\.\d+\.\d+(?:-rc\d+)?", value)]
        if not candidates:
            raise ValueError("No previous release tag is reachable from this branch")
        matches = [argument for value in candidates for argument in ("--match", value)]
        previous = git("describe", "--tags", "--abbrev=0", *matches, commit)
        with open(os.environ["GITHUB_OUTPUT"], "a") as output:
            output.write(f"tag={tag}\nprevious={previous}\n")
    elif args.command == "preview":
        notes = Path(os.environ["RELEASE_NOTES_FILE"]).read_text().strip()
        if not re.search(r"^  \* \S", notes, re.MULTILINE):
            raise ValueError("Generated release notes have no entries")
        Path("release.commit").write_text(f"{tag}\n\n{notes}\n")
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write(f"## Release preview\n\nBranch: `{branch}`\n\nBase commit: `{commit}`\n\n")
            summary.write(f"Tag and commit title: `{tag}`\n\nDry-run: `{os.environ['RELEASE_DRY_RUN']}`\n\n{notes}\n")
    elif git("rev-parse", "HEAD^") != commit or git("rev-parse", "HEAD^{tree}") != git("rev-parse", f"{commit}^{{tree}}"):
        raise ValueError("Release commit must be an empty marker on the selected commit")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, subprocess.CalledProcessError, urllib.error.URLError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
