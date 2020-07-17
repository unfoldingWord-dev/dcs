// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Door43MetadataV4 represents a repository's metadata of a tag or default branch
type Door43MetadataV4 struct {
	ID              int64         `json:"id"`
	Self            string        `json:"url"`
	Repo            string        `json:"repo"`
	Owner           string        `json:"owner"`
	RepoURL         string        `json:"repo_url"`
	ReleaseURL      string        `json:"release_url"`
	Language        string        `json:"lang_code"`
	Subject         string        `json:"subject"`
	Title           string        `json:"title"`
	Tag             string        `json:"branch_or_tag_name"`
	Stage           string        `json:"stage"`
	MetadataURL     string        `json:"metadata_url"`
	MetadataFile    string        `json:"metadata_file"`
	MetadataVersion string        `json:"metadata_version"`
	Released        string        `json:"released"`
	Books           []string      `json:"books"`
	Ingredients     []interface{} `json:"ingredients,omitempty"`
}

// CatalogSearchResultsV4 results of a successful search
type CatalogSearchResultsV4 struct {
	OK   bool                `json:"ok"`
	Data []*Door43MetadataV4 `json:"data"`
}
