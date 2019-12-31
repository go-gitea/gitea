/*!
 * # Fomantic-UI - Progress
 * http://github.com/fomantic/Fomantic-UI/
 *
 *
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */

;(function ($, window, document, undefined) {

'use strict';

$.isFunction = $.isFunction || function(obj) {
  return typeof obj === "function" && typeof obj.nodeType !== "number";
};

window = (typeof window != 'undefined' && window.Math == Math)
  ? window
  : (typeof self != 'undefined' && self.Math == Math)
    ? self
    : Function('return this')()
;

$.fn.progress = function(parameters) {
  var
    $allModules    = $(this),

    moduleSelector = $allModules.selector || '',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),

    returnedValue
  ;

  $allModules
    .each(function() {
      var
        settings          = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.progress.settings, parameters)
          : $.extend({}, $.fn.progress.settings),

        className       = settings.className,
        metadata        = settings.metadata,
        namespace       = settings.namespace,
        selector        = settings.selector,
        error           = settings.error,

        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        $module         = $(this),
        $bars           = $(this).find(selector.bar),
        $progresses     = $(this).find(selector.progress),
        $label          = $(this).find(selector.label),

        element         = this,
        instance        = $module.data(moduleNamespace),

        animating = false,
        transitionEnd,
        module
      ;
      module = {
        helper: {
          sum: function (nums) {
            return Array.isArray(nums) ? nums.reduce(function (left, right) {
              return left + Number(right);
            }, 0) : 0;
          },
          /**
           * Derive precision for multiple progress with total and values.
           *
           * This helper dervices a precision that is sufficiently large to show minimum value of multiple progress.
           *
           * Example1
           * - total: 1122
           * - values: [325, 111, 74, 612]
           * - min ratio: 74/1122 = 0.0659...
           * - required precision:  100
           *
           * Example2
           * - total: 10541
           * - values: [3235, 1111, 74, 6121]
           * - min ratio: 74/10541 = 0.0070...
           * - required precision:   1000
           *
           * @param min A minimum value within multiple values
           * @param total A total amount of multiple values
           * @returns {number} A precison. Could be 1, 10, 100, ... 1e+10.
           */
          derivePrecision: function(min, total) {
            var precisionPower = 0
            var precision = 1;
            var ratio = min / total;
            while (precisionPower < 10) {
              ratio = ratio * precision;
              if (ratio > 1) {
                break;
              }
              precision = Math.pow(10, precisionPower++);
            }
            return precision;
          },
          forceArray: function (element) {
            return Array.isArray(element)
              ? element
              : !isNaN(element)
                ? [element]
                : typeof element == 'string'
                  ? element.split(',')
                  : []
              ;
          }
        },

        initialize: function() {
          module.set.duration();
          module.set.transitionEvent();
          module.debug(element);

          module.read.metadata();
          module.read.settings();

          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance of progress', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },
        destroy: function() {
          module.verbose('Destroying previous progress for', $module);
          clearInterval(instance.interval);
          module.remove.state();
          $module.removeData(moduleNamespace);
          instance = undefined;
        },

        reset: function() {
          module.remove.nextValue();
          module.update.progress(0);
        },

        complete: function(keepState) {
          if(module.percent === undefined || module.percent < 100) {
            module.remove.progressPoll();
            if(keepState !== true){
                module.set.percent(100);
            }
          }
        },

        read: {
          metadata: function() {
            var
              data = {
                percent : module.helper.forceArray($module.data(metadata.percent)),
                total   : $module.data(metadata.total),
                value   : module.helper.forceArray($module.data(metadata.value))
              }
            ;
            if(data.total) {
              module.debug('Total value set from metadata', data.total);
              module.set.total(data.total);
            }
            if(data.value.length > 0) {
              module.debug('Current value set from metadata', data.value);
              module.set.value(data.value);
              module.set.progress(data.value);
            }
            if(data.percent.length > 0) {
              module.debug('Current percent value set from metadata', data.percent);
              module.set.percent(data.percent);
            }
          },
          settings: function() {
            if(settings.total !== false) {
              module.debug('Current total set in settings', settings.total);
              module.set.total(settings.total);
            }
            if(settings.value !== false) {
              module.debug('Current value set in settings', settings.value);
              module.set.value(settings.value);
              module.set.progress(module.value);
            }
            if(settings.percent !== false) {
              module.debug('Current percent set in settings', settings.percent);
              module.set.percent(settings.percent);
            }
          }
        },

        bind: {
          transitionEnd: function(callback) {
            var
              transitionEnd = module.get.transitionEnd()
            ;
            $bars
              .one(transitionEnd + eventNamespace, function(event) {
                clearTimeout(module.failSafeTimer);
                callback.call(this, event);
              })
            ;
            module.failSafeTimer = setTimeout(function() {
              $bars.triggerHandler(transitionEnd);
            }, settings.duration + settings.failSafeDelay);
            module.verbose('Adding fail safe timer', module.timer);
          }
        },

        increment: function(incrementValue) {
          var
            startValue,
            newValue
          ;
          if( module.has.total() ) {
            startValue     = module.get.value();
            incrementValue = incrementValue || 1;
          }
          else {
            startValue     = module.get.percent();
            incrementValue = incrementValue || module.get.randomValue();
          }
          newValue = startValue + incrementValue;
          module.debug('Incrementing percentage by', startValue, newValue, incrementValue);
          newValue = module.get.normalizedValue(newValue);
          module.set.progress(newValue);
        },
        decrement: function(decrementValue) {
          var
            total     = module.get.total(),
            startValue,
            newValue
          ;
          if(total) {
            startValue     =  module.get.value();
            decrementValue =  decrementValue || 1;
            newValue       =  startValue - decrementValue;
            module.debug('Decrementing value by', decrementValue, startValue);
          }
          else {
            startValue     =  module.get.percent();
            decrementValue =  decrementValue || module.get.randomValue();
            newValue       =  startValue - decrementValue;
            module.debug('Decrementing percentage by', decrementValue, startValue);
          }
          newValue = module.get.normalizedValue(newValue);
          module.set.progress(newValue);
        },

        has: {
          progressPoll: function() {
            return module.progressPoll;
          },
          total: function() {
            return (module.get.total() !== false);
          }
        },

        get: {
          text: function(templateText, index) {
            var
              index_  = index || 0,
              value   = module.get.value(index_),
              total   = module.total || 0,
              percent = (animating)
                ? module.get.displayPercent(index_)
                : module.get.percent(index_),
              left = (module.total > 0)
                ? (total - value)
                : (100 - percent)
            ;
            templateText = templateText || '';
            templateText = templateText
              .replace('{value}', value)
              .replace('{total}', total)
              .replace('{left}', left)
              .replace('{percent}', percent)
              .replace('{bar}', settings.text.bars[index_] || '')
            ;
            module.verbose('Adding variables to progress bar text', templateText);
            return templateText;
          },

          normalizedValue: function(value) {
            if(value < 0) {
              module.debug('Value cannot decrement below 0');
              return 0;
            }
            if(module.has.total()) {
              if(value > module.total) {
                module.debug('Value cannot increment above total', module.total);
                return module.total;
              }
            }
            else if(value > 100 ) {
              module.debug('Value cannot increment above 100 percent');
              return 100;
            }
            return value;
          },

          updateInterval: function() {
            if(settings.updateInterval == 'auto') {
              return settings.duration;
            }
            return settings.updateInterval;
          },

          randomValue: function() {
            module.debug('Generating random increment percentage');
            return Math.floor((Math.random() * settings.random.max) + settings.random.min);
          },

          numericValue: function(value) {
            return (typeof value === 'string')
              ? (value.replace(/[^\d.]/g, '') !== '')
                ? +(value.replace(/[^\d.]/g, ''))
                : false
              : value
            ;
          },

          transitionEnd: function() {
            var
              element     = document.createElement('element'),
              transitions = {
                'transition'       :'transitionend',
                'OTransition'      :'oTransitionEnd',
                'MozTransition'    :'transitionend',
                'WebkitTransition' :'webkitTransitionEnd'
              },
              transition
            ;
            for(transition in transitions){
              if( element.style[transition] !== undefined ){
                return transitions[transition];
              }
            }
          },

          // gets current displayed percentage (if animating values this is the intermediary value)
          displayPercent: function(index) {
            var
              $bar           = $($bars[index]),
              barWidth       = $bar.width(),
              totalWidth     = $module.width(),
              minDisplay     = parseInt($bar.css('min-width'), 10),
              displayPercent = (barWidth > minDisplay)
                ? (barWidth / totalWidth * 100)
                : module.percent
            ;
            return (settings.precision > 0)
              ? Math.round(displayPercent * (10 * settings.precision)) / (10 * settings.precision)
              : Math.round(displayPercent)
              ;
          },

          percent: function(index) {
            return module.percent && module.percent[index || 0] || 0;
          },
          value: function(index) {
            return module.nextValue || module.value && module.value[index || 0] || 0;
          },
          total: function() {
            return module.total || false;
          }
        },

        create: {
          progressPoll: function() {
            module.progressPoll = setTimeout(function() {
              module.update.toNextValue();
              module.remove.progressPoll();
            }, module.get.updateInterval());
          },
        },

        is: {
          complete: function() {
            return module.is.success() || module.is.warning() || module.is.error();
          },
          success: function() {
            return $module.hasClass(className.success);
          },
          warning: function() {
            return $module.hasClass(className.warning);
          },
          error: function() {
            return $module.hasClass(className.error);
          },
          active: function() {
            return $module.hasClass(className.active);
          },
          visible: function() {
            return $module.is(':visible');
          }
        },

        remove: {
          progressPoll: function() {
            module.verbose('Removing progress poll timer');
            if(module.progressPoll) {
              clearTimeout(module.progressPoll);
              delete module.progressPoll;
            }
          },
          nextValue: function() {
            module.verbose('Removing progress value stored for next update');
            delete module.nextValue;
          },
          state: function() {
            module.verbose('Removing stored state');
            delete module.total;
            delete module.percent;
            delete module.value;
          },
          active: function() {
            module.verbose('Removing active state');
            $module.removeClass(className.active);
          },
          success: function() {
            module.verbose('Removing success state');
            $module.removeClass(className.success);
          },
          warning: function() {
            module.verbose('Removing warning state');
            $module.removeClass(className.warning);
          },
          error: function() {
            module.verbose('Removing error state');
            $module.removeClass(className.error);
          }
        },

        set: {
          barWidth: function(values) {
            module.debug("set bar width with ", values);
            values = module.helper.forceArray(values);
            var firstNonZeroIndex = -1;
            var lastNonZeroIndex = -1;
            var valuesSum = module.helper.sum(values);
            var barCounts = $bars.length;
            var isMultiple = barCounts > 1;
            var percents = values.map(function(value, index) {
              var allZero = (index === barCounts - 1 && valuesSum === 0);
              var $bar = $($bars[index]);
              if (value === 0 && isMultiple && !allZero) {
                $bar.css('display', 'none');
              } else {
                if (isMultiple && allZero) {
                  $bar.css('background', 'transparent');
                }
                if (firstNonZeroIndex == -1) {
                  firstNonZeroIndex = index;
                }
                lastNonZeroIndex = index;
                $bar.css({
                  display: 'block',
                  width: value + '%'
                });
              }
              return parseFloat(value);
            });
            values.forEach(function(_, index) {
              var $bar = $($bars[index]);
              $bar.css({
                borderTopLeftRadius: index == firstNonZeroIndex ? '' : 0,
                borderBottomLeftRadius: index == firstNonZeroIndex ? '' : 0,
                borderTopRightRadius: index == lastNonZeroIndex ? '' : 0,
                borderBottomRightRadius: index == lastNonZeroIndex ? '' : 0
              });
            });
            $module
              .attr('data-percent', percents)
            ;
          },
          duration: function(duration) {
            duration = duration || settings.duration;
            duration = (typeof duration == 'number')
              ? duration + 'ms'
              : duration
            ;
            module.verbose('Setting progress bar transition duration', duration);
            $bars
              .css({
                'transition-duration':  duration
              })
            ;
          },
          percent: function(percents) {
            percents = module.helper.forceArray(percents).map(function(percent) {
              return (typeof percent == 'string')
                ? +(percent.replace('%', ''))
                : percent
                ;
            });
            var hasTotal = module.has.total();
            var totalPecent = module.helper.sum(percents);
            var isMultpleValues = percents.length > 1 && hasTotal;
            var sumTotal = module.helper.sum(module.helper.forceArray(module.value));
            if (isMultpleValues && sumTotal > module.total) {
              // Sum values instead of pecents to avoid precision issues when summing floats
              module.error(error.sumExceedsTotal, sumTotal, module.total);
            } else if (!isMultpleValues && totalPecent > 100) {
              // Sum before rouding since sum of rounded may have error though sum of actual is fine
              module.error(error.tooHigh, totalPecent);
            } else if (totalPecent < 0) {
              module.error(error.tooLow, totalPecent);
            } else {
              var autoPrecision = settings.precision > 0
                ? settings.precision
                : isMultpleValues
                  ? module.helper.derivePrecision(Math.min.apply(null, module.value), module.total)
                  : undefined;

              // round display percentage
              var roundedPercents = percents.map(function (percent) {
                return (autoPrecision > 0)
                  ? Math.round(percent * (10 * autoPrecision)) / (10 * autoPrecision)
                  : Math.round(percent)
                  ;
              });
              module.percent = roundedPercents;
              if (!hasTotal) {
                module.value = roundedPercents.map(function (percent) {
                  return (autoPrecision > 0)
                    ? Math.round((percent / 100) * module.total * (10 * autoPrecision)) / (10 * autoPrecision)
                    : Math.round((percent / 100) * module.total * 10) / 10
                    ;
                });
                if (settings.limitValues) {
                  module.value = module.value.map(function (value) {
                    return (value > 100)
                      ? 100
                      : (module.value < 0)
                        ? 0
                        : module.value;
                  });
                }
              }
              module.set.barWidth(percents);
              module.set.labelInterval();
              module.set.labels();
            }
            settings.onChange.call(element, percents, module.value, module.total);
          },
          labelInterval: function() {
            var
              animationCallback = function() {
                module.verbose('Bar finished animating, removing continuous label updates');
                clearInterval(module.interval);
                animating = false;
                module.set.labels();
              }
            ;
            clearInterval(module.interval);
            module.bind.transitionEnd(animationCallback);
            animating = true;
            module.interval = setInterval(function() {
              var
                isInDOM = $.contains(document.documentElement, element)
              ;
              if(!isInDOM) {
                clearInterval(module.interval);
                animating = false;
              }
              module.set.labels();
            }, settings.framerate);
          },
          labels: function() {
            module.verbose('Setting both bar progress and outer label text');
            module.set.barLabel();
            module.set.state();
          },
          label: function(text) {
            text = text || '';
            if(text) {
              text = module.get.text(text);
              module.verbose('Setting label to text', text);
              $label.text(text);
            }
          },
          state: function(percent) {
            percent = (percent !== undefined)
              ? percent
              : module.helper.sum(module.percent)
            ;
            if(percent === 100) {
              if(settings.autoSuccess && $bars.length === 1 && !(module.is.warning() || module.is.error() || module.is.success())) {
                module.set.success();
                module.debug('Automatically triggering success at 100%');
              }
              else {
                module.verbose('Reached 100% removing active state');
                module.remove.active();
                module.remove.progressPoll();
              }
            }
            else if(percent > 0) {
              module.verbose('Adjusting active progress bar label', percent);
              module.set.active();
            }
            else {
              module.remove.active();
              module.set.label(settings.text.active);
            }
          },
          barLabel: function(text) {
            $progresses.map(function(index, element){
              var $progress = $(element);
              if (text !== undefined) {
                $progress.text( module.get.text(text, index) );
              }
              else if (settings.label == 'ratio' && module.total) {
                module.verbose('Adding ratio to bar label');
                $progress.text( module.get.text(settings.text.ratio, index) );
              }
              else if (settings.label == 'percent') {
                module.verbose('Adding percentage to bar label');
                $progress.text( module.get.text(settings.text.percent, index) );
              }
            });
          },
          active: function(text) {
            text = text || settings.text.active;
            module.debug('Setting active state');
            if(settings.showActivity && !module.is.active() ) {
              $module.addClass(className.active);
            }
            module.remove.warning();
            module.remove.error();
            module.remove.success();
            text = settings.onLabelUpdate('active', text, module.value, module.total);
            if(text) {
              module.set.label(text);
            }
            module.bind.transitionEnd(function() {
              settings.onActive.call(element, module.value, module.total);
            });
          },
          success : function(text, keepState) {
            text = text || settings.text.success || settings.text.active;
            module.debug('Setting success state');
            $module.addClass(className.success);
            module.remove.active();
            module.remove.warning();
            module.remove.error();
            module.complete(keepState);
            if(settings.text.success) {
              text = settings.onLabelUpdate('success', text, module.value, module.total);
              module.set.label(text);
            }
            else {
              text = settings.onLabelUpdate('active', text, module.value, module.total);
              module.set.label(text);
            }
            module.bind.transitionEnd(function() {
              settings.onSuccess.call(element, module.total);
            });
          },
          warning : function(text, keepState) {
            text = text || settings.text.warning;
            module.debug('Setting warning state');
            $module.addClass(className.warning);
            module.remove.active();
            module.remove.success();
            module.remove.error();
            module.complete(keepState);
            text = settings.onLabelUpdate('warning', text, module.value, module.total);
            if(text) {
              module.set.label(text);
            }
            module.bind.transitionEnd(function() {
              settings.onWarning.call(element, module.value, module.total);
            });
          },
          error : function(text, keepState) {
            text = text || settings.text.error;
            module.debug('Setting error state');
            $module.addClass(className.error);
            module.remove.active();
            module.remove.success();
            module.remove.warning();
            module.complete(keepState);
            text = settings.onLabelUpdate('error', text, module.value, module.total);
            if(text) {
              module.set.label(text);
            }
            module.bind.transitionEnd(function() {
              settings.onError.call(element, module.value, module.total);
            });
          },
          transitionEvent: function() {
            transitionEnd = module.get.transitionEnd();
          },
          total: function(totalValue) {
            module.total = totalValue;
          },
          value: function(value) {
            module.value = module.helper.forceArray(value);
          },
          progress: function(value) {
            if(!module.has.progressPoll()) {
              module.debug('First update in progress update interval, immediately updating', value);
              module.update.progress(value);
              module.create.progressPoll();
            }
            else {
              module.debug('Updated within interval, setting next update to use new value', value);
              module.set.nextValue(value);
            }
          },
          nextValue: function(value) {
            module.nextValue = value;
          }
        },

        update: {
          toNextValue: function() {
            var
              nextValue = module.nextValue
            ;
            if(nextValue) {
              module.debug('Update interval complete using last updated value', nextValue);
              module.update.progress(nextValue);
              module.remove.nextValue();
            }
          },
          progress: function(values) {
            var hasTotal = module.has.total();
            if (hasTotal) {
              module.set.value(values);
            }
            var percentCompletes = module.helper.forceArray(values).map(function(value) {
              var
                percentComplete
              ;
              value = module.get.numericValue(value);
              if (value === false) {
                module.error(error.nonNumeric, value);
              }
              value = module.get.normalizedValue(value);
              if (hasTotal) {
                percentComplete = (value / module.total) * 100;
                module.debug('Calculating percent complete from total', percentComplete);
              }
              else {
                percentComplete = value;
                module.debug('Setting value to exact percentage value', percentComplete);
              }
              return percentComplete;
            });
            module.set.percent( percentCompletes );
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
          if(Array.isArray(returnedValue)) {
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

$.fn.progress.settings = {

  name         : 'Progress',
  namespace    : 'progress',

  silent       : false,
  debug        : false,
  verbose      : false,
  performance  : true,

  random       : {
    min : 2,
    max : 5
  },

  duration       : 300,

  updateInterval : 'auto',

  autoSuccess    : true,
  showActivity   : true,
  limitValues    : true,

  label          : 'percent',
  precision      : 0,
  framerate      : (1000 / 30), /// 30 fps

  percent        : false,
  total          : false,
  value          : false,

  // delay in ms for fail safe animation callback
  failSafeDelay : 100,

  onLabelUpdate : function(state, text, value, total){
    return text;
  },
  onChange      : function(percent, value, total){},
  onSuccess     : function(total){},
  onActive      : function(value, total){},
  onError       : function(value, total){},
  onWarning     : function(value, total){},

  error    : {
    method          : 'The method you called is not defined.',
    nonNumeric      : 'Progress value is non numeric',
    tooHigh         : 'Value specified is above 100%',
    tooLow          : 'Value specified is below 0%',
    sumExceedsTotal : 'Sum of multple values exceed total',
  },

  regExp: {
    variable: /\{\$*[A-z0-9]+\}/g
  },

  metadata: {
    percent : 'percent',
    total   : 'total',
    value   : 'value'
  },

  selector : {
    bar      : '> .bar',
    label    : '> .label',
    progress : '.bar > .progress'
  },

  text : {
    active  : false,
    error   : false,
    success : false,
    warning : false,
    percent : '{percent}%',
    ratio   : '{value} of {total}',
    bars    : ['']
  },

  className : {
    active  : 'active',
    error   : 'error',
    success : 'success',
    warning : 'warning'
  }

};


})( jQuery, window, document );
