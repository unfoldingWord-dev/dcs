// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplCompare base.TplName = "repo/diff/compare"
)

// ParseCompareInfo parse compare info between two commit for preparing comparing references
func ParseCompareInfo(ctx *context.Context) (*models.User, *models.Repository, *git.Repository, *git.CompareInfo, string, string) {
	baseRepo := ctx.Repo.Repository

	// Get compared branches information
	// format: <base branch>...[<head repo>:]<head branch>
	// base<-head: master...head:feature
	// same repo: master...feature

	var (
		headUser   *models.User
		headBranch string
		isSameRepo bool
		infoPath   string
		err        error
	)
	infoPath = ctx.Params("*")
	infos := strings.Split(infoPath, "...")
	if len(infos) != 2 {
		log.Trace("ParseCompareInfo[%d]: not enough compared branches information %s", baseRepo.ID, infos)
		ctx.NotFound("CompareAndPullRequest", nil)
		return nil, nil, nil, nil, "", ""
	}

	baseBranch := infos[0]
	ctx.Data["BaseBranch"] = baseBranch

	// If there is no head repository, it means compare between same repository.
	headInfos := strings.Split(infos[1], ":")
	if len(headInfos) == 1 {
		isSameRepo = true
		headUser = ctx.Repo.Owner
		headBranch = headInfos[0]

	} else if len(headInfos) == 2 {
		headUser, err = models.GetUserByName(headInfos[0])
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.NotFound("GetUserByName", nil)
			} else {
				ctx.ServerError("GetUserByName", err)
			}
			return nil, nil, nil, nil, "", ""
		}
		headBranch = headInfos[1]
		isSameRepo = headUser.ID == ctx.Repo.Owner.ID
	} else {
		ctx.NotFound("CompareAndPullRequest", nil)
		return nil, nil, nil, nil, "", ""
	}
	ctx.Data["HeadUser"] = headUser
	ctx.Data["HeadBranch"] = headBranch
	ctx.Repo.PullRequest.SameRepo = isSameRepo

	// Check if base branch is valid.
	baseIsCommit := ctx.Repo.GitRepo.IsCommitExist(baseBranch)
	baseIsBranch := ctx.Repo.GitRepo.IsBranchExist(baseBranch)
	baseIsTag := ctx.Repo.GitRepo.IsTagExist(baseBranch)
	if !baseIsCommit && !baseIsBranch && !baseIsTag {
		// Check if baseBranch is short sha commit hash
		if baseCommit, _ := ctx.Repo.GitRepo.GetCommit(baseBranch); baseCommit != nil {
			baseBranch = baseCommit.ID.String()
			ctx.Data["BaseBranch"] = baseBranch
			baseIsCommit = true
		} else {
			ctx.NotFound("IsRefExist", nil)
			return nil, nil, nil, nil, "", ""
		}
	}
	ctx.Data["BaseIsCommit"] = baseIsCommit
	ctx.Data["BaseIsBranch"] = baseIsBranch
	ctx.Data["BaseIsTag"] = baseIsTag

	// Check if current user has fork of repository or in the same repository.
	headRepo, has := models.HasForkedRepo(headUser.ID, baseRepo.ID)
	if !has && !isSameRepo {
		ctx.Data["PageIsComparePull"] = false
	}

	var headGitRepo *git.Repository
	if isSameRepo {
		headRepo = ctx.Repo.Repository
		headGitRepo = ctx.Repo.GitRepo
		ctx.Data["BaseName"] = headUser.Name
	} else {
		headGitRepo, err = git.OpenRepository(models.RepoPath(headUser.Name, headRepo.Name))
		ctx.Data["BaseName"] = baseRepo.OwnerName
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return nil, nil, nil, nil, "", ""
		}
	}

	// user should have permission to read baseRepo's codes and pulls, NOT headRepo's
	permBase, err := models.GetUserRepoPermission(baseRepo, ctx.User)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", ""
	}
	if !permBase.CanRead(models.UnitTypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.User,
				baseRepo,
				permBase)
		}
		ctx.NotFound("ParseCompareInfo", nil)
		return nil, nil, nil, nil, "", ""
	}

	// user should have permission to read headrepo's codes
	permHead, err := models.GetUserRepoPermission(headRepo, ctx.User)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", ""
	}
	if !permHead.CanRead(models.UnitTypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in headRepo has Permissions: %-+v",
				ctx.User,
				headRepo,
				permHead)
		}
		ctx.NotFound("ParseCompareInfo", nil)
		return nil, nil, nil, nil, "", ""
	}

	// Check if head branch is valid.
	headIsCommit := ctx.Repo.GitRepo.IsCommitExist(headBranch)
	headIsBranch := headGitRepo.IsBranchExist(headBranch)
	headIsTag := headGitRepo.IsTagExist(headBranch)
	if !headIsCommit && !headIsBranch && !headIsTag {
		// Check if headBranch is short sha commit hash
		if headCommit, _ := ctx.Repo.GitRepo.GetCommit(headBranch); headCommit != nil {
			headBranch = headCommit.ID.String()
			ctx.Data["HeadBranch"] = headBranch
			headIsCommit = true
		} else {
			ctx.NotFound("IsRefExist", nil)
			return nil, nil, nil, nil, "", ""
		}
	}
	ctx.Data["HeadIsCommit"] = headIsCommit
	ctx.Data["HeadIsBranch"] = headIsBranch
	ctx.Data["HeadIsTag"] = headIsTag

	// Treat as pull request if both references are branches
	if ctx.Data["PageIsComparePull"] == nil {
		ctx.Data["PageIsComparePull"] = headIsBranch && baseIsBranch
	}

	if ctx.Data["PageIsComparePull"] == true && !permBase.CanReadIssuesOrPulls(true) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot create/read pull requests in Repo: %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.User,
				baseRepo,
				permBase)
		}
		ctx.NotFound("ParseCompareInfo", nil)
		return nil, nil, nil, nil, "", ""
	}

	compareInfo, err := headGitRepo.GetCompareInfo(models.RepoPath(baseRepo.Owner.Name, baseRepo.Name), baseBranch, headBranch)
	if err != nil {
		ctx.ServerError("GetCompareInfo", err)
		return nil, nil, nil, nil, "", ""
	}
	ctx.Data["BeforeCommitID"] = compareInfo.MergeBase

	return headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch
}

