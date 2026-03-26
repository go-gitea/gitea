// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

// This file documents the error response structure and common error codes
// for the Gitea API. These error responses are used across all API endpoints.

// ============================================================================
// ERROR RESPONSE STRUCTURE
// ============================================================================

// All error responses follow a consistent structure:
//
// {
//   "message": "Human-readable error message",
//   "url": "https://docs.gitea.com/api"
// }
//
// Some specific error types may include additional fields.

// ============================================================================
// COMMON HTTP STATUS CODES
// ============================================================================

// The following HTTP status codes are commonly returned by the API:
//
// - 200 OK: Request succeeded
// - 201 Created: Resource created successfully
// - 204 No Content: Request succeeded, no content returned
// - 400 Bad Request: Invalid request parameters or body
// - 401 Unauthorized: Authentication required or invalid credentials
// - 403 Forbidden: Authenticated but not authorized for this action
// - 404 Not Found: Requested resource not found
// - 409 Conflict: Request conflicts with current state
// - 422 Unprocessable Entity: Validation error
// - 500 Internal Server Error: Server-side error

// ============================================================================
// ERROR RESPONSE EXAMPLES
// ============================================================================

// Example: 400 Bad Request - Invalid parameter
// {
//   "message": "invalid argument: owner organisation is required for filtering on team",
//   "url": "https://docs.gitea.com/api"
// }

// Example: 401 Unauthorized - Invalid token
// {
//   "message": "token is required",
//   "url": "https://docs.gitea.com/api"
// }

// Example: 403 Forbidden - Insufficient permissions
// {
//   "message": "user does not have permission to access this resource",
//   "url": "https://docs.gitea.com/api"
// }

// Example: 404 Not Found - Resource not found
// {
//   "message": "repository not found",
//   "url": "https://docs.gitea.com/api"
// }

// Example: 422 Unprocessable Entity - Validation error
// {
//   "message": "title is required",
//   "url": "https://docs.gitea.com/api"
// }

// Example: 422 Unprocessable Entity - Invalid topics
// {
//   "message": "invalid topics",
//   "invalidTopics": ["invalid-topic-name"]
// }

// ============================================================================
// ERROR HANDLING BEST PRACTICES
// ============================================================================

// When consuming the Gitea API:
//
// 1. Always check the HTTP status code first to determine success/failure
// 2. Parse the response body as JSON to get the error message
// 3. Use the "message" field for display purposes only (do not parse it)
// 4. Handle each HTTP status code appropriately:
//    - 4xx errors: Fix the request and retry
//    - 5xx errors: Retry with exponential backoff or report to administrators
// 5. For validation errors (422), check which fields are invalid

// ============================================================================
// COMMON ERROR SCENARIOS BY ENDPOINT CATEGORY
// ============================================================================

// Repository Operations:
// - 404: Repository does not exist
// - 403: User lacks access to repository
// - 409: Repository name already exists (on create)
// - 422: Invalid repository name or settings

// Issue Operations:
// - 404: Issue or repository does not exist
// - 403: User lacks permission to modify issue
// - 422: Invalid issue data (missing title, invalid labels, etc.)

// Pull Request Operations:
// - 404: Pull request or repository does not exist
// - 403: User lacks permission to merge
// - 409: Merge conflict or PR already merged
// - 422: Invalid PR data or state transition

// User Operations:
// - 401: Invalid credentials
// - 403: Cannot modify other users
// - 404: User not found
// - 409: Username or email already exists
// - 422: Invalid user data

// Organization Operations:
// - 404: Organization not found
// - 403: User lacks organization permissions
// - 409: Organization name already exists
// - 422: Invalid organization data