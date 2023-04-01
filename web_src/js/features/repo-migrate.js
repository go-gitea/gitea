import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';

const {appSubUrl, csrfToken} = window.config;

export function initRepoMigrationStatusChecker() {
  const migrating = $('#repo_migrating');
  hideElem($('#repo_migrating_failed'));
  hideElem($('#repo_migrating_failed_image'));
  hideElem($('#repo_migrating_progress_message'));
  if (migrating) {
    const task = migrating.attr('task');
    if (task === undefined) {
      return;
    }
    $.ajax({
      type: 'GET',
      url: `${appSubUrl}/user/task/${task}`,
      data: {
        _csrf: csrfToken,
      },
      complete(xhr) {
        if (xhr.status === 200 && xhr.responseJSON) {
          if (xhr.responseJSON.status === 4) {
            window.location.reload();
            return;
          } else if (xhr.responseJSON.status === 3) {
            hideElem($('#repo_migrating_progress'));
            hideElem($('#repo_migrating'));
            showElem($('#repo_migrating_failed'));
            showElem($('#repo_migrating_failed_image'));
            $('#repo_migrating_failed_error').text(xhr.responseJSON.message);
            return;
          }
          if (xhr.responseJSON.message) {
            showElem($('#repo_migrating_progress_message'));
            $('#repo_migrating_progress_message').text(xhr.responseJSON.message);
          }
          setTimeout(() => {
            initRepoMigrationStatusChecker();
          }, 2000);
          return;
        }
        hideElem($('#repo_migrating_progress'));
        hideElem($('#repo_migrating'));
        showElem($('#repo_migrating_failed'));
        showElem($('#repo_migrating_failed_image'));
      }
    });
  }
}
