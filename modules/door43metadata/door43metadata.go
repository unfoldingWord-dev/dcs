// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43metadata

import (
	"fmt"
	"reflect"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/mitchellh/mapstructure"
	"github.com/unknwon/com"
	"xorm.io/xorm"
)

// GenerateDoor43Metadata Generate door43 metadata for valid repos not in the door43_metadata table
func GenerateDoor43Metadata(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	// Query to find repos that need processing, either having releases that
	// haven't been processed, or their default branch hasn't been processed.
	records, err := sess.Query("SELECT rel.id as release_id, r.id as repo_id  FROM `repository` r " +
		"  JOIN `release` rel ON rel.repo_id = r.id " +
		"  LEFT JOIN `door43_metadata` dm ON r.id = dm.repo_id " +
		"  AND rel.id = dm.release_id " +
		"  WHERE dm.id IS NULL AND rel.is_tag = 0 " +
		"UNION " +
		"SELECT 0 as `release_id`, r2.id as repo_id FROM `repository` r2 " +
		"  LEFT JOIN `door43_metadata` dm2 ON r2.id = dm2.repo_id " +
		"  AND dm2.release_id = 0 " +
		"  WHERE dm2.id IS NULL " +
		"ORDER BY repo_id ASC, release_id ASC")
	if err != nil {
		return err
	}

	cacheRepos := make(map[int64]*models.Repository)

	for _, record := range records {
		releaseID := com.StrTo(record["release_id"]).MustInt64()
		repoID := com.StrTo(record["repo_id"]).MustInt64()
		if cacheRepos[repoID] == nil {
			cacheRepos[repoID], err = models.GetRepositoryByID(repoID)
			if err != nil {
				log.Warn("GetRepositoryByID Error: %v\n", err)
				continue
			}
		}
		repo := cacheRepos[repoID]
		var release *models.Release
		if releaseID > 0 {
			release, err = models.GetReleaseByID(releaseID)
			if err != nil {
				log.Warn("GetReleaseByID Error: %v\n", err)
				continue
			}
		}
		if err = ProcessDoor43MetadataForRepoRelease(repo, release); err != nil {
			continue
		}
	}

	return nil
}

// ConvertGenericMapToRC020Manifest converts a generic map to a RC020Manifest object
func ConvertGenericMapToRC020Manifest(manifest *map[string]interface{}) (*structs.RC020Manifest, error) {
	var rc020manifest structs.RC020Manifest
	err := mapstructure.Decode(*manifest, &rc020manifest)
	if err != nil {
		return nil, err
	}

	type Checking struct {
		CheckingLevel string `mapstructure:"checking_level"`
	}
	type Language struct {
		Identifier    string
		LangDirection string `mapstructure:"lang_direction"`
	}
	type DublinCore struct {
		Subject  string
		Language Language
		TestThis string `mapstructure:"test_this"`
	}
	type Project struct {
		Identifier string
	}
	type Person struct {
		Checking
		DublinCore `mapstructure:"dublin_core"`
		Projects   []Project
	}

	book1 := map[string]interface{}{"identifier": "gen"}
	book2 := map[string]interface{}{"identifier": "exo"}
	input := map[string]interface{}{
		"dublin_core": map[string]interface{}{
			"subject": "test",
			"language": map[string]interface{}{
				"identifier":     "en",
				"lang_direction": "ltr",
			},
			"test_this": "ok",
		},
		"checking": map[string]interface{}{"checking_level": "1"},
		"projects": []map[string]interface{}{book1, book2},
	}

	var result Person
	err = mapstructure.Decode(input, &result)
	if err != nil {
		panic(err)
	}

	return &rc020manifest, err
}

// ProcessDoor43MetadataForRepo handles the metadata for a given repo for all its releases
func ProcessDoor43MetadataForRepo(repo *models.Repository) error {
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}

	if repo.IsArchived || repo.IsPrivate {
		_, err := models.DeleteAllDoor43MetadatasByRepoID(repo.ID)
		if err != nil {
			log.Error("DeleteAllDoor43MetadatasByRepoID: %v", err)
		}
		return err
	}

	relIDs, err := models.GetRepoReleaseIDsForMetadata(repo.ID)
	if err != nil {
		log.Error("GetReleaseIDsNeedingDoor43Metadata: %v", err)
		return err
	}

	for _, releaseID := range relIDs {
		var release *models.Release
		releaseRef := repo.DefaultBranch
		if releaseID > 0 {
			release, err = models.GetReleaseByID(releaseID)
			if err != nil {
				fmt.Printf("GetReleaseByID Error: %v\n", err)
				continue
			}
			releaseRef = release.TagName
		}
		log.Info("Processing Metadata for repo %s (%d), %s (%d)\n", repo.Name, repo.ID, releaseRef, releaseID)
		if err = ProcessDoor43MetadataForRepoRelease(repo, release); err != nil {
			log.Warn("Error processing metadata for repo %s (%d), %s (%d): %v\n", repo.Name, repo.ID, releaseRef, releaseID, err)
		} else {
			log.Info("Processed Metadata for repo %s (%d), %s (%d)\n", repo.Name, repo.ID, releaseRef, releaseID)
		}
	}
	return nil
}

