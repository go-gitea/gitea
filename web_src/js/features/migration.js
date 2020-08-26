const $service = $('#service_type');
const $user = $('#auth_username');
const $pass = $('#auth_password');
const $token = $('#auth_token');
const $items = $('#migrate_items').find('.field');

export default function initMigration() {
  checkAuth();

  $service.on('change', checkAuth);
  $user.on('keyup', () => {checkItems(false)});
  $pass.on('keyup', () => {checkItems(false)});
  $token.on('keyup', () => {checkItems(true)});
}

function checkAuth() {
  const serviceType = $service.val();
  const tokenAuth = $(`#service-${serviceType}`).data('token');

  if (tokenAuth) {
    $user.parent().addClass('disabled');
    $pass.parent().addClass('disabled');
    $token.parent().removeClass('disabled');
  } else {
    $user.parent().removeClass('disabled');
    $pass.parent().removeClass('disabled');
    $token.parent().addClass('disabled');
  }

  checkItems(tokenAuth);
}

function checkItems(tokenAuth) {
  let enableItems;
  if (tokenAuth) {
    enableItems = $token.val() !== '';
  } else {
    enableItems = $user.val() !== '' || $pass.val() !== '';
  }
  if (enableItems && $service.val() > 1) {
    $items.removeClass('disabled');
  } else {
    $items.addClass('disabled');
  }
}
