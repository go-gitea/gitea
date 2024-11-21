import $ from 'jquery';

export function initAdminEmails(): void {
  $('.link-email-action').on('click', (e) => {
    const $this = $(this);
    $('#form-uid').val($this.data('uid'));
    $('#form-email').val($this.data('email'));
    $('#form-primary').val($this.data('primary'));
    $('#form-activate').val($this.data('activate'));
    $('#change-email-modal').modal('show');
    e.preventDefault();
  });
}
