// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	comment_service "code.gitea.io/gitea/services/comments"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"

	"github.com/unknwon/com"
)

const (
	tplAttachment base.TplName = "repo/issue/view_content/attachments"

	tplIssues    base.TplName = "repo/issue/list"
	tplIssueNew  base.TplName = "repo/issue/new"
	tplIssueView base.TplName = "repo/issue/view"

	tplReactions base.TplName = "repo/issue/view_content/reactions"

	issueTemplateKey = "IssueTemplate"
)

var (
	// ErrTooManyFiles upload too many files
	ErrTooManyFiles = errors.New("Maximum number of files to upload exceeded")
	// IssueTemplateCandidates issue templates
	IssueTemplateCandidates = []string{
		"ISSUE_TEMPLATE.md",
		"issue_template.md",
		".gitea/ISSUE_TEMPLATE.md",
		".gitea/issue_template.md",
		".github/ISSUE_TEMPLATE.md",
		".github/issue_template.md",
	}
)

// MustAllowUserComment checks to make sure if an issue is locked.
// If locked and user has permissions to write to the repository,
// then the comment is allowed, else it is blocked
func MustAllowUserComment(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) && !ctx.User.IsAdmin {
		ctx.Flash.Error(ctx.Tr("repo.issues.comment_on_locked"))
		ctx.Redirect(issue.HTMLURL())
		return
	}
}

// MustEnableIssues check if repository enable internal issues
func MustEnableIssues(ctx *context.Context) {
	if !ctx.Repo.CanRead(models.UnitTypeIssues) &&
		!ctx.Repo.CanRead(models.UnitTypeExternalTracker) {
		ctx.NotFound("MustEnableIssues", nil)
		return
	}

	unit, err := ctx.Repo.Repository.GetUnit(models.UnitTypeExternalTracker)
	if err == nil {
		ctx.Redirect(unit.ExternalTrackerConfig().ExternalTrackerURL)
		return
	}
}

// MustAllowPulls check if repository enable pull requests and user have right to do that
func MustAllowPulls(ctx *context.Context) {
	if !ctx.Repo.Repository.CanEnablePulls() || !ctx.Repo.CanRead(models.UnitTypePullRequests) {
		ctx.NotFound("MustAllowPulls", nil)
		return
	}

	// User can send pull request if owns a forked repository.
	if ctx.IsSigned && ctx.User.HasForkedRepo(ctx.Repo.Repository.ID) {
		ctx.Repo.PullRequest.Allowed = true
		ctx.Repo.PullRequest.HeadInfo = ctx.User.Name + ":" + ctx.Repo.BranchName
	}
}

func issues(ctx *context.Context, milestoneID int64, isPullOption util.OptionalBool) {
	var err error
	viewType := ctx.Query("type")
	sortType := ctx.Query("sort")
	types := []string{"all", "your_repositories", "assigned", "created_by", "mentioned"}
	if !com.IsSliceContainsStr(types, viewType) {
		viewType = "all"
	}

	var (
		assigneeID  = ctx.QueryInt64("assignee")
		posterID    int64
		mentionedID int64
		forceEmpty  bool
	)

	if ctx.IsSigned {
		switch viewType {
		case "created_by":
			posterID = ctx.User.ID
		case "mentioned":
			mentionedID = ctx.User.ID
		}
	}

	repo := ctx.Repo.Repository
	var labelIDs []int64
	selectLabels := ctx.Query("labels")
	if len(selectLabels) > 0 && selectLabels != "0" {
		labelIDs, err = base.StringsToInt64s(strings.Split(selectLabels, ","))
		if err != nil {
			ctx.ServerError("StringsToInt64s", err)
			return
		}
	}
	isShowClosed := ctx.Query("state") == "closed"

	keyword := strings.Trim(ctx.Query("q"), " ")
	if bytes.Contains([]byte(keyword), []byte{0x00}) {
		keyword = ""
	}

	var issueIDs []int64
	if len(keyword) > 0 {
		issueIDs, err = issue_indexer.SearchIssuesByKeyword([]int64{repo.ID}, keyword)
		if err != nil {
			ctx.ServerError("issueIndexer.Search", err)
			return
		}
		if len(issueIDs) == 0 {
			forceEmpty = true
		}
	}

	var issueStats *models.IssueStats
	if forceEmpty {
		issueStats = &models.IssueStats{}
	} else {
		issueStats, err = models.GetIssueStats(&models.IssueStatsOptions{
			RepoID:      repo.ID,
			Labels:      selectLabels,
			MilestoneID: milestoneID,
			AssigneeID:  assigneeID,
			MentionedID: mentionedID,
			PosterID:    posterID,
			IsPull:      isPullOption,
			IssueIDs:    issueIDs,
		})
		if err != nil {
			ctx.ServerError("GetIssueStats", err)
			return
		}
	}
	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}

	var total int
	if !isShowClosed {
		total = int(issueStats.OpenCount)
	} else {
		total = int(issueStats.ClosedCount)
	}
	pager := context.NewPagination(total, setting.UI.IssuePagingNum, page, 5)

	var mileIDs []int64
	if milestoneID > 0 {
		mileIDs = []int64{milestoneID}
	}

	var issues []*models.Issue
	if forceEmpty {
		issues = []*models.Issue{}
	} else {
		issues, err = models.Issues(&models.IssuesOptions{
			ListOptions: models.ListOptions{
				Page:     pager.Paginater.Current(),
				PageSize: setting.UI.IssuePagingNum,
			},
			RepoIDs:      []int64{repo.ID},
			AssigneeID:   assigneeID,
			PosterID:     posterID,
			MentionedID:  mentionedID,
			MilestoneIDs: mileIDs,
			IsClosed:     util.OptionalBoolOf(isShowClosed),
			IsPull:       isPullOption,
			LabelIDs:     labelIDs,
			SortType:     sortType,
			IssueIDs:     issueIDs,
		})
		if err != nil {
			ctx.ServerError("Issues", err)
			return
		}
	}

	approvalCounts, err := models.IssueList(issues).GetApprovalCounts()
	if err != nil {
		ctx.ServerError("ApprovalCounts", err)
		return
	}

	var commitStatus = make(map[int64]*models.CommitStatus, len(issues))

	// Get posters.
	for i := range issues {
		// Check read status
		if !ctx.IsSigned {
			issues[i].IsRead = true
		} else if err = issues[i].GetIsRead(ctx.User.ID); err != nil {
			ctx.ServerError("GetIsRead", err)
			return
		}

		if issues[i].IsPull {
			if err := issues[i].LoadPullRequest(); err != nil {
				ctx.ServerError("LoadPullRequest", err)
				return
			}

			commitStatus[issues[i].PullRequest.ID], _ = pull_service.GetLastCommitStatus(issues[i].PullRequest)
		}
	}

	ctx.Data["Issues"] = issues
	ctx.Data["CommitStatus"] = commitStatus

	// Get assignees.
	ctx.Data["Assignees"], err = repo.GetAssignees()
	if err != nil {
		ctx.ServerError("GetAssignees", err)
		return
	}

	labels, err := models.GetLabelsByRepoID(repo.ID, "", models.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return
	}

	if repo.Owner.IsOrganization() {
		orgLabels, err := models.GetLabelsByOrgID(repo.Owner.ID, ctx.Query("sort"), models.ListOptions{})
		if err != nil {
			ctx.ServerError("GetLabelsByOrgID", err)
			return
		}

		ctx.Data["OrgLabels"] = orgLabels
		labels = append(labels, orgLabels...)
	}

	for _, l := range labels {
		l.LoadSelectedLabelsAfterClick(labelIDs)
	}
	ctx.Data["Labels"] = labels
	ctx.Data["NumLabels"] = len(labels)

	if ctx.QueryInt64("assignee") == 0 {
		assigneeID = 0 // Reset ID to prevent unexpected selection of assignee.
	}

	ctx.Data["IssueRefEndNames"], ctx.Data["IssueRefURLs"] =
		issue_service.GetRefEndNamesAndURLs(issues, ctx.Repo.RepoLink)

	ctx.Data["ApprovalCounts"] = func(issueID int64, typ string) int64 {
		counts, ok := approvalCounts[issueID]
		if !ok || len(counts) == 0 {
			return 0
		}
		reviewTyp := models.ReviewTypeApprove
		if typ == "reject" {
			reviewTyp = models.ReviewTypeReject
		} else if typ == "waiting" {
			reviewTyp = models.ReviewTypeRequest
		}
		for _, count := range counts {
			if count.Type == reviewTyp {
				return count.Count
			}
		}
		return 0
	}
	ctx.Data["IssueStats"] = issueStats
	ctx.Data["SelLabelIDs"] = labelIDs
	ctx.Data["SelectLabels"] = selectLabels
	ctx.Data["ViewType"] = viewType
	ctx.Data["SortType"] = sortType
	ctx.Data["MilestoneID"] = milestoneID
	ctx.Data["AssigneeID"] = assigneeID
	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["Keyword"] = keyword
	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	pager.AddParam(ctx, "q", "Keyword")
	pager.AddParam(ctx, "type", "ViewType")
	pager.AddParam(ctx, "sort", "SortType")
	pager.AddParam(ctx, "state", "State")
	pager.AddParam(ctx, "labels", "SelectLabels")
	pager.AddParam(ctx, "milestone", "MilestoneID")
	pager.AddParam(ctx, "assignee", "AssigneeID")
	ctx.Data["Page"] = pager
}

