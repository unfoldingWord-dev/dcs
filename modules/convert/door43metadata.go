// Copyright 2021 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToDoor43MetadataV4 converts a Door43Metadata to api.Door43Metadata for Catalog V4
func ToDoor43MetadataV4(dm *models.Door43Metadata, mode models.AccessMode) *api.Door43MetadataV4 {
	err := dm.LoadAttributes()
	if err != nil {
		log.Error("loadAttributes: %v", err)
		return nil
	}
	return &api.Door43MetadataV4{
		ID:                     dm.ID,
		Self:                   dm.APIURLV4(),
		Repo:                   dm.Repo.Name,
		Owner:                  dm.Repo.OwnerName,
		RepoURL:                dm.Repo.APIURL(),
		ReleaseURL:             dm.GetReleaseURL(),
		TarballURL:             dm.GetTarballURL(),
		ZipballURL:             dm.GetZipballURL(),
		Language:               (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string),
		Subject:                (*dm.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string),
		Title:                  (*dm.Metadata)["dublin_core"].(map[string]interface{})["title"].(string),
		Books:                  dm.GetBooks(),
		BranchOrTag:            dm.BranchOrTag,
		Stage:                  dm.Stage.String(),
		Released:               dm.ReleaseDateUnix.FormatDate(),
		MetadataVersion:        dm.MetadataVersion,
		MetadataURL:            dm.GetMetadataURL(),
		MetadataJSONURL:        dm.GetMetadataJSONURL(),
		MetadataAPIContentsURL: dm.GetMetadataAPIContentsURL(),
		Ingredients:            (*dm.Metadata)["projects"].([]interface{}),
	}
}

// ToDoor43MetadataV5 converts a Door43Metadata to api.Door43Metadata for Catalog V5
func ToDoor43MetadataV5(dm *models.Door43Metadata, mode models.AccessMode) *api.Door43MetadataV5 {
	if err := dm.LoadAttributes(); err != nil {
		return nil
	}

	if err := dm.Repo.GetOwner(); err != nil {
		return nil
	}

	var release *api.Release
	if dm.Release != nil {
		release = ToRelease(dm.Release)
	}

	return &api.Door43MetadataV5{
		ID:                     dm.ID,
		Self:                   dm.APIURLV5(),
		Name:                   dm.Repo.Name,
		Owner:                  dm.Repo.OwnerName,
		FullName:               dm.Repo.FullName(),
		Repo:                   innerToRepo(dm.Repo, mode, true),
		Release:                release,
		TarballURL:             dm.GetTarballURL(),
		ZipballURL:             dm.GetZipballURL(),
		Language:               (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string),
		Subject:                (*dm.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string),
		Title:                  (*dm.Metadata)["dublin_core"].(map[string]interface{})["title"].(string),
		Books:                  dm.GetBooks(),
		BranchOrTag:            dm.BranchOrTag,
		Stage:                  dm.Stage.String(),
		Released:               dm.GetReleaseDateTime(),
		MetadataVersion:        dm.MetadataVersion,
		MetadataURL:            dm.GetMetadataURL(),
		MetadataJSONURL:        dm.GetMetadataJSONURL(),
		MetadataAPIContentsURL: dm.GetMetadataAPIContentsURL(),
		Ingredients:            (*dm.Metadata)["projects"].([]interface{}),
	}
}
