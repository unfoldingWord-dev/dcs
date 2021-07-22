// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"xorm.io/builder"
)

//CatalogOrderBy is used to sort the result
type CatalogOrderBy string

func (s CatalogOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	CatalogOrderByTitle           CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title') ASC"
	CatalogOrderByTitleReverse    CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title') DESC"
	CatalogOrderBySubject         CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject') ASC"
	CatalogOrderBySubjectReverse  CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject') DESC"
	CatalogOrderByTag             CatalogOrderBy = "CAST(TRIM(LEADING 'v' FROM `release`.tag_name) AS unsigned) ASC, `door43_metadata`.branch_or_tag ASC, `door43_metadata`.release_date_unix ASC"
	CatalogOrderByTagReverse      CatalogOrderBy = "CAST(TRIM(LEADING 'v' FROM `release`.tag_name) AS unsigned) DESC, `door43_metadata`.branch_or_tag DESC, `door43_metadata`.release_date_unix DESC"
	CatalogOrderByLangCode        CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier') ASC"
	CatalogOrderByLangCodeReverse CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier') DESC"
	CatalogOrderByOldest          CatalogOrderBy = "`door43_metadata`.release_date_unix ASC"
	CatalogOrderByNewest          CatalogOrderBy = "`door43_metadata`.release_date_unix DESC"
	CatalogOrderByReleases        CatalogOrderBy = "release_count ASC"
	CatalogOrderByReleasesReverse CatalogOrderBy = "release_count DESC"
	CatalogOrderByStars           CatalogOrderBy = "`repository`.num_stars ASC"
	CatalogOrderByStarsReverse    CatalogOrderBy = "`repository`.num_stars DESC"
	CatalogOrderByForks           CatalogOrderBy = "`repository`.num_forks ASC"
	CatalogOrderByForksReverse    CatalogOrderBy = "`repository`.num_forks DESC"
)

// Door43MetadataListDefaultPageSize is the default number of repositories
// to load in memory when running administrative tasks on all (or almost
// all) of them.
// The number should be low enough to avoid filling up all RAM with
// repository data...
const Door43MetadataListDefaultPageSize = 64

// Door43MetadataList contains a list of repositories
type Door43MetadataList []*Door43Metadata

func (dms Door43MetadataList) Len() int {
	return len(dms)
}

func (dms Door43MetadataList) Less(i, j int) bool {
	return dms[i].Repo.FullName() < dms[j].Repo.FullName()
}

func (dms Door43MetadataList) Swap(i, j int) {
	dms[i], dms[j] = dms[j], dms[i]
}

// Door43MetadataListOfMap make list from values of map
func Door43MetadataListOfMap(dmMap map[int64]*Door43Metadata) Door43MetadataList {
	return Door43MetadataList(valuesDoor43Metadata(dmMap))
}

// LoadAttributes loads the attributes for the given Door43MetadataList
func (dms Door43MetadataList) LoadAttributes() error {
	return dms.loadAttributes(x)
}

func (dms Door43MetadataList) loadAttributes(e Engine) error {
	if len(dms) == 0 {
		return nil
	}
	var lastErr error
	for _, dm := range dms {
		if err := dm.loadAttributes(e); err != nil && lastErr == nil {
			lastErr = err
		}
	}
	return lastErr
}

