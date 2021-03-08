// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/docker/distribution/notifications"
	"github.com/docker/distribution/registry/auth/token"
)

// DockerPluginLogin token service for docker registry
func DockerPluginLogin(ctx *context.Context) {
	if !setting.HasDockerPlugin() {
		ctx.Status(404)
		return
	}

	// handle params
	service := ctx.Query("service")
	if service != setting.Docker.ServiceName {
		ctx.Status(http.StatusBadRequest)
		return
	}

	scope := ctx.Query("scope")
	if len(scope) == 0 {
		if ctx.User == nil {
			ctx.Status(http.StatusUnauthorized)
			return
		}
		// Authentication-only request ("docker login"), pass through.
		handleResult(ctx, []authzResult{})
		return
	}

	scops, err := handleScopes(scope)
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	results := handlePermission(ctx, scops)
	if ctx.Written() {
		return
	}

	handleResult(ctx, results)
}

func handlePermission(ctx *context.Context, scops []authScope) []authzResult {
	rs := make([]authzResult, 0, len(scops))
	repos := make(map[string]*models.Repository)
	repoPerms := make(map[int64]models.Permission)

	for _, scop := range scops {
		hasPush := false
		hasPull := false
		needCreate := false
		for _, act := range scop.Actions {
			if act == "push" {
				hasPush = true
				continue
			}
			if act == "pull" {
				hasPull = true
			}
		}
		if hasPush && ctx.User == nil {
			continue
		}
		splits := strings.SplitN(scop.Name, "/", 3)
		var (
			owner    string
			repoName string
			image    string
		)
		if len(splits) >= 2 {
			owner = splits[0]
			repoName = splits[1]
			if len(splits) == 3 {
				image = splits[2]
			} else {
				image = repoName
			}
		}

		repo, has := repos[owner+"/"+repoName]
		var err error
		if !has {
			repo, err = models.GetRepositoryByOwnerAndName(owner, repoName)
			if err != nil {
				if models.IsErrRepoNotExist(err) {
					continue
				}
				ctx.Error(http.StatusInternalServerError, "handlePermission")
				log.Error("docker: GetRepositoryByOwnerAndName: %v", err)
				return nil
			}
			repos[owner+"/"+repoName] = repo
		}

		_, err = models.GetPackage(repo.ID, models.PackageTypeDockerImage, image)
		if err != nil {
			if !models.IsErrPackageNotExist(err) {
				ctx.Error(http.StatusInternalServerError, "handlePermission")
				log.Error("docker: GetPackage: %v", err)
				return nil
			}
			needCreate = true
		}

		perm, has := repoPerms[repo.ID]
		if !has {
			perm, err = models.GetUserRepoPermission(repo, ctx.User)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "handlePermission")
				log.Error("docker: GetUserRepoPermission: %v", err)
				return nil
			}
			repoPerms[repo.ID] = perm
		}

		events := make([]string, 0, 2)
		accessMode := models.AccessModeRead
		if hasPush {
			accessMode = models.AccessModeRead
			if needCreate {
				accessMode = models.AccessModeAdmin
			}
		}
		if perm.CanAccess(accessMode, models.UnitTypePackages) {
			if hasPush {
				events = append(events, "push")
			}
			if hasPull {
				events = append(events, "pull")
			}
		}

		if len(events) == 0 {
			continue
		}

		rs = append(rs, authzResult{
			scope:            scop,
			autorizedActions: events,
		})
	}

	return rs
}

var resourceTypeRegex = regexp.MustCompile(`([a-z0-9]+)(\([a-z0-9]+\))?`)

func handleResult(ctx *context.Context, ares []authzResult) {
	account := ""
	if ctx.User != nil {
		account = ctx.User.Name
	}
	token, err := generateToken(account, ares)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "generateToken")
		log.Error("Failed to generate token: %v", err)
		return
	}

	result, _ := json.Marshal(&map[string]string{"access_token": token, "token": token})
	ctx.Header().Set("Content-Type", "application/json")
	_, err = ctx.Write(result)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "generateToken")
		log.Error("Failed to response token: %v", err)
	}
}

type authScope struct {
	Type    string
	Class   string
	Name    string
	Actions []string
}

func handleScopes(scope string) ([]authScope, error) {
	scopes := strings.Split(scope, " ")
	rs := make([]authScope, 0, len(scopes))
	for _, scopeStr := range scopes {
		parts := strings.Split(scopeStr, ":")
		var scope authScope

		scopeType, scopeClass, err := parseResourceType(parts[0])
		if err != nil {
			return nil, err
		}

		switch len(parts) {
		case 3:
			scope = authScope{
				Type:    scopeType,
				Class:   scopeClass,
				Name:    parts[1],
				Actions: strings.Split(parts[2], ","),
			}
		case 4:
			scope = authScope{
				Type:    scopeType,
				Class:   scopeClass,
				Name:    parts[1] + ":" + parts[2],
				Actions: strings.Split(parts[3], ","),
			}
		default:
			return nil, fmt.Errorf("invalid scope: %q", scopeStr)
		}
		sort.Strings(scope.Actions)
		rs = append(rs, scope)
	}

	return rs, nil
}

