// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repofiles"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplEditFile        base.TplName = "repo/editor/edit"
	tplEditDiffPreview base.TplName = "repo/editor/diff_preview"
	tplDeleteFile      base.TplName = "repo/editor/delete"
	tplUploadFile      base.TplName = "repo/editor/upload"

	frmCommitChoiceDirect    string = "direct"
	frmCommitChoiceNewBranch string = "commit-to-new-branch"
)

func renderCommitRights(ctx *context.Context) bool {
	canCommit, err := ctx.Repo.CanCommitToBranch(ctx.User)
	if err != nil {
		log.Error(4, "CanCommitToBranch: %v", err)
	}
	ctx.Data["CanCommitToBranch"] = canCommit
	return canCommit
}

// getParentTreeFields returns list of parent tree names and corresponding tree paths
// based on given tree path.
func getParentTreeFields(treePath string) (treeNames []string, treePaths []string) {
	if len(treePath) == 0 {
		return treeNames, treePaths
	}

	treeNames = strings.Split(treePath, "/")
	treePaths = make([]string, len(treeNames))
	for i := range treeNames {
		treePaths[i] = strings.Join(treeNames[:i+1], "/")
	}
	return treeNames, treePaths
}

func editFile(ctx *context.Context, isNewFile bool) {
	ctx.Data["PageIsEdit"] = true
	ctx.Data["IsNewFile"] = isNewFile
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	canCommit := renderCommitRights(ctx)

	treePath := cleanUploadFileName(ctx.Repo.TreePath)
	if treePath != ctx.Repo.TreePath {
		if isNewFile {
			ctx.Redirect(path.Join(ctx.Repo.RepoLink, "_new", ctx.Repo.BranchName, treePath))
		} else {
			ctx.Redirect(path.Join(ctx.Repo.RepoLink, "_edit", ctx.Repo.BranchName, treePath))
		}
		return
	}

	treeNames, treePaths := getParentTreeFields(ctx.Repo.TreePath)

	if !isNewFile {
		entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
		if err != nil {
			ctx.NotFoundOrServerError("GetTreeEntryByPath", git.IsErrNotExist, err)
			return
		}

		// No way to edit a directory online.
		if entry.IsDir() {
			ctx.NotFound("entry.IsDir", nil)
			return
		}

		blob := entry.Blob()
		if blob.Size() >= setting.UI.MaxDisplayFileSize {
			ctx.NotFound("blob.Size", err)
			return
		}

		dataRc, err := blob.Data()
		if err != nil {
			ctx.NotFound("blob.Data", err)
			return
		}

		ctx.Data["FileSize"] = blob.Size()
		ctx.Data["FileName"] = blob.Name()

		buf := make([]byte, 1024)
		n, _ := dataRc.Read(buf)
		buf = buf[:n]

		// Only text file are editable online.
		if !base.IsTextFile(buf) {
			ctx.NotFound("base.IsTextFile", nil)
			return
		}

		d, _ := ioutil.ReadAll(dataRc)
		buf = append(buf, d...)
		if content, err := templates.ToUTF8WithErr(buf); err != nil {
			if err != nil {
				log.Error(4, "ToUTF8WithErr: %v", err)
			}
			ctx.Data["FileContent"] = string(buf)
		} else {
			ctx.Data["FileContent"] = content
		}
	} else {
		treeNames = append(treeNames, "") // Append empty string to allow user name the new file.
	}

	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = ""
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	ctx.Data["MarkdownFileExts"] = strings.Join(setting.Markdown.FileExtensions, ",")
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["PreviewableFileModes"] = strings.Join(setting.Repository.Editor.PreviewableFileModes, ",")
	ctx.Data["EditorconfigURLPrefix"] = fmt.Sprintf("%s/api/v1/repos/%s/editorconfig/", setting.AppSubURL, ctx.Repo.Repository.FullName())

	ctx.HTML(200, tplEditFile)
}

// EditFile render edit file page
func EditFile(ctx *context.Context) {
	editFile(ctx, false)
}

// NewFile render create file page
func NewFile(ctx *context.Context) {
	editFile(ctx, true)
}

