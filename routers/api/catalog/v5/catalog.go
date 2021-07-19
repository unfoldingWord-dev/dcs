// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v5

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

var searchOrderByMap = map[string]map[string]models.CatalogOrderBy{
	"asc": {
		"subject":  models.CatalogOrderBySubject,
		"title":    models.CatalogOrderByTitle,
		"released": models.CatalogOrderByOldest,
		"lang":     models.CatalogOrderByLangCode,
		"releases": models.CatalogOrderByReleases,
		"stars":    models.CatalogOrderByStars,
		"forks":    models.CatalogOrderByForks,
		"tag":      models.CatalogOrderByTag,
	},
	"desc": {
		"title":    models.CatalogOrderByTitleReverse,
		"subject":  models.CatalogOrderBySubjectReverse,
		"released": models.CatalogOrderByNewest,
		"lang":     models.CatalogOrderByLangCodeReverse,
		"releases": models.CatalogOrderByReleasesReverse,
		"stars":    models.CatalogOrderByStarsReverse,
		"forks":    models.CatalogOrderByForksReverse,
		"tag":      models.CatalogOrderByTagReverse,
	},
}

// Search search the catalog via options
func Search(ctx *context.APIContext) {
	// swagger:operation GET /v5/search v5 v5Search
	// ---
	// summary: Catalog search
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: keyword(s). Can use multiple `q=<keyword>`s or commas for more than one keyword
	//   type: string
	// - name: owner
	//   in: query
	//   description: search only for entries with the given owner name(s).
	//   type: string
	// - name: repo
	//   in: query
	//   description: search only for entries with the given repo name(s).
	//   type: string
	// - name: tag
	//   in: query
	//   description: search only for entries with the given release tag(s)
	//   type: string
	// - name: lang
	//   in: query
	//   description: search only for entries with the given language(s)
	//   type: string
	// - name: stage
	//   in: query
	//   description: 'specifies which release stage to be return of these stages:
	//                "prod" - return only the production releases (default);
	//                "preprod" - return the pre-production release if it exists instead of the production release;
	//                "draft" - return the draft release if it exists instead of pre-production or production release;
	//                "latest" -return the default branch (e.g. master) if it is a valid RC instead of the above'
	//   type: string
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). Must match the entire string (case insensitive)
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (project ids)
	//   type: string
	// - name: includeHistory
	//   in: query
	//   description: if true, all releases, not just the latest, are included. Default is false
	//   type: boolean
	// - name: includeMetadata
	//   in: query
	//   description: if false, only subject and title are searched with query terms, if true all metadata values are searched. Default is true
	//   type: boolean
	// - name: showIngredients
	//   in: query
	//   description: if true, a list of the projects in the resource and their file paths will be listed for each entry. Default is false
	//   type: boolean
	// - name: sort
	//   in: query
	//   description: sort repos alphanumerically by attribute. Supported values are
	//                "subject", "title", "tag", "released", "lang", "releases", "stars", "forks".
	//                Default is by "language", "subject" and then "tag"
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResultsV5"
	//   "422":
	//     "$ref": "#/responses/validationError"

	searchCatalog(ctx)
}

// SearchOwner search the catalog via owner and via options
func SearchOwner(ctx *context.APIContext) {
	// swagger:operation GET /v5/search/{owner} v5 v5SearchOwner
	// ---
	// summary: Catalog search by owner
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of entries
	//   type: string
	//   required: true
	// - name: q
	//   in: query
	//   description: keyword(s). Can use multiple `q=<keyword>`s or commas for more than one keyword
	//   type: string
	// - name: repo
	//   in: query
	//   description: search only for entries with the given repo name(s).
	//   type: string
	// - name: tag
	//   in: query
	//   description: search only for entries with the given release tag(s)
	//   type: string
	// - name: lang
	//   in: query
	//   description: search only for entries with the given language(s)
	//   type: string
	// - name: stage
	//   in: query
	//   description: 'specifies which release stage to be return of these stages:
	//                "prod" - return only the production releases (default);
	//                "preprod" - return the pre-production release if it exists instead of the production release;
	//                "draft" - return the draft release if it exists instead of pre-production or production release;
	//                "latest" -return the default branch (e.g. master) if it is a valid RC instead of the above'
	//   type: string
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). Must match the entire string (case insensitive)
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (project ids)
	//   type: string
	// - name: includeHistory
	//   in: query
	//   description: if true, all releases, not just the latest, are included. Default is false
	//   type: boolean
	// - name: includeMetadata
	//   in: query
	//   description: if false, only subject and title are searched with query terms, if true all metadata values are searched. Default is true
	//   type: boolean
	// - name: showIngredients
	//   in: query
	//   description: if true, a list of the projects in the resource and their file paths will be listed for each entry. Default is false
	//   type: boolean
	// - name: sort
	//   in: query
	//   description: sort repos alphanumerically by attribute. Supported values are
	//                "subject", "title", "tag", "released", "lang", "releases", "stars", "forks".
	//                Default is by "language", "subject" and then "tag"
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResultsV5"
	//   "422":
	//     "$ref": "#/responses/validationError"

	searchCatalog(ctx)
}

