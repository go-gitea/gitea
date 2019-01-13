package repo

import (
	"bytes"
	"fmt"
	gotemplate "html/template"
	"strings"

	"code.gitea.io/gitea/models"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
)

const (
	tplBlame base.TplName = "repo/home"
)

// RefBlame render blame page
func RefBlame(ctx *context.Context) {

	fileName := ctx.Repo.TreePath
	if len(fileName) == 0 {
		ctx.NotFound("Blame FileName", nil)
		return
	}

	userName := ctx.Repo.Owner.Name
	repoName := ctx.Repo.Repository.Name
	commitID := ctx.Repo.CommitID

	commit, err := ctx.Repo.GitRepo.GetCommit(commitID)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("Repo.GitRepo.GetCommit", err)
		} else {
			ctx.ServerError("Repo.GitRepo.GetCommit", err)
		}
		return
	}
	if len(commitID) != 40 {
		commitID = commit.ID.String()
	}

	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	treeLink := branchLink
	rawLink := ctx.Repo.RepoLink + "/raw/" + ctx.Repo.BranchNameSubURL()

	if len(ctx.Repo.TreePath) > 0 {
		treeLink += "/" + ctx.Repo.TreePath
	}

	var treeNames []string
	paths := make([]string, 0, 5)
	if len(ctx.Repo.TreePath) > 0 {
		treeNames = strings.Split(ctx.Repo.TreePath, "/")
		for i := range treeNames {
			paths = append(paths, strings.Join(treeNames[:i+1], "/"))
		}

		ctx.Data["HasParentPath"] = true
		if len(paths)-2 >= 0 {
			ctx.Data["ParentPath"] = "/" + paths[len(paths)-1]
		}
	}

	// Show latest commit info of repository in table header,
	// or of directory if not in root directory.
	latestCommit := ctx.Repo.Commit
	if len(ctx.Repo.TreePath) > 0 {
		latestCommit, err = ctx.Repo.Commit.GetCommitByPath(ctx.Repo.TreePath)
		if err != nil {
			ctx.ServerError("GetCommitByPath", err)
			return
		}
	}
	ctx.Data["LatestCommit"] = latestCommit
	ctx.Data["LatestCommitVerification"] = models.ParseCommitWithSignature(latestCommit)
	ctx.Data["LatestCommitUser"] = models.ValidateCommitWithEmail(latestCommit)

	statuses, err := models.GetLatestCommitStatus(ctx.Repo.Repository, ctx.Repo.Commit.ID.String(), 0)
	if err != nil {
		log.Error(3, "GetLatestCommitStatus: %v", err)
	}

	// Get current entry user currently looking at.
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.GetTreeEntryByPath", git.IsErrNotExist, err)
		return
	}

	blob := entry.Blob()

	ctx.Data["LatestCommitStatus"] = models.CalcCommitStatus(statuses)

	ctx.Data["Paths"] = paths
	ctx.Data["TreeLink"] = treeLink
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["BranchLink"] = branchLink
	ctx.Data["HighlightClass"] = highlight.FileNameToHighlightClass(entry.Name())
	if !markup.IsReadmeFile(blob.Name()) {
		ctx.Data["RequireHighlightJS"] = true
	}
	ctx.Data["RawFileLink"] = rawLink + "/" + ctx.Repo.TreePath
	ctx.Data["PageIsViewCode"] = true

	ctx.Data["IsBlame"] = true

	if ctx.Repo.CanEnableEditor() {
		ctx.Data["CanDeleteFile"] = true
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.delete_this_file")
	} else if !ctx.Repo.IsViewBranch {
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_be_on_a_branch")
	} else if !ctx.Repo.CanWrite(models.UnitTypeCode) {
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_have_write_access")
	}

	ctx.Data["FileSize"] = blob.Size()
	ctx.Data["FileName"] = blob.Name()

	blame, err := models.GetBlame(models.RepoPath(userName, repoName), commitID, fileName)

	if err != nil {
		ctx.NotFound("GetBlameCommit", err)
		return
	}

	commitNames := make(map[string]string)

	for _, part := range blame.Parts {
		sha := part.Sha

		commit, err := ctx.Repo.GitRepo.GetCommit(sha)
		if err != nil {
			if git.IsErrNotExist(err) {
				ctx.NotFound("Repo.GitRepo.GetCommit", err)
			} else {
				ctx.ServerError("Repo.GitRepo.GetCommit", err)
			}
			return
		}

		commitNames[sha] = commit.CommitMessage

	}

	renderBlame(ctx, blame, commitNames)

	ctx.HTML(200, tplBlame)
}

func renderBlame(ctx *context.Context, blame *models.BlameFile, commitNames map[string]string) {

	repoLink := ctx.Repo.RepoLink

	var lines = make([]string, 0, 0)

	for _, part := range blame.Parts {
		for _, line := range part.Lines {
			lines = append(lines, line)
		}
	}

	var output bytes.Buffer

	var i = 0

	for _, part := range blame.Parts {
		for index, line := range part.Lines {
			i++
			line = gotemplate.HTMLEscapeString(line)
			if i != len(lines) {
				line += "\n"
			}
			if index == len(part.Lines)-1 {
				output.WriteString(fmt.Sprintf(`<li class="L%d bottom-line" rel="L%d">%s</li>`, i, i, line))
			} else {
				output.WriteString(fmt.Sprintf(`<li class="L%d" rel="L%d">%s</li>`, i, i, line))
			}
		}
	}

	ctx.Data["BlameContent"] = gotemplate.HTML(output.String())

	output.Reset()

	for _, part := range blame.Parts {

		for index := range part.Lines {
			if index == 0 {
				if index == len(part.Lines)-1 {
					output.WriteString(fmt.Sprintf(`<span class="bottom-line"><a href="%s/commit/%s">%s</a></span>`, repoLink, part.Sha, commitNames[part.Sha]))
				} else {
					output.WriteString(fmt.Sprintf(`<span><a href="%s/commit/%s">%s</a></span>`, repoLink, part.Sha, commitNames[part.Sha]))
				}
			} else {
				if index == len(part.Lines)-1 {
					output.WriteString(`<span class="bottom-line"><a>&#8203;</a></span>`)
				} else {
					output.WriteString(`<span><a>&#8203;</a></span>`)
				}
			}
		}

	}

	ctx.Data["BlameCommitInfo"] = gotemplate.HTML(output.String())

	output.Reset()

	i = 0

	for _, part := range blame.Parts {
		for index := range part.Lines {
			i++
			if index == len(part.Lines)-1 {
				output.WriteString(fmt.Sprintf(`<span id="L%d" class="bottom-line">%d</span>`, i, i))
			} else {
				output.WriteString(fmt.Sprintf(`<span id="L%d">%d</span>`, i, i))
			}
		}
	}

	ctx.Data["BlameLineNums"] = gotemplate.HTML(output.String())

}
