import requests
import random
import string
from config import BASE_URL, API_TOKEN

def _headers():
    return {"Authorization": f"token {API_TOKEN}"}

def _rand(s="user"):
    return s + "_" + "".join(random.choices(string.ascii_lowercase + string.digits, k=6))

def test_create_repo_then_get_details():
    headers = _headers()

    # 1) Create a user (owner)
    username = _rand("owner")
    user_payload = {
        "email": f"{username}@example.com",
        "username": username,
        "password": "TestPass123!",
        "must_change_password": False,
        "send_notify": False,
    }
    r_user = requests.post(f"{BASE_URL}/api/v1/admin/users", headers=headers, json=user_payload)
    assert r_user.status_code == 201, f"User create failed: {r_user.status_code} {r_user.text}"

    # 2) Create a repository for that user
    repo_name = _rand("repo")
    repo_payload = {
        "name": repo_name,
        "description": "API test repo",
        "private": False,
        "auto_init": True,  # so default branch & README exist
    }
    r_repo = requests.post(f"{BASE_URL}/api/v1/admin/users/{username}/repos", headers=headers, json=repo_payload)
    assert r_repo.status_code == 201, f"Repo create failed: {r_repo.status_code} {r_repo.text}"
    created = r_repo.json()
    assert created["name"] == repo_name
    assert created["owner"]["login"] == username

    # 3) GET repo details and verify fields
    r_get = requests.get(f"{BASE_URL}/api/v1/repos/{username}/{repo_name}", headers=headers)
    assert r_get.status_code == 200, f"Get repo failed: {r_get.status_code} {r_get.text}"
    repo = r_get.json()

    # Basic shape checks
    assert repo["name"] == repo_name
    assert repo["owner"]["login"] == username
    assert repo["private"] is False
    # default branch commonly 'main' or 'master' depending on settings; assert it's non-empty
    assert repo.get("default_branch"), f"Expected default_branch to be set, got {repo.get('default_branch')}"

    # 4) Cleanup (best-effort)
    requests.delete(f"{BASE_URL}/api/v1/repos/{username}/{repo_name}", headers=headers)
    requests.delete(f"{BASE_URL}/api/v1/admin/users/{username}", headers=headers)
