// Package actions provides middleware for Actions token scope verification.
// Middleware to enforce token permissions. Modified by LAC | Ludwig investing
package actions

import (
	"net/http"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
	"github.com/golang-jwt/jwt/v4"
)

// ScopeChecker is a middleware that verifies the Actions token has the required scope for the API endpoint.
// Added to enforce token permissions. Modified by LAC | Ludwig investing
func ScopeChecker(requiredScope actions_model.Scope) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		tokenString := getTokenFromHeader(ctx)
		if tokenString == "" {
			ctx.Error(http.StatusUnauthorized, "Missing token")
			return
		}
		claims, err := parseToken(tokenString)
		if err != nil {
			ctx.Error(http.StatusUnauthorized, "Invalid token", err)
			return
		}
		perms, ok := claims["permissions"].(map[string]interface{})
		if !ok {
			ctx.Error(http.StatusForbidden, "No permissions in token")
			return
		}
		permStr, ok := perms[string(requiredScope)].(string)
		if !ok {
			ctx.Error(http.StatusForbidden, "Missing scope permission")
			return
		}
		perm := actions_model.PermissionFromString(permStr)
		if perm == actions_model.PermissionNone {
			ctx.Error(http.StatusForbidden, "Insufficient permissions")
			return
		}
		// For write operations, require write permission
		if ctx.Req.Method == "POST" || ctx.Req.Method == "PUT" || ctx.Req.Method == "DELETE" || ctx.Req.Method == "PATCH" {
			if perm != actions_model.PermissionWrite {
				ctx.Error(http.StatusForbidden, "Write permission required")
				return
			}
		}
		// Token is valid, proceed
	}
}

// getTokenFromHeader extracts the token from the Authorization header.
func getTokenFromHeader(ctx *context.APIContext) string {
	auth := ctx.Req.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	return ""
}

// parseToken parses and validates the JWT token.
func parseToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Use the same secret as token generation
		return []byte("gitea-actions-secret"), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}
