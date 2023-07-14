// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"net/http"
	"regexp"
	"strings"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/packages/alpine"
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
	context_service "code.gitea.io/gitea/services/context"
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
						ctx.Error(http.StatusInternalServerError, "HasScope", err.Error())
						return
					}
				} else if accessMode == perm.AccessModeWrite {
					scopeMatched, err = scope.HasScope(auth_model.AccessTokenScopeWritePackage)
					if err != nil {
						ctx.Error(http.StatusInternalServerError, "HasScope", err.Error())
						return
					}
				}
				if !scopeMatched {
					ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="Gitea Package API"`)
					ctx.Error(http.StatusUnauthorized, "reqPackageAccess", "user should have specific permission or be a site admin")
					return
				}
			}
		}

		if ctx.Package.AccessMode < accessMode && !ctx.IsUserSiteAdmin() {
			ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="Gitea Package API"`)
			ctx.Error(http.StatusUnauthorized, "reqPackageAccess", "user should have specific permission or be a site admin")
			return
		}
	}
}

func verifyAuth(r *web.Route, authMethods []auth.Method) {
	if setting.Service.EnableReverseProxyAuth {
		authMethods = append(authMethods, &auth.ReverseProxy{})
	}
	authGroup := auth.NewGroup(authMethods...)

	r.Use(func(ctx *context.Context) {
		var err error
		ctx.Doer, err = authGroup.Verify(ctx.Req, ctx.Resp, ctx, ctx.Session)
		if err != nil {
			log.Error("Failed to verify user: %v", err)
			ctx.Error(http.StatusUnauthorized, "authGroup.Verify")
			return
		}
		ctx.IsSigned = ctx.Doer != nil
	})
}

