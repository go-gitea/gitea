const {AppSubUrl, csrf} = window.config;

export function initRepoMigrationStatusChecker() {
  const migrating = $('#repo_migrating');
  $('#repo_migrating_failed').hide();
  $('#repo_migrating_failed_image').hide();
  $('#repo_migrating_progress_message').hide();
  if (migrating) {
    const task = migrating.attr('task');
    if (typeof task === 'undefined') {
      return;
    }
    $.ajax({
      type: 'GET',
      url: `${AppSubUrl}/user/task/${task}`,
      data: {
        _csrf: csrf,
      },
      complete(xhr) {
        if (xhr.status === 200 && xhr.responseJSON) {
          if (xhr.responseJSON.status === 4) {
            window.location.reload();
            return;
          } else if (xhr.responseJSON.status === 3) {
            $('#repo_migrating_progress').hide();
            $('#repo_migrating').hide();
            $('#repo_migrating_failed').show();
            $('#repo_migrating_failed_image').show();
            $('#repo_migrating_failed_error').text(xhr.responseJSON.message);
            return;
          }
          if (xhr.responseJSON.message) {
            $('#repo_migrating_progress_message').show();
            $('#repo_migrating_progress_message').text(xhr.responseJSON.message);
          }
          setTimeout(() => {
            initRepoMigrationStatusChecker();
          }, 2000);
          return;
        }
        $('#repo_migrating_progress').hide();
        $('#repo_migrating').hide();
        $('#repo_migrating_failed').show();
        $('#repo_migrating_failed_image').show();
      }
    });
  }
}