// PrepareCompareDiff renders compare diff page
func PrepareCompareDiff(
	ctx *context.Context,
	headUser *models.User,
	headRepo *models.Repository,
	headGitRepo *git.Repository,
	compareInfo *git.CompareInfo,
	baseBranch, headBranch string) bool {

	var (
		repo  = ctx.Repo.Repository
		err   error
		title string
	)

	// Get diff information.
	ctx.Data["CommitRepoLink"] = headRepo.Link()

	headCommitID := headBranch
	if ctx.Data["HeadIsCommit"] == false {
		if ctx.Data["HeadIsTag"] == true {
			headCommitID, err = headGitRepo.GetTagCommitID(headBranch)
		} else {
			headCommitID, err = headGitRepo.GetBranchCommitID(headBranch)
		}
		if err != nil {
			ctx.ServerError("GetRefCommitID", err)
			return false
		}
	}

	ctx.Data["AfterCommitID"] = headCommitID

	if headCommitID == compareInfo.MergeBase {
		ctx.Data["IsNothingToCompare"] = true
		return true
	}

	diff, err := models.GetDiffRange(models.RepoPath(headUser.Name, headRepo.Name),
		compareInfo.MergeBase, headCommitID, setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles)
	if err != nil {
		ctx.ServerError("GetDiffRange", err)
		return false
	}
	ctx.Data["Diff"] = diff
	ctx.Data["DiffNotAvailable"] = diff.NumFiles() == 0

	headCommit, err := headGitRepo.GetCommit(headCommitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return false
	}

	compareInfo.Commits = models.ValidateCommitsWithEmails(compareInfo.Commits)
	compareInfo.Commits = models.ParseCommitsWithSignature(compareInfo.Commits)
	compareInfo.Commits = models.ParseCommitsWithStatus(compareInfo.Commits, headRepo)
	ctx.Data["Commits"] = compareInfo.Commits
	ctx.Data["CommitCount"] = compareInfo.Commits.Len()
	if ctx.Data["CommitCount"] == 0 {
		ctx.Data["PageIsComparePull"] = false
	}

	if compareInfo.Commits.Len() == 1 {
		c := compareInfo.Commits.Front().Value.(models.SignCommitWithStatuses)
		title = strings.TrimSpace(c.UserCommit.Summary())

		body := strings.Split(strings.TrimSpace(c.UserCommit.Message()), "\n")
		if len(body) > 1 {
			ctx.Data["content"] = strings.Join(body[1:], "\n")
		}
	} else {
		title = headBranch
	}

	ctx.Data["title"] = title
	ctx.Data["Username"] = headUser.Name
	ctx.Data["Reponame"] = headRepo.Name
	ctx.Data["IsImageFile"] = headCommit.IsImageFile

	headTarget := path.Join(headUser.Name, repo.Name)
	ctx.Data["SourcePath"] = setting.AppSubURL + "/" + path.Join(headTarget, "src", "commit", headCommitID)
	ctx.Data["BeforeSourcePath"] = setting.AppSubURL + "/" + path.Join(headTarget, "src", "commit", compareInfo.MergeBase)
	ctx.Data["RawPath"] = setting.AppSubURL + "/" + path.Join(headTarget, "raw", "commit", headCommitID)
	return false
}

// CompareDiff show different from one commit to another commit
func CompareDiff(ctx *context.Context) {
	headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch := ParseCompareInfo(ctx)
	if ctx.Written() {
		return
	}

	nothingToCompare := PrepareCompareDiff(ctx, headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch)
	if ctx.Written() {
		return
	}

	if ctx.Data["PageIsComparePull"] == true {
		headBranches, err := headGitRepo.GetBranches()
		if err != nil {
			ctx.ServerError("GetBranches", err)
			return
		}
		ctx.Data["HeadBranches"] = headBranches

		pr, err := models.GetUnmergedPullRequest(headRepo.ID, ctx.Repo.Repository.ID, headBranch, baseBranch)
		if err != nil {
			if !models.IsErrPullRequestNotExist(err) {
				ctx.ServerError("GetUnmergedPullRequest", err)
				return
			}
		} else {
			ctx.Data["HasPullRequest"] = true
			ctx.Data["PullRequest"] = pr
			ctx.HTML(200, tplCompareDiff)
			return
		}

		if !nothingToCompare {
			// Setup information for new form.
			RetrieveRepoMetas(ctx, ctx.Repo.Repository)
			if ctx.Written() {
				return
			}
		}
	}
	beforeCommitID := ctx.Data["BeforeCommitID"].(string)
	afterCommitID := ctx.Data["AfterCommitID"].(string)

	ctx.Data["Title"] = "Comparing " + base.ShortSha(beforeCommitID) + "..." + base.ShortSha(afterCommitID)

	ctx.Data["IsRepoToolbarCommits"] = true
	ctx.Data["IsDiffCompare"] = true
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireTribute"] = true
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	setTemplateIfExists(ctx, pullRequestTemplateKey, pullRequestTemplateCandidates)
	renderAttachmentSettings(ctx)

	ctx.HTML(200, tplCompare)
}
