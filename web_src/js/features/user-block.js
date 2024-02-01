import $ from 'jquery';

export function initUserBlock() {
  $(document).on('click', '.block-user-action', function (event) {
    const $this = $(this);

    const $modal = $($this.data('modal'));
    $modal.find('span[name="blockee-name"]').text($this.data('blockee-name'));
    $modal.find('form').attr('action', $this.data('action'));
    $modal.find('input[name="blockee"]').val($this.data('blockee'));
    $modal.modal('show');
  
    event.preventDefault();
  });
  $(document).on('click', '.block-user-note-action', function (event) {
    const $this = $(this);

    const $modal = $($this.data('modal'));
    $modal.find('input[name="blockee"]').val($this.data('blockee'));
    $modal.find('input[name="note"]').val($this.data('note'));
    $modal.modal('show');
  
    event.preventDefault();
  });
}