func valuesDoor43Metadata(m map[int64]*Door43Metadata) []*Door43Metadata {
	var values = make([]*Door43Metadata, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// SearchCatalogOptions holds the search options
type SearchCatalogOptions struct {
	ListOptions
	RepoID          int64
	Keywords        []string
	Owners          []string
	Repos           []string
	Tags            []string
	Stage           Stage
	Subjects        []string
	CheckingLevels  []string
	Books           []string
	IncludeHistory  bool
	IncludeMetadata bool
	ShowIngredients bool
	Languages       []string
	OrderBy         []CatalogOrderBy
}

// SearchCatalogCondition creates a query condition according search repository options
func SearchCatalogCondition(opts *SearchCatalogOptions) builder.Cond {
	var repoCond, ownerCond builder.Cond
	if opts.RepoID > 0 {
		repoCond = builder.Eq{"`repository`.ID": opts.RepoID}
	} else {
		repoCond = GetRepoCond(opts.Repos)
		ownerCond = GetOwnerCond(opts.Owners)
	}

	keywordCond := builder.NewCond()
	for _, keyword := range opts.Keywords {
		keywordCond = keywordCond.Or(builder.Like{"`repository`.lower_name", strings.ToLower(keyword)})
		keywordCond = keywordCond.Or(builder.Like{"`user`.lower_name", strings.ToLower(keyword)})
		keywordCond = keywordCond.Or(builder.Like{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title'), '\"', ''))", strings.ToLower(keyword)})
		keywordCond = keywordCond.Or(builder.Like{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject'), '\"', ''))", strings.ToLower(keyword)})
		if opts.IncludeMetadata {
			keywordCond = keywordCond.Or(builder.Expr("JSON_SEARCH(LOWER(`door43_metadata`.metadata), 'one', ?) IS NOT NULL", "%"+strings.ToLower(keyword)+"%"))
		}
	}

	stageCond := GetStageCond(opts.Stage)
	historyCond := GetHistoryCond(opts.Stage, opts.IncludeHistory)

	cond := builder.NewCond().And(GetSubjectCond(opts.Subjects),
		GetBookCond(opts.Books),
		GetLanguageCond(opts.Languages),
		GetCheckingLevelCond(opts.CheckingLevels),
		GetTagCond(opts.Tags),
		repoCond,
		ownerCond,
		stageCond,
		historyCond,
		keywordCond,
		builder.Eq{"`repository`.is_private": false},
		builder.Eq{"`repository`.is_archived": false})

	return cond
}

// SearchCatalog returns catalog repositories based on search options,
// it returns results in given range and number of total results.
func SearchCatalog(opts *SearchCatalogOptions) (Door43MetadataList, int64, error) {
	cond := SearchCatalogCondition(opts)
	return SearchCatalogByCondition(opts, cond, true)
}

// SearchCatalogByCondition search repositories by condition
func SearchCatalogByCondition(opts *SearchCatalogOptions, cond builder.Cond, loadAttributes bool) (Door43MetadataList, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = []CatalogOrderBy{CatalogOrderByNewest}
	}

	sess := x.NewSession()
	defer sess.Close()

	dms := make(Door43MetadataList, 0, opts.PageSize)

	releaseInfoInner, err := builder.Select("`door43_metadata`.repo_id", "COUNT(*) AS release_count", "MAX(`door43_metadata`.release_date_unix) AS latest_unix").
		From("door43_metadata").
		GroupBy("`door43_metadata`.repo_id").
		Where(GetStageCond(opts.Stage)).
		ToBoundSQL()
	if err != nil {
		return nil, 0, err
	}

	releaseInfoOuter, err := builder.Select("`door43_metadata`.repo_id", "MAX(release_count) AS release_count", "MAX(latest_unix) AS latest_unix", "MIN(stage) AS latest_stage").
		From("door43_metadata").
		Join("INNER", "("+releaseInfoInner+") release_info_inner", "`release_info_inner`.repo_id = `door43_metadata`.repo_id AND `door43_metadata`.release_date_unix = `release_info_inner`.latest_unix").
		GroupBy("`door43_metadata`.repo_id").
		ToBoundSQL()
	if err != nil {
		return nil, 0, err
	}

	sess.
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Join("INNER", "user", "`repository`.owner_id = `user`.id").
		Join("LEFT", "release", "`release`.id = `door43_metadata`.release_id").
		Join("INNER", "("+releaseInfoOuter+") release_info", "release_info.repo_id = `door43_metadata`.repo_id").
		Where(cond)

	for _, orderBy := range opts.OrderBy {
		sess.OrderBy(orderBy.String())
	}

	if opts.PageSize > 0 {
		sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	count, err := sess.FindAndCount(&dms)
	if err != nil {
		return nil, 0, fmt.Errorf("FindAndCount: %v", err)
	}

	if loadAttributes {
		if err = dms.loadAttributes(sess); err != nil {
			return nil, 0, fmt.Errorf("loadAttributes: %v", err)
		}
	}

	return dms, count, nil
}

// SplitAtCommaNotInString split s at commas, ignoring commas in strings.
func SplitAtCommaNotInString(s string, requireSpaceAfterComma bool) []string {
	var res []string
	var beg int
	var inString bool
	var prevIsComma bool

	for i := 0; i < len(s); i++ {
		if requireSpaceAfterComma && s[i] == ',' && !inString {
			prevIsComma = true
			continue
		} else if s[i] == ' ' {
			if prevIsComma {
				res = append(res, strings.TrimSpace(s[beg:i-1]))
				beg = i + 1
			} else {
				continue
			}
		} else if !requireSpaceAfterComma && s[i] == ',' && !inString {
			res = append(res, strings.TrimSpace(s[beg:i]))
			beg = i + 1
		} else if s[i] == '"' {
			if !inString {
				inString = true
			} else if i > 0 && s[i-1] != '\\' {
				inString = false
			}
		}
		prevIsComma = false
	}
	return append(res, strings.TrimSpace(s[beg:]))
}

// GetStageCond gets the condition for the given stage
func GetStageCond(stage Stage) builder.Cond {
	return builder.Lte{"`door43_metadata`.stage": stage}
}

// GetHistoryCond gets the conditions if IncludeHistory is true based on stage
func GetHistoryCond(stage Stage, includeHistory bool) builder.Cond {
	if includeHistory {
		return nil
	}
	return builder.And(builder.Expr("`door43_metadata`.release_date_unix = latest_unix"), builder.Expr("`door43_metadata`.stage = latest_stage"))
}

// GetSubjectCond gets the subject condition
func GetSubjectCond(subjects []string) builder.Cond {
	var subjectCond = builder.NewCond()
	for _, subject := range subjects {
		subjectCond = subjectCond.Or(builder.Eq{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject'), '\"', ''))": strings.ToLower(subject)})
	}
	return subjectCond
}

// GetLanguageCond gets the language condition
func GetLanguageCond(languages []string) builder.Cond {
	var langCond = builder.NewCond()
	for _, lang := range languages {
		for _, v := range strings.Split(lang, ",") {
			langCond = langCond.Or(builder.Eq{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier'), '\"', ''))": strings.ToLower(v)})
		}
	}
	return langCond
}

// GetBookCond gets the book condition
func GetBookCond(books []string) builder.Cond {
	var bookCond = builder.NewCond()
	for _, book := range books {
		for _, v := range strings.Split(book, ",") {
			bookCond = bookCond.Or(builder.Expr("JSON_CONTAINS(LOWER(JSON_EXTRACT(`door43_metadata`.metadata, '$.projects')), JSON_OBJECT('identifier', ?))", strings.ToLower(v)))
		}
	}
	return bookCond
}

// GetCheckingLevelCond gets the checking level condition
func GetCheckingLevelCond(checkingLevels []string) builder.Cond {
	var checkingCond = builder.NewCond()
	for _, checking := range checkingLevels {
		for _, v := range strings.Split(checking, ",") {
			checkingCond = checkingCond.Or(builder.Gte{"REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.checking.checking_level'), '\"', '')": v})
		}
	}
	return checkingCond
}

// GetTagCond gets the tag condition
func GetTagCond(tags []string) builder.Cond {
	var tagCond = builder.NewCond()
	for _, tag := range tags {
		for _, v := range strings.Split(tag, ",") {
			tagCond = tagCond.Or(builder.Eq{"`release`.tag_name": v})
		}
	}
	return tagCond
}

// GetRepoCond gets the repo condition
func GetRepoCond(repos []string) builder.Cond {
	var repoCond = builder.NewCond()
	for _, repo := range repos {
		for _, v := range strings.Split(repo, ",") {
			repoCond = repoCond.Or(builder.Eq{"`repository`.lower_name": strings.ToLower(v)})
		}
	}
	return repoCond
}

// GetOwnerCond gets the owner condition
func GetOwnerCond(owners []string) builder.Cond {
	var ownerCond = builder.NewCond()
	for _, owner := range owners {
		for _, v := range strings.Split(owner, ",") {
			ownerCond = ownerCond.Or(builder.Eq{"`user`.lower_name": strings.ToLower(v)})
		}
	}
	return ownerCond
}