func editFilePost(ctx *context.Context, form auth.EditRepoFileForm, isNewFile bool) {
	canCommit := renderCommitRights(ctx)
	treeNames, treePaths := getParentTreeFields(form.TreePath)
	branchName := ctx.Repo.BranchName
	if form.CommitChoice == frmCommitChoiceNewBranch {
		branchName = form.NewBranchName
	}

	ctx.Data["PageIsEdit"] = true
	ctx.Data["IsNewFile"] = isNewFile
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["TreePath"] = form.TreePath
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/branch/" + ctx.Repo.BranchName
	ctx.Data["FileContent"] = form.Content
	ctx.Data["commit_summary"] = form.CommitSummary
	ctx.Data["commit_message"] = form.CommitMessage
	ctx.Data["commit_choice"] = form.CommitChoice
	ctx.Data["new_branch_name"] = form.NewBranchName
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	ctx.Data["MarkdownFileExts"] = strings.Join(setting.Markdown.FileExtensions, ",")
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["PreviewableFileModes"] = strings.Join(setting.Repository.Editor.PreviewableFileModes, ",")

	if ctx.HasError() {
		ctx.HTML(200, tplEditFile)
		return
	}

	// Cannot commit to a an existing branch if user doesn't have rights
	if branchName == ctx.Repo.BranchName && !canCommit {
		ctx.Data["Err_NewBranchName"] = true
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
		ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplEditFile, &form)
		return
	}

	// CommitSummary is optional in the web form, if empty, give it a default message based on add or update
	// `message` will be both the summary and message combined
	message := strings.TrimSpace(form.CommitSummary)
	if len(message) == 0 {
		if isNewFile {
			message = ctx.Tr("repo.editor.add", form.TreePath)
		} else {
			message = ctx.Tr("repo.editor.update", form.TreePath)
		}
	}
	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	if _, err := repofiles.CreateOrUpdateRepoFile(ctx.Repo.Repository, ctx.User, &repofiles.UpdateRepoFileOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    ctx.Repo.BranchName,
		NewBranch:    branchName,
		FromTreePath: ctx.Repo.TreePath,
		TreePath:     form.TreePath,
		Message:      message,
		Content:      strings.Replace(form.Content, "\r", "", -1),
		IsNewFile:    isNewFile,
	}); err != nil {
		// This is where we handle all the errors thrown by repofiles.CreateOrUpdateRepoFile
		if git.IsErrNotExist(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_editing_no_longer_exists", ctx.Repo.TreePath), tplEditFile, &form)
		} else if models.IsErrFilenameInvalid(err) {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_invalid", form.TreePath), tplEditFile, &form)
		} else if models.IsErrFilePathInvalid(err) {
			ctx.Data["Err_TreePath"] = true
			if fileErr, ok := err.(models.ErrFilePathInvalid); ok {
				switch fileErr.Type {
				case git.EntryModeSymlink:
					ctx.RenderWithErr(ctx.Tr("repo.editor.file_is_a_symlink", fileErr.Path), tplEditFile, &form)
					break
				case git.EntryModeTree:
					ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_a_directory", fileErr.Path), tplEditFile, &form)
					break
				case git.EntryModeBlob:
					ctx.RenderWithErr(ctx.Tr("repo.editor.directory_is_a_file", fileErr.Path), tplEditFile, &form)
					break
				default:
					ctx.Error(500, err.Error())
					break
				}
			} else {
				ctx.Error(500, err.Error())
			}
		} else if models.IsErrRepoFileAlreadyExists(err) {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_already_exists", form.TreePath), tplEditFile, &form)
		} else if models.IsErrBranchNotExist(err) {
			// For when a user adds/updates a file to a branch that no longer exists
			if branchErr, ok := err.(models.ErrBranchNotExist); ok {
				ctx.RenderWithErr(ctx.Tr("repo.editor.branch_does_not_exist", branchErr.Name), tplEditFile, &form)
			} else {
				ctx.Error(500, err.Error())
			}
		} else if models.IsErrBranchAlreadyExists(err) {
			// For when a user specifies a new branch that already exists
			ctx.Data["Err_NewBranchName"] = true
			if branchErr, ok := err.(models.ErrBranchAlreadyExists); ok {
				ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchErr.BranchName), tplEditFile, &form)
			} else {
				ctx.Error(500, err.Error())
			}
		} else if models.IsErrCommitIDDoesNotMatch(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+form.LastCommit+"..."+ctx.Repo.CommitID), tplEditFile, &form)
		} else {
			ctx.RenderWithErr(ctx.Tr("repo.editor.fail_to_update_file", form.TreePath, err), tplEditFile, &form)
		}
	} else {
		ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + branchName + "/" + strings.NewReplacer("%", "%25", "#", "%23", " ", "%20", "?", "%3F").Replace(cleanUploadFileName(form.TreePath)))
		ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(branchName) + "/" + util.PathEscapeSegments(form.TreePath))
	}
}

