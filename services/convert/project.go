package convert

// use this as reference to create the ToProject function:
/*
// ToLabel converts Label to API format
func ToLabel(label *issues_model.Label, repo *repo_model.Repository, org *user_model.User) *api.Label {
	result := &api.Label{
		ID:          label.ID,
		Name:        label.Name,
		Exclusive:   label.Exclusive,
		Color:       strings.TrimLeft(label.Color, "#"),
		Description: label.Description,
		IsArchived:  label.IsArchived(),
	}

	labelBelongsToRepo := label.BelongsToRepo()

	// calculate URL
	if labelBelongsToRepo && repo != nil {
		result.URL = fmt.Sprintf("%s/labels/%d", repo.APIURL(), label.ID)
	} else { // BelongsToOrg
		if org != nil {
			result.URL = fmt.Sprintf("%sapi/v1/orgs/%s/labels/%d", setting.AppURL, url.PathEscape(org.Name), label.ID)
		} else {
			log.Error("ToLabel did not get org to calculate url for label with id '%d'", label.ID)
		}
	}

	if labelBelongsToRepo && repo == nil {
		log.Error("ToLabel did not get repo to calculate url for label with id '%d'", label.ID)
	}

	return result
}
*/

// ToProject converts Project to API format
// func ToProject(project *project_model.Project, repo *repo_model.Repository, org *user_model.User) *api.Project {
// 	result := &api.Project{
// 		ID:           project.ID,
// 		Title:        project.Title,
// 		Description:  project.Description,
// 		TemplateType: project.TemplateType,
// 		CardType:     project.CardType,
// 	}

// 	projectBelongsToRepo := project.BelongsToRepo()

// 	// calculate URL
// 	if projectBelongsToRepo && repo != nil {
// 		result.URL = fmt.Sprintf("%s/projects/%d", repo.APIURL(), project.ID)
// 	} else { // BelongsToOrg
// 		if org != nil {
// 			result.URL = fmt.Sprintf("%sapi/v1/orgs/%s/projects/%d", setting.AppURL, url.PathEscape(org.Name), project.ID)
// 		} else {
// 			log.Error("ToProject did not get org to calculate url for project with id '%d'", project.ID)
// 		}
// 	}

// 	if projectBelongsToRepo && repo == nil {
// 		log.Error("ToProject did not get repo to calculate url for project with id '%d'", project.ID)
// 	}

// 	return result
// }
