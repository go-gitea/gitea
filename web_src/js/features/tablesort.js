import {svg} from '../utils.js';

export default function initTableSort() {
  $('th[data-sortt-asc]').each(function () {
    // get data
    const {sorttAsc, sorttDesc, sorttDefault} = this.dataset;

    // add onclick event
    $(this).on('click', () => {
      tableSort(sorttAsc, sorttDesc, sorttDefault);
    });

    // add arrow to column
    const arrow = getArrow(sorttAsc, sorttDesc, sorttDefault);
    // if function got a match ...
    if (arrow !== false) {
      $(this).append(arrow);
    }
  });
}

function tableSort(normSort, revSort, isDefault) {
  if (!normSort) return false;
  if (!revSort) revSort = '';

  const url = new URL(window.location);
  let urlSort = url.searchParams.get('sort');
  if (urlSort === null && isDefault) urlSort = normSort;


  if (urlSort !== normSort) {
    url.searchParams.set('sort', normSort);
  } else if (revSort !== '') {
    url.searchParams.set('sort', revSort);
  }

  window.location.replace(url.href);
}

// create global function with main routine
function getArrow(normSort, revSort, isDefault) {
  // normSort is needed
  if (!normSort) return false;

  // default values of optional parameters
  if (!revSort) revSort = '';

  // get sort param from url
  const urlSort = (new URL(window.location)).searchParams.get('sort');

  if (urlSort === null && isDefault) {
    // if sort is sorted as default add arrow tho this table header
    if (isDefault) return svg('octicon-triangle-down', 16);
  } else {
    // if sort arg is in url test if it correlates with column header sort arguments
    if (urlSort === normSort) {
      // the table is sorted with this header normal
      return svg('octicon-triangle-down', 16);
    } else if (urlSort === revSort) {
      // the table is sorted with this header reverse
      return svg('octicon-triangle-up', 16);
    }
    // the table is NOT sorted with this header
    return false;
  }
}
