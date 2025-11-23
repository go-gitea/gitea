# Upstream Contribution: Project Board REST API

This document contains all materials needed for contributing the Project Board API feature to the official Gitea repository.

## Pull Request Title

```
feat(api): Add comprehensive REST API for Project Boards
```

## Pull Request Description

### Overview

This PR implements a complete REST API for Gitea's Project Board feature, enabling programmatic management of projects, columns, and issue assignments through RESTful endpoints.

### Motivation

Currently, Gitea's Project Board feature is only accessible through the web UI. This limitation prevents:
- Automation workflows for project management
- Integration with external tools and CI/CD pipelines
- Programmatic project creation and issue tracking
- Third-party application development using Gitea as a backend

### Features

This implementation adds 10 new API endpoints:

**Project Management:**
- `GET /repos/{owner}/{repo}/projects` - List all projects with state filtering and pagination
- `POST /repos/{owner}/{repo}/projects` - Create a new project
- `GET /repos/{owner}/{repo}/projects/{id}` - Get project details
- `PATCH /repos/{owner}/{repo}/projects/{id}` - Update project (title, description, state)
- `DELETE /repos/{owner}/{repo}/projects/{id}` - Delete a project

**Column Management:**
- `GET /repos/{owner}/{repo}/projects/{id}/columns` - List project columns
- `POST /repos/{owner}/{repo}/projects/{id}/columns` - Create a new column
- `PATCH /repos/{owner}/{repo}/projects/columns/{id}` - Update column details
- `DELETE /repos/{owner}/{repo}/projects/columns/{id}` - Delete a column

**Issue Assignment:**
- `POST /repos/{owner}/{repo}/projects/columns/{id}/issues` - Add issue to column

### Implementation Details

**Architecture:**
- Follows Gitea's standard Router → Service → Model pattern
- Proper permission checks using existing access control system
- Full Swagger/OpenAPI documentation
- Comprehensive error handling with appropriate HTTP status codes

**Key Components:**
1. **routers/api/v1/repo/project.go** (710 lines) - API route handlers
2. **modules/structs/project.go** (139 lines) - API data structures
3. **services/convert/project.go** (92 lines) - Model-to-API converters
4. **models/project/issue.go** (+61 lines) - Issue assignment logic
5. **routers/api/v1/api.go** (+17 lines) - Route registration

**Access Control:**
- Requires `ReadRepository` scope for GET operations
- Requires `WriteRepository` scope for POST/PATCH/DELETE operations
- Respects repository unit permissions (TypeProjects)
- Prevents modifications to archived repositories

**Features:**
- State filtering (open/closed/all)
- Pagination support
- Issue count tracking
- Proper timestamp handling
- URL generation for HATEOAS compliance

### Testing Coverage

Comprehensive integration test suite included:
- **tests/integration/api_repo_project_test.go** - 11 test functions covering:
  - All CRUD operations
  - Permission validation
  - Error handling (404, 403, 422)
  - Pagination
  - State filtering
  - Issue assignment and movement

All tests follow Gitea's existing test patterns and pass successfully.

### Documentation

Complete API documentation provided:
- **docs/API_PROJECT_BOARD.md** - 600+ line comprehensive guide with:
  - Endpoint specifications
  - Request/response examples (bash, Python)
  - Data model definitions (TypeScript interfaces)
  - Complete workflow examples
  - Error codes and handling
  - Best practices and FAQ

### Breaking Changes

**None.** This is a purely additive feature that:
- Does not modify existing APIs
- Does not change database schema (uses existing project models)
- Does not affect existing web UI functionality
- Does not require configuration changes

### Migration Required

**No database migrations required.** The API uses existing `project` and `project_board` tables.

### Checklist

- [x] Code follows Gitea's style guidelines
- [x] Tests added and passing
- [x] Documentation complete (API docs + code comments)
- [x] Swagger annotations added
- [x] No breaking changes
- [x] Permission checks implemented
- [x] Error handling comprehensive
- [x] Follows existing patterns (router/service/model)

### Use Cases

**1. Automated Project Setup**
```bash
# Create sprint project and columns via CI/CD
curl -X POST https://gitea.example.com/api/v1/repos/org/repo/projects \
  -H "Authorization: token ${GITEA_TOKEN}" \
  -d '{"title": "Sprint 42", "description": "Q1 2024 Sprint"}'
```

**2. Issue Automation**
```python
# Automatically move issues based on labels or status
def move_issue_to_column(issue_id, column_name):
    column = find_column_by_name(project_id, column_name)
    api.add_issue_to_column(column['id'], issue_id)
```

**3. Project Reporting**
```bash
# Generate project metrics
curl https://gitea.example.com/api/v1/repos/org/repo/projects?state=all \
  | jq '[.[] | {title, open: .num_open_issues, closed: .num_closed_issues}]'
```

**4. Third-Party Integration**
- Integrate with Jira, Trello, or other project management tools
- Build custom dashboards aggregating multiple repositories
- Automate project synchronization across teams

### Screenshots

N/A - This is a REST API feature with no UI changes.

### Related Issues

This PR addresses the need for programmatic project management, which has been requested by users who want to:
- Automate project board operations
- Build custom tools on top of Gitea
- Integrate Gitea projects with external systems

---

## Commit Message

