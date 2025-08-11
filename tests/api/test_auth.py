import requests
from config import BASE_URL, API_TOKEN  # uses the same config.py you already generate in CI

def test_me_endpoint_returns_current_user_and_is_admin():
    headers = {"Authorization": f"token {API_TOKEN}"}

    r = requests.get(f"{BASE_URL}/api/v1/user", headers=headers)
    assert r.status_code == 200, f"Unexpected status: {r.status_code} body={r.text}"

    me = r.json()
    # Basic shape checks
    for key in ["id", "login", "email", "is_admin", "created"]:
        assert key in me, f"Missing key '{key}' in /user response: {me}"

    # If the token is an admin (your token is), this should be True
    assert me["is_admin"] is True
