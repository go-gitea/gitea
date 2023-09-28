import $ from 'jquery';

export function initRepoPullRequestCommitStatus() {
  const $prStatus = $('.pr-status');

  $('.hide-all-checks').on('click', async (e) => {
    e.preventDefault();
    const $this = $(e.currentTarget);

    if ($prStatus.hasClass('hide')) {
      $this.text($this.attr('data-hide-all'));
    } else {
      $this.text($this.attr('data-show-all'));
    }
    $prStatus.toggleClass('hide');
  });
}