// Issues render issues page
func Issues(ctx *context.Context) {
	isPullList := ctx.Params(":type") == "pulls"
	if isPullList {
		MustAllowPulls(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["Title"] = ctx.Tr("repo.pulls")
		ctx.Data["PageIsPullList"] = true
	} else {
		MustEnableIssues(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["Title"] = ctx.Tr("repo.issues")
		ctx.Data["PageIsIssueList"] = true
	}

	issues(ctx, ctx.QueryInt64("milestone"), util.OptionalBoolOf(isPullList))

	var err error
	// Get milestones.
	ctx.Data["Milestones"], err = models.GetMilestonesByRepoID(ctx.Repo.Repository.ID, api.StateType(ctx.Query("state")), models.ListOptions{})
	if err != nil {
		ctx.ServerError("GetAllRepoMilestones", err)
		return
	}

	ctx.Data["CanWriteIssuesOrPulls"] = ctx.Repo.CanWriteIssuesOrPulls(isPullList)

	ctx.HTML(200, tplIssues)
}

// RetrieveRepoMilestonesAndAssignees find all the milestones and assignees of a repository
func RetrieveRepoMilestonesAndAssignees(ctx *context.Context, repo *models.Repository) {
	var err error
	ctx.Data["OpenMilestones"], err = models.GetMilestones(repo.ID, -1, false, "")
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}
	ctx.Data["ClosedMilestones"], err = models.GetMilestones(repo.ID, -1, true, "")
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}

	ctx.Data["Assignees"], err = repo.GetAssignees()
	if err != nil {
		ctx.ServerError("GetAssignees", err)
		return
	}
}

// RetrieveRepoReviewers find all reviewers of a repository
func RetrieveRepoReviewers(ctx *context.Context, repo *models.Repository, issuePosterID int64) {
	var err error
	ctx.Data["Reviewers"], err = repo.GetReviewers(ctx.User.ID, issuePosterID)
	if err != nil {
		ctx.ServerError("GetReviewers", err)
		return
	}
}

// RetrieveRepoMetas find all the meta information of a repository
func RetrieveRepoMetas(ctx *context.Context, repo *models.Repository, isPull bool) []*models.Label {
	if !ctx.Repo.CanWriteIssuesOrPulls(isPull) {
		return nil
	}

	labels, err := models.GetLabelsByRepoID(repo.ID, "", models.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return nil
	}
	ctx.Data["Labels"] = labels
	if repo.Owner.IsOrganization() {
		orgLabels, err := models.GetLabelsByOrgID(repo.Owner.ID, ctx.Query("sort"), models.ListOptions{})
		if err != nil {
			return nil
		}

		ctx.Data["OrgLabels"] = orgLabels
		labels = append(labels, orgLabels...)
	}

	RetrieveRepoMilestonesAndAssignees(ctx, repo)
	if ctx.Written() {
		return nil
	}

	brs, err := ctx.Repo.GitRepo.GetBranches()
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return nil
	}
	ctx.Data["Branches"] = brs

	// Contains true if the user can create issue dependencies
	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx.User, isPull)

	return labels
}

func getFileContentFromDefaultBranch(ctx *context.Context, filename string) (string, bool) {
	var bytes []byte

	if ctx.Repo.Commit == nil {
		var err error
		ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
		if err != nil {
			return "", false
		}
	}

	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(filename)
	if err != nil {
		return "", false
	}
	if entry.Blob().Size() >= setting.UI.MaxDisplayFileSize {
		return "", false
	}
	r, err := entry.Blob().DataAsync()
	if err != nil {
		return "", false
	}
	defer r.Close()
	bytes, err = ioutil.ReadAll(r)
	if err != nil {
		return "", false
	}
	return string(bytes), true
}

func setTemplateIfExists(ctx *context.Context, ctxDataKey string, possibleFiles []string) {
	for _, filename := range possibleFiles {
		content, found := getFileContentFromDefaultBranch(ctx, filename)
		if found {
			ctx.Data[ctxDataKey] = content
			return
		}
	}
}

// NewIssue render creating issue page
func NewIssue(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["RequireTribute"] = true
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	body := ctx.Query("body")
	ctx.Data["BodyQuery"] = body

	milestoneID := ctx.QueryInt64("milestone")
	if milestoneID > 0 {
		milestone, err := models.GetMilestoneByID(milestoneID)
		if err != nil {
			log.Error("GetMilestoneByID: %d: %v", milestoneID, err)
		} else {
			ctx.Data["milestone_id"] = milestoneID
			ctx.Data["Milestone"] = milestone
		}
	}

	setTemplateIfExists(ctx, issueTemplateKey, IssueTemplateCandidates)
	renderAttachmentSettings(ctx)

	RetrieveRepoMetas(ctx, ctx.Repo.Repository, false)
	if ctx.Written() {
		return
	}

	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWrite(models.UnitTypeIssues)

	ctx.HTML(200, tplIssueNew)
}

