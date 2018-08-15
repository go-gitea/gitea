package private

import (
	"encoding/json"
	"fmt"
	"net/url"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// GetRepository return the repository by its ID
func GetRepository(repoID int64) (*models.Repository, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/repository/%d", repoID)
	log.GitLogger.Trace("GetRepository: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}

	var repository *models.Repository
	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("failed to retrieve repository: %s", decodeJSONError(resp).Err)
	}

	return repository, nil
}

// ActivePullRequest returns an active pull request if it exists
func ActivePullRequest(repoID int64, baseBranch, headBranch string) (*models.PullRequest, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/active-pull-request/%d/%s...%s", repoID, url.QueryEscape(baseBranch), url.QueryEscape(headBranch))
	log.GitLogger.Trace("ActivePullRequest: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		log.GitLogger.Trace(err.Error())
		return nil, err
	}

	var pr *models.PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("failed to retrieve pull request: %s", decodeJSONError(resp).Err)
	}

	return pr, nil
}
