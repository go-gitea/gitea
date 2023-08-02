import $ from 'jquery';

export function initRepoPullRequestCommitStatus() {
  const $prStatusList = $('.pr-status-list');

  $('.hide-all-checks').on('click', async (e) => {
    e.preventDefault();
    const $this = $(e.currentTarget);
    if ($prStatusList.hasClass('hide')) {
      $prStatusList.removeClass('hide');
      $this.text($this.attr('data-hide-all'));
    } else {
      $prStatusList.addClass('hide');
      $this.text($this.attr('data-show-all'));
    }
  });
}