// ValidateRepoMetas check and returns repository's meta informations
func ValidateRepoMetas(ctx *context.Context, form auth.CreateIssueForm, isPull bool) ([]int64, []int64, int64) {
	var (
		repo = ctx.Repo.Repository
		err  error
	)

	labels := RetrieveRepoMetas(ctx, ctx.Repo.Repository, isPull)
	if ctx.Written() {
		return nil, nil, 0
	}

	var labelIDs []int64
	hasSelected := false
	// Check labels.
	if len(form.LabelIDs) > 0 {
		labelIDs, err = base.StringsToInt64s(strings.Split(form.LabelIDs, ","))
		if err != nil {
			return nil, nil, 0
		}
		labelIDMark := base.Int64sToMap(labelIDs)

		for i := range labels {
			if labelIDMark[labels[i].ID] {
				labels[i].IsChecked = true
				hasSelected = true
			}
		}
	}

	ctx.Data["Labels"] = labels
	ctx.Data["HasSelectedLabel"] = hasSelected
	ctx.Data["label_ids"] = form.LabelIDs

	// Check milestone.
	milestoneID := form.MilestoneID
	if milestoneID > 0 {
		ctx.Data["Milestone"], err = repo.GetMilestoneByID(milestoneID)
		if err != nil {
			ctx.ServerError("GetMilestoneByID", err)
			return nil, nil, 0
		}
		ctx.Data["milestone_id"] = milestoneID
	}

	// Check assignees
	var assigneeIDs []int64
	if len(form.AssigneeIDs) > 0 {
		assigneeIDs, err = base.StringsToInt64s(strings.Split(form.AssigneeIDs, ","))
		if err != nil {
			return nil, nil, 0
		}

		// Check if the passed assignees actually exists and is assignable
		for _, aID := range assigneeIDs {
			assignee, err := models.GetUserByID(aID)
			if err != nil {
				ctx.ServerError("GetUserByID", err)
				return nil, nil, 0
			}

			valid, err := models.CanBeAssigned(assignee, repo, isPull)
			if err != nil {
				ctx.ServerError("canBeAssigned", err)
				return nil, nil, 0
			}
			if !valid {
				ctx.ServerError("canBeAssigned", models.ErrUserDoesNotHaveAccessToRepo{UserID: aID, RepoName: repo.Name})
				return nil, nil, 0
			}
		}
	}

	// Keep the old assignee id thingy for compatibility reasons
	if form.AssigneeID > 0 {
		assigneeIDs = append(assigneeIDs, form.AssigneeID)
	}

	return labelIDs, assigneeIDs, milestoneID
}

// NewIssuePost response for creating new issue
func NewIssuePost(ctx *context.Context, form auth.CreateIssueForm) {
	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["ReadOnly"] = false
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	renderAttachmentSettings(ctx)

	var (
		repo        = ctx.Repo.Repository
		attachments []string
	)

	labelIDs, assigneeIDs, milestoneID := ValidateRepoMetas(ctx, form, false)
	if ctx.Written() {
		return
	}

	if setting.AttachmentEnabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.HTML(200, tplIssueNew)
		return
	}

	if util.IsEmptyString(form.Title) {
		ctx.RenderWithErr(ctx.Tr("repo.issues.new.title_empty"), tplIssueNew, form)
		return
	}

	issue := &models.Issue{
		RepoID:      repo.ID,
		Title:       form.Title,
		PosterID:    ctx.User.ID,
		Poster:      ctx.User,
		MilestoneID: milestoneID,
		Content:     form.Content,
		Ref:         form.Ref,
	}

	if err := issue_service.NewIssue(repo, issue, labelIDs, attachments, assigneeIDs); err != nil {
		if models.IsErrUserDoesNotHaveAccessToRepo(err) {
			ctx.Error(400, "UserDoesNotHaveAccessToRepo", err.Error())
			return
		}
		ctx.ServerError("NewIssue", err)
		return
	}

	log.Trace("Issue created: %d/%d", repo.ID, issue.ID)
	ctx.Redirect(ctx.Repo.RepoLink + "/issues/" + com.ToStr(issue.Index))
}

// commentTag returns the CommentTag for a comment in/with the given repo, poster and issue
func commentTag(repo *models.Repository, poster *models.User, issue *models.Issue) (models.CommentTag, error) {
	perm, err := models.GetUserRepoPermission(repo, poster)
	if err != nil {
		return models.CommentTagNone, err
	}
	if perm.IsOwner() {
		return models.CommentTagOwner, nil
	} else if poster.ID == issue.PosterID {
		return models.CommentTagPoster, nil
	} else if perm.CanWrite(models.UnitTypeCode) {
		return models.CommentTagWriter, nil
	}

	return models.CommentTagNone, nil
}

func getBranchData(ctx *context.Context, issue *models.Issue) {
	ctx.Data["BaseBranch"] = nil
	ctx.Data["HeadBranch"] = nil
	ctx.Data["HeadUserName"] = nil
	ctx.Data["BaseName"] = ctx.Repo.Repository.OwnerName
	if issue.IsPull {
		pull := issue.PullRequest
		ctx.Data["BaseBranch"] = pull.BaseBranch
		ctx.Data["HeadBranch"] = pull.HeadBranch
		ctx.Data["HeadUserName"] = pull.MustHeadUserName()
	}
}