// EditFilePost response for editing file
func EditFilePost(ctx *context.Context, form auth.EditRepoFileForm) {
	editFilePost(ctx, form, false)
}

// NewFilePost response for creating file
func NewFilePost(ctx *context.Context, form auth.EditRepoFileForm) {
	editFilePost(ctx, form, true)
}

// DiffPreviewPost render preview diff page
func DiffPreviewPost(ctx *context.Context, form auth.EditPreviewDiffForm) {
	treePath := cleanUploadFileName(ctx.Repo.TreePath)
	if len(treePath) == 0 {
		ctx.Error(500, "file name to diff is invalid")
		return
	}

	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(treePath)
	if err != nil {
		ctx.Error(500, "GetTreeEntryByPath: "+err.Error())
		return
	} else if entry.IsDir() {
		ctx.Error(422)
		return
	}

	diff, err := repofiles.GetDiffPreview(ctx.Repo.Repository, ctx.Repo.BranchName, treePath, form.Content)
	if err != nil {
		ctx.Error(500, "GetDiffPreview: "+err.Error())
		return
	}

	if diff.NumFiles() == 0 {
		ctx.PlainText(200, []byte(ctx.Tr("repo.editor.no_changes_to_show")))
		return
	}
	ctx.Data["File"] = diff.Files[0]

	ctx.HTML(200, tplEditDiffPreview)
}

// DeleteFile render delete file page
func DeleteFile(ctx *context.Context) {
	ctx.Data["PageIsDelete"] = true
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	treePath := cleanUploadFileName(ctx.Repo.TreePath)

	if treePath != ctx.Repo.TreePath {
		ctx.Redirect(path.Join(ctx.Repo.RepoLink, "_delete", ctx.Repo.BranchName, treePath))
		return
	}

	ctx.Data["TreePath"] = treePath
	canCommit := renderCommitRights(ctx)

	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = ""

	ctx.HTML(200, tplDeleteFile)
}

