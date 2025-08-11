import requests
import string
import random
from config import BASE_URL, API_TOKEN

def _headers():
    return {"Authorization": f"token {API_TOKEN}"}

def _rand_username(prefix="dupuser"):
    return prefix + "_" + "".join(random.choices(string.ascii_lowercase + string.digits, k=6))

def test_create_duplicate_user_should_fail():
    headers = _headers()
    username = _rand_username()
    payload = {
        "email": f"{username}@example.com",
        "username": username,
        "password": "TestPass123!",
        "must_change_password": False,
        "send_notify": False,
    }

    # First creation should succeed
    r1 = requests.post(f"{BASE_URL}/api/v1/admin/users", headers=headers, json=payload)
    assert r1.status_code == 201, f"First create failed: {r1.status_code} {r1.text}"

    # Second creation with the same username should fail
    r2 = requests.post(f"{BASE_URL}/api/v1/admin/users", headers=headers, json=payload)

    # Gitea typically returns 422 for duplicate username; some setups may return 409
    assert r2.status_code in (422, 409), f"Expected 422/409 on duplicate, got {r2.status_code} {r2.text}"

    # Error body should hint user exists
    err_text = r2.text.lower()
    assert ("exist" in err_text) or ("already" in err_text), f"Unexpected error message: {r2.text}"

    # Cleanup: best-effort delete the user we created (ignore errors)
    requests.delete(f"{BASE_URL}/api/v1/admin/users/{username}", headers=headers)
