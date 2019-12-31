/*!
 * # Fomantic-UI - Toast
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

$.fn.toast = function(parameters) {
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
          ? $.extend(true, {}, $.fn.toast.settings, parameters)
          : $.extend({}, $.fn.toast.settings),

        className        = settings.className,
        selector         = settings.selector,
        error            = settings.error,
        namespace        = settings.namespace,
        fields           = settings.fields,

        eventNamespace   = '.' + namespace,
        moduleNamespace  = namespace + '-module',

        $module          = $(this),
        $toastBox,
        $toast,
        $actions,
        $progress,
        $progressBar,
        $animationObject,
        $close,
        $context         = (settings.context)
          ? $(settings.context)
          : $('body'),

        isToastComponent = $module.hasClass('toast') || $module.hasClass('message') || $module.hasClass('card'),

        element          = this,
        instance         = isToastComponent ? $module.data(moduleNamespace) : undefined,

        module
      ;
      module = {

        initialize: function() {
          module.verbose('Initializing element');
          if (!module.has.container()) {
            module.create.container();
          }
          if(isToastComponent || settings.message !== '' || settings.title !== '' || module.get.iconClass() !== '' || settings.showImage || module.has.configActions()) {
            if(typeof settings.showProgress !== 'string' || [className.top,className.bottom].indexOf(settings.showProgress) === -1 ) {
              settings.showProgress = false;
            }
            module.create.toast();
            if(settings.closeOnClick && (settings.closeIcon || $($toast).find(selector.input).length > 0 || module.has.configActions())){
              settings.closeOnClick = false;
            }
            if(!settings.closeOnClick) {
              $toastBox.addClass(className.unclickable);
            }
            module.bind.events();
          }
          module.instantiate();
          if($toastBox) {
            module.show();
          }
        },

        instantiate: function() {
          module.verbose('Storing instance of toast');
          instance = module;
          $module
              .data(moduleNamespace, instance)
          ;
        },

        destroy: function() {
          if($toastBox) {
            module.debug('Removing toast', $toastBox);
            module.unbind.events();
            $toastBox.remove();
            $toastBox = undefined;
            $toast = undefined;
            $animationObject = undefined;
            settings.onRemove.call($toastBox, element);
            $progress = undefined;
            $progressBar = undefined;
            $close = undefined;
          }
          $module
            .removeData(moduleNamespace)
          ;
        },

        show: function(callback) {
          callback = callback || function(){};
          module.debug('Showing toast');
          if(settings.onShow.call($toastBox, element) === false) {
            module.debug('onShow callback returned false, cancelling toast animation');
            return;
          }
          module.animate.show(callback);
        },

        close: function(callback) {
          callback = callback || function(){};
          module.remove.visible();
          module.unbind.events();
          module.animate.close(callback);

        },

        create: {
          container: function() {
            module.verbose('Creating container');
            $context.append($('<div/>',{class: settings.position + ' ' + className.container}));
          },
          toast: function() {
            $toastBox = $('<div/>', {class: className.box});
            if (!isToastComponent) {
              module.verbose('Creating toast');
              $toast = $('<div/>');
              var $content = $('<div/>', {class: className.content});
              var iconClass = module.get.iconClass();
              if (iconClass !== '') {
                $toast.append($('<i/>', {class: iconClass + ' ' + className.icon}));
              }

              if (settings.showImage) {
                $toast.append($('<img>', {
                  class: className.image + ' ' + settings.classImage,
                  src: settings.showImage
                }));
              }
              if (settings.title !== '') {
                $content.append($('<div/>', {
                  class: className.title,
                  text: settings.title
                }));
              }

              $content.append($('<div/>', {html: module.helpers.escape(settings.message, settings.preserveHTML)}));

              $toast
                .addClass(settings.class + ' ' + className.toast)
                .append($content)
              ;
              $toast.css('opacity', settings.opacity);
              if (settings.closeIcon) {
                $close = $('<i/>', {class: className.close + ' ' + (typeof settings.closeIcon === 'string' ? settings.closeIcon : '')});
                if($close.hasClass(className.left)) {
                  $toast.prepend($close);
                } else {
                  $toast.append($close);
                }
              }
            } else {
              $toast = settings.cloneModule ? $module.clone().removeAttr('id') : $module;
              $close = $toast.find('> i'+module.helpers.toClass(className.close));
              settings.closeIcon = ($close.length > 0);
            }
            if ($toast.hasClass(className.compact)) {
              settings.compact = true;
            }
            if ($toast.hasClass('card')) {
              settings.compact = false;
            }
            $actions = $toast.find('.actions');
            if (module.has.configActions()) {
              if ($actions.length === 0) {
                $actions = $('<div/>', {class: className.actions + ' ' + (settings.classActions || '')}).appendTo($toast);
              }
              if($toast.hasClass('card') && !$actions.hasClass(className.attached)) {
                $actions.addClass(className.extraContent);
                if($actions.hasClass(className.vertical)) {
                  $actions.removeClass(className.vertical);
                  module.error(error.verticalCard);
                }
              }
              settings.actions.forEach(function (el) {
                var icon = el[fields.icon] ? '<i class="' + module.helpers.deQuote(el[fields.icon]) + ' icon"></i>' : '',
                  text = module.helpers.escape(el[fields.text] || '', settings.preserveHTML),
                  cls = module.helpers.deQuote(el[fields.class] || ''),
                  click = el[fields.click] && $.isFunction(el[fields.click]) ? el[fields.click] : function () {};
                $actions.append($('<button/>', {
                  html: icon + text,
                  class: className.button + ' ' + cls,
                  click: function () {
                    if (click.call(element, $module) === false) {
                      return;
                    }
                    module.close();
                  }
                }));
              });
            }
            if ($actions && $actions.hasClass(className.vertical)) {
                $toast.addClass(className.vertical);
            }
            if($actions.length > 0 && !$actions.hasClass(className.attached)) {
              if ($actions && (!$actions.hasClass(className.basic) || $actions.hasClass(className.left))) {
                $toast.addClass(className.actions);
              }
            }
            if(settings.displayTime === 'auto'){
              settings.displayTime = Math.max(settings.minDisplayTime, $toast.text().split(" ").length / settings.wordsPerMinute * 60000);
            }
            $toastBox.append($toast);

            if($actions.length > 0 && $actions.hasClass(className.attached)) {
              $actions.addClass(className.buttons);
              $actions.detach();
              $toast.addClass(className.attached);
              if (!$actions.hasClass(className.vertical)) {
                if ($actions.hasClass(className.top)) {
                  $toastBox.prepend($actions);
                  $toast.addClass(className.bottom);
                } else {
                  $toastBox.append($actions);
                  $toast.addClass(className.top);
                }
              } else {
                $toast.wrap(
                  $('<div/>',{
                    class:className.vertical + ' ' +
                          className.attached + ' ' +
                          (settings.compact ? className.compact : '')
                  })
                );
                if($actions.hasClass(className.left)) {
                  $toast.addClass(className.left).parent().addClass(className.left).prepend($actions);
                } else {
                  $toast.parent().append($actions);
                }
              }
            }
            if($module !== $toast) {
              $module = $toast;
              element = $toast[0];
            }
            if(settings.displayTime > 0) {
              var progressingClass = className.progressing+' '+(settings.pauseOnHover ? className.pausable:'');
              if (!!settings.showProgress) {
                $progress = $('<div/>', {
                  class: className.progress + ' ' + (settings.classProgress || settings.class),
                  'data-percent': ''
                });
                if(!settings.classProgress) {
                  if ($toast.hasClass('toast') && !$toast.hasClass(className.inverted)) {
                    $progress.addClass(className.inverted);
                  } else {
                    $progress.removeClass(className.inverted);
                  }
                }
                $progressBar = $('<div/>', {class: 'bar '+(settings.progressUp ? 'up ' : 'down ')+progressingClass});
                $progress
                    .addClass(settings.showProgress)
                    .append($progressBar);
                if ($progress.hasClass(className.top)) {
                  $toastBox.prepend($progress);
                } else {
                  $toastBox.append($progress);
                }
                $progressBar.css('animation-duration', settings.displayTime / 1000 + 's');
              }
              $animationObject = $('<span/>',{class:'wait '+progressingClass});
              $animationObject.css('animation-duration', settings.displayTime / 1000 + 's');
              $animationObject.appendTo($toast);
            }
            if (settings.compact) {
              $toastBox.addClass(className.compact);
              $toast.addClass(className.compact);
              if($progress) {
                $progress.addClass(className.compact);
              }
            }
            if (settings.newestOnTop) {
              $toastBox.prependTo(module.get.container());
            }
            else {
              $toastBox.appendTo(module.get.container());
            }
          }
        },

        bind: {
          events: function() {
            module.debug('Binding events to toast');
            if(settings.closeOnClick || settings.closeIcon) {
              (settings.closeIcon ? $close : $toast)
                  .on('click' + eventNamespace, module.event.click)
              ;
            }
            if($animationObject) {
              $animationObject.on('animationend' + eventNamespace, module.close);
            }
            $toastBox
              .on('click' + eventNamespace, selector.approve, module.event.approve)
              .on('click' + eventNamespace, selector.deny, module.event.deny)
            ;
          }
        },

        unbind: {
          events: function() {
            module.debug('Unbinding events to toast');
            if(settings.closeOnClick || settings.closeIcon) {
              (settings.closeIcon ? $close : $toast)
                  .off('click' + eventNamespace)
              ;
            }
            if($animationObject) {
              $animationObject.off('animationend' + eventNamespace);
            }
            $toastBox
              .off('click' + eventNamespace)
            ;
          }
        },

        animate: {
          show: function(callback) {
            callback = $.isFunction(callback) ? callback : function(){};
            if(settings.transition && module.can.useElement('transition') && $module.transition('is supported')) {
              module.set.visible();
              $toastBox
                .transition({
                  animation  : settings.transition.showMethod + ' in',
                  queue      : false,
                  debug      : settings.debug,
                  verbose    : settings.verbose,
                  duration   : settings.transition.showDuration,
                  onComplete : function() {
                    callback.call($toastBox, element);
                    settings.onVisible.call($toastBox, element);
                  }
                })
              ;
            }
          },
          close: function(callback) {
            callback = $.isFunction(callback) ? callback : function(){};
            module.debug('Closing toast');
            if(settings.onHide.call($toastBox, element) === false) {
              module.debug('onHide callback returned false, cancelling toast animation');
              return;
            }
            if(settings.transition && $.fn.transition !== undefined && $module.transition('is supported')) {
              $toastBox
                .transition({
                  animation  : settings.transition.hideMethod + ' out',
                  queue      : false,
                  duration   : settings.transition.hideDuration,
                  debug      : settings.debug,
                  verbose    : settings.verbose,
                  interval   : 50,

                  onBeforeHide: function(callback){
                      callback = $.isFunction(callback)?callback : function(){};
                      if(settings.transition.closeEasing !== ''){
                          $toastBox.css('opacity',0);
                          $toastBox.wrap('<div/>').parent().slideUp(500,settings.transition.closeEasing,function(){
                            if($toastBox){
                              $toastBox.parent().remove();
                              callback.call($toastBox);
                            }
                          });
                      } else {
                        callback.call($toastBox);
                      }
                  },
                  onComplete : function() {
                    callback.call($toastBox, element);
                    settings.onHidden.call($toastBox, element);
                    module.destroy();
                  }
                })
              ;
            }
            else {
              module.error(error.noTransition);
            }
          },
          pause: function() {
            $animationObject.css('animationPlayState','paused');
            if($progressBar) {
              $progressBar.css('animationPlayState', 'paused');
            }
          },
          continue: function() {
            $animationObject.css('animationPlayState','running');
            if($progressBar) {
              $progressBar.css('animationPlayState', 'running');
            }
          }
        },

        has: {
          container: function() {
            module.verbose('Determining if there is already a container');
            return ($context.find(module.helpers.toClass(settings.position) + selector.container).length > 0);
          },
          toast: function(){
            return !!module.get.toast();
          },
          toasts: function(){
            return module.get.toasts().length > 0;
          },
          configActions: function () {
            return Array.isArray(settings.actions) && settings.actions.length > 0;
          }
        },

        get: {
          container: function() {
            return ($context.find(module.helpers.toClass(settings.position) + selector.container)[0]);
          },
          toastBox: function() {
            return $toastBox || null;
          },
          toast: function() {
            return $toast || null;
          },
          toasts: function() {
            return $(module.get.container()).find(selector.box);
          },
          iconClass: function() {
            return typeof settings.showIcon === 'string' ? settings.showIcon : settings.showIcon && settings.icons[settings.class] ? settings.icons[settings.class] : '';
          },
          remainingTime: function() {
            return $animationObject ? $animationObject.css('opacity') * settings.displayTime : 0;
          }
        },

        set: {
          visible: function() {
            $toast.addClass(className.visible);
          }
        },

        remove: {
          visible: function() {
            $toast.removeClass(className.visible);
          }
        },

        event: {
          click: function(event) {
            if($(event.target).closest('a').length === 0) {
              settings.onClick.call($toastBox, element);
              module.close();
            }
          },
          approve: function() {
            if(settings.onApprove.call(element, $module) === false) {
              module.verbose('Approve callback returned false cancelling close');
              return;
            }
            module.close();
          },
          deny: function() {
            if(settings.onDeny.call(element, $module) === false) {
              module.verbose('Deny callback returned false cancelling close');
              return;
            }
            module.close();
          }
        },

        helpers: {
          toClass: function(selector) {
            var
              classes = selector.split(' '),
              result = ''
            ;

            classes.forEach(function (element) {
              result += '.' + element;
            });

            return result;
          },
          deQuote: function(string) {
            return String(string).replace(/"/g,"");
          },
          escape: function(string, preserveHTML) {
            if (preserveHTML){
              return string;
            }
            var
              badChars     = /[<>"'`]/g,
              shouldEscape = /[&<>"'`]/,
              escape       = {
                "<": "&lt;",
                ">": "&gt;",
                '"': "&quot;",
                "'": "&#x27;",
                "`": "&#x60;"
              },
              escapedChar  = function(chr) {
                return escape[chr];
              }
            ;
            if(shouldEscape.test(string)) {
              string = string.replace(/&(?![a-z0-9#]{1,6};)/, "&amp;");
              return string.replace(badChars, escapedChar);
            }
            return string;
          }
        },

        can: {
          useElement: function(element){
            if ($.fn[element] !== undefined) {
              return true;
            }
            module.error(error.noElement.replace('{element}',element));
            return false;
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
        returnedValue = $module;
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.toast.settings = {

  name           : 'Toast',
  namespace      : 'toast',

  silent         : false,
  debug          : false,
  verbose        : false,
  performance    : true,

  context        : 'body',

  position       : 'top right',
  class          : 'neutral',
  classProgress  : false,
  classActions   : false,
  classImage     : 'mini',

  title          : '',
  message        : '',
  displayTime    : 3000, // set to zero to require manually dismissal, otherwise hides on its own
  minDisplayTime : 1000, // minimum displaytime in case displayTime is set to 'auto'
  wordsPerMinute : 120,
  showIcon       : false,
  newestOnTop    : false,
  showProgress   : false,
  pauseOnHover   : true,
  progressUp     : false, //if true, the bar will start at 0% and increase to 100%
  opacity        : 1,
  compact        : true,
  closeIcon      : false,
  closeOnClick   : true,
  cloneModule    : true,
  actions        : false,
  preserveHTML   : true,
  showImage      : false,

  // transition settings
  transition     : {
    showMethod   : 'scale',
    showDuration : 500,
    hideMethod   : 'scale',
    hideDuration : 500,
    closeEasing  : 'easeOutCubic'  //Set to empty string to stack the closed toast area immediately (old behaviour)
  },

  error: {
    method       : 'The method you called is not defined.',
    noElement    : 'This module requires ui {element}',
    verticalCard : 'Vertical but not attached actions are not supported for card layout'
  },

  className      : {
    container    : 'ui toast-container',
    box          : 'floating toast-box',
    progress     : 'ui attached active progress',
    toast        : 'ui toast',
    icon         : 'centered icon',
    visible      : 'visible',
    content      : 'content',
    title        : 'ui header',
    actions      : 'actions',
    extraContent : 'extra content',
    button       : 'ui button',
    buttons      : 'ui buttons',
    close        : 'close icon',
    image        : 'ui image',
    vertical     : 'vertical',
    attached     : 'attached',
    inverted     : 'inverted',
    compact      : 'compact',
    pausable     : 'pausable',
    progressing  : 'progressing',
    top          : 'top',
    bottom       : 'bottom',
    left         : 'left',
    basic        : 'basic',
    unclickable  : 'unclickable'
  },

  icons          : {
    info         : 'info',
    success      : 'checkmark',
    warning      : 'warning',
    error        : 'times'
  },

  selector       : {
    container    : '.ui.toast-container',
    box          : '.toast-box',
    toast        : '.ui.toast',
    input        : 'input:not([type="hidden"]), textarea, select, button, .ui.button, ui.dropdown',
    approve      : '.actions .positive, .actions .approve, .actions .ok',
    deny         : '.actions .negative, .actions .deny, .actions .cancel'
  },

  fields         : {
    class        : 'class',
    text         : 'text',
    icon         : 'icon',
    click        : 'click'
  },

  // callbacks
  onShow         : function(){},
  onVisible      : function(){},
  onClick        : function(){},
  onHide         : function(){},
  onHidden       : function(){},
  onRemove       : function(){},
  onApprove      : function(){},
  onDeny         : function(){}
};

$.extend( $.easing, {
    easeOutBounce: function (x, t, b, c, d) {
        if ((t/=d) < (1/2.75)) {
            return c*(7.5625*t*t) + b;
        } else if (t < (2/2.75)) {
            return c*(7.5625*(t-=(1.5/2.75))*t + .75) + b;
        } else if (t < (2.5/2.75)) {
            return c*(7.5625*(t-=(2.25/2.75))*t + .9375) + b;
        } else {
            return c*(7.5625*(t-=(2.625/2.75))*t + .984375) + b;
        }
    },
    easeOutCubic: function (t) {
      return (--t)*t*t+1;
    }
});


})( jQuery, window, document );