// DeleteFilePost response for deleting file
func DeleteFilePost(ctx *context.Context, form auth.DeleteRepoFileForm) {
	canCommit := renderCommitRights(ctx)
	branchName := ctx.Repo.BranchName
	if form.CommitChoice == frmCommitChoiceNewBranch {
		branchName = form.NewBranchName
	}

	ctx.Data["PageIsDelete"] = true
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	ctx.Data["TreePath"] = ctx.Repo.TreePath
	ctx.Data["commit_summary"] = form.CommitSummary
	ctx.Data["commit_message"] = form.CommitMessage
	ctx.Data["commit_choice"] = form.CommitChoice
	ctx.Data["new_branch_name"] = form.NewBranchName
	ctx.Data["last_commit"] = ctx.Repo.CommitID

	if ctx.HasError() {
		ctx.HTML(200, tplDeleteFile)
		return
	}

	if branchName != ctx.Repo.BranchName && !canCommit {
		ctx.Data["Err_NewBranchName"] = true
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
		ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplDeleteFile, &form)
		return
	}

	message := strings.TrimSpace(form.CommitSummary)
	if len(message) == 0 {
		message = ctx.Tr("repo.editor.delete", ctx.Repo.TreePath)
	}
	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	if _, err := repofiles.DeleteRepoFile(ctx.Repo.Repository, ctx.User, &repofiles.DeleteRepoFileOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    ctx.Repo.BranchName,
		NewBranch:    branchName,
		TreePath:     ctx.Repo.TreePath,
		Message:      message,
	}); err != nil {
		// This is where we handle all the errors thrown by repofiles.DeleteRepoFile
		if git.IsErrNotExist(err) || models.IsErrRepoFileDoesNotExist(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_deleting_no_longer_exists", ctx.Repo.TreePath), tplEditFile, &form)
		} else if models.IsErrFilenameInvalid(err) {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_invalid", ctx.Repo.TreePath), tplEditFile, &form)
		} else if models.IsErrFilePathInvalid(err) {
			ctx.Data["Err_TreePath"] = true
			if fileErr, ok := err.(models.ErrFilePathInvalid); ok {
				switch fileErr.Type {
				case git.EntryModeSymlink:
					ctx.RenderWithErr(ctx.Tr("repo.editor.file_is_a_symlink", fileErr.Path), tplEditFile, &form)
					break
				case git.EntryModeTree:
					ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_a_directory", fileErr.Path), tplEditFile, &form)
					break
				case git.EntryModeBlob:
					ctx.RenderWithErr(ctx.Tr("repo.editor.directory_is_a_file", fileErr.Path), tplEditFile, &form)
					break
				default:
					ctx.ServerError("DeleteRepoFile", err)
					break
				}
			} else {
				ctx.ServerError("DeleteRepoFile", err)
			}
		} else if models.IsErrBranchNotExist(err) {
			// For when a user deletes a file to a branch that no longer exists
			if branchErr, ok := err.(models.ErrBranchNotExist); ok {
				ctx.RenderWithErr(ctx.Tr("repo.editor.branch_does_not_exist", branchErr.Name), tplEditFile, &form)
			} else {
				ctx.Error(500, err.Error())
			}
		} else if models.IsErrBranchAlreadyExists(err) {
			// For when a user specifies a new branch that already exists
			if branchErr, ok := err.(models.ErrBranchAlreadyExists); ok {
				ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchErr.BranchName), tplEditFile, &form)
			} else {
				ctx.Error(500, err.Error())
			}
		} else if models.IsErrCommitIDDoesNotMatch(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+form.LastCommit+"..."+ctx.Repo.CommitID), tplEditFile, &form)
		} else {
			ctx.ServerError("DeleteRepoFile", err)
		}
	} else {
		ctx.Flash.Success(ctx.Tr("repo.editor.file_delete_success", ctx.Repo.TreePath))
		ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(branchName))
	}
}

func renderUploadSettings(ctx *context.Context) {
	ctx.Data["RequireDropzone"] = true
	ctx.Data["UploadAllowedTypes"] = strings.Join(setting.Repository.Upload.AllowedTypes, ",")
	ctx.Data["UploadMaxSize"] = setting.Repository.Upload.FileMaxSize
	ctx.Data["UploadMaxFiles"] = setting.Repository.Upload.MaxFiles
}

// UploadFile render upload file page
func UploadFile(ctx *context.Context) {
	ctx.Data["PageIsUpload"] = true
	renderUploadSettings(ctx)
	canCommit := renderCommitRights(ctx)
	treePath := cleanUploadFileName(ctx.Repo.TreePath)
	if treePath != ctx.Repo.TreePath {
		ctx.Redirect(path.Join(ctx.Repo.RepoLink, "_upload", ctx.Repo.BranchName, treePath))
		return
	}
	ctx.Repo.TreePath = treePath

	treeNames, treePaths := getParentTreeFields(ctx.Repo.TreePath)
	if len(treeNames) == 0 {
		// We must at least have one element for user to input.
		treeNames = []string{""}
	}

	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = ""

	ctx.HTML(200, tplUploadFile)
}

