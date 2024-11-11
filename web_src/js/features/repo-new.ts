import $ from 'jquery';

export function initRepoNew() {
  // Repo Creation
  if ($('.repository.new.repo').length > 0) {
    $('input[name="gitignores"], input[name="license"]').on('change', () => {
      const gitignores = $('input[name="gitignores"]').val();
      const license = $('input[name="license"]').val();
      if (gitignores || license) {
        document.querySelector('input[name="auto_init"]').checked = true;
      }
    });
  }
}
