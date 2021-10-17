export function initRepoCommitButton() {
  $('.commit-button').on('click', function (e) {
    e.preventDefault();
    $(this).parent().find('.commit-body').toggle();
  });
}