// ViewIssue render issue view page
func ViewIssue(ctx *context.Context) {
	if ctx.Params(":type") == "issues" {
		// If issue was requested we check if repo has external tracker and redirect
		extIssueUnit, err := ctx.Repo.Repository.GetUnit(models.UnitTypeExternalTracker)
		if err == nil && extIssueUnit != nil {
			if extIssueUnit.ExternalTrackerConfig().ExternalTrackerStyle == markup.IssueNameStyleNumeric || extIssueUnit.ExternalTrackerConfig().ExternalTrackerStyle == "" {
				metas := ctx.Repo.Repository.ComposeMetas()
				metas["index"] = ctx.Params(":index")
				ctx.Redirect(com.Expand(extIssueUnit.ExternalTrackerConfig().ExternalTrackerFormat, metas))
				return
			}
		} else if err != nil && !models.IsErrUnitTypeNotExist(err) {
			ctx.ServerError("GetUnit", err)
			return
		}
	}

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound("GetIssueByIndex", err)
		} else {
			ctx.ServerError("GetIssueByIndex", err)
		}
		return
	}

	// Make sure type and URL matches.
	if ctx.Params(":type") == "issues" && issue.IsPull {
		ctx.Redirect(ctx.Repo.RepoLink + "/pulls/" + com.ToStr(issue.Index))
		return
	} else if ctx.Params(":type") == "pulls" && !issue.IsPull {
		ctx.Redirect(ctx.Repo.RepoLink + "/issues/" + com.ToStr(issue.Index))
		return
	}

	if issue.IsPull {
		MustAllowPulls(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["PageIsPullList"] = true
		ctx.Data["PageIsPullConversation"] = true
	} else {
		MustEnableIssues(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["PageIsIssueList"] = true
	}

	if issue.IsPull && !ctx.Repo.CanRead(models.UnitTypeIssues) {
		ctx.Data["IssueType"] = "pulls"
	} else if !issue.IsPull && !ctx.Repo.CanRead(models.UnitTypePullRequests) {
		ctx.Data["IssueType"] = "issues"
	} else {
		ctx.Data["IssueType"] = "all"
	}

	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireTribute"] = true
	ctx.Data["RequireSimpleMDE"] = true
	renderAttachmentSettings(ctx)

	if err = issue.LoadAttributes(); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	if err = filterXRefComments(ctx, issue); err != nil {
		ctx.ServerError("filterXRefComments", err)
		return
	}

	ctx.Data["Title"] = fmt.Sprintf("#%d - %s", issue.Index, issue.Title)

	iw := new(models.IssueWatch)
	if ctx.User != nil {
		iw.UserID = ctx.User.ID
		iw.IssueID = issue.ID
		iw.IsWatching, err = models.CheckIssueWatch(ctx.User, issue)
		if err != nil {
			ctx.InternalServerError(err)
			return
		}
	}
	ctx.Data["IssueWatch"] = iw

	issue.RenderedContent = string(markdown.Render([]byte(issue.Content), ctx.Repo.RepoLink,
		ctx.Repo.Repository.ComposeMetas()))

	repo := ctx.Repo.Repository

	// Get more information if it's a pull request.
	if issue.IsPull {
		if issue.PullRequest.HasMerged {
			ctx.Data["DisableStatusChange"] = issue.PullRequest.HasMerged
			PrepareMergedViewPullInfo(ctx, issue)
		} else {
			PrepareViewPullInfo(ctx, issue)
			ctx.Data["DisableStatusChange"] = ctx.Data["IsPullRequestBroken"] == true && issue.IsClosed
		}
		if ctx.Written() {
			return
		}
	}

	// Metas.
	// Check labels.
	labelIDMark := make(map[int64]bool)
	for i := range issue.Labels {
		labelIDMark[issue.Labels[i].ID] = true
	}
	labels, err := models.GetLabelsByRepoID(repo.ID, "", models.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return
	}
	ctx.Data["Labels"] = labels

	if repo.Owner.IsOrganization() {
		orgLabels, err := models.GetLabelsByOrgID(repo.Owner.ID, ctx.Query("sort"), models.ListOptions{})
		if err != nil {
			ctx.ServerError("GetLabelsByOrgID", err)
			return
		}
		ctx.Data["OrgLabels"] = orgLabels

		labels = append(labels, orgLabels...)
	}

	hasSelected := false
	for i := range labels {
		if labelIDMark[labels[i].ID] {
			labels[i].IsChecked = true
			hasSelected = true
		}
	}
	ctx.Data["HasSelectedLabel"] = hasSelected

	// Check milestone and assignee.
	if ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		RetrieveRepoMilestonesAndAssignees(ctx, repo)
		if ctx.Written() {
			return
		}
	}

	if issue.IsPull {
		canChooseReviewer := ctx.Repo.CanWrite(models.UnitTypePullRequests)
		if !canChooseReviewer && ctx.User != nil && ctx.IsSigned {
			canChooseReviewer, err = models.IsOfficialReviewer(issue, ctx.User)
			if err != nil {
				ctx.ServerError("IsOfficialReviewer", err)
				return
			}
		}

		if canChooseReviewer {
			RetrieveRepoReviewers(ctx, repo, issue.PosterID)
			ctx.Data["CanChooseReviewer"] = true
		} else {
			ctx.Data["CanChooseReviewer"] = false
		}

		if ctx.Written() {
			return
		}
	}

	if ctx.IsSigned {
		// Update issue-user.
		if err = issue.ReadBy(ctx.User.ID); err != nil {
			ctx.ServerError("ReadBy", err)
			return
		}
	}

	var (
		tag          models.CommentTag
		ok           bool
		marked       = make(map[int64]models.CommentTag)
		comment      *models.Comment
		participants = make([]*models.User, 1, 10)
	)
	if ctx.Repo.Repository.IsTimetrackerEnabled() {
		if ctx.IsSigned {
			// Deal with the stopwatch
			ctx.Data["IsStopwatchRunning"] = models.StopwatchExists(ctx.User.ID, issue.ID)
			if !ctx.Data["IsStopwatchRunning"].(bool) {
				var exists bool
				var sw *models.Stopwatch
				if exists, sw, err = models.HasUserStopwatch(ctx.User.ID); err != nil {
					ctx.ServerError("HasUserStopwatch", err)
					return
				}
				ctx.Data["HasUserStopwatch"] = exists
				if exists {
					// Add warning if the user has already a stopwatch
					var otherIssue *models.Issue
					if otherIssue, err = models.GetIssueByID(sw.IssueID); err != nil {
						ctx.ServerError("GetIssueByID", err)
						return
					}
					if err = otherIssue.LoadRepo(); err != nil {
						ctx.ServerError("LoadRepo", err)
						return
					}
					// Add link to the issue of the already running stopwatch
					ctx.Data["OtherStopwatchURL"] = otherIssue.HTMLURL()
				}
			}
			ctx.Data["CanUseTimetracker"] = ctx.Repo.CanUseTimetracker(issue, ctx.User)
		} else {
			ctx.Data["CanUseTimetracker"] = false
		}
		if ctx.Data["WorkingUsers"], err = models.TotalTimes(models.FindTrackedTimesOptions{IssueID: issue.ID}); err != nil {
			ctx.ServerError("TotalTimes", err)
			return
		}
	}

	// Check if the user can use the dependencies
	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx.User, issue.IsPull)

	// check if dependencies can be created across repositories
	ctx.Data["AllowCrossRepositoryDependencies"] = setting.Service.AllowCrossRepositoryDependencies

	// Render comments and and fetch participants.
	participants[0] = issue.Poster
	for _, comment = range issue.Comments {
		comment.Issue = issue

		if err := comment.LoadPoster(); err != nil {
			ctx.ServerError("LoadPoster", err)
			return
		}

		if comment.Type == models.CommentTypeComment {
			if err := comment.LoadAttachments(); err != nil {
				ctx.ServerError("LoadAttachments", err)
				return
			}

			comment.RenderedContent = string(markdown.Render([]byte(comment.Content), ctx.Repo.RepoLink,
				ctx.Repo.Repository.ComposeMetas()))

			// Check tag.
			tag, ok = marked[comment.PosterID]
			if ok {
				comment.ShowTag = tag
				continue
			}

			comment.ShowTag, err = commentTag(repo, comment.Poster, issue)
			if err != nil {
				ctx.ServerError("commentTag", err)
				return
			}
			marked[comment.PosterID] = comment.ShowTag
			participants = addParticipant(comment.Poster, participants)
		} else if comment.Type == models.CommentTypeLabel {
			if err = comment.LoadLabel(); err != nil {
				ctx.ServerError("LoadLabel", err)
				return
			}
		} else if comment.Type == models.CommentTypeMilestone {
			if err = comment.LoadMilestone(); err != nil {
				ctx.ServerError("LoadMilestone", err)
				return
			}
			ghostMilestone := &models.Milestone{
				ID:   -1,
				Name: ctx.Tr("repo.issues.deleted_milestone"),
			}
			if comment.OldMilestoneID > 0 && comment.OldMilestone == nil {
				comment.OldMilestone = ghostMilestone
			}
			if comment.MilestoneID > 0 && comment.Milestone == nil {
				comment.Milestone = ghostMilestone
			}
		} else if comment.Type == models.CommentTypeAssignees || comment.Type == models.CommentTypeReviewRequest {
			if err = comment.LoadAssigneeUser(); err != nil {
				ctx.ServerError("LoadAssigneeUser", err)
				return
			}
		} else if comment.Type == models.CommentTypeRemoveDependency || comment.Type == models.CommentTypeAddDependency {
			if err = comment.LoadDepIssueDetails(); err != nil {
				if !models.IsErrIssueNotExist(err) {
					ctx.ServerError("LoadDepIssueDetails", err)
					return
				}
			}
		} else if comment.Type == models.CommentTypeCode || comment.Type == models.CommentTypeReview {
			comment.RenderedContent = string(markdown.Render([]byte(comment.Content), ctx.Repo.RepoLink,
				ctx.Repo.Repository.ComposeMetas()))
			if err = comment.LoadReview(); err != nil && !models.IsErrReviewNotExist(err) {
				ctx.ServerError("LoadReview", err)
				return
			}
			participants = addParticipant(comment.Poster, participants)
			if comment.Review == nil {
				continue
			}
			if err = comment.Review.LoadAttributes(); err != nil {
				if !models.IsErrUserNotExist(err) {
					ctx.ServerError("Review.LoadAttributes", err)
					return
				}
				comment.Review.Reviewer = models.NewGhostUser()
			}
			if err = comment.Review.LoadCodeComments(); err != nil {
				ctx.ServerError("Review.LoadCodeComments", err)
				return
			}

			if err = comment.LoadResolveDoer(); err != nil {
				ctx.ServerError("LoadResolveDoer", err)
				return
			}
		}
	}

	getBranchData(ctx, issue)
	if issue.IsPull {
		pull := issue.PullRequest
		pull.Issue = issue
		canDelete := false
		ctx.Data["AllowMerge"] = false

		if ctx.IsSigned {
			if err := pull.LoadHeadRepo(); err != nil {
				log.Error("LoadHeadRepo: %v", err)
			} else if pull.HeadRepo != nil && pull.HeadBranch != pull.HeadRepo.DefaultBranch {
				perm, err := models.GetUserRepoPermission(pull.HeadRepo, ctx.User)
				if err != nil {
					ctx.ServerError("GetUserRepoPermission", err)
					return
				}
				if perm.CanWrite(models.UnitTypeCode) {
					// Check if branch is not protected
					if protected, err := pull.HeadRepo.IsProtectedBranch(pull.HeadBranch, ctx.User); err != nil {
						log.Error("IsProtectedBranch: %v", err)
					} else if !protected {
						canDelete = true
						ctx.Data["DeleteBranchLink"] = ctx.Repo.RepoLink + "/pulls/" + com.ToStr(issue.Index) + "/cleanup"
					}
				}
			}

			if err := pull.LoadBaseRepo(); err != nil {
				log.Error("LoadBaseRepo: %v", err)
			}
			perm, err := models.GetUserRepoPermission(pull.BaseRepo, ctx.User)
			if err != nil {
				ctx.ServerError("GetUserRepoPermission", err)
				return
			}
			ctx.Data["AllowMerge"], err = pull_service.IsUserAllowedToMerge(pull, perm, ctx.User)
			if err != nil {
				ctx.ServerError("IsUserAllowedToMerge", err)
				return
			}

			if ctx.Data["CanMarkConversation"], err = models.CanMarkConversation(issue, ctx.User); err != nil {
				ctx.ServerError("CanMarkConversation", err)
				return
			}
		}

		prUnit, err := repo.GetUnit(models.UnitTypePullRequests)
		if err != nil {
			ctx.ServerError("GetUnit", err)
			return
		}
		prConfig := prUnit.PullRequestsConfig()

		// Check correct values and select default
		if ms, ok := ctx.Data["MergeStyle"].(models.MergeStyle); !ok ||
			!prConfig.IsMergeStyleAllowed(ms) {
			if prConfig.AllowMerge {
				ctx.Data["MergeStyle"] = models.MergeStyleMerge
			} else if prConfig.AllowRebase {
				ctx.Data["MergeStyle"] = models.MergeStyleRebase
			} else if prConfig.AllowRebaseMerge {
				ctx.Data["MergeStyle"] = models.MergeStyleRebaseMerge
			} else if prConfig.AllowSquash {
				ctx.Data["MergeStyle"] = models.MergeStyleSquash
			} else {
				ctx.Data["MergeStyle"] = ""
			}
		}
		if err = pull.LoadProtectedBranch(); err != nil {
			ctx.ServerError("LoadProtectedBranch", err)
			return
		}
		if pull.ProtectedBranch != nil {
			cnt := pull.ProtectedBranch.GetGrantedApprovalsCount(pull)
			ctx.Data["IsBlockedByApprovals"] = !pull.ProtectedBranch.HasEnoughApprovals(pull)
			ctx.Data["IsBlockedByRejection"] = pull.ProtectedBranch.MergeBlockedByRejectedReview(pull)
			ctx.Data["IsBlockedByOutdatedBranch"] = pull.ProtectedBranch.MergeBlockedByOutdatedBranch(pull)
			ctx.Data["GrantedApprovals"] = cnt
			ctx.Data["RequireSigned"] = pull.ProtectedBranch.RequireSignedCommits
		}
		ctx.Data["WillSign"] = false
		if ctx.User != nil {
			sign, key, err := pull.SignMerge(ctx.User, pull.BaseRepo.RepoPath(), pull.BaseBranch, pull.GetGitRefName())
			ctx.Data["WillSign"] = sign
			ctx.Data["SigningKey"] = key
			if err != nil {
				if models.IsErrWontSign(err) {
					ctx.Data["WontSignReason"] = err.(*models.ErrWontSign).Reason
				} else {
					ctx.Data["WontSignReason"] = "error"
					log.Error("Error whilst checking if could sign pr %d in repo %s. Error: %v", pull.ID, pull.BaseRepo.FullName(), err)
				}
			}
		}
		ctx.Data["IsPullBranchDeletable"] = canDelete &&
			pull.HeadRepo != nil &&
			git.IsBranchExist(pull.HeadRepo.RepoPath(), pull.HeadBranch) &&
			(!pull.HasMerged || ctx.Data["HeadBranchCommitID"] == ctx.Data["PullHeadCommitID"])

		ctx.Data["PullReviewers"], err = models.GetReviewersByIssueID(issue.ID)
		if err != nil {
			ctx.ServerError("GetReviewersByIssueID", err)
			return
		}
	}

	// Get Dependencies
	ctx.Data["BlockedByDependencies"], err = issue.BlockedByDependencies()
	if err != nil {
		ctx.ServerError("BlockedByDependencies", err)
		return
	}
	ctx.Data["BlockingDependencies"], err = issue.BlockingDependencies()
	if err != nil {
		ctx.ServerError("BlockingDependencies", err)
		return
	}

	ctx.Data["Participants"] = participants
	ctx.Data["NumParticipants"] = len(participants)
	ctx.Data["Issue"] = issue
	ctx.Data["ReadOnly"] = true
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login?redirect_to=" + ctx.Data["Link"].(string)
	ctx.Data["IsIssuePoster"] = ctx.IsSigned && issue.IsPoster(ctx.User.ID)
	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)
	ctx.Data["IsRepoAdmin"] = ctx.IsSigned && (ctx.Repo.IsAdmin() || ctx.User.IsAdmin)
	ctx.Data["LockReasons"] = setting.Repository.Issue.LockReasons
	ctx.Data["RefEndName"] = git.RefEndName(issue.Ref)
	ctx.HTML(200, tplIssueView)
}

