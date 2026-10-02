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


def release_notes(changelog, tag):
    version = tag.removeprefix("v")
    sections = list(re.finditer(r"^## .+$", changelog, re.MULTILINE))
    pattern = rf"## (?:{re.escape(version)}|\[{re.escape(version)}\]\([^\n]+\)) - \d{{4}}-\d{{2}}-\d{{2}}"
    matches = [index for index, section in enumerate(sections) if re.fullmatch(pattern, section.group())]
    if len(matches) != 1:
        raise ValueError(f"CHANGELOG.md must contain exactly one release section for {version}")
    index = matches[0]
    if index != 0:
        raise ValueError(f"{version} must be the first release in CHANGELOG.md")
    start = sections[index].start()
    end = sections[index + 1].start() if index + 1 < len(sections) else len(changelog)
    notes = changelog[start:end].strip()
    if not re.search(r"^  \* \S", notes, re.MULTILINE):
        raise ValueError(f"The changelog for {version} has no entries")
    return f"{notes}\n"


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


def authorize(repository, team, actors, token):
    if not re.fullmatch(r"[\w.-]+/[\w.-]+", repository) or not re.fullmatch(r"[\w-]+", team):
        raise ValueError("Configure RELEASE_MAINTAINERS_TEAM with the release team's slug")
    for actor in set(actors):
        if not re.fullmatch(r"[\w-]+", actor):
            raise ValueError("Missing or invalid workflow actor")
        membership = api(f"orgs/{repository.split('/')[0]}/teams/{team}/memberships/{actor}", token)
        if membership.get("state") != "active":
            raise ValueError(f"{actor} is not an active member of the release-maintainers team")


def check_version(repository, tag, token):
    for path in (f"repos/{repository}/git/ref/tags/{tag}", f"repos/{repository}/releases/tags/{tag}"):
        if api(path, token, missing_ok=True) is not None:
            raise ValueError(f"Version {tag} already exists as a remote tag or GitHub release")


def git(*args):
    return subprocess.check_output(["git", *args], text=True).strip()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["prepare", "check"])
    args = parser.parse_args()
    tag = version_tag(os.environ["RELEASE_VERSION"])
    branch = os.environ["GITHUB_REF_NAME"]
    if os.environ["GITHUB_REF_TYPE"] != "branch":
        raise ValueError("This workflow must run on a release branch")
    validate_branch(branch, tag)
    repository = os.environ["GITHUB_REPOSITORY"]
    authorize(repository, os.environ.get("RELEASE_MAINTAINERS_TEAM", ""),
              [os.environ["GITHUB_ACTOR"], os.environ["GITHUB_TRIGGERING_ACTOR"]],
              os.environ.get("RELEASE_TEAM_TOKEN", ""))
    check_version(repository, tag, os.environ.get("GITHUB_TOKEN", ""))
    commit = os.environ["GITHUB_SHA"]
    if git("rev-parse", "HEAD") != commit:
        raise ValueError("Checkout does not match the workflow's selected commit")
    remote = git("ls-remote", "origin", f"refs/heads/{branch}").split()
    if not remote or remote[0] != commit:
        raise ValueError("The release branch has moved; start a new workflow run")
    notes = release_notes(git("show", f"{commit}:CHANGELOG.md"), tag)
    if args.command == "prepare":
        Path(os.environ["RELEASE_NOTES_FILE"]).write_text(notes)
        with open(os.environ["GITHUB_OUTPUT"], "a") as output:
            output.write(f"tag={tag}\n")
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write(f"## Release tag preview\n\nBranch: `{branch}`\n\nCommit: `{commit}`\n\n")
            summary.write(f"Tag: `{tag}`\n\nDry-run: `{os.environ['RELEASE_DRY_RUN']}`\n\n{notes}")
        print(f"Validated {tag} at {commit}. Dry-run: {os.environ['RELEASE_DRY_RUN']}")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, subprocess.CalledProcessError, urllib.error.URLError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
