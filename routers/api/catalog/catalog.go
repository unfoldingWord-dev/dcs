// Copyright 2021 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package catalog

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
)

// ListCatalogVersionEndpoints Lists all the Catalog Endpoints for all versions
func ListCatalogVersionEndpoints(ctx *context.APIContext) {
	// swagger:operation GET /misc/versions misc miscListCatalogVersionEndpoints
	// ---
	// summary: Catalog Next version endpoint list, including what version "latest" points to
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogVersionEndpointsResponse"
	//   "422":
	//     "$ref": "#/responses/validationError"

	versionEndpoints := structs.CatalogVersionEndpoints{
		Latest:   latestVersion,
		Versions: map[string]string{},
	}

	for _, version := range versions {
		versionEndpoints.Versions[version] = fmt.Sprintf("%sapi/catalog/%s", setting.AppURL, version)
	}

	resp := map[string]interface{}{
		"ok":   true,
		"data": versionEndpoints,
	}
	ctx.JSON(http.StatusOK, resp)
}