// GetActionIssue will return the issue which is used in the context.
func GetActionIssue(ctx *context.Context) *models.Issue {
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		ctx.NotFoundOrServerError("GetIssueByIndex", models.IsErrIssueNotExist, err)
		return nil
	}
	issue.Repo = ctx.Repo.Repository
	checkIssueRights(ctx, issue)
	if ctx.Written() {
		return nil
	}
	if err = issue.LoadAttributes(); err != nil {
		ctx.ServerError("LoadAttributes", nil)
		return nil
	}
	return issue
}

func checkIssueRights(ctx *context.Context, issue *models.Issue) {
	if issue.IsPull && !ctx.Repo.CanRead(models.UnitTypePullRequests) ||
		!issue.IsPull && !ctx.Repo.CanRead(models.UnitTypeIssues) {
		ctx.NotFound("IssueOrPullRequestUnitNotAllowed", nil)
	}
}

func getActionIssues(ctx *context.Context) []*models.Issue {
	commaSeparatedIssueIDs := ctx.Query("issue_ids")
	if len(commaSeparatedIssueIDs) == 0 {
		return nil
	}
	issueIDs := make([]int64, 0, 10)
	for _, stringIssueID := range strings.Split(commaSeparatedIssueIDs, ",") {
		issueID, err := strconv.ParseInt(stringIssueID, 10, 64)
		if err != nil {
			ctx.ServerError("ParseInt", err)
			return nil
		}
		issueIDs = append(issueIDs, issueID)
	}
	issues, err := models.GetIssuesByIDs(issueIDs)
	if err != nil {
		ctx.ServerError("GetIssuesByIDs", err)
		return nil
	}
	// Check access rights for all issues
	issueUnitEnabled := ctx.Repo.CanRead(models.UnitTypeIssues)
	prUnitEnabled := ctx.Repo.CanRead(models.UnitTypePullRequests)
	for _, issue := range issues {
		if issue.IsPull && !prUnitEnabled || !issue.IsPull && !issueUnitEnabled {
			ctx.NotFound("IssueOrPullRequestUnitNotAllowed", nil)
			return nil
		}
		if err = issue.LoadAttributes(); err != nil {
			ctx.ServerError("LoadAttributes", err)
			return nil
		}
	}
	return issues
}

