package models

import (
	"code.gitea.io/gitea/modules/log"
	"sort"
	"strings"
)

func contains(strings []string, str string) bool {
	for _, a := range strings {
		if a == str {
			return true
		}
	}
	return false
}

// GetRepoLanguages gets the languages of the user's repos and returns alphabetized list
func (u *User) GetRepoLanguages() []string {
	var languages []string
	if repos, _, err := GetUserRepositories(&SearchRepoOptions{Actor: u, Private: false, ListOptions: ListOptions{PageSize: 0}}); err != nil {
		log.Error("Error GetUserRepositories: %v", err)
	} else {
		for _, repo := range repos {
			if dm, err := repo.GetDefaultBranchMetadata(); err != nil {
				log.Error("Error GetDefaultBranchMetadata: %v", err)
			} else if dm != nil {
				lang := (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string)
				if lang != "" && !contains(languages, lang) {
					languages = append(languages, lang)
				}
			}
		}
	}
	sort.SliceStable(languages, func(i, j int) bool { return strings.ToLower(languages[i]) < strings.ToLower(languages[j]) })
	return languages
}