func parseResourceType(scope string) (string, string, error) {
	parts := resourceTypeRegex.FindStringSubmatch(scope)
	if parts == nil {
		return "", "", fmt.Errorf("malformed scope request")
	}

	switch len(parts) {
	case 3:
		return parts[1], "", nil
	case 4:
		return parts[1], parts[3], nil
	default:
		return "", "", fmt.Errorf("malformed scope request")
	}
}

type authzResult struct {
	scope            authScope
	autorizedActions []string
}

func generateToken(account string, ares []authzResult) (string, error) {
	now := time.Now().Unix()

	// Sign something dummy to find out which algorithm is used.
	_, sigAlg, err := setting.Docker.PrivateKey.Sign(strings.NewReader("dummy"), 0)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %s", err)
	}
	header := token.Header{
		Type:       "JWT",
		SigningAlg: sigAlg,
		KeyID:      setting.Docker.PublicKey.KeyID(),
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal header: %s", err)
	}

	claims := token.ClaimSet{
		Issuer:     setting.Docker.IssuerName,
		Subject:    account,
		Audience:   setting.Docker.ServiceName,
		NotBefore:  now - 10,
		IssuedAt:   now,
		Expiration: now + setting.Docker.Expiration,
		JWTID:      fmt.Sprintf("%d", rand.Int63()),
		Access:     []*token.ResourceActions{},
	}
	for _, a := range ares {
		ra := &token.ResourceActions{
			Type:    a.scope.Type,
			Name:    a.scope.Name,
			Actions: a.autorizedActions,
		}
		if ra.Actions == nil {
			ra.Actions = []string{}
		}
		sort.Strings(ra.Actions)
		claims.Access = append(claims.Access, ra)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %s", err)
	}

	payload := fmt.Sprintf("%s%s%s", joseBase64UrlEncode(headerJSON), token.TokenSeparator, joseBase64UrlEncode(claimsJSON))

	sig, sigAlg2, err := setting.Docker.PrivateKey.Sign(strings.NewReader(payload), 0)
	if err != nil || sigAlg2 != sigAlg {
		return "", fmt.Errorf("failed to sign token: %s", err)
	}
	return fmt.Sprintf("%s%s%s", payload, token.TokenSeparator, joseBase64UrlEncode(sig)), nil
}

// Copy-pasted from libtrust where it is private.
func joseBase64UrlEncode(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

// DockerPluginEvent event service for docker registry
func DockerPluginEvent(ctx *context.Context) {
	if !setting.HasDockerPlugin() {
		ctx.Status(404)
		return
	}

	token := ctx.Req.Header.Get("X-Token")
	if token != setting.Docker.NotifyToken {
		ctx.Error(http.StatusUnauthorized)
		return
	}

	decoder := json.NewDecoder(ctx.Req.Body)
	decoder.DisallowUnknownFields()

	data := new(notifications.Envelope)

	err := decoder.Decode(data)
	if err != nil {
		ctx.Error(http.StatusInternalServerError)
		log.Error("Failed to unmarshal event: %v", err)
	}

	// handle events
	repos := make(map[string]*models.Repository)
	pkgs := make(map[string]*models.Package)
	for _, event := range data.Events {
		if event.Action == notifications.EventActionPush {
			var (
				owner    string
				repoName string
				image    string
			)
			splits := strings.SplitN(event.Target.Repository, "/", 3)
			if len(splits) < 2 {
				continue
			}
			owner = splits[0]
			repoName = splits[1]
			if len(splits) == 3 {
				image = splits[2]
			} else {
				image = repoName
			}

			repo, has := repos[owner+"/"+repoName]
			if !has {
				repo, err = models.GetRepositoryByOwnerAndName(owner, repoName)
				if err != nil {
					if models.IsErrRepoNotExist(err) {
						continue
					}
					ctx.Error(http.StatusInternalServerError, "DockerPluginEvent")
					log.Error("docker: GetRepositoryByOwnerAndName: %v", err)
					return
				}
				repos[owner+"/"+repoName] = repo
			}

			pkg, has := pkgs[event.Target.Repository]
			if !has {
				pkg, err = models.GetPackage(repo.ID, models.PackageTypeDockerImage, image)
				if err != nil {
					if models.IsErrPackageNotExist(err) {
						// create a new pkg
						err = models.AddPackage(models.AddPackageOptions{
							Repo: repo,
							Name: image,
							Type: models.PackageTypeDockerImage,
						})
						if err != nil {
							ctx.Error(http.StatusInternalServerError, "DockerPluginEvent")
							log.Error("docker: AddPackage: %v", err)
							return
						}
						continue
					}
					ctx.Error(http.StatusInternalServerError, "DockerPluginEvent")
					log.Error("docker: GetPackage: %v", err)
					return
				}
				if pkg != nil {
					pkgs[event.Target.Repository] = pkg
				}
			}

			// update update time
			pkg.UpdatedUnix = timeutil.TimeStamp(event.Timestamp.Unix())
			err = pkg.UpdateCols("updated_unix")
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "DockerPluginEvent")
				log.Error("docker: UpdateCols(updated_unix): %v", err)
				return
			}
		}
	}

	ctx.JSON(200, map[string]string{
		"result": "ok",
	})
}
