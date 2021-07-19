// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToRepoDCS adds some fields for DCS Customizations
func ToRepoDCS(repo *models.Repository, mode models.AccessMode) *api.Repository {
	apiRepo := ToRepo(repo, mode)

	catalog := &api.CatalogStages{}
	prod, err := models.GetDoor43MetadataByRepoIDAndStage(repo.ID, models.StageProd)
	if err != nil {
		log.Error("GetDoor43MetadataByRepoIDAndStage: %v", err)
	}
	preprod, err := models.GetDoor43MetadataByRepoIDAndStage(repo.ID, models.StagePreProd)
	if err != nil {
		log.Error("GetDoor43MetadataByRepoIDAndStage: %v", err)
	}
	draft, err := models.GetDoor43MetadataByRepoIDAndStage(repo.ID, models.StageDraft)
	if err != nil {
		log.Error("GetDoor43MetadataByRepoIDAndStage: %v", err)
	}
	latest, err := models.GetDoor43MetadataByRepoIDAndStage(repo.ID, models.StageLatest)
	if err != nil {
		log.Error("GetDoor43MetadataByRepoIDAndStage: %v", err)
	}

	if draft != nil && ((prod != nil && prod.ReleaseDateUnix >= draft.ReleaseDateUnix) ||
		(preprod != nil && preprod.ReleaseDateUnix >= draft.ReleaseDateUnix)) {
		draft = nil
	}
	if prod != nil && preprod != nil && prod.ReleaseDateUnix >= preprod.ReleaseDateUnix {
		preprod = nil
	}
	if prod != nil {
		prod.Repo = repo
		url := prod.GetReleaseURL()
		catalog.Production = &api.CatalogStage{
			Tag:        prod.BranchOrTag,
			ReleaseURL: &url,
			Released:   prod.GetReleaseDateTime(),
			ZipballURL: prod.GetZipballURL(),
			TarballURL: prod.GetTarballURL(),
		}
	}
	if preprod != nil {
		preprod.Repo = repo
		url := preprod.GetReleaseURL()
		catalog.PreProduction = &api.CatalogStage{
			Tag:        preprod.BranchOrTag,
			ReleaseURL: &url,
			Released:   preprod.GetReleaseDateTime(),
			ZipballURL: preprod.GetZipballURL(),
			TarballURL: preprod.GetTarballURL(),
		}
	}
	if draft != nil {
		draft.Repo = repo
		url := draft.GetReleaseURL()
		catalog.Draft = &api.CatalogStage{
			Tag:        draft.BranchOrTag,
			ReleaseURL: &url,
			Released:   draft.GetReleaseDateTime(),
			ZipballURL: draft.GetZipballURL(),
			TarballURL: draft.GetTarballURL(),
		}
	}
	if latest != nil {
		latest.Repo = repo
		catalog.Latest = &api.CatalogStage{
			Tag:        latest.BranchOrTag,
			ReleaseURL: nil,
			Released:   latest.GetReleaseDateTime(),
			ZipballURL: latest.GetZipballURL(),
			TarballURL: latest.GetTarballURL(),
		}
	}

	apiRepo.Catalog = catalog

	return apiRepo
}
