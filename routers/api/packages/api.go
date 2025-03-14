// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"net/http"
	"regexp"
	"strings"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/packages/alpine"
	"code.gitea.io/gitea/routers/api/packages/arch"
	"code.gitea.io/gitea/routers/api/packages/cargo"
	"code.gitea.io/gitea/routers/api/packages/chef"
	"code.gitea.io/gitea/routers/api/packages/composer"
	"code.gitea.io/gitea/routers/api/packages/conan"
	"code.gitea.io/gitea/routers/api/packages/conda"
	"code.gitea.io/gitea/routers/api/packages/container"
	"code.gitea.io/gitea/routers/api/packages/cran"
	"code.gitea.io/gitea/routers/api/packages/debian"
	"code.gitea.io/gitea/routers/api/packages/generic"
	"code.gitea.io/gitea/routers/api/packages/goproxy"
	"code.gitea.io/gitea/routers/api/packages/helm"
	"code.gitea.io/gitea/routers/api/packages/maven"
	"code.gitea.io/gitea/routers/api/packages/npm"
	"code.gitea.io/gitea/routers/api/packages/nuget"
	"code.gitea.io/gitea/routers/api/packages/pub"
	"code.gitea.io/gitea/routers/api/packages/pypi"
	"code.gitea.io/gitea/routers/api/packages/rpm"
	"code.gitea.io/gitea/routers/api/packages/rubygems"
	"code.gitea.io/gitea/routers/api/packages/swift"
	"code.gitea.io/gitea/routers/api/packages/vagrant"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/context"
)

