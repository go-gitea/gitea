# Project Board API Documentation

## Overview

The Project Board API provides a complete RESTful interface for managing repository project boards (Kanban boards) in Gitea. This feature allows you to create, update, delete, and query project boards, their columns, and associated issues.

## API Version

- **Base Path**: `/api/v1`
- **Gitea Version**: 1.25.1+
- **Authentication**: Required for most operations (OAuth2 or Personal Access Token)

## Table of Contents

- [Authentication](#authentication)
- [Projects](#projects)
  - [List Projects](#list-projects)
  - [Get Project](#get-project)
  - [Create Project](#create-project)
  - [Update Project](#update-project)
  - [Delete Project](#delete-project)
- [Project Columns](#project-columns)
  - [List Columns](#list-columns)
  - [Create Column](#create-column)
  - [Update Column](#update-column)
  - [Delete Column](#delete-column)
- [Issues](#issues)
  - [Add Issue to Column](#add-issue-to-column)
- [Error Codes](#error-codes)
- [Examples](#examples)

---

## Authentication

All API requests require authentication except for listing public projects. You can authenticate using:

1. **Personal Access Token** (recommended)
   ```bash
   curl -H "Authorization: token YOUR_TOKEN" https://gitea.example.com/api/v1/...
   ```

2. **Basic Authentication**
   ```bash
   curl -u username:password https://gitea.example.com/api/v1/...
   ```

### Required Permissions

| Operation | Required Permission |
|-----------|-------------------|
| List/Get Projects | Read access to repository |
| Create/Update/Delete Projects | Write access to repository projects |
| Manage Columns | Write access to repository projects |
| Add Issues to Columns | Write access to repository projects |

---

## Projects

### List Projects

List all projects in a repository.

**Endpoint**: `GET /repos/{owner}/{repo}/projects`

**Parameters**:

| Parameter | Type | Location | Description | Default |
|-----------|------|----------|-------------|---------|
| owner | string | path | Repository owner (username or organization) | Required |
| repo | string | path | Repository name | Required |
| state | string | query | Filter by state: `open`, `closed`, `all` | `open` |
| page | integer | query | Page number of results | 1 |
| limit | integer | query | Page size of results | 30 |

**Response**: `200 OK`

```json
[
  {
    "id": 1,
    "title": "Sprint Planning",
    "description": "Project for sprint planning and tracking",
    "owner_id": 0,
    "repo_id": 123,
    "creator_id": 1,
    "is_closed": false,
    "template_type": 1,
    "card_type": 1,
    "type": 2,
    "num_open_issues": 5,
    "num_closed_issues": 3,
    "num_issues": 8,
    "created": "2025-01-01T00:00:00Z",
    "updated": "2025-01-15T10:30:00Z",
    "url": "https://gitea.example.com/user/repo/projects/1"
  }
]
```

**Example**:

```bash
curl -H "Authorization: token YOUR_TOKEN" \
  "https://gitea.example.com/api/v1/repos/owner/repo/projects?state=open&limit=10"
```

---

### Get Project

Get a single project by ID.

**Endpoint**: `GET /repos/{owner}/{repo}/projects/{id}`

**Parameters**:

| Parameter | Type | Location | Description |
|-----------|------|----------|-------------|
| owner | string | path | Repository owner |
| repo | string | path | Repository name |
| id | integer | path | Project ID |

**Response**: `200 OK`

```json
{
  "id": 1,
  "title": "Sprint Planning",
  "description": "Project for sprint planning and tracking",
  "owner_id": 0,
  "repo_id": 123,
  "creator_id": 1,
  "is_closed": false,
  "template_type": 1,
  "card_type": 1,
  "type": 2,
  "num_open_issues": 5,
  "num_closed_issues": 3,
  "num_issues": 8,
  "created": "2025-01-01T00:00:00Z",
  "updated": "2025-01-15T10:30:00Z",
  "url": "https://gitea.example.com/user/repo/projects/1"
}
```

**Error Responses**:
- `404 Not Found` - Project not found or no access

**Example**:

```bash
curl -H "Authorization: token YOUR_TOKEN" \
  "https://gitea.example.com/api/v1/repos/owner/repo/projects/1"
```

---

### Create Project

Create a new project in a repository.

**Endpoint**: `POST /repos/{owner}/{repo}/projects`

**Parameters**:

| Parameter | Type | Location | Description |
|-----------|------|----------|-------------|
| owner | string | path | Repository owner |
| repo | string | path | Repository name |

**Request Body**:

```json
{
  "title": "Sprint Planning",
  "description": "Project for sprint planning and tracking",
  "template_type": 1,
  "card_type": 1
}
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| title | string | Yes | Project title |
| description | string | No | Project description |
| template_type | integer | No | Template type: `0`=none, `1`=basic_kanban, `2`=bug_triage |
| card_type | integer | No | Card type: `0`=text_only, `1`=images_and_text |

**Response**: `201 Created`

Returns the newly created project object.

**Error Responses**:
- `400 Bad Request` - Invalid input
- `403 Forbidden` - No write permission
- `422 Unprocessable Entity` - Validation failed

**Example**:

```bash
curl -X POST \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Q1 2025 Roadmap",
    "description": "Planning for Q1 2025",
    "template_type": 1,
    "card_type": 1
  }' \
  "https://gitea.example.com/api/v1/repos/owner/repo/projects"
```

---

### Update Project

Update an existing project.

**Endpoint**: `PATCH /repos/{owner}/{repo}/projects/{id}`

**Parameters**:

| Parameter | Type | Location | Description |
|-----------|------|----------|-------------|
| owner | string | path | Repository owner |
| repo | string | path | Repository name |
| id | integer | path | Project ID |

**Request Body**:

```json
{
  "title": "Updated Sprint Planning",
  "description": "Updated description",
  "card_type": 0,
  "is_closed": false
}
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| title | string | No | New project title |
| description | string | No | New project description |
| card_type | integer | No | New card type |
| is_closed | boolean | No | Whether to close/open the project |

**Note**: All fields are optional. Only provided fields will be updated.

**Response**: `200 OK`

Returns the updated project object.

**Error Responses**:
- `404 Not Found` - Project not found
- `403 Forbidden` - No write permission
- `422 Unprocessable Entity` - Validation failed

**Example**:

```bash
curl -X PATCH \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Q1 2025 Roadmap - Updated",
    "is_closed": true
  }' \
  "https://gitea.example.com/api/v1/repos/owner/repo/projects/1"
```

---

### Delete Project

Delete a project and all its columns.

**Endpoint**: `DELETE /repos/{owner}/{repo}/projects/{id}`

**Parameters**:

| Parameter | Type | Location | Description |
|-----------|------|----------|-------------|
| owner | string | path | Repository owner |
| repo | string | path | Repository name |
| id | integer | path | Project ID |

**Response**: `204 No Content`

**Error Responses**:
- `404 Not Found` - Project not found
- `403 Forbidden` - No write permission

**Example**:

```bash
curl -X DELETE \
  -H "Authorization: token YOUR_TOKEN" \
  "https://gitea.example.com/api/v1/repos/owner/repo/projects/1"
```

---

## Project Columns

### List Columns

List all columns in a project.

**Endpoint**: `GET /repos/{owner}/{repo}/projects/{id}/columns`

**Parameters**:

| Parameter | Type | Location | Description | Default |
|-----------|------|----------|-------------|---------|
| owner | string | path | Repository owner | Required |
| repo | string | path | Repository name | Required |
| id | integer | path | Project ID | Required |
| page | integer | query | Page number | 1 |
| limit | integer | query | Page size | 30 |

**Response**: `200 OK`

```json
[
  {
    "id": 1,
    "title": "To Do",
    "default": true,
    "sorting": 0,
    "color": "#28a745",
    "project_id": 1,
    "creator_id": 1,
    "num_issues": 5,
    "created": "2025-01-01T00:00:00Z",
    "updated": "2025-01-15T10:30:00Z"
  },
  {
    "id": 2,
    "title": "In Progress",
    "default": false,
    "sorting": 1,
    "color": "#0366d6",
    "project_id": 1,
    "creator_id": 1,
    "num_issues": 3,
    "created": "2025-01-01T00:00:00Z",
    "updated": "2025-01-15T10:30:00Z"
  }
]
```

**Example**:

```bash
curl -H "Authorization: token YOUR_TOKEN" \
  "https://gitea.example.com/api/v1/repos/owner/repo/projects/1/columns"
```

---

### Create Column

Create a new column in a project.

**Endpoint**: `POST /repos/{owner}/{repo}/projects/{id}/columns`

**Parameters**:

| Parameter | Type | Location | Description |
|-----------|------|----------|-------------|
| owner | string | path | Repository owner |
| repo | string | path | Repository name |
| id | integer | path | Project ID |

**Request Body**:

```json
{
  "title": "Done",
  "color": "#6f42c1"
}
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| title | string | Yes | Column title |
| color | string | No | Column color in hex format (e.g., `#FF0000`) |

**Response**: `201 Created`

Returns the newly created column object.

**Error Responses**:
- `404 Not Found` - Project not found
- `403 Forbidden` - No write permission
- `422 Unprocessable Entity` - Validation failed

**Example**:

```bash
curl -X POST \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Code Review",
    "color": "#f66a0a"
  }' \
  "https://gitea.example.com/api/v1/repos/owner/repo/projects/1/columns"
```

---

### Update Column

Update an existing project column.

**Endpoint**: `PATCH /repos/{owner}/{repo}/projects/columns/{id}`

**Parameters**:

| Parameter | Type | Location | Description |
|-----------|------|----------|-------------|
| owner | string | path | Repository owner |
| repo | string | path | Repository name |
| id | integer | path | Column ID |

**Request Body**:

```json
{
  "title": "Completed",
  "color": "#28a745",
  "sorting": 3
}
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| title | string | No | New column title |
| color | string | No | New column color |
| sorting | integer | No | New sorting order (0-based) |

**Response**: `200 OK`

Returns the updated column object.

**Example**:

```bash
curl -X PATCH \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Verified & Done",
    "sorting": 4
  }' \
  "https://gitea.example.com/api/v1/repos/owner/repo/projects/columns/5"
```

---

### Delete Column

Delete a project column. Issues in the column will be unlinked from the project.

**Endpoint**: `DELETE /repos/{owner}/{repo}/projects/columns/{id}`

**Parameters**:

| Parameter | Type | Location | Description |
|-----------|------|----------|-------------|
| owner | string | path | Repository owner |
| repo | string | path | Repository name |
| id | integer | path | Column ID |

**Response**: `204 No Content`

**Error Responses**:
- `404 Not Found` - Column not found
- `403 Forbidden` - No write permission

**Example**:

```bash
curl -X DELETE \
  -H "Authorization: token YOUR_TOKEN" \
  "https://gitea.example.com/api/v1/repos/owner/repo/projects/columns/5"
```

---

## Issues

### Add Issue to Column

Add an existing issue to a project column.

**Endpoint**: `POST /repos/{owner}/{repo}/projects/columns/{id}/issues`

**Parameters**:

| Parameter | Type | Location | Description |
|-----------|------|----------|-------------|
| owner | string | path | Repository owner |
| repo | string | path | Repository name |
| id | integer | path | Column ID |

**Request Body**:

```json
{
  "issue_id": 42
}
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| issue_id | integer | Yes | Issue ID to add to the column |

**Response**: `201 Created`

**Note**: If the issue is already in the project but in a different column, it will be moved to the specified column.

**Error Responses**:
- `404 Not Found` - Column or issue not found
- `403 Forbidden` - No write permission
- `422 Unprocessable Entity` - Invalid issue ID

**Example**:

```bash
curl -X POST \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "issue_id": 42
  }' \
  "https://gitea.example.com/api/v1/repos/owner/repo/projects/columns/2/issues"
```

---

## Error Codes

| HTTP Code | Description |
|-----------|-------------|
| 200 | OK - Request succeeded |
| 201 | Created - Resource successfully created |
| 204 | No Content - Request succeeded, no content to return |
| 400 | Bad Request - Invalid request format |
| 401 | Unauthorized - Authentication required |
| 403 | Forbidden - No permission to access resource |
| 404 | Not Found - Resource not found |
| 422 | Unprocessable Entity - Validation failed |
| 500 | Internal Server Error - Server error |

---

## Examples

### Complete Workflow Example

This example demonstrates creating a complete project board with columns and adding issues:

```bash
#!/bin/bash

TOKEN="your_token_here"
BASE_URL="https://gitea.example.com/api/v1"
OWNER="myuser"
REPO="myrepo"

# 1. Create a new project
PROJECT_ID=$(curl -s -X POST \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Sprint 1",
    "description": "First sprint planning",
    "template_type": 1,
    "card_type": 1
  }' \
  "$BASE_URL/repos/$OWNER/$REPO/projects" | jq -r '.id')

echo "Created project ID: $PROJECT_ID"

# 2. Create columns
COLUMN_TODO=$(curl -s -X POST \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "To Do",
    "color": "#d73a4a"
  }' \
  "$BASE_URL/repos/$OWNER/$REPO/projects/$PROJECT_ID/columns" | jq -r '.id')

COLUMN_PROGRESS=$(curl -s -X POST \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "In Progress",
    "color": "#0366d6"
  }' \
  "$BASE_URL/repos/$OWNER/$REPO/projects/$PROJECT_ID/columns" | jq -r '.id')

COLUMN_DONE=$(curl -s -X POST \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Done",
    "color": "#28a745"
  }' \
  "$BASE_URL/repos/$OWNER/$REPO/projects/$PROJECT_ID/columns" | jq -r '.id')

echo "Created columns: To Do ($COLUMN_TODO), In Progress ($COLUMN_PROGRESS), Done ($COLUMN_DONE)"

# 3. Add issues to columns
curl -X POST \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"issue_id": 1}' \
  "$BASE_URL/repos/$OWNER/$REPO/projects/columns/$COLUMN_TODO/issues"

curl -X POST \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"issue_id": 2}' \
  "$BASE_URL/repos/$OWNER/$REPO/projects/columns/$COLUMN_PROGRESS/issues"

echo "Added issues to columns"

# 4. List all projects
curl -H "Authorization: token $TOKEN" \
  "$BASE_URL/repos/$OWNER/$REPO/projects?state=open"

# 5. Close the project when done
curl -X PATCH \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"is_closed": true}' \
  "$BASE_URL/repos/$OWNER/$REPO/projects/$PROJECT_ID"

echo "Sprint completed and closed"
```

### Python Example

```python
import requests

class GiteaProjectAPI:
    def __init__(self, base_url, token):
        self.base_url = base_url.rstrip('/')
        self.headers = {
            'Authorization': f'token {token}',
            'Content-Type': 'application/json'
        }

    def create_project(self, owner, repo, title, description="", template_type=1):
        """Create a new project board"""
        url = f"{self.base_url}/api/v1/repos/{owner}/{repo}/projects"
        data = {
            "title": title,
            "description": description,
            "template_type": template_type,
            "card_type": 1
        }
        response = requests.post(url, json=data, headers=self.headers)
        response.raise_for_status()
        return response.json()

    def list_projects(self, owner, repo, state="open"):
        """List all projects in a repository"""
        url = f"{self.base_url}/api/v1/repos/{owner}/{repo}/projects"
        params = {"state": state}
        response = requests.get(url, params=params, headers=self.headers)
        response.raise_for_status()
        return response.json()

    def create_column(self, owner, repo, project_id, title, color=None):
        """Create a new column in a project"""
        url = f"{self.base_url}/api/v1/repos/{owner}/{repo}/projects/{project_id}/columns"
        data = {"title": title}
        if color:
            data["color"] = color
        response = requests.post(url, json=data, headers=self.headers)
        response.raise_for_status()
        return response.json()

    def add_issue_to_column(self, owner, repo, column_id, issue_id):
        """Add an issue to a project column"""
        url = f"{self.base_url}/api/v1/repos/{owner}/{repo}/projects/columns/{column_id}/issues"
        data = {"issue_id": issue_id}
        response = requests.post(url, json=data, headers=self.headers)
        response.raise_for_status()
        return response.status_code == 201

# Usage example
api = GiteaProjectAPI("https://gitea.example.com", "your_token_here")

# Create a project
project = api.create_project("owner", "repo", "Sprint Planning Q1 2025")
print(f"Created project: {project['id']}")

# Create columns
todo = api.create_column("owner", "repo", project['id'], "To Do", "#d73a4a")
progress = api.create_column("owner", "repo", project['id'], "In Progress", "#0366d6")
done = api.create_column("owner", "repo", project['id'], "Done", "#28a745")

# Add issues
api.add_issue_to_column("owner", "repo", todo['id'], 1)
api.add_issue_to_column("owner", "repo", progress['id'], 2)

# List all projects
projects = api.list_projects("owner", "repo")
print(f"Total projects: {len(projects)}")
```

---

## Data Models

### Project Object

```typescript
interface Project {
  id: number;                    // Unique identifier
  title: string;                 // Project title
  description: string;           // Project description
  owner_id: number;              // Owner ID (for org/user projects)
  repo_id: number;               // Repository ID (for repo projects)
  creator_id: number;            // Creator user ID
  is_closed: boolean;            // Whether project is closed
  template_type: number;         // 0=none, 1=basic_kanban, 2=bug_triage
  card_type: number;             // 0=text_only, 1=images_and_text
  type: number;                  // 1=individual, 2=repository, 3=organization
  num_open_issues: number;       // Count of open issues
  num_closed_issues: number;     // Count of closed issues
  num_issues: number;            // Total issue count
  created: string;               // ISO 8601 datetime
  updated: string;               // ISO 8601 datetime
  closed_date?: string;          // ISO 8601 datetime (if closed)
  url: string;                   // Project URL
}
```

### ProjectColumn Object

```typescript
interface ProjectColumn {
  id: number;                    // Unique identifier
  title: string;                 // Column title
  default: boolean;              // Whether this is the default column
  sorting: number;               // Sorting order (0-based)
  color: string;                 // Hex color code (e.g., "#28a745")
  project_id: number;            // Parent project ID
  creator_id: number;            // Creator user ID
  num_issues: number;            // Number of issues in column
  created: string;               // ISO 8601 datetime
  updated: string;               // ISO 8601 datetime
}
```

---

## Best Practices

1. **Pagination**: Always use pagination for large datasets. Default limit is 30, maximum is 50.

2. **Error Handling**: Always check HTTP status codes and handle errors appropriately.

3. **Rate Limiting**: Be aware of API rate limits. Implement exponential backoff for retries.

4. **Authentication**: Store tokens securely. Never commit tokens to version control.

5. **Concurrent Updates**: Use optimistic locking or refresh data before updates to avoid conflicts.

6. **Sorting**: When creating multiple columns, they are automatically sorted. Use the `sorting` field in PATCH requests to reorder.

7. **Issue Management**: Before adding an issue to a column, ensure the issue exists in the repository.

8. **Project Lifecycle**:
   - Use `open` state for active projects
   - Close projects when sprints/milestones complete
   - Archive old projects by closing them

---

## FAQ

**Q: Can I move an issue between columns?**
A: Yes, simply add the issue to the new column using the "Add Issue to Column" endpoint. If the issue is already in the project, it will be moved.

**Q: What happens to issues when I delete a column?**
A: Issues are unlinked from the project but not deleted. They remain in the repository.

**Q: Can I have multiple projects in one repository?**
A: Yes, you can create as many projects as needed in a repository.

**Q: Are project boards visible to all repository members?**
A: Visibility follows repository permissions. Users with read access can view projects, write access can modify them.

**Q: What's the difference between template types?**
A:
- `0` (none): Empty project, manually create columns
- `1` (basic_kanban): Auto-creates "To Do", "In Progress", "Done" columns
- `2` (bug_triage): Auto-creates "Needs Triage", "High Priority", "Low Priority", "Closed" columns

**Q: Can I customize column colors?**
A: Yes, use any valid hex color code (e.g., `#FF0000` for red) when creating or updating columns.

---

## Changelog

### Version 1.25.1-kysion (2025-11-22)
- Initial release of Project Board API
- Complete CRUD operations for projects and columns
- Issue assignment to columns
- Support for project templates and card types
- Full Swagger documentation

---

## Support & Contributing

For bug reports and feature requests, please open an issue on the [Gitea repository](https://github.com/go-gitea/gitea).

For questions and discussions, visit the [Gitea forum](https://discourse.gitea.io/).

---

## License

This API documentation is part of Gitea and is licensed under the MIT License.
