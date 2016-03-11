// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "github.com/gogits/go-gogs-client"

	"github.com/gogits/gogs/models"
	"github.com/gogits/gogs/modules/context"
	"github.com/gogits/gogs/modules/setting"
	"github.com/gogits/gogs/routers/api/v1/convert"
	"github.com/gogits/gogs/routers/api/v1/repo"
)

func GetUserByParamsName(ctx *context.Context, name string) *models.User {
	user, err := models.GetUserByName(ctx.Params(name))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(404)
		} else {
			ctx.APIError(500, "GetUserByName", err)
		}
		return nil
	}
	return user
}

// GetUserByParams returns user whose name is presented in URL paramenter.
func GetUserByParams(ctx *context.Context) *models.User {
	return GetUserByParamsName(ctx, ":username")
}

func composePublicKeysAPILink() string {
	return setting.AppUrl + "api/v1/user/keys/"
}

func listPublicKeys(ctx *context.Context, uid int64) {
	keys, err := models.ListPublicKeys(uid)
	if err != nil {
		ctx.APIError(500, "ListPublicKeys", err)
		return
	}

	apiLink := composePublicKeysAPILink()
	apiKeys := make([]*api.PublicKey, len(keys))
	for i := range keys {
		apiKeys[i] = convert.ToApiPublicKey(apiLink, keys[i])
	}

	ctx.JSON(200, &apiKeys)
}

// https://github.com/gogits/go-gogs-client/wiki/Users-Public-Keys#list-your-public-keys
func ListMyPublicKeys(ctx *context.Context) {
	listPublicKeys(ctx, ctx.User.Id)
}

// https://github.com/gogits/go-gogs-client/wiki/Users-Public-Keys#list-public-keys-for-a-user
func ListPublicKeys(ctx *context.Context) {
	user := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listPublicKeys(ctx, user.Id)
}

// https://github.com/gogits/go-gogs-client/wiki/Users-Public-Keys#get-a-single-public-key
func GetPublicKey(ctx *context.Context) {
	key, err := models.GetPublicKeyByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrKeyNotExist(err) {
			ctx.Error(404)
		} else {
			ctx.Handle(500, "GetPublicKeyByID", err)
		}
		return
	}

	apiLink := composePublicKeysAPILink()
	ctx.JSON(200, convert.ToApiPublicKey(apiLink, key))
}

// CreateUserPublicKey creates new public key to given user by ID.
func CreateUserPublicKey(ctx *context.Context, form api.CreateKeyOption, uid int64) {
	content, err := models.CheckPublicKeyString(form.Key)
	if err != nil {
		repo.HandleCheckKeyStringError(ctx, err)
		return
	}

	key, err := models.AddPublicKey(uid, form.Title, content)
	if err != nil {
		repo.HandleAddKeyError(ctx, err)
		return
	}
	apiLink := composePublicKeysAPILink()
	ctx.JSON(201, convert.ToApiPublicKey(apiLink, key))
}

// https://github.com/gogits/go-gogs-client/wiki/Users-Public-Keys#create-a-public-key
func CreatePublicKey(ctx *context.Context, form api.CreateKeyOption) {
	CreateUserPublicKey(ctx, form, ctx.User.Id)
}

// https://github.com/gogits/go-gogs-client/wiki/Users-Public-Keys#delete-a-public-key
func DeletePublicKey(ctx *context.Context) {
	if err := models.DeletePublicKey(ctx.User, ctx.ParamsInt64(":id")); err != nil {
		if models.IsErrKeyAccessDenied(err) {
			ctx.APIError(403, "", "You do not have access to this key")
		} else {
			ctx.APIError(500, "DeletePublicKey", err)
		}
		return
	}

	ctx.Status(204)
}
