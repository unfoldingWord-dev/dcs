// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package checks

import (
	"errors"
	"os/exec"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Models = &analysis.Analyzer{
	Name: "models",
	Doc:  "check models for black-listed packages.",
	Run:  checkModels,
}

var (
	modelsImpBlockList = []string{
		"code.gitea.io/gitea/modules/git",
	}
)

func checkModels(pass *analysis.Pass) (interface{}, error) {
	if !strings.EqualFold(pass.Pkg.Path(), "code.gitea.io/gitea/models") {
		return nil, nil
	}

	if _, err := exec.LookPath("go"); err != nil {
		return nil, errors.New("go was not found in the PATH")
	}

	impsCmd := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, "code.gitea.io/gitea/models")
	impsOut, err := impsCmd.Output()
	if err != nil {
		return nil, err
	}

	imps := strings.Split(string(impsOut), "\n")
	for _, imp := range imps {
		if stringInSlice(imp, modelsImpBlockList) {
			pass.Reportf(0, "code.gitea.io/gitea/models cannot import the following packages: %s", modelsImpBlockList)
			return nil, nil
		}
	}

	return nil, nil
}