// SearchRepo search the catalog via repo and options
func SearchRepo(ctx *context.APIContext) {
	// swagger:operation GET /v5/search/{owner}/{repo} v5 v5SearchRepo
	// ---
	// summary: Catalog search by repo
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: q
	//   in: query
	//   description: keyword(s). Can use multiple `q=<keyword>`s or commas for more than one keyword
	//   type: string
	// - name: tag
	//   in: query
	//   description: search only for entries with the given release tag(s)
	//   type: string
	// - name: lang
	//   in: query
	//   description: search only for entries with the given language(s)
	//   type: string
	// - name: stage
	//   in: query
	//   description: 'specifies which release stage to be return of these stages:
	//                "prod" - return only the production releases (default);
	//                "preprod" - return the pre-production release if it exists instead of the production release;
	//                "draft" - return the draft release if it exists instead of pre-production or production release;
	//                "latest" -return the default branch (e.g. master) if it is a valid RC instead of the above'
	//   type: string
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). Must match the entire string (case insensitive)
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (project ids)
	//   type: string
	// - name: includeHistory
	//   in: query
	//   description: if true, all releases, not just the latest, are included. Default is false
	//   type: boolean
	// - name: includeMetadata
	//   in: query
	//   description: if false, only subject and title are searched with query terms, if true all metadata values are searched. Default is true
	//   type: boolean
	// - name: showIngredients
	//   in: query
	//   description: if true, a list of the projects in the resource and their file paths will be listed for each entry. Default is false
	//   type: boolean
	// - name: sort
	//   in: query
	//   description: sort repos alphanumerically by attribute. Supported values are
	//                "subject", "title", "tag", "released", "lang", "releases", "stars", "forks".
	//                Default is language,subject,tag
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResultsV5"
	//   "422":
	//     "$ref": "#/responses/validationError"

	searchCatalog(ctx)
}

// GetCatalogEntry Get the catalog entry from the given ownername, reponame and ref
func GetCatalogEntry(ctx *context.APIContext) {
	// swagger:operation GET /v5/entry/{owner}/{repo}/{tag} v5 v5GetCatalogEntry
	// ---
	// summary: Catalog entry
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: tag
	//   in: path
	//   description: release tag or default branch
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogEntryV5"
	//   "422":
	//     "$ref": "#/responses/validationError"

	tag := ctx.Params("tag")
	var dm *models.Door43Metadata
	var err error
	if tag == ctx.Repo.Repository.DefaultBranch {
		dm, err = models.GetDoor43MetadataByRepoIDAndReleaseID(ctx.Repo.Repository.ID, 0)
	} else {
		dm, err = models.GetDoor43MetadataByRepoIDAndTagName(ctx.Repo.Repository.ID, tag)
	}
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndTagName", err)
		return
	}
	if err := dm.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndTagName", err)
		return
	}
	accessMode, err := models.AccessLevel(ctx.User, dm.Repo)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
	}
	ctx.JSON(http.StatusOK, convert.ToDoor43MetadataV5(dm, accessMode))
}

