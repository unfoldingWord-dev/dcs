// Copyright 2019 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

/*** DCS Customizations - Router for YAML API ***/

package misc

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/yaml"
)

// YamlOption options for YAML
type YamlOption struct {
	Text string
}

// Yaml https://github.com/gogits/go-gogs-client/wiki/Miscellaneous#render-an-arbitrary-markdown-document
func Yaml(ctx *context.APIContext) {
	form := web.GetForm(ctx).(*YamlOption)

	if ctx.HasAPIError() {
		ctx.Error(422, "", ctx.GetErrMsg())
		return
	}

	if len(form.Text) == 0 {
		_, err := ctx.Write([]byte(""))
		if err != nil {
			ctx.Error(400, "Unable to write YAML", err)
		}
		return
	}
	if rendered, err := yaml.RenderSanitized([]byte(form.Text)); err != nil {
		ctx.Error(400, "Unable to parse YAML", err)
	} else if _, err := ctx.Write(rendered); err != nil {
		ctx.Error(400, "Unable to write YAML", err)
	}
}