// UpdateIssueTitle change issue's title
func UpdateIssueTitle(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (!issue.IsPoster(ctx.User.ID) && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.Error(403)
		return
	}

	title := ctx.QueryTrim("title")
	if len(title) == 0 {
		ctx.Error(204)
		return
	}

	if err := issue_service.ChangeTitle(issue, ctx.User, title); err != nil {
		ctx.ServerError("ChangeTitle", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"title": issue.Title,
	})
}

// UpdateIssueContent change issue's content
func UpdateIssueContent(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != issue.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.Error(403)
		return
	}

	content := ctx.Query("content")
	if err := issue_service.ChangeContent(issue, ctx.User, content); err != nil {
		ctx.ServerError("ChangeContent", err)
		return
	}

	files := ctx.QueryStrings("files[]")
	if err := updateAttachments(issue, files); err != nil {
		ctx.ServerError("UpdateAttachments", err)
	}

	ctx.JSON(200, map[string]interface{}{
		"content":     string(markdown.Render([]byte(issue.Content), ctx.Query("context"), ctx.Repo.Repository.ComposeMetas())),
		"attachments": attachmentsHTML(ctx, issue.Attachments),
	})
}

// UpdateIssueMilestone change issue's milestone
func UpdateIssueMilestone(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	milestoneID := ctx.QueryInt64("id")
	for _, issue := range issues {
		oldMilestoneID := issue.MilestoneID
		if oldMilestoneID == milestoneID {
			continue
		}
		issue.MilestoneID = milestoneID
		if err := issue_service.ChangeMilestoneAssign(issue, ctx.User, oldMilestoneID); err != nil {
			ctx.ServerError("ChangeMilestoneAssign", err)
			return
		}
	}

	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// UpdateIssueAssignee change issue's or pull's assignee
func UpdateIssueAssignee(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	assigneeID := ctx.QueryInt64("id")
	action := ctx.Query("action")

	for _, issue := range issues {
		switch action {
		case "clear":
			if err := issue_service.DeleteNotPassedAssignee(issue, ctx.User, []*models.User{}); err != nil {
				ctx.ServerError("ClearAssignees", err)
				return
			}
		default:
			assignee, err := models.GetUserByID(assigneeID)
			if err != nil {
				ctx.ServerError("GetUserByID", err)
				return
			}

			valid, err := models.CanBeAssigned(assignee, issue.Repo, issue.IsPull)
			if err != nil {
				ctx.ServerError("canBeAssigned", err)
				return
			}
			if !valid {
				ctx.ServerError("canBeAssigned", models.ErrUserDoesNotHaveAccessToRepo{UserID: assigneeID, RepoName: issue.Repo.Name})
				return
			}

			_, _, err = issue_service.ToggleAssignee(issue, ctx.User, assigneeID)
			if err != nil {
				ctx.ServerError("ToggleAssignee", err)
				return
			}
		}
	}
	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

func isLegalReviewRequest(reviewer, doer *models.User, isAdd bool, issue *models.Issue) error {
	if reviewer.IsOrganization() {
		return fmt.Errorf("Organization can't be added as reviewer [user_id: %d, repo_id: %d]", reviewer.ID, issue.PullRequest.BaseRepo.ID)
	}
	if doer.IsOrganization() {
		return fmt.Errorf("Organization can't be doer to add reviewer [user_id: %d, repo_id: %d]", doer.ID, issue.PullRequest.BaseRepo.ID)
	}

	permReviewer, err := models.GetUserRepoPermission(issue.Repo, reviewer)
	if err != nil {
		return err
	}

	permDoer, err := models.GetUserRepoPermission(issue.Repo, doer)
	if err != nil {
		return err
	}

	lastreview, err := models.GetReviewerByIssueIDAndUserID(issue.ID, reviewer.ID)
	if err != nil {
		return err
	}

	var pemResult bool
	if isAdd {
		pemResult = permReviewer.CanAccessAny(models.AccessModeRead, models.UnitTypePullRequests)
		if !pemResult {
			return fmt.Errorf("Reviewer can't read [user_id: %d, repo_name: %s]", reviewer.ID, issue.Repo.Name)
		}

		if doer.ID == issue.PosterID && lastreview != nil && lastreview.Type != models.ReviewTypeRequest {
			return nil
		}

		pemResult = permDoer.CanAccessAny(models.AccessModeWrite, models.UnitTypePullRequests)
		if !pemResult {
			pemResult, err = models.IsOfficialReviewer(issue, doer)
			if err != nil {
				return err
			}
			if !pemResult {
				return fmt.Errorf("Doer can't choose reviewer [user_id: %d, repo_name: %s, issue_id: %d]", doer.ID, issue.Repo.Name, issue.ID)
			}
		}

		if doer.ID == reviewer.ID {
			return fmt.Errorf("doer can't be reviewer [user_id: %d, repo_name: %s]", doer.ID, issue.Repo.Name)
		}

		if reviewer.ID == issue.PosterID {
			return fmt.Errorf("poster of pr can't be reviewer [user_id: %d, repo_name: %s]", reviewer.ID, issue.Repo.Name)
		}
	} else {
		if lastreview.Type == models.ReviewTypeRequest && lastreview.ReviewerID == doer.ID {
			return nil
		}

		pemResult = permDoer.IsAdmin()
		if !pemResult {
			return fmt.Errorf("Doer is not admin [user_id: %d, repo_name: %s]", doer.ID, issue.Repo.Name)
		}
	}

	return nil
}

// updatePullReviewRequest change pull's request reviewers
func updatePullReviewRequest(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	reviewID := ctx.QueryInt64("id")
	event := ctx.Query("is_add")

	if event != "add" && event != "remove" {
		ctx.ServerError("updatePullReviewRequest", fmt.Errorf("is_add should not be \"%s\"", event))
		return
	}

	for _, issue := range issues {
		if issue.IsPull {

			reviewer, err := models.GetUserByID(reviewID)
			if err != nil {
				ctx.ServerError("GetUserByID", err)
				return
			}

			err = isLegalReviewRequest(reviewer, ctx.User, event == "add", issue)
			if err != nil {
				ctx.ServerError("isLegalRequestReview", err)
				return
			}

			err = issue_service.ReviewRequest(issue, ctx.User, reviewer, event == "add")
			if err != nil {
				ctx.ServerError("ReviewRequest", err)
				return
			}
		} else {
			ctx.ServerError("updatePullReviewRequest", fmt.Errorf("%d in %d is not Pull Request", issue.ID, issue.Repo.ID))
		}
	}

	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// UpdatePullReviewRequest add or remove review request
func UpdatePullReviewRequest(ctx *context.Context) {
	updatePullReviewRequest(ctx)
}

// UpdateIssueStatus change issue's status
func UpdateIssueStatus(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	var isClosed bool
	switch action := ctx.Query("action"); action {
	case "open":
		isClosed = false
	case "close":
		isClosed = true
	default:
		log.Warn("Unrecognized action: %s", action)
	}

	if _, err := models.IssueList(issues).LoadRepositories(); err != nil {
		ctx.ServerError("LoadRepositories", err)
		return
	}
	for _, issue := range issues {
		if issue.IsClosed != isClosed {
			if err := issue_service.ChangeStatus(issue, ctx.User, isClosed); err != nil {
				if models.IsErrDependenciesLeft(err) {
					ctx.JSON(http.StatusPreconditionFailed, map[string]interface{}{
						"error": "cannot close this issue because it still has open dependencies",
					})
					return
				}
				ctx.ServerError("ChangeStatus", err)
				return
			}
		}
	}
	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// NewComment create a comment for issue
func NewComment(ctx *context.Context, form auth.CreateCommentForm) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != issue.PosterID && !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull)) {
		if log.IsTrace() {
			if ctx.IsSigned {
				issueType := "issues"
				if issue.IsPull {
					issueType = "pulls"
				}
				log.Trace("Permission Denied: User %-v not the Poster (ID: %d) and cannot read %s in Repo %-v.\n"+
					"User in Repo has Permissions: %-+v",
					ctx.User,
					log.NewColoredIDValue(issue.PosterID),
					issueType,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Not logged in")
			}
		}

		ctx.Error(403)
		return
	}

	if issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) && !ctx.User.IsAdmin {
		ctx.Flash.Error(ctx.Tr("repo.issues.comment_on_locked"))
		ctx.Redirect(issue.HTMLURL(), http.StatusSeeOther)
		return
	}

	var attachments []string
	if setting.AttachmentEnabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.Data["ErrorMsg"].(string))
		ctx.Redirect(issue.HTMLURL())
		return
	}

	var comment *models.Comment
	defer func() {
		// Check if issue admin/poster changes the status of issue.
		if (ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) || (ctx.IsSigned && issue.IsPoster(ctx.User.ID))) &&
			(form.Status == "reopen" || form.Status == "close") &&
			!(issue.IsPull && issue.PullRequest.HasMerged) {

			// Duplication and conflict check should apply to reopen pull request.
			var pr *models.PullRequest

			if form.Status == "reopen" && issue.IsPull {
				pull := issue.PullRequest
				var err error
				pr, err = models.GetUnmergedPullRequest(pull.HeadRepoID, pull.BaseRepoID, pull.HeadBranch, pull.BaseBranch)
				if err != nil {
					if !models.IsErrPullRequestNotExist(err) {
						ctx.ServerError("GetUnmergedPullRequest", err)
						return
					}
				}

				// Regenerate patch and test conflict.
				if pr == nil {
					pull_service.AddToTaskQueue(issue.PullRequest)
				}
			}

			if pr != nil {
				ctx.Flash.Info(ctx.Tr("repo.pulls.open_unmerged_pull_exists", pr.Index))
			} else {
				isClosed := form.Status == "close"
				if err := issue_service.ChangeStatus(issue, ctx.User, isClosed); err != nil {
					log.Error("ChangeStatus: %v", err)

					if models.IsErrDependenciesLeft(err) {
						if issue.IsPull {
							ctx.Flash.Error(ctx.Tr("repo.issues.dependency.pr_close_blocked"))
							ctx.Redirect(fmt.Sprintf("%s/pulls/%d", ctx.Repo.RepoLink, issue.Index), http.StatusSeeOther)
						} else {
							ctx.Flash.Error(ctx.Tr("repo.issues.dependency.issue_close_blocked"))
							ctx.Redirect(fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issue.Index), http.StatusSeeOther)
						}
						return
					}
				} else {
					if err := stopTimerIfAvailable(ctx.User, issue); err != nil {
						ctx.ServerError("CreateOrStopIssueStopwatch", err)
						return
					}

					log.Trace("Issue [%d] status changed to closed: %v", issue.ID, issue.IsClosed)
				}
			}
		}

		// Redirect to comment hashtag if there is any actual content.
		typeName := "issues"
		if issue.IsPull {
			typeName = "pulls"
		}
		if comment != nil {
			ctx.Redirect(fmt.Sprintf("%s/%s/%d#%s", ctx.Repo.RepoLink, typeName, issue.Index, comment.HashTag()))
		} else {
			ctx.Redirect(fmt.Sprintf("%s/%s/%d", ctx.Repo.RepoLink, typeName, issue.Index))
		}
	}()

	// Fix #321: Allow empty comments, as long as we have attachments.
	if len(form.Content) == 0 && len(attachments) == 0 {
		return
	}

	comment, err := comment_service.CreateIssueComment(ctx.User, ctx.Repo.Repository, issue, form.Content, attachments)
	if err != nil {
		ctx.ServerError("CreateIssueComment", err)
		return
	}

	log.Trace("Comment created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, comment.ID)
}

