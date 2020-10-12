// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"

	"github.com/urfave/cli"
)

// CmdMigrateStorage represents the available migrate storage sub-command.
var CmdMigrateStorage = cli.Command{
	Name:        "migrate-storage",
	Usage:       "Migrate the storage",
	Description: "This is a command for migrating storage.",
	Action:      runMigrateStorage,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "type, t",
			Value: "",
			Usage: "Kinds of files to migrate, currently only 'attachments' is supported",
		},
		cli.StringFlag{
			Name:  "storage, s",
			Value: setting.LocalStorageType,
			Usage: "New storage type, local or minio",
		},
		cli.StringFlag{
			Name:  "path, p",
			Value: "",
			Usage: "New storage placement if store is local (leave blank for default)",
		},
		cli.StringFlag{
			Name:  "minio-endpoint",
			Value: "",
			Usage: "Minio storage endpoint",
		},
		cli.StringFlag{
			Name:  "minio-access-key-id",
			Value: "",
			Usage: "Minio storage accessKeyID",
		},
		cli.StringFlag{
			Name:  "minio-secret-access-key",
			Value: "",
			Usage: "Minio storage secretAccessKey",
		},
		cli.StringFlag{
			Name:  "minio-bucket",
			Value: "",
			Usage: "Minio storage bucket",
		},
		cli.StringFlag{
			Name:  "minio-location",
			Value: "",
			Usage: "Minio storage location to create bucket",
		},
		cli.StringFlag{
			Name:  "minio-base-path",
			Value: "",
			Usage: "Minio storage basepath on the bucket",
		},
		cli.BoolFlag{
			Name:  "minio-use-ssl",
			Usage: "Enable SSL for minio",
		},
	},
}

func migrateAttachments(dstStorage storage.ObjectStorage) error {
	return models.IterateAttachment(func(attach *models.Attachment) error {
		_, err := storage.Copy(dstStorage, attach.RelativePath(), storage.Attachments, attach.RelativePath())
		return err
	})
}

func migrateLFS(dstStorage storage.ObjectStorage) error {
	return models.IterateLFS(func(mo *models.LFSMetaObject) error {
		_, err := storage.Copy(dstStorage, mo.RelativePath(), storage.LFS, mo.RelativePath())
		return err
	})
}

func runMigrateStorage(ctx *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)
	setting.InitDBConfig()

	if err := models.NewEngine(context.Background(), migrations.Migrate); err != nil {
		log.Fatal("Failed to initialize ORM engine: %v", err)
		return err
	}

	if err := storage.Init(); err != nil {
		return err
	}

	var dstStorage storage.ObjectStorage
	var err error
	switch strings.ToLower(ctx.String("storage")) {
	case setting.LocalStorageType:
		p := ctx.String("path")
		if p == "" {
			log.Fatal("Path must be given when storage is loal")
			return nil
		}
		dstStorage, err = storage.NewLocalStorage(p)
	case setting.MinioStorageType:
		dstStorage, err = storage.NewMinioStorage(
			context.Background(),
			ctx.String("minio-endpoint"),
			ctx.String("minio-access-key-id"),
			ctx.String("minio-secret-access-key"),
			ctx.String("minio-bucket"),
			ctx.String("minio-location"),
			ctx.String("minio-base-path"),
			ctx.Bool("minio-use-ssl"),
		)
	default:
		return fmt.Errorf("Unsupported attachments storage type: %s", ctx.String("storage"))
	}

	if err != nil {
		return err
	}

	tp := strings.ToLower(ctx.String("type"))
	switch tp {
	case "attachments":
		if err := migrateAttachments(dstStorage); err != nil {
			return err
		}
	case "lfs":
		if err := migrateLFS(dstStorage); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Unsupported storage: %s", ctx.String("type"))
	}

	log.Warn("All files have been copied to the new placement but old files are still on the orignial placement.")

	return nil
}