func reqPackageAccess(accessMode perm.AccessMode) func(ctx *context.Context) {
	return func(ctx *context.Context) {
		if ctx.Data["IsApiToken"] == true {
			scope, ok := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
			if ok { // it's a personal access token but not oauth2 token
				scopeMatched := false
				var err error
				if accessMode == perm.AccessModeRead {
					scopeMatched, err = scope.HasScope(auth_model.AccessTokenScopeReadPackage)
					if err != nil {
						ctx.HTTPError(http.StatusInternalServerError, "HasScope", err.Error())
						return
					}
				} else if accessMode == perm.AccessModeWrite {
					scopeMatched, err = scope.HasScope(auth_model.AccessTokenScopeWritePackage)
					if err != nil {
						ctx.HTTPError(http.StatusInternalServerError, "HasScope", err.Error())
						return
					}
				}
				if !scopeMatched {
					ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="Gitea Package API"`)
					ctx.HTTPError(http.StatusUnauthorized, "reqPackageAccess", "user should have specific permission or be a site admin")
					return
				}

				// check if scope only applies to public resources
				publicOnly, err := scope.PublicOnly()
				if err != nil {
					ctx.HTTPError(http.StatusForbidden, "tokenRequiresScope", "parsing public resource scope failed: "+err.Error())
					return
				}

				if publicOnly {
					if ctx.Package != nil && ctx.Package.Owner.Visibility.IsPrivate() {
						ctx.HTTPError(http.StatusForbidden, "reqToken", "token scope is limited to public packages")
						return
					}
				}
			}
		}

		if ctx.Package.AccessMode < accessMode && !ctx.IsUserSiteAdmin() {
			ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="Gitea Package API"`)
			ctx.HTTPError(http.StatusUnauthorized, "reqPackageAccess", "user should have specific permission or be a site admin")
			return
		}
	}
}

func verifyAuth(r *web.Router, authMethods []auth.Method) {
	if setting.Service.EnableReverseProxyAuth {
		authMethods = append(authMethods, &auth.ReverseProxy{})
	}
	authGroup := auth.NewGroup(authMethods...)

	r.Use(func(ctx *context.Context) {
		var err error
		ctx.Doer, err = authGroup.Verify(ctx.Req, ctx.Resp, ctx, ctx.Session)
		if err != nil {
			log.Error("Failed to verify user: %v", err)
			ctx.HTTPError(http.StatusUnauthorized, "authGroup.Verify")
			return
		}
		ctx.IsSigned = ctx.Doer != nil
	})
}

// CommonRoutes provide endpoints for most package managers (except containers - see below)
// These are mounted on `/api/packages` (not `/api/v1/packages`)
func CommonRoutes() *web.Router {
	r := web.NewRouter()

	r.Use(context.PackageContexter())

	verifyAuth(r, []auth.Method{
		&auth.OAuth2{},
		&auth.Basic{},
		&nuget.Auth{},
		&conan.Auth{},
		&chef.Auth{},
	})

	r.Group("/{username}", func() {
		r.Group("/alpine", func() {
			r.Get("/key", alpine.GetRepositoryKey)
			r.Group("/{branch}/{repository}", func() {
				r.Put("", reqPackageAccess(perm.AccessModeWrite), alpine.UploadPackageFile)
				r.Group("/{architecture}", func() {
					r.Get("/APKINDEX.tar.gz", alpine.GetRepositoryFile)
					r.Group("/{filename}", func() {
						r.Get("", alpine.DownloadPackageFile)
						r.Delete("", reqPackageAccess(perm.AccessModeWrite), alpine.DeletePackageFile)
					})
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/arch", func() {
			r.Methods("HEAD,GET", "/repository.key", arch.GetRepositoryKey)
			r.Methods("PUT", "" /* no repository */, reqPackageAccess(perm.AccessModeWrite), arch.UploadPackageFile)
			r.PathGroup("/*", func(g *web.RouterPathGroup) {
				g.MatchPath("PUT", "/<repository:*>", reqPackageAccess(perm.AccessModeWrite), arch.UploadPackageFile)
				g.MatchPath("HEAD,GET", "/<repository:*>/<architecture>/<filename>", arch.GetPackageOrRepositoryFile)
				g.MatchPath("DELETE", "/<repository:*>/<name>/<version>/<architecture>", reqPackageAccess(perm.AccessModeWrite), arch.DeletePackageVersion)
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/cargo", func() {
			r.Group("/api/v1/crates", func() {
				r.Get("", cargo.SearchPackages)
				r.Put("/new", reqPackageAccess(perm.AccessModeWrite), cargo.UploadPackage)
				r.Group("/{package}", func() {
					r.Group("/{version}", func() {
						r.Get("/download", cargo.DownloadPackageFile)
						r.Delete("/yank", reqPackageAccess(perm.AccessModeWrite), cargo.YankPackage)
						r.Put("/unyank", reqPackageAccess(perm.AccessModeWrite), cargo.UnyankPackage)
					})
					r.Get("/owners", cargo.ListOwners)
				})
			})
			r.Get("/config.json", cargo.RepositoryConfig)
			r.Get("/1/{package}", cargo.EnumeratePackageVersions)
			r.Get("/2/{package}", cargo.EnumeratePackageVersions)
			// Use dummy placeholders because these parts are not of interest
			r.Get("/3/{_}/{package}", cargo.EnumeratePackageVersions)
			r.Get("/{_}/{__}/{package}", cargo.EnumeratePackageVersions)
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/chef", func() {
			r.Group("/api/v1", func() {
				r.Get("/universe", chef.PackagesUniverse)
				r.Get("/search", chef.EnumeratePackages)
				r.Group("/cookbooks", func() {
					r.Get("", chef.EnumeratePackages)
					r.Post("", reqPackageAccess(perm.AccessModeWrite), chef.UploadPackage)
					r.Group("/{name}", func() {
						r.Get("", chef.PackageMetadata)
						r.Group("/versions/{version}", func() {
							r.Get("", chef.PackageVersionMetadata)
							r.Delete("", reqPackageAccess(perm.AccessModeWrite), chef.DeletePackageVersion)
							r.Get("/download", chef.DownloadPackage)
						})
						r.Delete("", reqPackageAccess(perm.AccessModeWrite), chef.DeletePackage)
					})
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/composer", func() {
			r.Get("/packages.json", composer.ServiceIndex)
			r.Get("/search.json", composer.SearchPackages)
			r.Get("/list.json", composer.EnumeratePackages)
			r.Get("/p2/{vendorname}/{projectname}~dev.json", composer.PackageMetadata)
			r.Get("/p2/{vendorname}/{projectname}.json", composer.PackageMetadata)
			r.Get("/files/{package}/{version}/{filename}", composer.DownloadPackageFile)
			r.Put("", reqPackageAccess(perm.AccessModeWrite), composer.UploadPackage)
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/conan", func() {
			r.Group("/v1", func() {
				r.Get("/ping", conan.Ping)
				r.Group("/users", func() {
					r.Get("/authenticate", conan.Authenticate)
					r.Get("/check_credentials", conan.CheckCredentials)
				})
				r.Group("/conans", func() {
					r.Get("/search", conan.SearchRecipes)
					r.Group("/{name}/{version}/{user}/{channel}", func() {
						r.Get("", conan.RecipeSnapshot)
						r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeleteRecipeV1)
						r.Get("/search", conan.SearchPackagesV1)
						r.Get("/digest", conan.RecipeDownloadURLs)
						r.Post("/upload_urls", reqPackageAccess(perm.AccessModeWrite), conan.RecipeUploadURLs)
						r.Get("/download_urls", conan.RecipeDownloadURLs)
						r.Group("/packages", func() {
							r.Post("/delete", reqPackageAccess(perm.AccessModeWrite), conan.DeletePackageV1)
							r.Group("/{package_reference}", func() {
								r.Get("", conan.PackageSnapshot)
								r.Get("/digest", conan.PackageDownloadURLs)
								r.Post("/upload_urls", reqPackageAccess(perm.AccessModeWrite), conan.PackageUploadURLs)
								r.Get("/download_urls", conan.PackageDownloadURLs)
							})
						})
					}, conan.ExtractPathParameters)
				})
				r.Group("/files/{name}/{version}/{user}/{channel}/{recipe_revision}", func() {
					r.Group("/recipe/{filename}", func() {
						r.Get("", conan.DownloadRecipeFile)
						r.Put("", reqPackageAccess(perm.AccessModeWrite), conan.UploadRecipeFile)
					})
					r.Group("/package/{package_reference}/{package_revision}/{filename}", func() {
						r.Get("", conan.DownloadPackageFile)
						r.Put("", reqPackageAccess(perm.AccessModeWrite), conan.UploadPackageFile)
					})
				}, conan.ExtractPathParameters)
			})
			r.Group("/v2", func() {
				r.Get("/ping", conan.Ping)
				r.Group("/users", func() {
					r.Get("/authenticate", conan.Authenticate)
					r.Get("/check_credentials", conan.CheckCredentials)
				})
				r.Group("/conans", func() {
					r.Get("/search", conan.SearchRecipes)
					r.Group("/{name}/{version}/{user}/{channel}", func() {
						r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeleteRecipeV2)
						r.Get("/search", conan.SearchPackagesV2)
						r.Get("/latest", conan.LatestRecipeRevision)
						r.Group("/revisions", func() {
							r.Get("", conan.ListRecipeRevisions)
							r.Group("/{recipe_revision}", func() {
								r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeleteRecipeV2)
								r.Get("/search", conan.SearchPackagesV2)
								r.Group("/files", func() {
									r.Get("", conan.ListRecipeRevisionFiles)
									r.Group("/{filename}", func() {
										r.Get("", conan.DownloadRecipeFile)
										r.Put("", reqPackageAccess(perm.AccessModeWrite), conan.UploadRecipeFile)
									})
								})
								r.Group("/packages", func() {
									r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeletePackageV2)
									r.Group("/{package_reference}", func() {
										r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeletePackageV2)
										r.Get("/latest", conan.LatestPackageRevision)
										r.Group("/revisions", func() {
											r.Get("", conan.ListPackageRevisions)
											r.Group("/{package_revision}", func() {
												r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeletePackageV2)
												r.Group("/files", func() {
													r.Get("", conan.ListPackageRevisionFiles)
													r.Group("/{filename}", func() {
														r.Get("", conan.DownloadPackageFile)
														r.Put("", reqPackageAccess(perm.AccessModeWrite), conan.UploadPackageFile)
													})
												})
											})
										})
									})
								})
							})
						})
					}, conan.ExtractPathParameters)
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/conda", func() {
			var (
				downloadPattern = regexp.MustCompile(`\A(.+/)?(.+)/((?:[^/]+(?:\.tar\.bz2|\.conda))|(?:current_)?repodata\.json(?:\.bz2)?)\z`)
				uploadPattern   = regexp.MustCompile(`\A(.+/)?([^/]+(?:\.tar\.bz2|\.conda))\z`)
			)

			r.Get("/*", func(ctx *context.Context) {
				m := downloadPattern.FindStringSubmatch(ctx.PathParam("*"))
				if len(m) == 0 {
					ctx.Status(http.StatusNotFound)
					return
				}

				ctx.SetPathParam("channel", strings.TrimSuffix(m[1], "/"))
				ctx.SetPathParam("architecture", m[2])
				ctx.SetPathParam("filename", m[3])

				switch m[3] {
				case "repodata.json", "repodata.json.bz2", "current_repodata.json", "current_repodata.json.bz2":
					conda.EnumeratePackages(ctx)
				default:
					conda.DownloadPackageFile(ctx)
				}
			})
			r.Put("/*", reqPackageAccess(perm.AccessModeWrite), func(ctx *context.Context) {
				m := uploadPattern.FindStringSubmatch(ctx.PathParam("*"))
				if len(m) == 0 {
					ctx.Status(http.StatusNotFound)
					return
				}

				ctx.SetPathParam("channel", strings.TrimSuffix(m[1], "/"))
				ctx.SetPathParam("filename", m[2])

				conda.UploadPackageFile(ctx)
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/cran", func() {
			r.Group("/src", func() {
				r.Group("/contrib", func() {
					r.Get("/PACKAGES", cran.EnumerateSourcePackages)
					r.Get("/PACKAGES{format}", cran.EnumerateSourcePackages)
					r.Get("/{filename}", cran.DownloadSourcePackageFile)
					r.Get("/Archive/{packagename}/{filename}", cran.DownloadSourcePackageFile)
				})
				r.Put("", reqPackageAccess(perm.AccessModeWrite), cran.UploadSourcePackageFile)
			})
			r.Group("/bin", func() {
				r.Group("/{platform}/contrib/{rversion}", func() {
					r.Get("/PACKAGES", cran.EnumerateBinaryPackages)
					r.Get("/PACKAGES{format}", cran.EnumerateBinaryPackages)
					r.Get("/{filename}", cran.DownloadBinaryPackageFile)
				})
				r.Put("", reqPackageAccess(perm.AccessModeWrite), cran.UploadBinaryPackageFile)
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/debian", func() {
			r.Get("/repository.key", debian.GetRepositoryKey)
			r.Group("/dists/{distribution}", func() {
				r.Get("/{filename}", debian.GetRepositoryFile)
				r.Get("/by-hash/{algorithm}/{hash}", debian.GetRepositoryFileByHash)
				r.Group("/{component}/{architecture}", func() {
					r.Get("/{filename}", debian.GetRepositoryFile)
					r.Get("/by-hash/{algorithm}/{hash}", debian.GetRepositoryFileByHash)
				})
			})
			r.Group("/pool/{distribution}/{component}", func() {
				r.Get("/{name}_{version}_{architecture}.deb", debian.DownloadPackageFile)
				r.Group("", func() {
					r.Put("/upload", debian.UploadPackageFile)
					r.Delete("/{name}/{version}/{architecture}", debian.DeletePackageFile)
				}, reqPackageAccess(perm.AccessModeWrite))
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/go", func() {
			r.Put("/upload", reqPackageAccess(perm.AccessModeWrite), goproxy.UploadPackage)
			r.Get("/sumdb/sum.golang.org/supported", func(ctx *context.Context) {
				ctx.Status(http.StatusNotFound)
			})

			// Manual mapping of routes because the package name contains slashes which chi does not support
			// https://go.dev/ref/mod#goproxy-protocol
			r.Get("/*", func(ctx *context.Context) {
				path := ctx.PathParam("*")

				if strings.HasSuffix(path, "/@latest") {
					ctx.SetPathParam("name", path[:len(path)-len("/@latest")])
					ctx.SetPathParam("version", "latest")

					goproxy.PackageVersionMetadata(ctx)
					return
				}

				parts := strings.SplitN(path, "/@v/", 2)
				if len(parts) != 2 {
					ctx.Status(http.StatusNotFound)
					return
				}

				ctx.SetPathParam("name", parts[0])

				// <package/name>/@v/list
				if parts[1] == "list" {
					goproxy.EnumeratePackageVersions(ctx)
					return
				}

				// <package/name>/@v/<version>.zip
				if strings.HasSuffix(parts[1], ".zip") {
					ctx.SetPathParam("version", parts[1][:len(parts[1])-len(".zip")])

					goproxy.DownloadPackageFile(ctx)
					return
				}
				// <package/name>/@v/<version>.info
				if strings.HasSuffix(parts[1], ".info") {
					ctx.SetPathParam("version", parts[1][:len(parts[1])-len(".info")])

					goproxy.PackageVersionMetadata(ctx)
					return
				}
				// <package/name>/@v/<version>.mod
				if strings.HasSuffix(parts[1], ".mod") {
					ctx.SetPathParam("version", parts[1][:len(parts[1])-len(".mod")])

					goproxy.PackageVersionGoModContent(ctx)
					return
				}

				ctx.Status(http.StatusNotFound)
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/generic", func() {
			r.Group("/{packagename}/{packageversion}", func() {
				r.Delete("", reqPackageAccess(perm.AccessModeWrite), generic.DeletePackage)
				r.Group("/{filename}", func() {
					r.Get("", generic.DownloadPackageFile)
					r.Group("", func() {
						r.Put("", generic.UploadPackage)
						r.Delete("", generic.DeletePackageFile)
					}, reqPackageAccess(perm.AccessModeWrite))
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/helm", func() {
			r.Get("/index.yaml", helm.Index)
			r.Get("/{filename}", helm.DownloadPackageFile)
			r.Post("/api/charts", reqPackageAccess(perm.AccessModeWrite), helm.UploadPackage)
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/maven", func() {
			r.Put("/*", reqPackageAccess(perm.AccessModeWrite), maven.UploadPackageFile)
			r.Get("/*", maven.DownloadPackageFile)
			r.Head("/*", maven.ProvidePackageFileHeader)
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/nuget", func() {
			r.Group("", func() { // Needs to be unauthenticated for the NuGet client.
				r.Get("/", nuget.ServiceIndexV2)
				r.Get("/index.json", nuget.ServiceIndexV3)
				r.Get("/$metadata", nuget.FeedCapabilityResource)
			})
			r.Group("", func() {
				r.Get("/query", nuget.SearchServiceV3)
				r.Group("/registration/{id}", func() {
					r.Get("/index.json", nuget.RegistrationIndex)
					r.Get("/{version}", nuget.RegistrationLeafV3)
				})
				r.Group("/package/{id}", func() {
					r.Get("/index.json", nuget.EnumeratePackageVersionsV3)
					r.Get("/{version}/{filename}", nuget.DownloadPackageFile)
				})
				r.Group("", func() {
					r.Put("/", nuget.UploadPackage)
					r.Put("/symbolpackage", nuget.UploadSymbolPackage)
					r.Delete("/{id}/{version}", nuget.DeletePackage)
				}, reqPackageAccess(perm.AccessModeWrite))
				r.Get("/symbols/{filename}/{guid:[0-9a-fA-F]{32}[fF]{8}}/{filename2}", nuget.DownloadSymbolFile)
				r.Get("/Packages(Id='{id:[^']+}',Version='{version:[^']+}')", nuget.RegistrationLeafV2)
				r.Group("/Packages()", func() {
					r.Get("", nuget.SearchServiceV2)
					r.Get("/$count", nuget.SearchServiceV2Count)
				})
				r.Group("/FindPackagesById()", func() {
					r.Get("", nuget.EnumeratePackageVersionsV2)
					r.Get("/$count", nuget.EnumeratePackageVersionsV2Count)
				})
				r.Group("/Search()", func() {
					r.Get("", nuget.SearchServiceV2)
					r.Get("/$count", nuget.SearchServiceV2Count)
				})
			}, reqPackageAccess(perm.AccessModeRead))
		})
		r.Group("/npm", func() {
			r.Group("/@{scope}/{id}", func() {
				r.Get("", npm.PackageMetadata)
				r.Put("", reqPackageAccess(perm.AccessModeWrite), npm.UploadPackage)
				r.Group("/-/{version}/{filename}", func() {
					r.Get("", npm.DownloadPackageFile)
					r.Delete("/-rev/{revision}", reqPackageAccess(perm.AccessModeWrite), npm.DeletePackageVersion)
				})
				r.Get("/-/{filename}", npm.DownloadPackageFileByName)
				r.Group("/-rev/{revision}", func() {
					r.Delete("", npm.DeletePackage)
					r.Put("", npm.DeletePreview)
				}, reqPackageAccess(perm.AccessModeWrite))
			})
			r.Group("/{id}", func() {
				r.Get("", npm.PackageMetadata)
				r.Put("", reqPackageAccess(perm.AccessModeWrite), npm.UploadPackage)
				r.Group("/-/{version}/{filename}", func() {
					r.Get("", npm.DownloadPackageFile)
					r.Delete("/-rev/{revision}", reqPackageAccess(perm.AccessModeWrite), npm.DeletePackageVersion)
				})
				r.Get("/-/{filename}", npm.DownloadPackageFileByName)
				r.Group("/-rev/{revision}", func() {
					r.Delete("", npm.DeletePackage)
					r.Put("", npm.DeletePreview)
				}, reqPackageAccess(perm.AccessModeWrite))
			})
			r.Group("/-/package/@{scope}/{id}/dist-tags", func() {
				r.Get("", npm.ListPackageTags)
				r.Group("/{tag}", func() {
					r.Put("", npm.AddPackageTag)
					r.Delete("", npm.DeletePackageTag)
				}, reqPackageAccess(perm.AccessModeWrite))
			})
			r.Group("/-/package/{id}/dist-tags", func() {
				r.Get("", npm.ListPackageTags)
				r.Group("/{tag}", func() {
					r.Put("", npm.AddPackageTag)
					r.Delete("", npm.DeletePackageTag)
				}, reqPackageAccess(perm.AccessModeWrite))
			})
			r.Group("/-/v1/search", func() {
				r.Get("", npm.PackageSearch)
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/pub", func() {
			r.Group("/api/packages", func() {
				r.Group("/versions/new", func() {
					r.Get("", pub.RequestUpload)
					r.Post("/upload", pub.UploadPackageFile)
					r.Get("/finalize/{id}/{version}", pub.FinalizePackage)
				}, reqPackageAccess(perm.AccessModeWrite))
				r.Group("/{id}", func() {
					r.Get("", pub.EnumeratePackageVersions)
					r.Get("/files/{version}", pub.DownloadPackageFile)
					r.Get("/{version}", pub.PackageVersionMetadata)
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/pypi", func() {
			r.Post("/", reqPackageAccess(perm.AccessModeWrite), pypi.UploadPackageFile)
			r.Get("/files/{id}/{version}/{filename}", pypi.DownloadPackageFile)
			r.Get("/simple/{id}", pypi.PackageMetadata)
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/rpm", func() {
			r.Group("/repository.key", func() {
				r.Head("", rpm.GetRepositoryKey)
				r.Get("", rpm.GetRepositoryKey)
			})

			var (
				repoPattern     = regexp.MustCompile(`\A(.*?)\.repo\z`)
				uploadPattern   = regexp.MustCompile(`\A(.*?)/upload\z`)
				filePattern     = regexp.MustCompile(`\A(.*?)/package/([^/]+)/([^/]+)/([^/]+)(?:/([^/]+\.rpm)|)\z`)
				repoFilePattern = regexp.MustCompile(`\A(.*?)/repodata/([^/]+)\z`)
			)

			r.Methods("HEAD,GET,PUT,DELETE", "*", func(ctx *context.Context) {
				path := ctx.PathParam("*")
				isHead := ctx.Req.Method == "HEAD"
				isGetHead := ctx.Req.Method == "HEAD" || ctx.Req.Method == "GET"
				isPut := ctx.Req.Method == "PUT"
				isDelete := ctx.Req.Method == "DELETE"

				m := repoPattern.FindStringSubmatch(path)
				if len(m) == 2 && isGetHead {
					ctx.SetPathParam("group", strings.Trim(m[1], "/"))
					rpm.GetRepositoryConfig(ctx)
					return
				}

				m = repoFilePattern.FindStringSubmatch(path)
				if len(m) == 3 && isGetHead {
					ctx.SetPathParam("group", strings.Trim(m[1], "/"))
					ctx.SetPathParam("filename", m[2])
					if isHead {
						rpm.CheckRepositoryFileExistence(ctx)
					} else {
						rpm.GetRepositoryFile(ctx)
					}
					return
				}

				m = uploadPattern.FindStringSubmatch(path)
				if len(m) == 2 && isPut {
					reqPackageAccess(perm.AccessModeWrite)(ctx)
					if ctx.Written() {
						return
					}
					ctx.SetPathParam("group", strings.Trim(m[1], "/"))
					rpm.UploadPackageFile(ctx)
					return
				}

				m = filePattern.FindStringSubmatch(path)
				if len(m) == 6 && (isGetHead || isDelete) {
					ctx.SetPathParam("group", strings.Trim(m[1], "/"))
					ctx.SetPathParam("name", m[2])
					ctx.SetPathParam("version", m[3])
					ctx.SetPathParam("architecture", m[4])
					if isGetHead {
						rpm.DownloadPackageFile(ctx)
					} else {
						reqPackageAccess(perm.AccessModeWrite)(ctx)
						if ctx.Written() {
							return
						}
						rpm.DeletePackageFile(ctx)
					}
					return
				}

				ctx.Status(http.StatusNotFound)
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/rubygems", func() {
			r.Get("/specs.4.8.gz", rubygems.EnumeratePackages)
			r.Get("/latest_specs.4.8.gz", rubygems.EnumeratePackagesLatest)
			r.Get("/prerelease_specs.4.8.gz", rubygems.EnumeratePackagesPreRelease)
			r.Get("/quick/Marshal.4.8/{filename}", rubygems.ServePackageSpecification)
			r.Get("/gems/{filename}", rubygems.DownloadPackageFile)
			r.Get("/info/{packagename}", rubygems.GetPackageInfo)
			r.Get("/versions", rubygems.GetAllPackagesVersions)
			r.Group("/api/v1/gems", func() {
				r.Post("/", rubygems.UploadPackageFile)
				r.Delete("/yank", rubygems.DeletePackage)
			}, reqPackageAccess(perm.AccessModeWrite))
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/swift", func() {
			r.Group("", func() { // Needs to be unauthenticated.
				r.Post("", swift.CheckAuthenticate)
				r.Post("/login", swift.CheckAuthenticate)
			})
			r.Group("", func() {
				r.Group("/{scope}/{name}", func() {
					r.Group("", func() {
						r.Get("", swift.EnumeratePackageVersions)
						r.Get(".json", swift.EnumeratePackageVersions)
					}, swift.CheckAcceptMediaType(swift.AcceptJSON))
					r.Group("/{version}", func() {
						r.Get("/Package.swift", swift.CheckAcceptMediaType(swift.AcceptSwift), swift.DownloadManifest)
						r.Put("", reqPackageAccess(perm.AccessModeWrite), swift.CheckAcceptMediaType(swift.AcceptJSON), swift.UploadPackageFile)
						r.Get("", func(ctx *context.Context) {
							// Can't use normal routes here: https://github.com/go-chi/chi/issues/781

							version := ctx.PathParam("version")
							if strings.HasSuffix(version, ".zip") {
								swift.CheckAcceptMediaType(swift.AcceptZip)(ctx)
								if ctx.Written() {
									return
								}
								ctx.SetPathParam("version", version[:len(version)-4])
								swift.DownloadPackageFile(ctx)
							} else {
								swift.CheckAcceptMediaType(swift.AcceptJSON)(ctx)
								if ctx.Written() {
									return
								}
								if strings.HasSuffix(version, ".json") {
									ctx.SetPathParam("version", version[:len(version)-5])
								}
								swift.PackageVersionMetadata(ctx)
							}
						})
					})
				})
				r.Get("/identifiers", swift.CheckAcceptMediaType(swift.AcceptJSON), swift.LookupPackageIdentifiers)
			}, reqPackageAccess(perm.AccessModeRead))
		})
		r.Group("/vagrant", func() {
			r.Group("/authenticate", func() {
				r.Get("", vagrant.CheckAuthenticate)
			})
			r.Group("/{name}", func() {
				r.Head("", vagrant.CheckBoxAvailable)
				r.Get("", vagrant.EnumeratePackageVersions)
				r.Group("/{version}/{provider}", func() {
					r.Get("", vagrant.DownloadPackageFile)
					r.Put("", reqPackageAccess(perm.AccessModeWrite), vagrant.UploadPackageFile)
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))
	}, context.UserAssignmentWeb(), context.PackageAssignment())

	return r
}

// ContainerRoutes provides endpoints that implement the OCI API to serve containers
// These have to be mounted on `/v2/...` to comply with the OCI spec:
// https://github.com/opencontainers/distribution-spec/blob/main/spec.md
func ContainerRoutes() *web.Router {
	r := web.NewRouter()

	r.Use(context.PackageContexter())

	verifyAuth(r, []auth.Method{
		&auth.Basic{},
		&container.Auth{},
	})

	r.Get("", container.ReqContainerAccess, container.DetermineSupport)
	r.Group("/token", func() {
		r.Get("", container.Authenticate)
		r.Post("", container.AuthenticateNotImplemented)
	})
	r.Get("/_catalog", container.ReqContainerAccess, container.GetRepositoryList)
	r.Group("/{username}", func() {
		r.PathGroup("/*", func(g *web.RouterPathGroup) {
			g.MatchPath("POST", "/<image:*>/blobs/uploads", reqPackageAccess(perm.AccessModeWrite), container.VerifyImageName, container.InitiateUploadBlob)
			g.MatchPath("GET", "/<image:*>/tags/list", container.VerifyImageName, container.GetTagList)
			g.MatchPath("GET,PATCH,PUT,DELETE", `/<image:*>/blobs/uploads/<uuid:[-.=\w]+>`, reqPackageAccess(perm.AccessModeWrite), container.VerifyImageName, func(ctx *context.Context) {
				if ctx.Req.Method == http.MethodGet {
					container.GetUploadBlob(ctx)
				} else if ctx.Req.Method == http.MethodPatch {
					container.UploadBlob(ctx)
				} else if ctx.Req.Method == http.MethodPut {
					container.EndUploadBlob(ctx)
				} else /* DELETE */ {
					container.CancelUploadBlob(ctx)
				}
			})
			g.MatchPath("HEAD", `/<image:*>/blobs/<digest>`, container.VerifyImageName, container.HeadBlob)
			g.MatchPath("GET", `/<image:*>/blobs/<digest>`, container.VerifyImageName, container.GetBlob)
			g.MatchPath("DELETE", `/<image:*>/blobs/<digest>`, container.VerifyImageName, reqPackageAccess(perm.AccessModeWrite), container.DeleteBlob)

			g.MatchPath("HEAD", `/<image:*>/manifests/<reference>`, container.VerifyImageName, container.HeadManifest)
			g.MatchPath("GET", `/<image:*>/manifests/<reference>`, container.VerifyImageName, container.GetManifest)
			g.MatchPath("PUT", `/<image:*>/manifests/<reference>`, container.VerifyImageName, reqPackageAccess(perm.AccessModeWrite), container.UploadManifest)
			g.MatchPath("DELETE", `/<image:*>/manifests/<reference>`, container.VerifyImageName, reqPackageAccess(perm.AccessModeWrite), container.DeleteManifest)
		})
	}, container.ReqContainerAccess, context.UserAssignmentWeb(), context.PackageAssignment(), reqPackageAccess(perm.AccessModeRead))

	return r
}