// UpdateCommentContent change comment of issue's content
func UpdateCommentContent(ctx *context.Context) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", models.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", models.IsErrIssueNotExist, err)
		return
	}

	if comment.Type == models.CommentTypeComment {
		if err := comment.LoadAttachments(); err != nil {
			ctx.ServerError("LoadAttachments", err)
			return
		}
	}

	if !ctx.IsSigned || (ctx.User.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull)) {
		ctx.Error(403)
		return
	} else if comment.Type != models.CommentTypeComment && comment.Type != models.CommentTypeCode {
		ctx.Error(204)
		return
	}

	oldContent := comment.Content
	comment.Content = ctx.Query("content")
	if len(comment.Content) == 0 {
		ctx.JSON(200, map[string]interface{}{
			"content": "",
		})
		return
	}
	if err = comment_service.UpdateComment(comment, ctx.User, oldContent); err != nil {
		ctx.ServerError("UpdateComment", err)
		return
	}

	files := ctx.QueryStrings("files[]")
	if err := updateAttachments(comment, files); err != nil {
		ctx.ServerError("UpdateAttachments", err)
	}

	ctx.JSON(200, map[string]interface{}{
		"content":     string(markdown.Render([]byte(comment.Content), ctx.Query("context"), ctx.Repo.Repository.ComposeMetas())),
		"attachments": attachmentsHTML(ctx, comment.Attachments),
	})
}

// DeleteComment delete comment of issue
func DeleteComment(ctx *context.Context) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", models.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", models.IsErrIssueNotExist, err)
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull)) {
		ctx.Error(403)
		return
	} else if comment.Type != models.CommentTypeComment && comment.Type != models.CommentTypeCode {
		ctx.Error(204)
		return
	}

	if err = comment_service.DeleteComment(comment, ctx.User); err != nil {
		ctx.ServerError("DeleteCommentByID", err)
		return
	}

	ctx.Status(200)
}

