const {csrf} = window.config;

export async function initLastCommitLoader() {
  const entryMap = {};

  const entries = $('table#repo-files-table tr.notready')
    .map((_, v) => {
      const elem = $(v);
      const key = elem.attr('data-entryname');
      entryMap[key] = elem;
      return key;
    })
    .get();

  if (entries.length === 0) {
    return;
  }

  const lastCommitLoaderURL = $('table#repo-files-table').data('lastCommitLoaderUrl');

  if (entries.length > 200) {
    $.post(lastCommitLoaderURL, {
      _csrf: csrf,
    }, (data) => {
      $('table#repo-files-table').replaceWith(data);
    });
  } else {
    $.post(lastCommitLoaderURL, {
      _csrf: csrf,
      'f': entries,
    }, (data) => {
      $(data).find('tr').each((_, row) => {
        if (row.className === 'commit-list') {
          $('table#repo-files-table .commit-list').replaceWith(row);
        } else {
          entryMap[$(row).attr('data-entryname')].replaceWith(row);
        }
      });
    });
  }
}
