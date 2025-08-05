import requests
import random
import string

BASE_URL = "http://54.247.213.28:3000/"
API_TOKEN = "ed10e99db13b34b5b45be5c44d1197eb1f161a32"

def test_get_users():
    headers = {
        "Authorization": f"token {API_TOKEN}"
    }

    response = requests.get(f"{BASE_URL}/api/v1/admin/users", headers=headers)

    assert response.status_code == 200
    users = response.json()
    assert isinstance(users, list)


def generate_random_username(prefix="user"):
    suffix = ''.join(random.choices(string.ascii_lowercase + string.digits, k=5))
    return f"{prefix}_{suffix}"

def test_create_user():
    headers = {
        "Authorization": f"token {API_TOKEN}"
    }

    username = generate_random_username()
    payload = {
        "email": f"{username}@example.com",
        "username": username,
        "password": "TestPass123!",
        "must_change_password": False,
        "send_notify": False
    }

    response = requests.post(f"{BASE_URL}/api/v1/admin/users", headers=headers, json=payload)

    assert response.status_code == 201, f"Response: {response.text}"
    user = response.json()
    assert user["username"] == username

def test_list_user_repos():
    headers = {
        "Authorization": f"token {API_TOKEN}"
    }

    # Create a new user first
    username = generate_random_username()
    payload = {
        "email": f"{username}@example.com",
        "username": username,
        "password": "TestPass123!",
        "must_change_password": False,
        "send_notify": False
    }
    create_response = requests.post(f"{BASE_URL}/api/v1/admin/users", headers=headers, json=payload)
    assert create_response.status_code == 201

    # Now fetch their repositories
    list_response = requests.get(f"{BASE_URL}/api/v1/users/{username}/repos", headers=headers)

    assert list_response.status_code == 200
    repos = list_response.json()
    assert isinstance(repos, list)
    assert len(repos) == 0  # New user should have no repos

def test_create_repo_for_user():
    headers = {
        "Authorization": f"token {API_TOKEN}"
    }

    # Step 1: Create a new user
    username = generate_random_username()
    payload = {
        "email": f"{username}@example.com",
        "username": username,
        "password": "TestPass123!",
        "must_change_password": False,
        "send_notify": False
    }
    create_user_response = requests.post(f"{BASE_URL}/api/v1/admin/users", headers=headers, json=payload)
    assert create_user_response.status_code == 201

    # Step 2: Create a repository for the new user
    repo_name = f"repo_{username}"
    repo_payload = {
        "name": repo_name,
        "description": "This is a test repo",
        "private": False,
        "auto_init": True
    }
    create_repo_response = requests.post(
        f"{BASE_URL}/api/v1/admin/users/{username}/repos",
        headers=headers,
        json=repo_payload
    )

    assert create_repo_response.status_code == 201, f"Error: {create_repo_response.text}"
    repo = create_repo_response.json()
    assert repo["name"] == repo_name
    assert repo["owner"]["login"] == username
