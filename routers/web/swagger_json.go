// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

// tplSwaggerV1Json swagger v1 json template
const tplSwaggerV1Json base.TplName = "swagger/v1_json"

// SwaggerV1Json render swagger v1 json
func SwaggerV1Json(ctx *context.Context) {
	t := ctx.Render.TemplateLookup(string(tplSwaggerV1Json))
	ctx.Resp.Header().Set("Content-Type", "application/json")
	if err := t.Execute(ctx.Resp, ctx.Data); err != nil {
		log.Error("%v", err)
		ctx.Error(http.StatusInternalServerError)
	}
}

/*** DCS Customizations ***/
const tplSwaggerCatalogJSON base.TplName = "swagger/catalog/catalog_json"

// SwaggerCatalogJSON render swagger catalog v5 json
func SwaggerCatalogJSON(ctx *context.Context) {
	t := ctx.Render.TemplateLookup(string(tplSwaggerCatalogJSON))
	ctx.Resp.Header().Set("Content-Type", "application/json")
	if err := t.Execute(ctx.Resp, ctx.Data); err != nil {
		log.Error("%v", err)
		ctx.Error(500)
	}
}

/*** EMD DCS Customizations ***/
