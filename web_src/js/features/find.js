function hitAllKeys(keys, entry) {
  let i = 0;
  let j = 0;
  const hitIndexes = [];
  while (i < keys.length) {
    for (; j < entry.length; j++) {
      if (keys[i] === entry[j]) {
        hitIndexes.push(j);
        j++;
        break;
      }
    }
    i++;
  }

  return hitIndexes;
}

function addHighLightToHit($a, entry, indexes) {
  let highLightText = '';
  for (let i = 0; i < entry.length; i++) {
    if (indexes.includes(i)) {
      highLightText += `<b>${entry[i]}</b>`;
    } else {
      highLightText += entry[i];
    }
  }
  $a.html(highLightText);
}

function removeHighLight($a, entry) {
  $a.text(entry.replace(/(<([^>]+)>)/ig, ''));
}

function filterRepoFiles(keys) {
  if (keys.length > 0) {
    let hit = false;
    $('#repo-find-files-table tr').each(function() {
      const entry = $(this).find('td:first').text();
      const $a = $(this).find('td:first').find('a:first');
      const keysTrim = keys.trim();
      const entryTrim = entry.trim();
      const hitIndexes = hitAllKeys(keysTrim, entryTrim);
      if (hitIndexes.length > 0 && keysTrim.length === hitIndexes.length) {
        addHighLightToHit($a, entryTrim, hitIndexes);
        $(this).show();
        hit = true;
      } else {
        removeHighLight($a, entryTrim);
        $(this).hide();
      }
    });
    if (hit) {
      $('#no-hit-prompt').hide();
    } else {
      $('#no-hit-prompt').show();
    }
  } else {
    // Remove all highlight
    $('#repo-find-files-table tr').each(function() {
      const entry = $(this).find('td:first').text();
      const $a = $(this).find('td:first').find('a:first');
      removeHighLight($a, entry.trim());
    });
    $('#no-hit-prompt').hide();
    $('#repo-find-files-table tr').show();
  }
}

export default function initFindFileInRepo() {
  $('#repo-file-find-input').on('change paste keyup', () => {
    filterRepoFiles($('#repo-file-find-input').val());
  });
}
