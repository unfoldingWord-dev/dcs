// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43metadata

import (
	"context"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
)

// UpdateDoor43Metadata generates door43_metadata table entries for valid repos/releases that don't have them
func UpdateDoor43Metadata(ctx context.Context) error {
	log.Trace("Doing: UpdateDoor43Metadata")

	repoIDs, err := models.GetReposForMetadata()
	if err != nil {
		log.Error("GetReposForMetadata: %v", err)
	}

	if err = models.Iterate(
		models.DefaultDBContext(),
		new(models.Repository),
		builder.In("id", repoIDs),
		func(idx int, bean interface{}) error {
			repo := bean.(*models.Repository)
			select {
			case <-ctx.Done():
				return models.ErrCancelledf("before update door43 metadata of %s", repo.FullName())
			default:
			}
			log.Trace("Running generate metadata on %v", repo)
			if err := ProcessDoor43MetadataForRepo(repo); err != nil {
				log.Warn("Failed to process metadata for repo (%v): %v", repo, err)
				if err = models.CreateRepositoryNotice("Failed to process metadata for repository (%s): %v", repo.FullName(), err); err != nil {
					log.Error("ProcessDoor43MetadataForRepo: %v", err)
				}
			}
			return nil
		},
	); err != nil {
		log.Trace("Error: UpdateDoor43Metadata: %v", err)
		return err
	}

	log.Trace("Finished: UpdateDoor43Metadata")
	return nil
}
