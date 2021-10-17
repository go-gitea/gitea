export function initUserSettings() {
  if ($('.user.settings.profile').length > 0) {
    $('#username').on('keyup', function () {
      const $prompt = $('#name-change-prompt');
      const $prompt_redirect = $('#name-change-redirect-prompt');
      if ($(this).val().toString().toLowerCase() !== $(this).data('name').toString().toLowerCase()) {
        $prompt.show();
        $prompt_redirect.show();
      } else {
        $prompt.hide();
        $prompt_redirect.hide();
      }
    });
  }
}
