/*!
 * # Fomantic-UI - Slider
 * http://github.com/fomantic/Fomantic-UI/
 *
 *
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */

;(function ( $, window, document, undefined ) {

"use strict";

window = (typeof window != 'undefined' && window.Math == Math)
  ? window
  : (typeof self != 'undefined' && self.Math == Math)
    ? self
    : Function('return this')()
;

$.fn.slider = function(parameters) {

  var
    $allModules    = $(this),
    $window        = $(window),

    moduleSelector = $allModules.selector || '',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),

    alphabet       = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'],

    SINGLE_STEP     = 1,
    BIG_STEP        = 2,
    NO_STEP         = 0,
    SINGLE_BACKSTEP = -1,
    BIG_BACKSTEP    = -2,

    // Used to manage document bound events.
    // Use this so that we can distinguish between which document events are bound to which range.
    currentRange    = 0,

    returnedValue
  ;

  $allModules
    .each(function() {

      var
        settings        = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.slider.settings, parameters)
          : $.extend({}, $.fn.slider.settings),

        className       = settings.className,
        metadata        = settings.metadata,
        namespace       = settings.namespace,
        error           = settings.error,
        keys            = settings.keys,
        interpretLabel  = settings.interpretLabel,

        isHover         = false,
        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        $module         = $(this),
        $currThumb,
        $thumb,
        $secondThumb,
        $track,
        $trackFill,
        $labels,

        element         = this,
        instance        = $module.data(moduleNamespace),

        documentEventID,

        value,
        position,
        secondPos,
        offset,
        precision,
        isTouch,
        gapRatio = 1,

        initialPosition,
        initialLoad,
        module
      ;

      module = {

        initialize: function() {
          module.debug('Initializing slider', settings);
          initialLoad = true;

          currentRange += 1;
          documentEventID = currentRange;

          isTouch = module.setup.testOutTouch();
          module.setup.layout();
          module.setup.labels();

          if(!module.is.disabled()) {
            module.bind.events();
          }

          module.read.metadata();
          module.read.settings();

          initialLoad = false;
          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance of slider', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },

        destroy: function() {
          module.verbose('Destroying previous slider for', $module);
          clearInterval(instance.interval);
          module.unbind.events();
          module.unbind.slidingEvents();
          $module.removeData(moduleNamespace);
          instance = undefined;
        },

        setup: {
          layout: function() {
            if( $module.attr('tabindex') === undefined) {
              $module.attr('tabindex', 0);
            }
            if($module.find('.inner').length == 0) {
              $module.append("<div class='inner'>"
                             + "<div class='track'></div>"
                             + "<div class='track-fill'></div>"
                             + "<div class='thumb'></div>"
                             + "</div>");
            }
            precision = module.get.precision();
            $thumb = $module.find('.thumb:not(.second)');
            $currThumb = $thumb;
            if(module.is.range()) {
              if($module.find('.thumb.second').length == 0) {
                $module.find('.inner').append("<div class='thumb second'></div>");
              }
              $secondThumb = $module.find('.thumb.second');
            }
            $track = $module.find('.track');
            $trackFill = $module.find('.track-fill');
            offset = $thumb.width() / 2;
          },
          labels: function() {
            if(module.is.labeled()) {
              $labels = $module.find('.labels:not(.auto)');
              if($labels.length != 0) {
                module.setup.customLabel();
              } else {
                module.setup.autoLabel();
              }

              if (settings.showLabelTicks) {
                $module.addClass(className.ticked)
              }
            }
          },
          testOutTouch: function() {
            try {
             document.createEvent('TouchEvent');
             return true;
            } catch (e) {
             return false;
            }
          },
          customLabel: function() {
            var
              $children   = $labels.find('.label'),
              numChildren = $children.length,
              min         = module.get.min(),
              max         = module.get.max(),
              ratio
            ;
            $children.each(function(index) {
              var
                $child = $(this),
                attrValue = $child.attr('data-value')
              ;
              if(attrValue) {
                attrValue = attrValue > max ? max : attrValue < min ? min : attrValue;
                ratio = (attrValue - min) / (max - min);
              } else {
                ratio = (index + 1) / (numChildren + 1);
              }
              module.update.labelPosition(ratio, $(this));
            });
          },
          autoLabel: function() {
            if(module.get.step() != 0) {
              $labels = $module.find('.labels');
              if($labels.length != 0) {
                $labels.empty();
              }
              else {
                $labels = $module.append('<ul class="auto labels"></ul>').find('.labels');
              }
              for(var i = 0, len = module.get.numLabels(); i <= len; i++) {
                var
                  labelText = module.get.label(i),
                  $label = (labelText !== "") 
                    ? !(i % module.get.gapRatio())
                      ? $('<li class="label">' + labelText + '</li>') 
                      : $('<li class="halftick label"></li>')
                    : null,
                  ratio  = i / len
                ;
                if($label) {
                  module.update.labelPosition(ratio, $label);
                  $labels.append($label);
                }
              }
            }
          }
        },

        bind: {
          events: function() {
            module.bind.globalKeyboardEvents();
            module.bind.keyboardEvents();
            module.bind.mouseEvents();
            if(module.is.touch()) {
              module.bind.touchEvents();
            }
            if (settings.autoAdjustLabels) {
              module.bind.windowEvents();
            }
          },
          keyboardEvents: function() {
            module.verbose('Binding keyboard events');
            $module.on('keydown' + eventNamespace, module.event.keydown);
          },
          globalKeyboardEvents: function() {
            $(document).on('keydown' + eventNamespace + documentEventID, module.event.activateFocus);
          },
          mouseEvents: function() {
            module.verbose('Binding mouse events');
            $module.find('.track, .thumb, .inner').on('mousedown' + eventNamespace, function(event) {
              event.stopImmediatePropagation();
              event.preventDefault();
              module.event.down(event);
            });
            $module.on('mousedown' + eventNamespace, module.event.down);
            $module.on('mouseenter' + eventNamespace, function(event) {
              isHover = true;
            });
            $module.on('mouseleave' + eventNamespace, function(event) {
              isHover = false;
            });
          },
          touchEvents: function() {
            module.verbose('Binding touch events');
            $module.find('.track, .thumb, .inner').on('touchstart' + eventNamespace, function(event) {
              event.stopImmediatePropagation();
              event.preventDefault();
              module.event.down(event);
            });
            $module.on('touchstart' + eventNamespace, module.event.down);
          },
          slidingEvents: function() {
            // these don't need the identifier because we only ever want one of them to be registered with document
            module.verbose('Binding page wide events while handle is being draged');
            if(module.is.touch()) {
              $(document).on('touchmove' + eventNamespace, module.event.move);
              $(document).on('touchend' + eventNamespace, module.event.up);
            }
            else {
              $(document).on('mousemove' + eventNamespace, module.event.move);
              $(document).on('mouseup' + eventNamespace, module.event.up);
            }
          },
          windowEvents: function() {
            $window.on('resize' + eventNamespace, module.event.resize);
          }
        },

        unbind: {
          events: function() {
            $module.find('.track, .thumb, .inner').off('mousedown' + eventNamespace);
            $module.find('.track, .thumb, .inner').off('touchstart' + eventNamespace);
            $module.off('mousedown' + eventNamespace);
            $module.off('mouseenter' + eventNamespace);
            $module.off('mouseleave' + eventNamespace);
            $module.off('touchstart' + eventNamespace);
            $module.off('keydown' + eventNamespace);
            $module.off('focusout' + eventNamespace);
            $(document).off('keydown' + eventNamespace + documentEventID, module.event.activateFocus);
            $window.off('resize' + eventNamespace);
          },
          slidingEvents: function() {
            if(module.is.touch()) {
              $(document).off('touchmove' + eventNamespace);
              $(document).off('touchend' + eventNamespace);
            } else {
              $(document).off('mousemove' + eventNamespace);
              $(document).off('mouseup' + eventNamespace);
            }
          },
        },

        event: {
          down: function(event) {
            event.preventDefault();
            if(module.is.range()) {
              var
                eventPos = module.determine.eventPos(event),
                newPos = module.determine.pos(eventPos)
              ;
              // Special handling if range mode and both thumbs have the same value
              if(settings.preventCrossover && module.is.range() && module.thumbVal === module.secondThumbVal) {
                initialPosition = newPos;
                $currThumb = undefined;
              } else {
                $currThumb = module.determine.closestThumb(newPos);
              }
            }
            if(!module.is.disabled()) {
              module.bind.slidingEvents();
            }
          },
          move: function(event) {
            event.preventDefault();
            var value = module.determine.valueFromEvent(event);
            if($currThumb === undefined) {
              var
                eventPos = module.determine.eventPos(event),
                newPos = module.determine.pos(eventPos)
              ;
              $currThumb = initialPosition > newPos ? $thumb : $secondThumb;
            }
            if(module.get.step() == 0 || module.is.smooth()) {
              var
                thumbVal = module.thumbVal,
                secondThumbVal = module.secondThumbVal,
                thumbSmoothVal = module.determine.smoothValueFromEvent(event)
              ;
              if(!$currThumb.hasClass('second')) {
                if(settings.preventCrossover && module.is.range()) {
                  value = Math.min(secondThumbVal, value);
                  thumbSmoothVal = Math.min(secondThumbVal, thumbSmoothVal);
                }
                thumbVal = value;
              } else {
                if(settings.preventCrossover && module.is.range()) {
                  value = Math.max(thumbVal, value);
                  thumbSmoothVal = Math.max(thumbVal, thumbSmoothVal);
                }
                secondThumbVal = value;
              }
              value = Math.abs(thumbVal - (secondThumbVal || 0));
              module.update.position(thumbSmoothVal);
              settings.onMove.call(element, value, thumbVal, secondThumbVal);
            } else {
              module.update.value(value, function(value, thumbVal, secondThumbVal) {
                settings.onMove.call(element, value, thumbVal, secondThumbVal);
              });
            }
          },
          up: function(event) {
            event.preventDefault();
            var value = module.determine.valueFromEvent(event);
            module.set.value(value);
            module.unbind.slidingEvents();
          },
          keydown: function(event, first) {
            if(settings.preventCrossover && module.is.range() && module.thumbVal === module.secondThumbVal) {
              $currThumb = undefined;
            }
            if(module.is.focused()) {
              $(document).trigger(event);
            }
            if(first || module.is.focused()) {
              var step = module.determine.keyMovement(event);
              if(step != NO_STEP) {
                event.preventDefault();
                switch(step) {
                  case SINGLE_STEP:
                    module.takeStep();
                    break;
                  case BIG_STEP:
                    module.takeStep(module.get.multiplier());
                    break;
                  case SINGLE_BACKSTEP:
                    module.backStep();
                    break;
                  case BIG_BACKSTEP:
                    module.backStep(module.get.multiplier());
                    break;
                }
              }
            }
          },
          activateFocus: function(event) {
            if(!module.is.focused() && module.is.hover() && module.determine.keyMovement(event) != NO_STEP) {
              event.preventDefault();
              module.event.keydown(event, true);
              $module.focus();
            }
          },
          resize: function(_event) {
            // To avoid a useless performance cost, we only call the label refresh when its necessary
            if (gapRatio != module.get.gapRatio()) {
              module.setup.labels();
              gapRatio = module.get.gapRatio();
            }
          }
        },

        resync: function() {
          module.verbose('Resyncing thumb position based on value');
          if(module.is.range()) {
            module.update.position(module.secondThumbVal, $secondThumb);
          }
          module.update.position(module.thumbVal, $thumb);
          module.setup.labels();
        },
        takeStep: function(multiplier) {
          var
            multiplier = multiplier != undefined ? multiplier : 1,
            step = module.get.step(),
            currValue = module.get.currentThumbValue()
          ;
          module.verbose('Taking a step');
          if(step > 0) {
            module.set.value(currValue + step * multiplier);
          } else if (step == 0){
            var
              precision = module.get.precision(),
              newValue = currValue + (multiplier/precision)
            ;
            module.set.value(Math.round(newValue * precision) / precision);
          }
        },

        backStep: function(multiplier) {
          var
            multiplier = multiplier != undefined ? multiplier : 1,
            step = module.get.step(),
            currValue = module.get.currentThumbValue()
          ;
          module.verbose('Going back a step');
          if(step > 0) {
            module.set.value(currValue - step * multiplier);
          } else if (step == 0) {
            var
              precision = module.get.precision(),
              newValue = currValue - (multiplier/precision)
            ;
            module.set.value(Math.round(newValue * precision) / precision);
          }
        },

        is: {
          range: function() {
            return $module.hasClass(settings.className.range);
          },
          hover: function() {
            return isHover;
          },
          focused: function() {
            return $module.is(':focus');
          },
          disabled: function() {
            return $module.hasClass(settings.className.disabled);
          },
          labeled: function() {
            return $module.hasClass(settings.className.labeled);
          },
          reversed: function() {
            return $module.hasClass(settings.className.reversed);
          },
          vertical: function() {
            return $module.hasClass(settings.className.vertical);
          },
          smooth: function() {
            return settings.smooth || $module.hasClass(settings.className.smooth);
          },
          touch: function() {
            return isTouch;
          }
        },

        get: {
          trackOffset: function() {
            if (module.is.vertical()) {
              return $track.offset().top;
            } else {
              return $track.offset().left;
            }
          },
          trackLength: function() {
            if (module.is.vertical()) {
              return $track.height();
            } else {
              return $track.width();
            }
          },
          trackLeft: function() {
            if (module.is.vertical()) {
              return $track.position().top;
            } else {
              return $track.position().left;
            }
          },
          trackStartPos: function() {
            return module.is.reversed() ? module.get.trackLeft() + module.get.trackLength() : module.get.trackLeft();
          },
          trackEndPos: function() {
            return module.is.reversed() ? module.get.trackLeft() : module.get.trackLeft() + module.get.trackLength();
          },
          trackStartMargin: function () {
            var margin;
            if (module.is.vertical()) {
              margin = module.is.reversed() ? $module.css('padding-bottom') : $module.css('padding-top');
            } else {
              margin = module.is.reversed() ? $module.css('padding-right') : $module.css('padding-left');
            }
            return margin || '0px';
          },
          trackEndMargin: function () {
            var margin;
            if (module.is.vertical()) {
              margin = module.is.reversed() ? $module.css('padding-top') : $module.css('padding-bottom');
            } else {
              margin = module.is.reversed() ? $module.css('padding-left') : $module.css('padding-right');
            }
            return margin || '0px';
          },
          precision: function() {
            var
              decimalPlaces,
              step = module.get.step()
            ;
            if(step != 0) {
              var split = String(step).split('.');
              if(split.length == 2) {
                decimalPlaces = split[1].length;
              } else {
                decimalPlaces = 0;
              }
            } else {
              decimalPlaces = settings.decimalPlaces;
            }
            var precision = Math.pow(10, decimalPlaces);
            module.debug('Precision determined', precision);
            return precision;
          },
          min: function() {
            return settings.min;
          },
          max: function() {
            var step = module.get.step(),
                min = module.get.min(),
                quotient = step === 0 ? 0 : Math.floor((settings.max - min) / step),
                remainder = step === 0 ? 0 : (settings.max - min) % step;
            return remainder === 0 ? settings.max : min + quotient * step;
          },
          step: function() {
            return settings.step;
          },
          numLabels: function() {
            var value = Math.round((module.get.max() - module.get.min()) / module.get.step());
            module.debug('Determined that there should be ' + value + ' labels');
            return value;
          },
          labelType: function() {
            return settings.labelType;
          },
          label: function(value) {
            if(interpretLabel) {
              return interpretLabel(value);
            }

            switch (settings.labelType) {
              case settings.labelTypes.number:
                return Math.round(((value * module.get.step()) + module.get.min()) * precision ) / precision;
              case settings.labelTypes.letter:
                return alphabet[(value) % 26];
              default:
                return value;
            }
          },
          value: function() {
            return value;
          },
          currentThumbValue: function() {
            return $currThumb !== undefined && $currThumb.hasClass('second') ? module.secondThumbVal : module.thumbVal;
          },
          thumbValue: function(which) {
            switch(which) {
              case 'second':
                if(module.is.range()) {
                  return module.secondThumbVal;
                }
                else {
                  module.error(error.notrange);
                  break;
                }
              case 'first':
              default:
                return module.thumbVal;
            }
          },
          multiplier: function() {
            return settings.pageMultiplier;
          },
          thumbPosition: function(which) {
            switch(which) {
              case 'second':
                if(module.is.range()) {
                  return secondPos;
                }
                else {
                  module.error(error.notrange);
                  break;
                }
              case 'first':
              default:
                return position;
            }
          },
          gapRatio: function() {
            var gapRatio = 1;
            
            if( settings.autoAdjustLabels ) {
              var 
                numLabels = module.get.numLabels(),
                trackLength = module.get.trackLength(),
                gapCounter = 1
              ;

              // While the distance between two labels is too short,
              // we divide the number of labels at each iteration
              // and apply only if the modulo of the operation is an odd number.
              if(trackLength>0){
                while ((trackLength / numLabels) * gapCounter < settings.labelDistance) {
                  if( !(numLabels % gapCounter) ) {
                    gapRatio = gapCounter;
                  }
                  gapCounter += 1;
                }
              }
            }
            return gapRatio;
          }
        },

        determine: {
          pos: function(pagePos) {
            return module.is.reversed()
              ?
              module.get.trackStartPos() - pagePos + module.get.trackOffset()
              :
              pagePos - module.get.trackOffset() - module.get.trackStartPos()
            ;
          },
          closestThumb: function(eventPos) {
            var
              thumbPos = parseFloat(module.determine.thumbPos($thumb)),
              thumbDelta = Math.abs(eventPos - thumbPos),
              secondThumbPos = parseFloat(module.determine.thumbPos($secondThumb)),
              secondThumbDelta = Math.abs(eventPos - secondThumbPos)
            ;
            if(thumbDelta === secondThumbDelta && module.get.thumbValue() === module.get.min()) {
              return $secondThumb;
            }
            return thumbDelta <= secondThumbDelta ? $thumb : $secondThumb;
          },
          closestThumbPos: function(eventPos) {
            var
              thumbPos = parseFloat(module.determine.thumbPos($thumb)),
              thumbDelta = Math.abs(eventPos - thumbPos),
              secondThumbPos = parseFloat(module.determine.thumbPos($secondThumb)),
              secondThumbDelta = Math.abs(eventPos - secondThumbPos)
            ;
            return thumbDelta <= secondThumbDelta ? thumbPos : secondThumbPos;
          },
          thumbPos: function($element) {
            var pos =
              module.is.vertical()
              ?
              module.is.reversed() ? $element.css('bottom') : $element.css('top')
              :
              module.is.reversed() ? $element.css('right') : $element.css('left')
            ;
            return pos;
          },
          positionFromValue: function(value) {
            var
              min = module.get.min(),
              max = module.get.max(),
              value = value > max ? max : value < min ? min : value,
              trackLength = module.get.trackLength(),
              ratio = (value - min) / (max - min),
              position = Math.round(ratio * trackLength)
            ;
            module.verbose('Determined position: ' + position + ' from value: ' + value);
            return position;
          },
          positionFromRatio: function(ratio) {
            var
              trackLength = module.get.trackLength(),
              step = module.get.step(),
              position = Math.round(ratio * trackLength),
              adjustedPos = (step == 0) ? position : Math.round(position / step) * step
            ;
            return adjustedPos;
          },
          valueFromEvent: function(event) {
            var
              eventPos = module.determine.eventPos(event),
              newPos = module.determine.pos(eventPos),
              value
            ;
            if(eventPos < module.get.trackOffset()) {
              value = module.is.reversed() ? module.get.max() : module.get.min();
            } else if(eventPos > module.get.trackOffset() + module.get.trackLength()) {
              value = module.is.reversed() ? module.get.min() : module.get.max();
            } else {
              value = module.determine.value(newPos);
            }
            return value;
          },
          smoothValueFromEvent: function(event) {
            var
              min = module.get.min(),
              max = module.get.max(),
              trackLength = module.get.trackLength(),
              eventPos = module.determine.eventPos(event),
              newPos = eventPos - module.get.trackOffset(),
              ratio,
              value
            ;
            newPos = newPos < 0 ? 0 : newPos > trackLength ? trackLength : newPos;
            ratio = newPos / trackLength;
            if (module.is.reversed()) {
              ratio = 1 - ratio;
            }
            value = ratio * (max - min) + min;
            return value;
          },
          eventPos: function(event) {
            if(module.is.touch()) {
              var
                touchEvent = event.changedTouches ? event : event.originalEvent,
                touches = touchEvent.changedTouches[0] ? touchEvent.changedTouches : touchEvent.touches,
                touchY = touches[0].pageY,
                touchX = touches[0].pageX
              ;
              return module.is.vertical() ? touchY : touchX;
            }
            var
              clickY = event.pageY || event.originalEvent.pageY,
              clickX = event.pageX || event.originalEvent.pageX
            ;
            return module.is.vertical() ? clickY : clickX;
          },
          value: function(position) {
            var
              startPos = module.is.reversed() ? module.get.trackEndPos() : module.get.trackStartPos(),
              endPos = module.is.reversed() ? module.get.trackStartPos() : module.get.trackEndPos(),
              ratio = (position - startPos) / (endPos - startPos),
              range = module.get.max() - module.get.min(),
              step = module.get.step(),
              value = (ratio * range),
              difference = (step == 0) ? value : Math.round(value / step) * step
            ;
            module.verbose('Determined value based upon position: ' + position + ' as: ' + value);
            if(value != difference) {
              module.verbose('Rounding value to closest step: ' + difference);
            }
            // Use precision to avoid ugly Javascript floating point rounding issues
            // (like 35 * .01 = 0.35000000000000003)
            difference = Math.round(difference * precision) / precision;
            module.verbose('Cutting off additional decimal places');
            return difference + module.get.min();
          },
          keyMovement: function(event) {
            var
              key = event.which,
              downArrow =
                module.is.vertical()
                ?
                module.is.reversed() ? keys.downArrow : keys.upArrow
                :
                keys.downArrow
              ,
              upArrow =
                module.is.vertical()
                ?
                module.is.reversed() ? keys.upArrow : keys.downArrow
                :
                keys.upArrow
              ,
              leftArrow =
                !module.is.vertical()
                ?
                module.is.reversed() ? keys.rightArrow : keys.leftArrow
                :
                keys.leftArrow
              ,
              rightArrow =
                !module.is.vertical()
                ?
                module.is.reversed() ? keys.leftArrow : keys.rightArrow
                :
                keys.rightArrow
            ;
            if(key == downArrow || key == leftArrow) {
              return SINGLE_BACKSTEP;
            } else if(key == upArrow || key == rightArrow) {
              return SINGLE_STEP;
            } else if (key == keys.pageDown) {
              return BIG_BACKSTEP;
            } else if (key == keys.pageUp) {
              return BIG_STEP;
            } else {
              return NO_STEP;
            }
          }
        },

        handleNewValuePosition: function(val) {
          var
            min = module.get.min(),
            max = module.get.max(),
            newPos
          ;
          if (val <= min) {
            val = min;
          } else if (val >= max) {
            val = max;
          }
          newPos = module.determine.positionFromValue(val);
          return newPos;
        },

        set: {
          value: function(newValue) {
            module.update.value(newValue, function(value, thumbVal, secondThumbVal) {
              if (!initialLoad || settings.fireOnInit){
                settings.onChange.call(element, value, thumbVal, secondThumbVal);
                settings.onMove.call(element, value, thumbVal, secondThumbVal);
              }
            });
          },
          rangeValue: function(first, second) {
            if(module.is.range()) {
              var
                min = module.get.min(),
                max = module.get.max()
              ;
              if (first <= min) {
                first = min;
              } else if(first >= max){
                first = max;
              }
              if (second <= min) {
                second = min;
              } else if(second >= max){
                second = max;
              }
              module.thumbVal = first;
              module.secondThumbVal = second;
              value = Math.abs(module.thumbVal - module.secondThumbVal);
              module.update.position(module.thumbVal, $thumb);
              module.update.position(module.secondThumbVal, $secondThumb);
              if (!initialLoad || settings.fireOnInit) {
                settings.onChange.call(element, value, module.thumbVal, module.secondThumbVal);
                settings.onMove.call(element, value, module.thumbVal, module.secondThumbVal);
              }
            } else {
              module.error(error.notrange);
            }
          },
          position: function(position, which) {
            var thumbVal = module.determine.value(position);
            switch (which) {
              case 'second':
                module.secondThumbVal = thumbVal;
                module.update.position(thumbVal, $secondThumb);
                break;
              default:
                module.thumbVal = thumbVal;
                module.update.position(thumbVal, $thumb);
            }
            value = Math.abs(module.thumbVal - (module.secondThumbVal || 0));
            module.set.value(value);
          }
        },

        update: {
          value: function(newValue, callback) {
            var
              min = module.get.min(),
              max = module.get.max()
            ;
            if (newValue <= min) {
              newValue = min;
            } else if(newValue >= max){
              newValue = max;
            }
            if(!module.is.range()) {
              value = newValue;
              module.thumbVal = value;
            } else {
              if($currThumb === undefined) {
                $currThumb = newValue <= module.get.currentThumbValue() ? $thumb : $secondThumb;
              }
              if(!$currThumb.hasClass('second')) {
                if(settings.preventCrossover && module.is.range()) {
                  newValue = Math.min(module.secondThumbVal, newValue);
                }
                module.thumbVal = newValue;
              } else {
                if(settings.preventCrossover && module.is.range()) {
                  newValue = Math.max(module.thumbVal, newValue);
                }
                module.secondThumbVal = newValue;
              }
              value = Math.abs(module.thumbVal - module.secondThumbVal);
            }
            module.update.position(newValue);
            module.debug('Setting slider value to ' + value);
            if(typeof callback === 'function') {
              callback(value, module.thumbVal, module.secondThumbVal);
            }
          },
          position: function(newValue, $element) {
            var
              newPos = module.handleNewValuePosition(newValue),
              $targetThumb = $element != undefined ? $element : $currThumb,
              thumbVal = module.thumbVal || module.get.min(),
              secondThumbVal = module.secondThumbVal || module.get.min()
            ;
            if(module.is.range()) {
              if(!$targetThumb.hasClass('second')) {
                position = newPos;
                thumbVal = newValue;
              } else {
                secondPos = newPos;
                secondThumbVal = newValue;
              }
            } else {
              position = newPos;
              thumbVal = newValue;
            }
            var
              trackPosValue,
              thumbPosValue,
              min = module.get.min(),
              max = module.get.max(),
              thumbPosPercent = 100 * (newValue - min) / (max - min),
              trackStartPosPercent = 100 * (Math.min(thumbVal, secondThumbVal) - min) / (max - min),
              trackEndPosPercent = 100 * (1 - (Math.max(thumbVal, secondThumbVal) - min) / (max - min))
            ;
            if (module.is.vertical()) {
              if (module.is.reversed()) {
                thumbPosValue = {bottom: 'calc(' + thumbPosPercent + '% - ' + offset + 'px)', top: 'auto'};
                trackPosValue = {bottom: trackStartPosPercent + '%', top: trackEndPosPercent + '%'};
              }
              else {
                thumbPosValue = {top: 'calc(' + thumbPosPercent + '% - ' + offset + 'px)', bottom: 'auto'};
                trackPosValue = {top: trackStartPosPercent + '%', bottom: trackEndPosPercent + '%'};
              }
            } else {
              if (module.is.reversed()) {
                thumbPosValue = {right: 'calc(' + thumbPosPercent + '% - ' + offset + 'px)', left: 'auto'};
                trackPosValue = {right: trackStartPosPercent + '%', left: trackEndPosPercent + '%'};
              }
              else {
                thumbPosValue = {left: 'calc(' + thumbPosPercent + '% - ' + offset + 'px)', right: 'auto'};
                trackPosValue = {left: trackStartPosPercent + '%', right: trackEndPosPercent + '%'};
              }
            }
            $targetThumb.css(thumbPosValue);
            $trackFill.css(trackPosValue);
            module.debug('Setting slider position to ' + newPos);
          },
          labelPosition: function (ratio, $label) {
            var
              startMargin = module.get.trackStartMargin(),
              endMargin   = module.get.trackEndMargin(),
              posDir =
                module.is.vertical()
                ?
                module.is.reversed() ? 'bottom' : 'top'
                :
                  module.is.reversed() ? 'right' : 'left',
              startMarginMod = module.is.reversed() && !module.is.vertical() ? ' - ' : ' + '
            ;
            var position = '(100% - ' + startMargin + ' - ' + endMargin + ') * ' + ratio;
            $label.css(posDir, 'calc(' + position + startMarginMod + startMargin + ')');
          }
        },

        goto: {
          max: function() {
            module.set.value(module.get.max());
          },
          min: function() {
            module.set.value(module.get.min());
          },
        },

        read: {
          metadata: function() {
            var
              data = {
                thumbVal        : $module.data(metadata.thumbVal),
                secondThumbVal  : $module.data(metadata.secondThumbVal)
              }
            ;
            if(data.thumbVal) {
              if(module.is.range() && data.secondThumbVal) {
                module.debug('Current value set from metadata', data.thumbVal, data.secondThumbVal);
                module.set.rangeValue(data.thumbVal, data.secondThumbVal);
              } else {
                module.debug('Current value set from metadata', data.thumbVal);
                module.set.value(data.thumbVal);
              }
            }
          },
          settings: function() {
            if(settings.start !== false) {
              if(module.is.range()) {
                module.debug('Start position set from settings', settings.start, settings.end);
                module.set.rangeValue(settings.start, settings.end);
              } else {
                module.debug('Start position set from settings', settings.start);
                module.set.value(settings.start);
              }
            }
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
          }
          else {
            return settings[name];
          }
        },
        internal: function(name, value) {
          if( $.isPlainObject(name) ) {
            $.extend(true, module, name);
          }
          else if(value !== undefined) {
            module[name] = value;
          }
          else {
            return module[name];
          }
        },
        debug: function() {
          if(!settings.silent && settings.debug) {
            if(settings.performance) {
              module.performance.log(arguments);
            }
            else {
              module.debug = Function.prototype.bind.call(console.info, console, settings.name + ':');
              module.debug.apply(console, arguments);
            }
          }
        },
        verbose: function() {
          if(!settings.silent && settings.verbose && settings.debug) {
            if(settings.performance) {
              module.performance.log(arguments);
            }
            else {
              module.verbose = Function.prototype.bind.call(console.info, console, settings.name + ':');
              module.verbose.apply(console, arguments);
            }
          }
        },
        error: function() {
          if(!settings.silent) {
            module.error = Function.prototype.bind.call(console.error, console, settings.name + ':');
            module.error.apply(console, arguments);
          }
        },

        performance: {
          log: function(message) {
            var
              currentTime,
              executionTime,
              previousTime
            ;
            if(settings.performance) {
              currentTime   = new Date().getTime();
              previousTime  = time || currentTime;
              executionTime = currentTime - previousTime;
              time          = currentTime;
              performance.push({
                'Name'           : message[0],
                'Arguments'      : [].slice.call(message, 1) || '',
                'Element'        : element,
                'Execution Time' : executionTime
              });
            }
            clearTimeout(module.performance.timer);
            module.performance.timer = setTimeout(module.performance.display, 500);
          },
          display: function() {
            var
              title = settings.name + ':',
              totalTime = 0
            ;
            time = false;
            clearTimeout(module.performance.timer);
            $.each(performance, function(index, data) {
              totalTime += data['Execution Time'];
            });
            title += ' ' + totalTime + 'ms';
            if(moduleSelector) {
              title += ' \'' + moduleSelector + '\'';
            }
            if( (console.group !== undefined || console.table !== undefined) && performance.length > 0) {
              console.groupCollapsed(title);
              if(console.table) {
                console.table(performance);
              }
              else {
                $.each(performance, function(index, data) {
                  console.log(data['Name'] + ': ' + data['Execution Time']+'ms');
                });
              }
              console.groupEnd();
            }
            performance = [];
          }
        },

        invoke: function(query, passedArguments, context) {
          var
            object = instance,
            maxDepth,
            found,
            response
          ;
          passedArguments = passedArguments || queryArguments;
          context         = element         || context;
          if(typeof query == 'string' && object !== undefined) {
            query    = query.split(/[\. ]/);
            maxDepth = query.length - 1;
            $.each(query, function(depth, value) {
              var camelCaseValue = (depth != maxDepth)
                ? value + query[depth + 1].charAt(0).toUpperCase() + query[depth + 1].slice(1)
                : query
              ;
              if( $.isPlainObject( object[camelCaseValue] ) && (depth != maxDepth) ) {
                object = object[camelCaseValue];
              }
              else if( object[camelCaseValue] !== undefined ) {
                found = object[camelCaseValue];
                return false;
              }
              else if( $.isPlainObject( object[value] ) && (depth != maxDepth) ) {
                object = object[value];
              }
              else if( object[value] !== undefined ) {
                found = object[value];
                return false;
              }
              else {
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
            response = found.apply(context, passedArguments);
          }
          else if(found !== undefined) {
            response = found;
          }
          if($.isArray(returnedValue)) {
            returnedValue.push(response);
          }
          else if(returnedValue !== undefined) {
            returnedValue = [returnedValue, response];
          }
          else if(response !== undefined) {
            returnedValue = response;
          }
          return found;
        }
      };

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;

};

$.fn.slider.settings = {

  silent       : false,
  debug        : false,
  verbose      : false,
  performance  : true,

  name         : 'Slider',
  namespace    : 'slider',

  error    : {
    method    : 'The method you called is not defined.',
    notrange : 'This slider is not a range slider'
  },

  metadata: {
    thumbVal        : 'thumbVal',
    secondThumbVal  : 'secondThumbVal'
  },

  min              : 0,
  max              : 20,
  step             : 1,
  start            : 0,
  end              : 20,
  labelType        : 'number',
  showLabelTicks   : false,
  smooth           : false,
  autoAdjustLabels : true,
  labelDistance    : 100,
  preventCrossover : true,
  fireOnInit       : false,

  //the decimal place to round to if step is undefined
  decimalPlaces  : 2,

  // page up/down multiplier. How many more times the steps to take on page up/down press
  pageMultiplier : 2,

  selector: {

  },

  className     : {
    reversed : 'reversed',
    disabled : 'disabled',
    labeled  : 'labeled',
    ticked   : 'ticked',
    vertical : 'vertical',
    range    : 'range',
    smooth   : 'smooth'
  },

  keys : {
    pageUp     : 33,
    pageDown   : 34,
    leftArrow  : 37,
    upArrow    : 38,
    rightArrow : 39,
    downArrow  : 40
  },

  labelTypes    : {
    number  : 'number',
    letter  : 'letter'
  },

  onChange : function(value, thumbVal, secondThumbVal){},
  onMove   : function(value, thumbVal, secondThumbVal){},
};


})( jQuery, window, document );
