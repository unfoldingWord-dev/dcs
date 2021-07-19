// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Door43MetadataV4 represents a repository's metadata of a tag or default branch
type Door43MetadataV4 struct {
	ID                     int64         `json:"id"`
	Self                   string        `json:"url"`
	Repo                   string        `json:"repo"`
	Owner                  string        `json:"owner"`
	RepoURL                string        `json:"repo_url"`
	ReleaseURL             string        `json:"release_url"`
	TarballURL             string        `json:"tarbar_url"`
	ZipballURL             string        `json:"zipball_url"`
	Language               string        `json:"lang_code"`
	Subject                string        `json:"subject"`
	Title                  string        `json:"title"`
	BranchOrTag            string        `json:"branch_or_tag_name"`
	Stage                  string        `json:"stage"`
	MetadataURL            string        `json:"metadata_url"`
	MetadataJSONURL        string        `json:"metadata_json_url"`
	MetadataAPIContentsURL string        `json:"metadata_api_contents_url"`
	MetadataVersion        string        `json:"metadata_version"`
	Released               string        `json:"released"`
	Books                  []string      `json:"books"`
	Ingredients            []interface{} `json:"ingredients,omitempty"`
}

// Door43MetadataV5 represents a repository's metadata of a tag or default branch for V5
type Door43MetadataV5 struct {
	ID                     int64         `json:"id"`
	Self                   string        `json:"url"`
	Name                   string        `json:"name"`
	Owner                  string        `json:"owner"`
	FullName               string        `json:"full_name"`
	Repo                   *Repository   `json:"repo"`
	Release                *Release      `json:"release"`
	TarballURL             string        `json:"tarbar_url"`
	ZipballURL             string        `json:"zipball_url"`
	Language               string        `json:"language"`
	Subject                string        `json:"subject"`
	Title                  string        `json:"title"`
	BranchOrTag            string        `json:"branch_or_tag_name"`
	Stage                  string        `json:"stage"`
	MetadataURL            string        `json:"metadata_url"`
	MetadataJSONURL        string        `json:"metadata_json_url"`
	MetadataAPIContentsURL string        `json:"metadata_api_contents_url"`
	MetadataVersion        string        `json:"metadata_version"`
	Released               string        `json:"released"`
	Books                  []string      `json:"books"`
	Ingredients            []interface{} `json:"ingredients,omitempty"`
}

// CatalogSearchResultsV4 results of a successful search for V4
type CatalogSearchResultsV4 struct {
	OK   bool                `json:"ok"`
	Data []*Door43MetadataV4 `json:"data"`
}

// CatalogSearchResultsV5 results of a successful search for V5
type CatalogSearchResultsV5 struct {
	OK   bool                `json:"ok"`
	Data []*Door43MetadataV5 `json:"data"`
}

// CatalogVersionEndpoints Info on the versions of the catalog
type CatalogVersionEndpoints struct {
	Latest   string            `json:"latest"`
	Versions map[string]string `json:"versions"`
}

// CatalogVersionEndpointsResponse response with the endpoints for all versions of the catalog
type CatalogVersionEndpointsResponse struct {
	OK   bool                       `json:"ok"`
	Data []*CatalogVersionEndpoints `json:"data"`
}

// CatalogStages a repo's catalog stages
type CatalogStages struct {
	Production    *CatalogStage `json:"prod"`
	PreProduction *CatalogStage `json:"preprod"`
	Draft         *CatalogStage `json:"draft"`
	Latest        *CatalogStage `json:"latest"`
}

// CatalogStage a repo's catalog stage metadata
type CatalogStage struct {
	Tag        string  `json:"branch_or_tag_name"`
	ReleaseURL *string `json:"release_url"`
	Released   string  `json:"released"`
	ZipballURL string  `json:"zipball_url"`
	TarballURL string  `json:"tarball_url"`
}
