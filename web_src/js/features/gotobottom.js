import $ from 'jquery';

export function initGotoBottom() {
  const commentContainer = document.querySelectorAll('.comment-container');

  if (commentContainer.length) {
    window.commentIndex = 0;
    $('#goup').removeClass('gt-hidden');
    $('#godown').removeClass('gt-hidden');
  }
  if ($(window).height() < document.body.scrollHeight) {
    $('#gobottom').removeClass('gt-hidden');
  }

  $('#goup').on('click', () => {
    if (window.commentIndex > 0) {
      commentContainer[--window.commentIndex].scrollIntoView({behavior: 'smooth', block: 'start'});
      $('#godown').removeClass('gt-hidden');
    }
    if (window.commentIndex <= 0) {
      $('#goup').addClass('gt-hidden');
    }
  });
  $('#godown').on('click', () => {
    if (window.commentIndex <= commentContainer.length) {
      commentContainer[++window.commentIndex].scrollIntoView({behavior: 'smooth', block: 'start'});
      $('#goup').removeClass('gt-hidden');
    }
    if (window.commentIndex >= commentContainer.length - 1) {
      $('#godown').addClass('gt-hidden');
    }
  });

  $('#gotop').on('click', () => {
    $(this).addClass('gt-hidden');
    window.scrollTo({top: 0, behavior: 'smooth'});
  });
  $('#gobottom').on('click', () => {
    $(this).addClass('gt-hidden');
    window.scrollTo({top: document.body.scrollHeight, behavior: 'smooth'});
  });

  clearTimeout($.data(document.body, 'scrollStopTimer'));
  $.data(document.body, 'scrollStopTimer', setTimeout(() => {
    $(window).on('scroll', () => {
      if ($(window).scrollTop() > $(window).height() / 2) {
        $('#gotop').removeClass('gt-hidden');
      } else {
        $('#gotop').addClass('gt-hidden');
      }
      if (document.body.scrollHeight - $(window).height() - window.scrollY > 10) {
        $('#gobottom').removeClass('gt-hidden');
      } else {
        $('#gobottom').addClass('gt-hidden');
      }
    });
  }));
}