// CommonRoutes provide endpoints for most package managers (except containers - see below)
// These are mounted on `/api/packages` (not `/api/v1/packages`)
func CommonRoutes() *web.Route {
	r := web.NewRoute()

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
				m := downloadPattern.FindStringSubmatch(ctx.Params("*"))
				if len(m) == 0 {
					ctx.Status(http.StatusNotFound)
					return
				}

				ctx.SetParams("channel", strings.TrimSuffix(m[1], "/"))
				ctx.SetParams("architecture", m[2])
				ctx.SetParams("filename", m[3])

				switch m[3] {
				case "repodata.json", "repodata.json.bz2", "current_repodata.json", "current_repodata.json.bz2":
					conda.EnumeratePackages(ctx)
				default:
					conda.DownloadPackageFile(ctx)
				}
			})
			r.Put("/*", reqPackageAccess(perm.AccessModeWrite), func(ctx *context.Context) {
				m := uploadPattern.FindStringSubmatch(ctx.Params("*"))
				if len(m) == 0 {
					ctx.Status(http.StatusNotFound)
					return
				}

				ctx.SetParams("channel", strings.TrimSuffix(m[1], "/"))
				ctx.SetParams("filename", m[2])

				conda.UploadPackageFile(ctx)
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/cran", func() {
			r.Group("/src", func() {
				r.Group("/contrib", func() {
					r.Get("/PACKAGES", cran.EnumerateSourcePackages)
					r.Get("/PACKAGES{format}", cran.EnumerateSourcePackages)
					r.Get("/{filename}", cran.DownloadSourcePackageFile)
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
				path := ctx.Params("*")

				if strings.HasSuffix(path, "/@latest") {
					ctx.SetParams("name", path[:len(path)-len("/@latest")])
					ctx.SetParams("version", "latest")

					goproxy.PackageVersionMetadata(ctx)
					return
				}

				parts := strings.SplitN(path, "/@v/", 2)
				if len(parts) != 2 {
					ctx.Status(http.StatusNotFound)
					return
				}

				ctx.SetParams("name", parts[0])

				// <package/name>/@v/list
				if parts[1] == "list" {
					goproxy.EnumeratePackageVersions(ctx)
					return
				}

				// <package/name>/@v/<version>.zip
				if strings.HasSuffix(parts[1], ".zip") {
					ctx.SetParams("version", parts[1][:len(parts[1])-len(".zip")])

					goproxy.DownloadPackageFile(ctx)
					return
				}
				// <package/name>/@v/<version>.info
				if strings.HasSuffix(parts[1], ".info") {
					ctx.SetParams("version", parts[1][:len(parts[1])-len(".info")])

					goproxy.PackageVersionMetadata(ctx)
					return
				}
				// <package/name>/@v/<version>.mod
				if strings.HasSuffix(parts[1], ".mod") {
					ctx.SetParams("version", parts[1][:len(parts[1])-len(".mod")])

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
			r.Get(".repo", rpm.GetRepositoryConfig)
			r.Get("/repository.key", rpm.GetRepositoryKey)
			r.Put("/upload", reqPackageAccess(perm.AccessModeWrite), rpm.UploadPackageFile)
			r.Group("/package/{name}/{version}/{architecture}", func() {
				r.Get("", rpm.DownloadPackageFile)
				r.Delete("", reqPackageAccess(perm.AccessModeWrite), rpm.DeletePackageFile)
			})
			r.Get("/repodata/{filename}", rpm.GetRepositoryFile)
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/rubygems", func() {
			r.Get("/specs.4.8.gz", rubygems.EnumeratePackages)
			r.Get("/latest_specs.4.8.gz", rubygems.EnumeratePackagesLatest)
			r.Get("/prerelease_specs.4.8.gz", rubygems.EnumeratePackagesPreRelease)
			r.Get("/quick/Marshal.4.8/{filename}", rubygems.ServePackageSpecification)
			r.Get("/gems/{filename}", rubygems.DownloadPackageFile)
			r.Group("/api/v1/gems", func() {
				r.Post("/", rubygems.UploadPackageFile)
				r.Delete("/yank", rubygems.DeletePackage)
			}, reqPackageAccess(perm.AccessModeWrite))
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/swift", func() {
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

						version := ctx.Params("version")
						if strings.HasSuffix(version, ".zip") {
							swift.CheckAcceptMediaType(swift.AcceptZip)(ctx)
							if ctx.Written() {
								return
							}
							ctx.SetParams("version", version[:len(version)-4])
							swift.DownloadPackageFile(ctx)
						} else {
							swift.CheckAcceptMediaType(swift.AcceptJSON)(ctx)
							if ctx.Written() {
								return
							}
							if strings.HasSuffix(version, ".json") {
								ctx.SetParams("version", version[:len(version)-5])
							}
							swift.PackageVersionMetadata(ctx)
						}
					})
				})
			})
			r.Get("/identifiers", swift.CheckAcceptMediaType(swift.AcceptJSON), swift.LookupPackageIdentifiers)
		}, reqPackageAccess(perm.AccessModeRead))
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
	}, context_service.UserAssignmentWeb(), context.PackageAssignment())

	return r
}

