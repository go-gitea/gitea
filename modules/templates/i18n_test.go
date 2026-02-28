package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_FindTemplateKeys(t *testing.T) {
	var kases = []struct {
		path string
		expectedKeys []string
	}{
		{
			path : "../../templates/repo/issue/view_content/comments.tmpl",
			expectedKeys : []string{
				"repo.issues.remove_time_estimate_at",
				"repo.issues.closed_at",
				"repo.issues.force_push_codes",
				"repo.issues.review.remove_review_request",
				"repo.issues.add_assignee_at",
				"repo.issues.self_assign_at",
				"repo.issues.remove_project_at",
				"repo.issues.dependency.removed_dependency",
				"repo.issues.change_title_at",
				"repo.pulls.reopened_at",
				"repo.issues.remove_ref_at",
				"repo.pulls.auto_merge_newly_scheduled_comment",
				"repo.issues.remove_milestone_at",
				"repo.issues.unpin_comment",
				"repo.issues.change_milestone_at",
				"repo.pulls.auto_merge_canceled_schedule_comment",
				"repo.issues.remove_label",
				"repo.issues.move_to_column_of_project",
				"repo.issues.add_remove_labels",
				"repo.issues.review.remove_review_request_self",
				"repo.issues.comment_manually_pull_merged_at",
				"repo.issues.commented_at",
				"repo.issues.reopened_at",
				"repo.pulls.closed_at",
				"repo.issues.comment_pull_merged_at",
				"repo.issues.ref_from",
				"repo.issues.commit_ref_at",
				"repo.issues.add_label",
				"repo.issues.add_labels",
				"repo.issues.remove_labels",
				"repo.issues.add_milestone_at",
				"repo.issues.remove_self_assignment",
				"repo.issues.remove_assignee_at",
				"repo.issues.delete_branch_at",
				"repo.issues.start_tracking_history",
				"repo.issues.stop_tracking_history",
				"repo.issues.add_time_history",
				"repo.issues.cancel_tracking_history",
				"repo.issues.due_date_added",
				"repo.issues.due_date_modified",
				"repo.issues.due_date_remove",
				"repo.issues.dependency.added_dependency",
				"repo.issues.review.approve",
				"repo.issues.review.comment",
				"repo.issues.review.reject",
				"repo.issues.review.dismissed_label",
				"repo.migrated_from",
				"repo.issues.review.left_comment",
				"repo.issues.no_content",
				"repo.issues.lock_with_reason",
				"repo.issues.lock_no_reason",
				"repo.issues.unlock_comment",
				"repo.pulls.change_target_branch_at",
				"repo.issues.del_time_history",
				"repo.issues.review.add_review_request",
				"repo.issues.push_commit_1",
				"repo.issues.push_commits_n",
				"repo.issues.force_push_compare",
				"projects.deleted.display_name",
				"projects.type-%d.display_name",
				"repo.issues.change_project_at",
				"repo.issues.add_project_at",
				"repo.issues.review.dismissed",
				"action.review_dismissed_reason",
				"repo.issues.change_ref_at",
				"repo.issues.add_ref_at",
				"repo.issues.pin_comment",
				"repo.issues.change_time_estimate_at",
			},
		},
		{
			path: "../../templates/repo/header.tmpl",
			expectedKeys: []string{
				 "repo.desc.archived",
				 "repo.desc.private",
				 "repo.desc.internal",
				 "repo.desc.public_access",
				 "repo.desc.template",
				 "repo.desc.sha256",
				 "repo.transfer.accept_desc",
				 "repo.transfer.no_permission_to_accept",
				 "repo.transfer.accept",
				 "repo.transfer.reject_desc",
				 "repo.transfer.no_permission_to_reject",
				 "repo.transfer.reject",
				"rss_feed",
				 "repo.fork_guest_user",
				 "repo.fork_from_self",
				"repo.fork",
				 "repo.already_forked",
				 "repo.fork_to_different_account",
				 "repo.mirror_from",
				 "repo.mirror_sync",
				 "repo.forked_from",
				 "repo.generated_from",
				"repo.code",
				"repo.issues",
				"repo.pulls",
				"actions.actions",
				"packages.title",
				"repo.projects",
				"repo.releases",
				"repo.wiki",
				"repo.activity",
				"repo.settings",
				"repo.migration_status",
			},
		},
		{
			path: "../../templates/repo/wiki/new.tmpl",
			expectedKeys: []string{
				"repo.wiki.page_content",
				 "repo.wiki.save_page",
				 "repo.wiki.new_page",
				 "repo.wiki.new_page_button",
				 "repo.wiki.page_title",
				 "repo.wiki.page_name_desc",
				 "repo.wiki.welcome",
				 "repo.wiki.default_commit_message",
				"cancel",
			},
		},
	}

	for _, kase := range kases {
		t.Run(kase.path, func(t *testing.T) {
			keys, err := FindTemplateKeys(kase.path)
			assert.NoError(t, err)
			assert.ElementsMatch(t, kase.expectedKeys, keys.Values())
		})
	}
}
