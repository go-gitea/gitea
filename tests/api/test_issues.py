import requests
import random
import string
from config import BASE_URL, API_TOKEN

def _headers():
    return {"Authorization": f"token {API_TOKEN}"}

def _rand(prefix):
    return prefix + "_" + "".join(random.choices(string.ascii_lowercase + string.digits, k=6))

def test_create_issue_and_list_it():
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

    # 2) Create repo
    repo_name = _rand("repo")
    repo_payload = {"name": repo_name, "description": "Issues test", "private": False, "auto_init": True}
    r_repo = requests.post(f"{BASE_URL}/api/v1/admin/users/{username}/repos", headers=headers, json=repo_payload)
    assert r_repo.status_code == 201, f"Repo create failed: {r_repo.status_code} {r_repo.text}"

    # 3) Create an issue
    title = f"Issue { _rand('t') }"
    body = "Created by API test"
    issue_payload = {"title": title, "body": body}
    r_issue = requests.post(
        f"{BASE_URL}/api/v1/repos/{username}/{repo_name}/issues",
        headers=headers,
        json=issue_payload
    )
    assert r_issue.status_code == 201, f"Issue create failed: {r_issue.status_code} {r_issue.text}"
    issue = r_issue.json()
    number = issue.get("number")
    assert number, f"Expected issue number, got {issue}"
    assert issue["title"] == title
    assert issue["state"] == "open"

    # 4) List issues and verify the created one is present
    r_list = requests.get(f"{BASE_URL}/api/v1/repos/{username}/{repo_name}/issues", headers=headers)
    assert r_list.status_code == 200, f"Issues list failed: {r_list.status_code} {r_list.text}"
    issues = r_list.json()
    numbers = {i.get("number") for i in issues}
    assert number in numbers, f"Issue #{number} not found in list: {numbers}"

    # 5) Cleanup (best-effort)
    requests.delete(f"{BASE_URL}/api/v1/repos/{username}/{repo_name}", headers=headers)
    requests.delete(f"{BASE_URL}/api/v1/admin/users/{username}", headers=headers)
