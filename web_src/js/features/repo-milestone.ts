import $ from 'jquery';

export function initRepoMilestone() {
  // Milestones
  if ($('.repository.new.milestone').length > 0) {
    $('#clear-date').on('click', () => {
      $('#deadline').val('');
      return false;
    });
  }
}
