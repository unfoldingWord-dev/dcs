// Copyright 2021 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package v5 Catalog Next v5 API.
//
// This documentation describes the DCS Catalog Next v5 API.
//
//     Schemes: http, https
//     BasePath: /api/catalog/v5
//     Version: 4.0.1
//     License: MIT http://opensource.org/licenses/MIT
//
//     Consumes:
//     - application/json
//     - text/plain
//
//     Produces:
//     - application/json
//     - text/html
//
//     Security:
//     - BasicAuth :
//     - Token :
//     - AccessToken :
//     - AuthorizationHeaderToken :
//     - SudoParam :
//     - SudoHeader :
//     - TOTPHeader :
//
//     SecurityDefinitions:
//     BasicAuth:
//          type: basic
//     Token:
//          type: apiKey
//          name: token
//          in: query
//     AccessToken:
//          type: apiKey
//          name: access_token
//          in: query
//     AuthorizationHeaderToken:
//          type: apiKey
//          name: Authorization
//          in: header
//          description: API tokens must be prepended with "token" followed by a space.
//     SudoParam:
//          type: apiKey
//          name: sudo
//          in: query
//          description: Sudo API request as the user provided as the key. Admin privileges are required.
//     SudoHeader:
//          type: apiKey
//          name: Sudo
//          in: header
//          description: Sudo API request as the user provided as the key. Admin privileges are required.
//     TOTPHeader:
//          type: apiKey
//          name: X-GITEA-OTP
//          in: header
//          description: Must be used in combination with BasicAuth if two-factor authentication is enabled.
//
// swagger:meta
package v5

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	_ "code.gitea.io/gitea/routers/api/v1/swagger" // for swagger generation

	"gitea.com/go-chi/session"
	"github.com/go-chi/cors"
)

func sudo() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		sudo := ctx.Query("sudo")
		if len(sudo) == 0 {
			sudo = ctx.Req.Header.Get("Sudo")
		}

		if len(sudo) > 0 {
			if ctx.IsSigned && ctx.User.IsAdmin {
				user, err := models.GetUserByName(sudo)
				if err != nil {
					if models.IsErrUserNotExist(err) {
						ctx.NotFound()
					} else {
						ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
					}
					return
				}
				log.Trace("Sudo from (%s) to: %s", ctx.User.Name, user.Name)
				ctx.User = user
			} else {
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "Only administrators allowed to sudo.",
				})
				return
			}
		}
	}
}

func repoAssignment() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		userName := ctx.Params("username")
		repoName := ctx.Params("reponame")

		var (
			owner *models.User
			err   error
		)

		// Check if the user is the same as the repository owner.
		if ctx.IsSigned && ctx.User.LowerName == strings.ToLower(userName) {
			owner = ctx.User
		} else {
			owner, err = models.GetUserByName(userName)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					if redirectUserID, err := models.LookupUserRedirect(userName); err == nil {
						context.RedirectToUser(ctx.Context, userName, redirectUserID)
					} else if models.IsErrUserRedirectNotExist(err) {
						ctx.NotFound("GetUserByName", err)
					} else {
						ctx.Error(http.StatusInternalServerError, "LookupUserRedirect", err)
					}
				} else {
					ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
				}
				return
			}
		}
		ctx.Repo.Owner = owner

		// Get repository.
		repo, err := models.GetRepositoryByName(owner.ID, repoName)
		if err != nil {
			if models.IsErrRepoNotExist(err) {
				redirectRepoID, err := models.LookupRepoRedirect(owner.ID, repoName)
				if err == nil {
					context.RedirectToRepo(ctx.Context, redirectRepoID)
				} else if models.IsErrRepoRedirectNotExist(err) {
					ctx.NotFound()
				} else {
					ctx.Error(http.StatusInternalServerError, "LookupRepoRedirect", err)
				}
			} else {
				ctx.Error(http.StatusInternalServerError, "GetRepositoryByName", err)
			}
			return
		}

		repo.Owner = owner
		ctx.Repo.Repository = repo

		ctx.Repo.Permission, err = models.GetUserRepoPermission(repo, ctx.User)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
			return
		}

		if !ctx.Repo.HasAccess() {
			ctx.NotFound()
			return
		}
	}
}

// Routes registers all catalog v5 APIs routes to web application.
func Routes() *web.Route {
	var m = web.NewRoute()

	m.Use(session.Sessioner(session.Options{
		Provider:       setting.SessionConfig.Provider,
		ProviderConfig: setting.SessionConfig.ProviderConfig,
		CookieName:     setting.SessionConfig.CookieName,
		CookiePath:     setting.SessionConfig.CookiePath,
		Gclifetime:     setting.SessionConfig.Gclifetime,
		Maxlifetime:    setting.SessionConfig.Maxlifetime,
		Secure:         setting.SessionConfig.Secure,
		Domain:         setting.SessionConfig.Domain,
	}))
	m.Use(securityHeaders())
	if setting.CORSConfig.Enabled {
		m.Use(cors.Handler(cors.Options{
			//Scheme:           setting.CORSConfig.Scheme, // FIXME: the cors middleware needs scheme option
			AllowedOrigins: setting.CORSConfig.AllowDomain,
			//setting.CORSConfig.AllowSubdomain // FIXME: the cors middleware needs allowSubdomain option
			AllowedMethods:   setting.CORSConfig.Methods,
			AllowCredentials: setting.CORSConfig.AllowCredentials,
			MaxAge:           int(setting.CORSConfig.MaxAge.Seconds()),
		}))
	}
	m.Use(context.APIContexter())
	m.Use(context.ToggleAPI(&context.ToggleOptions{
		SignInRequired: setting.Service.RequireSignInView,
	}))

	m.Group("", func() {
		// Miscellaneous
		if setting.API.EnableSwagger {
			m.Get("/swagger", func(ctx *context.APIContext) {
				ctx.Redirect("../swagger")
			})
		}

		m.Get("", Search)

		m.Group("/search", func() {
			m.Get("", Search)
			m.Group("/{username}", func() {
				m.Get("", SearchOwner)
				m.Group("/{reponame}", func() {
					m.Get("", SearchRepo)
				}, repoAssignment())
			})
		})
		m.Group("/entry/{username}/{reponame}/{tag}", func() {
			m.Get("", GetCatalogEntry)
			m.Get("/metadata", GetCatalogMetadata)
		}, repoAssignment())
	}, sudo())

	return m
}

func securityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			// CORB: https://www.chromium.org/Home/chromium-security/corb-for-developers
			// http://stackoverflow.com/a/3146618/244009
			resp.Header().Set("x-content-type-options", "nosniff")
			next.ServeHTTP(resp, req)
		})
	}
}
