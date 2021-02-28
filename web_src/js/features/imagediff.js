export default async function initImageDiff() {
  function createContext(image1, image2) {
    const size1 = {
      width: image1 && image1.width || 0,
      height: image1 && image1.height || 0
    };
    const size2 = {
      width: image2 && image2.width || 0,
      height: image2 && image2.height || 0
    };
    const max = {
      width: Math.max(size2.width, size1.width),
      height: Math.max(size2.height, size1.height)
    };

    return {
      image1: $(image1),
      image2: $(image2),
      size1,
      size2,
      max,
      ratio: [
        Math.floor(max.width - size1.width) / 2,
        Math.floor(max.height - size1.height) / 2,
        Math.floor(max.width - size2.width) / 2,
        Math.floor(max.height - size2.height) / 2
      ]
    };
  }

  $('.image-diff').each(function() {
    const $container = $(this);
    const pathAfter = $container.data('path-after');
    const pathBefore = $container.data('path-before');

    const imageInfos = [{
      loaded: false,
      path: pathAfter,
      $image: $container.find('img.image-after')
    }, {
      loaded: false,
      path: pathBefore,
      $image: $container.find('img.image-before')
    }];

    for (const info of imageInfos) {
      if (info.$image.length > 0) {
        info.$image.on('load', () => {
          info.loaded = true;
          setReadyIfLoaded();
        });
        info.$image.attr('src', info.path);
      } else {
        info.loaded = true;
        setReadyIfLoaded();
      }
    }

    const diffContainerWidth = $container.width() - 300;

    function setReadyIfLoaded() {
      if (imageInfos[0].loaded && imageInfos[1].loaded) {
        initViews(imageInfos[0].$image, imageInfos[1].$image);
      }
    }

    function initViews($imageAfter, $imageBefore) {
      initSideBySide(createContext($imageAfter[0], $imageBefore[0]));
      if ($imageAfter.length > 0 && $imageBefore.length > 0) {
        initSwipe(createContext($imageAfter[1], $imageBefore[1]));
        initOverlay(createContext($imageAfter[2], $imageBefore[2]));
      }

      $container.find('> .loader').hide();
      $container.find('> .hide').removeClass('hide');
    }

    function initSideBySide(sizes) {
      let factor = 1;
      if (sizes.max.width > (diffContainerWidth - 24) / 2) {
        factor = (diffContainerWidth - 24) / 2 / sizes.max.width;
      }

      sizes.image1.css({
        width: sizes.size1.width * factor,
        height: sizes.size1.height * factor
      });
      sizes.image1.parent().css({
        margin: `${sizes.ratio[1] * factor + 15}px ${sizes.ratio[0] * factor}px ${sizes.ratio[1] * factor}px`,
        width: sizes.size1.width * factor + 2,
        height: sizes.size1.height * factor + 2
      });
      sizes.image2.css({
        width: sizes.size2.width * factor,
        height: sizes.size2.height * factor
      });
      sizes.image2.parent().css({
        margin: `${sizes.ratio[3] * factor}px ${sizes.ratio[2] * factor}px`,
        width: sizes.size2.width * factor + 2,
        height: sizes.size2.height * factor + 2
      });
    }

    function initSwipe(sizes) {
      let factor = 1;
      if (sizes.max.width > diffContainerWidth - 12) {
        factor = (diffContainerWidth - 12) / sizes.max.width;
      }

      sizes.image1.css({
        width: sizes.size1.width * factor,
        height: sizes.size1.height * factor
      });
      sizes.image1.parent().css({
        margin: `0px ${sizes.ratio[0] * factor}px`,
        width: sizes.size1.width * factor + 2,
        height: sizes.size1.height * factor + 2
      });
      sizes.image1.parent().parent().css({
        padding: `${sizes.ratio[1] * factor}px 0 0 0`,
        width: sizes.max.width * factor + 2
      });
      sizes.image2.css({
        width: sizes.size2.width * factor,
        height: sizes.size2.height * factor
      });
      sizes.image2.parent().css({
        margin: `${sizes.ratio[3] * factor}px ${sizes.ratio[2] * factor}px`,
        width: sizes.size2.width * factor + 2,
        height: sizes.size2.height * factor + 2
      });
      sizes.image2.parent().parent().css({
        width: sizes.max.width * factor + 2,
        height: sizes.max.height * factor + 2
      });
      $container.find('.diff-swipe').css({
        width: sizes.max.width * factor + 2,
        height: sizes.max.height * factor + 4
      });
      $container.find('.swipe-bar').on('mousedown', function(e) {
        e.preventDefault();

        const $swipeBar = $(this);
        const $swipeFrame = $swipeBar.parent();
        const width = $swipeFrame.width() - $swipeBar.width() - 2;

        $(document).on('mousemove.diff-swipe', (e2) => {
          e2.preventDefault();

          const value = Math.max(0, Math.min(e2.clientX - $swipeFrame.offset().left, width));

          $swipeBar.css({
            left: value
          });
          $container.find('.swipe-container').css({
            width: $swipeFrame.width() - value
          });
          $(document).on('mouseup.diff-swipe', () => {
            $(document).off('.diff-swipe');
          });
        });
      });
    }

    function initOverlay(sizes) {
      let factor = 1;
      if (sizes.max.width > diffContainerWidth - 12) {
        factor = (diffContainerWidth - 12) / sizes.max.width;
      }

      sizes.image1.css({
        width: sizes.size1.width * factor,
        height: sizes.size1.height * factor
      });
      sizes.image2.css({
        width: sizes.size2.width * factor,
        height: sizes.size2.height * factor
      });
      sizes.image1.parent().css({
        margin: `${sizes.ratio[1] * factor}px ${sizes.ratio[0] * factor}px`,
        width: sizes.size1.width * factor + 2,
        height: sizes.size1.height * factor + 2
      });
      sizes.image2.parent().css({
        margin: `${sizes.ratio[3] * factor}px ${sizes.ratio[2] * factor}px`,
        width: sizes.size2.width * factor + 2,
        height: sizes.size2.height * factor + 2
      });
      sizes.image2.parent().parent().css({
        width: sizes.max.width * factor + 2,
        height: sizes.max.height * factor + 2
      });
      $container.find('.onion-skin').css({
        width: sizes.max.width * factor + 2,
        height: sizes.max.height * factor + 4
      });

      const $range = $container.find("input[type='range'");
      const onInput = () => sizes.image1.parent().css({
        opacity: $range.val() / 100
      });
      $range.on('input', onInput);
      onInput();
    }
  });
}
