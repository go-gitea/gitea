const $service = $('#service_type');
const $user = $('#auth_username');
const $pass = $('#auth_password');
const $token = $('#auth_token');
const $items = $('#migrate_items').find('.field');

export default function initMigration() {
  checkAuth();

  $user.on('keyup', () => {checkItems(false)});
  $pass.on('keyup', () => {checkItems(false)});
  $token.on('keyup', () => {checkItems(true)});

  const $cloneAddr = $('#clone_addr');
  $cloneAddr.on('change', () => {
    const $repoName = $('#repo_name');
    if ($cloneAddr.val().length > 0 && $repoName.val().length === 0) { // Only modify if repo_name input is blank
      $repoName.val($cloneAddr.val().match(/^(.*\/)?((.+?)(\.git)?)$/)[3]);
    }
  });
}

function checkAuth() {
  const serviceType = $service.val();

  checkItems(serviceType !== 1);
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