// ProcessDoor43MetadataForRepoRelease handles the metadata for a given repo by release based on if the container is a valid RC or not
func ProcessDoor43MetadataForRepoRelease(repo *models.Repository, release *models.Release) error {
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}
	if release != nil && release.IsTag {
		return fmt.Errorf("release can only be a release, not a tag")
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		fmt.Printf("OpenRepository Error: %v\n", err)
		return err
	}
	defer gitRepo.Close()

	var commit *git.Commit
	if release == nil {
		commit, err = gitRepo.GetBranchCommit(repo.DefaultBranch)
		if err != nil {
			log.Error("GetBranchCommit: %v\n", err)
			return err
		}
	} else if !release.IsDraft {
		commit, err = gitRepo.GetTagCommit(release.TagName)
		if err != nil {
			log.Error("GetTagCommit: %v\n", err)
			return err
		}
	} else {
		commit, err = gitRepo.GetBranchCommit(release.Target)
		if err != nil {
			log.Error("GetBranchCommit: %v\n", err)
			return err
		}
	}

	blob, err := commit.GetBlobByPath("manifest.yaml")
	if err != nil && !git.IsErrNotExist(err) {
		return err
	}
	if blob == nil {
		return nil
	}

	manifest, err := base.ReadYAMLFromBlob(blob)
	if err != nil {
		return err
	}

	result, err := base.ValidateBlobByRC020Schema(manifest)
	if err != nil {
		return err
	}

	var releaseID int64
	var stage models.Stage
	if release != nil {
		releaseID = release.ID
		if release.IsDraft {
			stage = models.StageDraft
		} else if release.IsPrerelease {
			stage = models.StagePreProd
		} else {
			stage = models.StageProd
		}
	} else {
		stage = models.StageLatest
	}

	dm, err := models.GetDoor43MetadataByRepoIDAndReleaseID(repo.ID, releaseID)
	if err != nil && !models.IsErrDoor43MetadataNotExist(err) {
		return err
	}

	//metadata, err := ConvertGenericMapToRC020Manifest(manifest)
	//if err != nil {
	//	return err
	//}

	var releaseDateUnix timeutil.TimeStamp
	var branchOrTag string
	if release != nil && !release.IsDraft {
		releaseDateUnix = release.CreatedUnix
		branchOrTag = release.TagName
	} else {
		releaseDateUnix = timeutil.TimeStamp(commit.Author.When.Unix())
		if release != nil {
			branchOrTag = release.Target
		} else {
			branchOrTag = repo.DefaultBranch
		}
	}

	if dm == nil ||
		releaseDateUnix != dm.ReleaseDateUnix ||
		dm.Stage != stage ||
		dm.BranchOrTag != branchOrTag ||
		!reflect.DeepEqual(dm.Metadata, manifest) {
		if !result.Valid() {
			log.Warn("%s/%s: manifest.yaml is not valid. see errors:", repo.FullName(), branchOrTag)
			log.Warn("REPO ID: %d, RELEASE ID: %d", repo.ID, releaseID)
			if release != nil {
				log.Warn("RELEASE: %v", release.TagName)
			} else {
				log.Warn("BRANCH: %s", repo.DefaultBranch)
			}
			for _, desc := range result.Errors() {
				log.Warn("- %s", desc.Description())
				log.Warn("- %s = %s", desc.Field(), desc.Value())
			}
			if dm != nil {
				return models.DeleteDoor43Metadata(dm)
			}
		} else {
			log.Warn("%s/%s: manifest.yaml is valid.", repo.FullName(), branchOrTag)
			if dm == nil {
				dm = &models.Door43Metadata{
					RepoID:          repo.ID,
					Repo:            repo,
					ReleaseID:       releaseID,
					Release:         release,
					ReleaseDateUnix: releaseDateUnix,
					MetadataVersion: "rc0.2",
					Metadata:        manifest,
					Stage:           stage,
					BranchOrTag:     branchOrTag,
				}
				return models.InsertDoor43Metadata(dm)
			}
			dm.Metadata = manifest
			dm.ReleaseDateUnix = releaseDateUnix
			dm.Stage = stage
			dm.BranchOrTag = branchOrTag
			return models.UpdateDoor43MetadataCols(dm, "metadata", "release_date_unix", "stage", "branch_or_tag")
		}
	}

	return nil
}