// ChangeIssueReaction create a reaction for issue
func ChangeIssueReaction(ctx *context.Context, form auth.ReactionForm) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != issue.PosterID && !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull)) {
		if log.IsTrace() {
			if ctx.IsSigned {
				issueType := "issues"
				if issue.IsPull {
					issueType = "pulls"
				}
				log.Trace("Permission Denied: User %-v not the Poster (ID: %d) and cannot read %s in Repo %-v.\n"+
					"User in Repo has Permissions: %-+v",
					ctx.User,
					log.NewColoredIDValue(issue.PosterID),
					issueType,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Not logged in")
			}
		}

		ctx.Error(403)
		return
	}

	if ctx.HasError() {
		ctx.ServerError("ChangeIssueReaction", errors.New(ctx.GetErrMsg()))
		return
	}

	switch ctx.Params(":action") {
	case "react":
		reaction, err := models.CreateIssueReaction(ctx.User, issue, form.Content)
		if err != nil {
			if models.IsErrForbiddenIssueReaction(err) {
				ctx.ServerError("ChangeIssueReaction", err)
				return
			}
			log.Info("CreateIssueReaction: %s", err)
			break
		}
		// Reload new reactions
		issue.Reactions = nil
		if err = issue.LoadAttributes(); err != nil {
			log.Info("issue.LoadAttributes: %s", err)
			break
		}

		log.Trace("Reaction for issue created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, reaction.ID)
	case "unreact":
		if err := models.DeleteIssueReaction(ctx.User, issue, form.Content); err != nil {
			ctx.ServerError("DeleteIssueReaction", err)
			return
		}

		// Reload new reactions
		issue.Reactions = nil
		if err := issue.LoadAttributes(); err != nil {
			log.Info("issue.LoadAttributes: %s", err)
			break
		}

		log.Trace("Reaction for issue removed: %d/%d", ctx.Repo.Repository.ID, issue.ID)
	default:
		ctx.NotFound(fmt.Sprintf("Unknown action %s", ctx.Params(":action")), nil)
		return
	}

	if len(issue.Reactions) == 0 {
		ctx.JSON(200, map[string]interface{}{
			"empty": true,
			"html":  "",
		})
		return
	}

	html, err := ctx.HTMLString(string(tplReactions), map[string]interface{}{
		"ctx":       ctx.Data,
		"ActionURL": fmt.Sprintf("%s/issues/%d/reactions", ctx.Repo.RepoLink, issue.Index),
		"Reactions": issue.Reactions.GroupByType(),
	})
	if err != nil {
		ctx.ServerError("ChangeIssueReaction.HTMLString", err)
		return
	}
	ctx.JSON(200, map[string]interface{}{
		"html": html,
	})
}

// ChangeCommentReaction create a reaction for comment
func ChangeCommentReaction(ctx *context.Context, form auth.ReactionForm) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", models.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", models.IsErrIssueNotExist, err)
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != comment.PosterID && !ctx.Repo.CanReadIssuesOrPulls(comment.Issue.IsPull)) {
		if log.IsTrace() {
			if ctx.IsSigned {
				issueType := "issues"
				if comment.Issue.IsPull {
					issueType = "pulls"
				}
				log.Trace("Permission Denied: User %-v not the Poster (ID: %d) and cannot read %s in Repo %-v.\n"+
					"User in Repo has Permissions: %-+v",
					ctx.User,
					log.NewColoredIDValue(comment.Issue.PosterID),
					issueType,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Not logged in")
			}
		}

		ctx.Error(403)
		return
	} else if comment.Type != models.CommentTypeComment && comment.Type != models.CommentTypeCode {
		ctx.Error(204)
		return
	}

	switch ctx.Params(":action") {
	case "react":
		reaction, err := models.CreateCommentReaction(ctx.User, comment.Issue, comment, form.Content)
		if err != nil {
			if models.IsErrForbiddenIssueReaction(err) {
				ctx.ServerError("ChangeIssueReaction", err)
				return
			}
			log.Info("CreateCommentReaction: %s", err)
			break
		}
		// Reload new reactions
		comment.Reactions = nil
		if err = comment.LoadReactions(ctx.Repo.Repository); err != nil {
			log.Info("comment.LoadReactions: %s", err)
			break
		}

		log.Trace("Reaction for comment created: %d/%d/%d/%d", ctx.Repo.Repository.ID, comment.Issue.ID, comment.ID, reaction.ID)
	case "unreact":
		if err := models.DeleteCommentReaction(ctx.User, comment.Issue, comment, form.Content); err != nil {
			ctx.ServerError("DeleteCommentReaction", err)
			return
		}

		// Reload new reactions
		comment.Reactions = nil
		if err = comment.LoadReactions(ctx.Repo.Repository); err != nil {
			log.Info("comment.LoadReactions: %s", err)
			break
		}

		log.Trace("Reaction for comment removed: %d/%d/%d", ctx.Repo.Repository.ID, comment.Issue.ID, comment.ID)
	default:
		ctx.NotFound(fmt.Sprintf("Unknown action %s", ctx.Params(":action")), nil)
		return
	}

	if len(comment.Reactions) == 0 {
		ctx.JSON(200, map[string]interface{}{
			"empty": true,
			"html":  "",
		})
		return
	}

	html, err := ctx.HTMLString(string(tplReactions), map[string]interface{}{
		"ctx":       ctx.Data,
		"ActionURL": fmt.Sprintf("%s/comments/%d/reactions", ctx.Repo.RepoLink, comment.ID),
		"Reactions": comment.Reactions.GroupByType(),
	})
	if err != nil {
		ctx.ServerError("ChangeCommentReaction.HTMLString", err)
		return
	}
	ctx.JSON(200, map[string]interface{}{
		"html": html,
	})
}

func addParticipant(poster *models.User, participants []*models.User) []*models.User {
	for _, part := range participants {
		if poster.ID == part.ID {
			return participants
		}
	}
	return append(participants, poster)
}

func filterXRefComments(ctx *context.Context, issue *models.Issue) error {
	// Remove comments that the user has no permissions to see
	for i := 0; i < len(issue.Comments); {
		c := issue.Comments[i]
		if models.CommentTypeIsRef(c.Type) && c.RefRepoID != issue.RepoID && c.RefRepoID != 0 {
			var err error
			// Set RefRepo for description in template
			c.RefRepo, err = models.GetRepositoryByID(c.RefRepoID)
			if err != nil {
				return err
			}
			perm, err := models.GetUserRepoPermission(c.RefRepo, ctx.User)
			if err != nil {
				return err
			}
			if !perm.CanReadIssuesOrPulls(c.RefIsPull) {
				issue.Comments = append(issue.Comments[:i], issue.Comments[i+1:]...)
				continue
			}
		}
		i++
	}
	return nil
}

// GetIssueAttachments returns attachments for the issue
func GetIssueAttachments(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	var attachments = make([]*api.Attachment, len(issue.Attachments))
	for i := 0; i < len(issue.Attachments); i++ {
		attachments[i] = issue.Attachments[i].APIFormat()
	}
	ctx.JSON(200, attachments)
}

// GetCommentAttachments returns attachments for the comment
func GetCommentAttachments(ctx *context.Context) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", models.IsErrCommentNotExist, err)
		return
	}
	var attachments = make([]*api.Attachment, 0)
	if comment.Type == models.CommentTypeComment {
		if err := comment.LoadAttachments(); err != nil {
			ctx.ServerError("LoadAttachments", err)
			return
		}
		for i := 0; i < len(comment.Attachments); i++ {
			attachments = append(attachments, comment.Attachments[i].APIFormat())
		}
	}
	ctx.JSON(200, attachments)
}

func updateAttachments(item interface{}, files []string) error {
	var attachments []*models.Attachment
	switch content := item.(type) {
	case *models.Issue:
		attachments = content.Attachments
	case *models.Comment:
		attachments = content.Attachments
	default:
		return fmt.Errorf("Unknow Type")
	}
	for i := 0; i < len(attachments); i++ {
		if util.IsStringInSlice(attachments[i].UUID, files) {
			continue
		}
		if err := models.DeleteAttachment(attachments[i], true); err != nil {
			return err
		}
	}
	var err error
	if len(files) > 0 {
		switch content := item.(type) {
		case *models.Issue:
			err = content.UpdateAttachments(files)
		case *models.Comment:
			err = content.UpdateAttachments(files)
		default:
			return fmt.Errorf("Unknow Type")
		}
		if err != nil {
			return err
		}
	}
	switch content := item.(type) {
	case *models.Issue:
		content.Attachments, err = models.GetAttachmentsByIssueID(content.ID)
	case *models.Comment:
		content.Attachments, err = models.GetAttachmentsByCommentID(content.ID)
	default:
		return fmt.Errorf("Unknow Type")
	}
	return err
}

func attachmentsHTML(ctx *context.Context, attachments []*models.Attachment) string {
	attachHTML, err := ctx.HTMLString(string(tplAttachment), map[string]interface{}{
		"ctx":         ctx.Data,
		"Attachments": attachments,
	})
	if err != nil {
		ctx.ServerError("attachmentsHTML.HTMLString", err)
		return ""
	}
	return attachHTML
}