// ContainerRoutes provides endpoints that implement the OCI API to serve containers
// These have to be mounted on `/v2/...` to comply with the OCI spec:
// https://github.com/opencontainers/distribution-spec/blob/main/spec.md
func ContainerRoutes() *web.Route {
	r := web.NewRoute()

	r.Use(context.PackageContexter())

	verifyAuth(r, []auth.Method{
		&auth.Basic{},
		&container.Auth{},
	})

	r.Get("", container.ReqContainerAccess, container.DetermineSupport)
	r.Get("/token", container.Authenticate)
	r.Get("/_catalog", container.ReqContainerAccess, container.GetRepositoryList)
	r.Group("/{username}", func() {
		r.Group("/{image}", func() {
			r.Group("/blobs/uploads", func() {
				r.Post("", container.InitiateUploadBlob)
				r.Group("/{uuid}", func() {
					r.Get("", container.GetUploadBlob)
					r.Patch("", container.UploadBlob)
					r.Put("", container.EndUploadBlob)
					r.Delete("", container.CancelUploadBlob)
				})
			}, reqPackageAccess(perm.AccessModeWrite))
			r.Group("/blobs/{digest}", func() {
				r.Head("", container.HeadBlob)
				r.Get("", container.GetBlob)
				r.Delete("", reqPackageAccess(perm.AccessModeWrite), container.DeleteBlob)
			})
			r.Group("/manifests/{reference}", func() {
				r.Put("", reqPackageAccess(perm.AccessModeWrite), container.UploadManifest)
				r.Head("", container.HeadManifest)
				r.Get("", container.GetManifest)
				r.Delete("", reqPackageAccess(perm.AccessModeWrite), container.DeleteManifest)
			})
			r.Get("/tags/list", container.GetTagList)
		}, container.VerifyImageName)

		var (
			blobsUploadsPattern = regexp.MustCompile(`\A(.+)/blobs/uploads/([a-zA-Z0-9-_.=]+)\z`)
			blobsPattern        = regexp.MustCompile(`\A(.+)/blobs/([^/]+)\z`)
			manifestsPattern    = regexp.MustCompile(`\A(.+)/manifests/([^/]+)\z`)
		)

		// Manual mapping of routes because {image} can contain slashes which chi does not support
		r.RouteMethods("/*", "HEAD,GET,POST,PUT,PATCH,DELETE", func(ctx *context.Context) {
			path := ctx.Params("*")
			isHead := ctx.Req.Method == "HEAD"
			isGet := ctx.Req.Method == "GET"
			isPost := ctx.Req.Method == "POST"
			isPut := ctx.Req.Method == "PUT"
			isPatch := ctx.Req.Method == "PATCH"
			isDelete := ctx.Req.Method == "DELETE"

			if isPost && strings.HasSuffix(path, "/blobs/uploads") {
				reqPackageAccess(perm.AccessModeWrite)(ctx)
				if ctx.Written() {
					return
				}

				ctx.SetParams("image", path[:len(path)-14])
				container.VerifyImageName(ctx)
				if ctx.Written() {
					return
				}

				container.InitiateUploadBlob(ctx)
				return
			}
			if isGet && strings.HasSuffix(path, "/tags/list") {
				ctx.SetParams("image", path[:len(path)-10])
				container.VerifyImageName(ctx)
				if ctx.Written() {
					return
				}

				container.GetTagList(ctx)
				return
			}

			m := blobsUploadsPattern.FindStringSubmatch(path)
			if len(m) == 3 && (isGet || isPut || isPatch || isDelete) {
				reqPackageAccess(perm.AccessModeWrite)(ctx)
				if ctx.Written() {
					return
				}

				ctx.SetParams("image", m[1])
				container.VerifyImageName(ctx)
				if ctx.Written() {
					return
				}

				ctx.SetParams("uuid", m[2])

				if isGet {
					container.GetUploadBlob(ctx)
				} else if isPatch {
					container.UploadBlob(ctx)
				} else if isPut {
					container.EndUploadBlob(ctx)
				} else {
					container.CancelUploadBlob(ctx)
				}
				return
			}
			m = blobsPattern.FindStringSubmatch(path)
			if len(m) == 3 && (isHead || isGet || isDelete) {
				ctx.SetParams("image", m[1])
				container.VerifyImageName(ctx)
				if ctx.Written() {
					return
				}

				ctx.SetParams("digest", m[2])

				if isHead {
					container.HeadBlob(ctx)
				} else if isGet {
					container.GetBlob(ctx)
				} else {
					reqPackageAccess(perm.AccessModeWrite)(ctx)
					if ctx.Written() {
						return
					}
					container.DeleteBlob(ctx)
				}
				return
			}
			m = manifestsPattern.FindStringSubmatch(path)
			if len(m) == 3 && (isHead || isGet || isPut || isDelete) {
				ctx.SetParams("image", m[1])
				container.VerifyImageName(ctx)
				if ctx.Written() {
					return
				}

				ctx.SetParams("reference", m[2])

				if isHead {
					container.HeadManifest(ctx)
				} else if isGet {
					container.GetManifest(ctx)
				} else {
					reqPackageAccess(perm.AccessModeWrite)(ctx)
					if ctx.Written() {
						return
					}
					if isPut {
						container.UploadManifest(ctx)
					} else {
						container.DeleteManifest(ctx)
					}
				}
				return
			}

			ctx.Status(http.StatusNotFound)
		})
	}, container.ReqContainerAccess, context_service.UserAssignmentWeb(), context.PackageAssignment(), reqPackageAccess(perm.AccessModeRead))

	return r
}