// GetCatalogMetadata Get the metadata (RC 0.2.0 manifest) in JSON format for the given ownername, reponame and ref
func GetCatalogMetadata(ctx *context.APIContext) {
	// swagger:operation GET /v5/entry/{owner}/{repo}/{tag}/metadata v5 v5GetMetadata
	// ---
	// summary: Catalog entry metadata (manifest.yaml in JSON format)
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: tag
	//   in: path
	//   description: release tag or default branch
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogMetadata"
	//   "422":
	//     "$ref": "#/responses/validationError"

	dm, err := models.GetDoor43MetadataByRepoIDAndTagName(ctx.Repo.Repository.ID, ctx.Repo.TagName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndTagName", err)
		return
	}
	ctx.JSON(http.StatusOK, dm.Metadata)
}

// QueryStrings After calling QueryStrings on the context, it also separates strings that have commas into substrings
func QueryStrings(ctx *context.APIContext, name string) []string {
	strs := ctx.QueryStrings(name)
	if len(strs) == 0 {
		return strs
	}
	var newStrs []string
	for _, str := range strs {
		newStrs = append(newStrs, models.SplitAtCommaNotInString(str, false)...)
	}
	return newStrs
}

func searchCatalog(ctx *context.APIContext) {
	var repoID int64
	var owners, repos []string
	includeMetadata := true
	if ctx.Repo.Repository != nil {
		repoID = ctx.Repo.Repository.ID
	} else {
		if ctx.Params("username") != "" {
			owners = []string{ctx.Params("username")}
		} else {
			owners = QueryStrings(ctx, "owner")
		}
		repos = QueryStrings(ctx, "repo")
	}
	if ctx.Query("includeMetadata") != "" {
		includeMetadata = ctx.QueryBool("includeMetadata")
	}

	stageStr := ctx.Query("stage")
	var stage models.Stage
	if stageStr != "" {
		var ok bool
		stage, ok = models.StageMap[stageStr]
		if !ok {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("invalid stage: \"%s\"", stageStr))
			return
		}
	}

	keywords := []string{}
	query := strings.Trim(ctx.Query("q"), " ")
	if query != "" {
		keywords = models.SplitAtCommaNotInString(query, false)
	}
	listOptions := utils.GetListOptions(ctx)

	opts := &models.SearchCatalogOptions{
		ListOptions:     listOptions,
		Keywords:        keywords,
		Owners:          owners,
		Repos:           repos,
		RepoID:          repoID,
		Tags:            QueryStrings(ctx, "tag"),
		Stage:           stage,
		Languages:       QueryStrings(ctx, "lang"),
		Subjects:        QueryStrings(ctx, "subject"),
		CheckingLevels:  QueryStrings(ctx, "checkingLevel"),
		Books:           QueryStrings(ctx, "book"),
		IncludeHistory:  ctx.QueryBool("includeHistory"),
		ShowIngredients: ctx.QueryBool("showIngredients"),
		IncludeMetadata: includeMetadata,
	}

	var sortModes = QueryStrings(ctx, "sort")
	if len(sortModes) > 0 {
		var sortOrder = ctx.Query("order")
		if sortOrder == "" {
			sortOrder = "asc"
		}
		if searchModeMap, ok := searchOrderByMap[sortOrder]; ok {
			for _, sortMode := range sortModes {
				if orderBy, ok := searchModeMap[strings.ToLower(sortMode)]; ok {
					opts.OrderBy = append(opts.OrderBy, orderBy)
				} else {
					ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("invalid sort mode: \"%s\"", sortMode))
					return
				}
			}
		} else {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("invalid sort order: \"%s\"", sortOrder))
			return
		}
	} else {
		opts.OrderBy = []models.CatalogOrderBy{models.CatalogOrderByLangCode, models.CatalogOrderBySubject, models.CatalogOrderByTagReverse}
	}

	dms, count, err := models.SearchCatalog(opts)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	results := make([]*api.Door43MetadataV5, len(dms))
	for i, dm := range dms {
		accessMode, err := models.AccessLevel(ctx.User, dm.Repo)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, api.SearchError{
				OK:    false,
				Error: err.Error(),
			})
		}
		results[i] = convert.ToDoor43MetadataV5(dm, accessMode)
		if !opts.ShowIngredients {
			results[i].Ingredients = nil
		}
	}

	ctx.SetLinkHeader(int(count), opts.PageSize)
	ctx.Header().Set("X-Total-Count", fmt.Sprintf("%d", count))
	ctx.JSON(http.StatusOK, api.CatalogSearchResultsV5{
		OK:   true,
		Data: results,
	})
}