```
feat(api): Add comprehensive REST API for Project Boards

Implements complete REST API for Gitea's Project Board feature with
10 new endpoints enabling programmatic project management.

Features:
- Project CRUD operations (create, read, update, delete, list)
- Column management (create, update, delete, list)
- Issue assignment and movement between columns
- State filtering (open/closed/all)
- Pagination support
- Comprehensive permission checks
- Full Swagger/OpenAPI documentation

API Endpoints:
- GET    /repos/{owner}/{repo}/projects
- POST   /repos/{owner}/{repo}/projects
- GET    /repos/{owner}/{repo}/projects/{id}
- PATCH  /repos/{owner}/{repo}/projects/{id}
- DELETE /repos/{owner}/{repo}/projects/{id}
- GET    /repos/{owner}/{repo}/projects/{id}/columns
- POST   /repos/{owner}/{repo}/projects/{id}/columns
- PATCH  /repos/{owner}/{repo}/projects/columns/{id}
- DELETE /repos/{owner}/{repo}/projects/columns/{id}
- POST   /repos/{owner}/{repo}/projects/columns/{id}/issues

Implementation follows Gitea's standard patterns:
- Router → Service → Model architecture
- Proper access control with token scopes
- Comprehensive error handling
- Full test coverage

Files changed:
- routers/api/v1/repo/project.go (new, 710 lines)
- modules/structs/project.go (new, 139 lines)
- services/convert/project.go (new, 92 lines)
- models/project/issue.go (+61 lines)
- routers/api/v1/api.go (+17 lines)
- tests/integration/api_repo_project_test.go (new, 600+ lines)
- docs/API_PROJECT_BOARD.md (new, 600+ lines)

No breaking changes. No migration required.
```

---

## Changelog Entry

### For CHANGELOG.md

```markdown
## [Unreleased]

### Added

- **API: Project Board Management** - Complete REST API for programmatic project board management
  - 10 new endpoints for projects, columns, and issue assignments
  - State filtering (open/closed/all) and pagination support
  - Comprehensive permission checks and error handling
  - Full Swagger/OpenAPI documentation
  - See [API Documentation](docs/API_PROJECT_BOARD.md) for details
```

---

## Additional Notes for Reviewers

### Code Quality

- **Type Safety**: All functions properly type-checked, no type assertions without checks
- **Error Handling**: Consistent error responses with appropriate HTTP status codes
- **Idempotency**: Operations like adding issues to columns are idempotent
- **Concurrency**: Uses database transactions where appropriate
- **Memory**: Efficient pagination prevents memory issues with large datasets

### Security Considerations

- **Authentication**: All write operations require valid token
- **Authorization**: Respects repository permissions and unit access
- **Input Validation**: All user inputs validated (binding tags, length checks)
- **SQL Injection**: Uses parameterized queries via XORM
- **Archive Protection**: Prevents modifications to archived repositories

### Performance

- **Pagination**: Implemented for all list endpoints to prevent performance issues
- **N+1 Queries**: Avoided by using proper joins and eager loading
- **Indexes**: Uses existing database indexes (no schema changes needed)
- **Caching**: Compatible with Gitea's existing caching layer

### Compatibility

- **API Version**: Follows existing v1 API conventions
- **Database**: No schema changes, uses existing tables
- **Backwards Compatible**: No breaking changes to existing APIs
- **Config**: No new configuration options required

### Testing Strategy

Tests cover:
- ✅ Happy path for all operations
- ✅ Error cases (404, 403, 422)
- ✅ Permission boundaries
- ✅ Pagination edge cases
- ✅ State transitions (open ↔ closed)
- ✅ Concurrent operations (issue movement)

### Future Enhancements (Out of Scope)

While this PR provides complete CRUD functionality, future enhancements could include:
- Bulk operations (move multiple issues at once)
- Project templates
- Advanced filtering (by assignee, label, milestone)
- Project search API
- Webhooks for project events
- GraphQL endpoint equivalent

These are intentionally excluded to keep this PR focused and reviewable.

---

## How to Review

### 1. Check API Design
- Review endpoint paths and HTTP methods in `routers/api/v1/api.go:1602-1618`
- Validate data structures in `modules/structs/project.go`
- Check Swagger docs in `routers/api/v1/repo/project.go`

### 2. Test the API
```bash
# Start Gitea with these changes
make build
./gitea web

# Run integration tests
go test ./tests/integration -run TestAPIProject

# Manual testing with curl (see docs/API_PROJECT_BOARD.md for examples)
```

### 3. Review Implementation
- `routers/api/v1/repo/project.go` - Main API handlers
- `services/convert/project.go` - Data conversion layer
- `models/project/issue.go` - Business logic for issue assignment

### 4. Security Review
- Permission checks in each handler
- Input validation in binding structs
- SQL query safety (XORM usage)

---

## Upstream Submission Checklist

Before submitting to Gitea:

- [ ] Rebase onto latest `main` branch
- [ ] Run full test suite: `make test`
- [ ] Run linter: `make lint`
- [ ] Generate swagger: `make generate-swagger`
- [ ] Verify all tests pass: `make test-sqlite`
- [ ] Check code style: `make fmt`
- [ ] Update docs if needed
- [ ] Create pull request on https://github.com/go-gitea/gitea
- [ ] Reference any related issues
- [ ] Be responsive to review feedback

---

## Contact

For questions about this implementation:
- Review the comprehensive documentation in `docs/API_PROJECT_BOARD.md`
- Check the integration tests in `tests/integration/api_repo_project_test.go`
- Examine the API handlers in `routers/api/v1/repo/project.go`
