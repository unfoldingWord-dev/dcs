// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/door43metadata"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

// CmdDoor43MetadataGenerate represents the available door43-metadata-generate sub-command.
var CmdDoor43MetadataGenerate = cli.Command{
	Name:        "generate-door43-metadata",
	Usage:       "Generate Door43 Metadata",
	Description: "This is a command for generating door43 metadata, making sure all valid repos have metadata in the door43_metadata table.",
	Action:      runDoor43MetadataGenerate,
}

func runDoor43MetadataGenerate(ctx *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)
	setting.InitDBConfig()

	if err := models.NewEngine(context.Background(), door43metadata.GenerateDoor43Metadata); err != nil {
		log.Fatal("Failed to initialize ORM engine: %v", err)
		return err
	}

	return nil
}
