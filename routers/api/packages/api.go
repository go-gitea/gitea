// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"net/http"

	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/packages/composer"
	"code.gitea.io/gitea/routers/api/packages/conan"
	"code.gitea.io/gitea/routers/api/packages/generic"
	"code.gitea.io/gitea/routers/api/packages/maven"
	"code.gitea.io/gitea/routers/api/packages/npm"
	"code.gitea.io/gitea/routers/api/packages/nuget"
	"code.gitea.io/gitea/routers/api/packages/pypi"
	"code.gitea.io/gitea/routers/api/packages/rubygems"
	"code.gitea.io/gitea/services/auth"
)

func reqPackageAccess(accessMode perm.AccessMode) func(ctx *context.Context) {
	return func(ctx *context.Context) {
		if ctx.Package.AccessMode < accessMode && !ctx.IsUserSiteAdmin() {
			ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="Gitea Package API"`)
			ctx.Error(http.StatusUnauthorized, "reqPackageAccess", "user should have specific permission or be a site admin")
			return
		}
	}
}

func Routes() *web.Route {
	r := web.NewRoute()

	if !setting.Packages.Enabled {
		return r
	}

	r.Use(context.PackageContexter())

	authGroup := auth.NewGroup(&auth.Basic{}, &conan.Auth{})
	r.Use(func(ctx *context.Context) {
		ctx.User = authGroup.Verify(ctx.Req, ctx.Resp, ctx, ctx.Session)
	})

	r.Group("/{username}", func() {
		r.Group("/composer", func() {
			r.Get("/packages.json", composer.ServiceIndex)
			r.Get("/search.json", composer.SearchPackages)
			r.Get("/list.json", composer.EnumeratePackages)
			r.Get("/p2/{vendorname}/{projectname}~dev.json", composer.PackageMetadata)
			r.Get("/p2/{vendorname}/{projectname}.json", composer.PackageMetadata)
			r.Get("/files/{package}/{version}/{filename}", composer.DownloadPackageFile)
			r.Put("", reqPackageAccess(perm.AccessModeWrite), composer.UploadPackage)
		})
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
		})
		r.Group("/generic", func() {
			r.Group("/{packagename}/{packageversion}/{filename}", func() {
				r.Get("", generic.DownloadPackageFile)
				r.Group("", func() {
					r.Put("", generic.UploadPackage)
					r.Delete("", generic.DeletePackage)
				}, reqPackageAccess(perm.AccessModeWrite))
			})
		})
		r.Group("/maven", func() {
			r.Put("/*", reqPackageAccess(perm.AccessModeWrite), maven.UploadPackageFile)
			r.Get("/*", maven.DownloadPackageFile)
		})
		r.Group("/nuget", func() {
			r.Get("/index.json", nuget.ServiceIndex)
			r.Get("/query", nuget.SearchService)
			r.Group("/registration/{id}", func() {
				r.Get("/index.json", nuget.RegistrationIndex)
				r.Get("/{version}", nuget.RegistrationLeaf)
			})
			r.Group("/package/{id}", func() {
				r.Get("/index.json", nuget.EnumeratePackageVersions)
				r.Get("/{version}/{filename}", nuget.DownloadPackageFile)
			})
			r.Group("", func() {
				r.Put("/", nuget.UploadPackage)
				r.Put("/symbolpackage", nuget.UploadSymbolPackage)
				r.Delete("/{id}/{version}", nuget.DeletePackage)
			}, reqPackageAccess(perm.AccessModeWrite))
			r.Get("/symbols/{filename}/{guid:[0-9a-f]{32}}FFFFFFFF/{filename2}", nuget.DownloadSymbolFile)
		})
		r.Group("/npm", func() {
			r.Group("/@{scope}/{id}", func() {
				r.Get("", npm.PackageMetadata)
				r.Put("", reqPackageAccess(perm.AccessModeWrite), npm.UploadPackage)
				r.Get("/-/{version}/{filename}", npm.DownloadPackageFile)
			})
			r.Group("/{id}", func() {
				r.Get("", npm.PackageMetadata)
				r.Put("", reqPackageAccess(perm.AccessModeWrite), npm.UploadPackage)
				r.Get("/-/{version}/{filename}", npm.DownloadPackageFile)
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
		})
		r.Group("/pypi", func() {
			r.Post("/", reqPackageAccess(perm.AccessModeWrite), pypi.UploadPackageFile)
			r.Get("/files/{id}/{version}/{filename}", pypi.DownloadPackageFile)
			r.Get("/simple/{id}", pypi.PackageMetadata)
		})
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
		})
	}, context.UserAssignment(), context.PackageAssignment(), reqPackageAccess(perm.AccessModeRead))

	return r
}
