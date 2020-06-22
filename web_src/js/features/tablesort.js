export default function initTableSort() {
  $('th[data-sortt-asc]').each(function () {
    // get data
    const {sorttAsc, sorttDesc, sorttDefault} = this.dataset;

    // add onclick event
    $(this).on('click', () => {
      tableSort(sorttAsc, sorttDesc, sorttDefault);
    });
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
