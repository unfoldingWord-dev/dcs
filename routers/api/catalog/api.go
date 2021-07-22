// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package catalog Catalog Next API.
//
// This documentation describes the Catalog Next API for all versions and other miscellaneous endpoints.
//
//     Schemes: http, https
//     BasePath: /api/catalog
//     Version: 5.0.0
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
package catalog

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	_ "code.gitea.io/gitea/routers/api/catalog/swagger" // for swagger generation
	"code.gitea.io/gitea/routers/api/catalog/v4"
	"code.gitea.io/gitea/routers/api/catalog/v5"

	"gitea.com/go-chi/session"
	"github.com/go-chi/cors"
)

var versions = []string{
	"v4",
	"v5",
}
var latestVersion = versions[len(versions)-1]

// AllRoutes call all the other route functions for the catalog api
func AllRoutes(r *web.Route) {
	r.Mount("/api/catalog/latest", LatestRoutes())
	r.Mount("/api/catalog/misc", MiscRoutes())
	r.Mount("/api/catalog/v4", v4.Routes())
	r.Mount("/api/catalog/v5", v5.Routes())
}

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

// LatestRoutes registers latest redirects to latest version of the catalog API.
func LatestRoutes() *web.Route {
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
		m.Get("", func(ctx *context.APIContext) {
			ctx.Redirect(fmt.Sprintf("/api/catalog/%s", latestVersion))
		})
		m.Get("/*", func(ctx *context.APIContext) {
			ctx.Redirect(fmt.Sprintf("/api/catalog/%s/%s", latestVersion, ctx.Params("*")))
		})
	}, sudo())

	return m
}

// MiscRoutes registers catalog API endpoints that are relevant for all versions
func MiscRoutes() *web.Route {
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

	m.Get("versions", ListCatalogVersionEndpoints)

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
