import requests
import random
import string
from config import BASE_URL, API_TOKEN

def _headers():
    return {"Authorization": f"token {API_TOKEN}"}

def _rand(prefix):
    return prefix + "_" + "".join(random.choices(string.ascii_lowercase + string.digits, k=6))

def test_list_branches_contains_default_branch():
    headers = _headers()

    # 1) Create owner user
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

    # 2) Create repo with auto_init so default branch exists
    repo_name = _rand("repo")
    repo_payload = {"name": repo_name, "description": "Branches test", "private": False, "auto_init": True}
    r_repo = requests.post(f"{BASE_URL}/api/v1/admin/users/{username}/repos", headers=headers, json=repo_payload)
    assert r_repo.status_code == 201, f"Repo create failed: {r_repo.status_code} {r_repo.text}"

    # 3) Get repo to read its default_branch
    r_get = requests.get(f"{BASE_URL}/api/v1/repos/{username}/{repo_name}", headers=headers)
    assert r_get.status_code == 200, f"Get repo failed: {r_get.status_code} {r_get.text}"
    default_branch = r_get.json().get("default_branch")
    assert default_branch, "Expected default_branch to be set"

    # 4) List branches and verify default branch is present
    r_branches = requests.get(f"{BASE_URL}/api/v1/repos/{username}/{repo_name}/branches", headers=headers)
    assert r_branches.status_code == 200, f"List branches failed: {r_branches.status_code} {r_branches.text}"
    branches = r_branches.json()
    assert isinstance(branches, list) and branches, "Expected non-empty branches list"

    names = {b.get("name") for b in branches}
    assert default_branch in names, f"default branch '{default_branch}' not found in {names}"

    # 5) Cleanup (best-effort)
    requests.delete(f"{BASE_URL}/api/v1/repos/{username}/{repo_name}", headers=headers)
    requests.delete(f"{BASE_URL}/api/v1/admin/users/{username}", headers=headers)
