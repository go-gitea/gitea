# usage: uv run pytest -v verify35383.py

import os
import time
import requests
import pytest

GITEA_URL = os.environ["GITEA_URL"].rstrip("/")
OWNER = os.environ["OWNER"]
REPO = os.environ["REPO"]
COMMIT_SHA = os.environ["COMMIT_SHA"]

STATUS_URL = f"{GITEA_URL}/api/v1/repos/{OWNER}/{REPO}/statuses/{COMMIT_SHA}"

TOKENS = {
    "repo-write": {
        "token": os.environ["TOKEN_REPO_WRITE"],
        "can_write": True,
        "can_read": True,
    },
    "repo-read+commitstatus-write": {
        "token": os.environ["TOKEN_STATUS_WRITE"],
        "can_write": True,
        "can_read": True,
    },
    "repo-read-only": {
        "token": os.environ["TOKEN_REPO_READ"],
        "can_write": False,
        "can_read": True,
    },
}


def auth_headers(token: str) -> dict:
    return {
        "Authorization": f"token {token}",
        "Content-Type": "application/json",
        "Accept": "application/json",
    }


@pytest.mark.parametrize("name,cfg", TOKENS.items())
def test_commit_status_write_permission(name: str, cfg: dict) -> None:
    context = f"perm-test-{name}-{int(time.time())}"

    payload = {
        "state": "success",
        "context": context,
        "description": "Permission verification test",
        "target_url": "https://example.com",
    }

    response = requests.post(
        STATUS_URL,
        json=payload,
        headers=auth_headers(cfg["token"]),
        timeout=10,
    )

    if cfg["can_write"]:
        assert response.status_code == 201, response.text
        body = response.json()
        assert body["status"] == "success"
        assert body["context"] == context
    else:
        assert response.status_code == 403


@pytest.mark.parametrize("name,cfg", TOKENS.items())
def test_commit_status_read_permission(name: str, cfg: dict) -> None:
    response = requests.get(
        STATUS_URL,
        headers=auth_headers(cfg["token"]),
        timeout=10,
    )

    if cfg["can_read"]:
        assert response.status_code == 200, response.text
        body = response.json()
        assert isinstance(body, list)

        if body:
            status = body[0]
            assert "status" in status
            assert "context" in status
            assert "created_at" in status
    else:
        assert response.status_code in (401, 403)