// UploadFilePost response for uploading file
func UploadFilePost(ctx *context.Context, form auth.UploadRepoFileForm) {
	ctx.Data["PageIsUpload"] = true
	renderUploadSettings(ctx)
	canCommit := renderCommitRights(ctx)

	oldBranchName := ctx.Repo.BranchName
	branchName := oldBranchName

	if form.CommitChoice == frmCommitChoiceNewBranch {
		branchName = form.NewBranchName
	}

	form.TreePath = cleanUploadFileName(form.TreePath)
	if len(form.TreePath) == 0 {
		ctx.Error(500, "Upload file name is invalid")
		return
	}

	treeNames, treePaths := getParentTreeFields(form.TreePath)
	if len(treeNames) == 0 {
		// We must at least have one element for user to input.
		treeNames = []string{""}
	}

	ctx.Data["TreePath"] = form.TreePath
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/branch/" + branchName
	ctx.Data["commit_summary"] = form.CommitSummary
	ctx.Data["commit_message"] = form.CommitMessage
	ctx.Data["commit_choice"] = form.CommitChoice
	ctx.Data["new_branch_name"] = branchName

	if ctx.HasError() {
		ctx.HTML(200, tplUploadFile)
		return
	}

	if oldBranchName != branchName {
		if _, err := ctx.Repo.Repository.GetBranch(branchName); err == nil {
			ctx.Data["Err_NewBranchName"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchName), tplUploadFile, &form)
			return
		}
	} else if !canCommit {
		ctx.Data["Err_NewBranchName"] = true
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
		ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplUploadFile, &form)
		return
	}

	var newTreePath string
	for _, part := range treeNames {
		newTreePath = path.Join(newTreePath, part)
		entry, err := ctx.Repo.Commit.GetTreeEntryByPath(newTreePath)
		if err != nil {
			if git.IsErrNotExist(err) {
				// Means there is no item with that name, so we're good
				break
			}

			ctx.ServerError("Repo.Commit.GetTreeEntryByPath", err)
			return
		}

		// User can only upload files to a directory.
		if !entry.IsDir() {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.directory_is_a_file", part), tplUploadFile, &form)
			return
		}
	}

	message := strings.TrimSpace(form.CommitSummary)
	if len(message) == 0 {
		message = ctx.Tr("repo.editor.upload_files_to_dir", form.TreePath)
	}

	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	if err := repofiles.UploadRepoFiles(ctx.Repo.Repository, ctx.User, &repofiles.UploadRepoFileOptions{
		LastCommitID: ctx.Repo.CommitID,
		OldBranch:    oldBranchName,
		NewBranch:    branchName,
		TreePath:     form.TreePath,
		Message:      message,
		Files:        form.Files,
	}); err != nil {
		ctx.Data["Err_TreePath"] = true
		ctx.RenderWithErr(ctx.Tr("repo.editor.unable_to_upload_files", form.TreePath, err), tplUploadFile, &form)
		return
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + branchName + "/" + form.TreePath)
}

func cleanUploadFileName(name string) string {
	// Rebase the filename
	name = strings.Trim(path.Clean("/"+name), " /")
	// Git disallows any filenames to have a .git directory in them.
	for _, part := range strings.Split(name, "/") {
		if strings.ToLower(part) == ".git" {
			return ""
		}
	}
	return name
}

// UploadFileToServer upload file to server file dir not git
func UploadFileToServer(ctx *context.Context) {
	file, header, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("FormFile: %v", err))
		return
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	if n > 0 {
		buf = buf[:n]
	}
	fileType := http.DetectContentType(buf)

	if len(setting.Repository.Upload.AllowedTypes) > 0 {
		allowed := false
		for _, t := range setting.Repository.Upload.AllowedTypes {
			t := strings.Trim(t, " ")
			if t == "*/*" || t == fileType {
				allowed = true
				break
			}
		}

		if !allowed {
			ctx.Error(400, ErrFileTypeForbidden.Error())
			return
		}
	}

	name := cleanUploadFileName(header.Filename)
	if len(name) == 0 {
		ctx.Error(500, "Upload file name is invalid")
		return
	}

	upload, err := models.NewUpload(name, buf, file)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("NewUpload: %v", err))
		return
	}

	log.Trace("New file uploaded: %s", upload.UUID)
	ctx.JSON(200, map[string]string{
		"uuid": upload.UUID,
	})
}

// RemoveUploadFileFromServer remove file from server file dir
func RemoveUploadFileFromServer(ctx *context.Context, form auth.RemoveUploadFileForm) {
	if len(form.File) == 0 {
		ctx.Status(204)
		return
	}

	if err := models.DeleteUploadByUUID(form.File); err != nil {
		ctx.Error(500, fmt.Sprintf("DeleteUploadByUUID: %v", err))
		return
	}

	log.Trace("Upload file removed: %s", form.File)
	ctx.Status(204)
}
