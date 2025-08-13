# tests/test_create_repo.py
import random
import string
import unittest
import requests
from config import BASE_URL, API_TOKEN


def _headers() -> dict:
    return {"Authorization": f"token {API_TOKEN}"}


def _rand(prefix: str = "user") -> str:
    return prefix + "_" + "".join(random.choices(string.ascii_lowercase + string.digits, k=6))


class TestCreateRepoThenGetDetails(unittest.TestCase):
    @classmethod
    def setUp(self):
        #try CI
        # Runs before each test
        if not BASE_URL:
            raise RuntimeError("BASE_URL is not set")
        if not API_TOKEN:
            raise RuntimeError("API_TOKEN is not set")
        self.base_url = BASE_URL.rstrip("/")
        self.headers = {"Authorization": f"token {API_TOKEN}"}

        # Track created resources for cleanup
        self._created_user = None
        self._created_repo = None

    def tearDown(self) -> None:
        # Delete repo first if it was created
        if self._created_repo:
            owner, repo_name = self._created_repo
            try:
                url = f"{self.base_url}/api/v1/repos/{owner}/{repo_name}"
                r = requests.delete(url, headers=self.headers, timeout=30)
                print(f"[teardown] DELETE repo {owner}/{repo_name} -> {r.status_code}")
            except Exception as e:
                print(f"[teardown] Failed to delete repo {owner}/{repo_name}: {e}")

        # Delete user if it was created
        if self._created_user:
            try:
                url = f"{self.base_url}/api/v1/admin/users/{self._created_user}"
                r = requests.delete(url, headers=self.headers, timeout=30)
                print(f"[teardown] DELETE user {self._created_user} -> {r.status_code}")
            except Exception as e:
                print(f"[teardown] Failed to delete user {self._created_user}: {e}")

    def test_create_repo_then_get_details(self) -> None:
        # 1) Create a user (owner)
        username = _rand("owner")
        user_payload = {
            "email": f"{username}@example.com",
            "username": username,
            "password": "TestPass123!",
            "must_change_password": False,
            "send_notify": False,
        }
        r_user = requests.post(
            f"{self.base_url}/api/v1/admin/users",
            headers=self.headers,
            json=user_payload,
            timeout=30,
        )
        self.assertEqual(r_user.status_code, 201, f"User create failed: {r_user.status_code} {r_user.text}")
        self._created_user = username  # mark for cleanup

        # 2) Create a repository for that user
        repo_name = _rand("repo")
        repo_payload = {
            "name": repo_name,
            "description": "API test repo",
            "private": False,
            "auto_init": True,
        }
        r_repo = requests.post(
            f"{self.base_url}/api/v1/admin/users/{username}/repos",
            headers=self.headers,
            json=repo_payload,
            timeout=30,
        )
        self.assertEqual(r_repo.status_code, 201, f"Repo create failed: {r_repo.status_code} {r_repo.text}")
        created = r_repo.json()
        self.assertEqual(created["name"], repo_name)
        self.assertEqual(created["owner"]["login"], username)
        self._created_repo = (username, repo_name)  # mark for cleanup

        # 3) GET repo details and verify fields
        r_get = requests.get(
            f"{self.base_url}/api/v1/repos/{username}/{repo_name}",
            headers=self.headers,
            timeout=30,
        )
        self.assertEqual(r_get.status_code, 200, f"Get repo failed: {r_get.status_code} {r_get.text}")
        repo = r_get.json()

        self.assertEqual(repo["name"], repo_name)
        self.assertEqual(repo["owner"]["login"], username)
        self.assertIs(repo["private"], False)
        self.assertTrue(repo.get("default_branch"), "Expected default_branch to be set")


if __name__ == "__main__":
    unittest.main(verbosity=2)
