// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// ToUserDCS convert models.User to api.User with DCS customized fields populated
// signed shall only be set if requester is logged in. authed shall only be set if user is site admin or user himself
func ToUserDCS(user, doer *models.User) *api.User {
	if user == nil {
		return nil
	}
	result := ToUser(user, doer)
	result.RepoLanguages = user.GetRepoLanguages()
	result.RepoSubjects = user.GetRepoSubjects()
	return result
}

// ToUsersDCS convert list of models.User to list of api.User with DCS Customizations
func ToUsersDCS(doer *models.User, users []*models.User) []*api.User {
	result := make([]*api.User, len(users))
	for i := range users {
		result[i] = ToUserDCS(users[i], doer)
	}
	return result
}
