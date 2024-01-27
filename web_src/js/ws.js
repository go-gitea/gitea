/*
WebSockets Extension
============================
This extension adds support for WebSockets to htmx.  See /www/extensions/ws.md for usage instructions.
*/

export function ws() {
  /** @type {import("../htmx").HtmxInternalApi} */
  var api;

  htmx.defineExtension("ws", {
    /**
     * init is called once, when this extension is first registered.
     * @param {import("../htmx").HtmxInternalApi} apiRef
     */
    init: function (apiRef) {
      // Store reference to internal API
      api = apiRef;

      // Default function for creating new EventSource objects
      if (!htmx.createWebSocket) {
        htmx.createWebSocket = createWebSocket;
      }

      // Default setting for reconnect delay
      if (!htmx.config.wsReconnectDelay) {
        htmx.config.wsReconnectDelay = "full-jitter";
      }
    },

    /**
     * onEvent handles all events passed to this extension.
     *
     * @param {string} name
     * @param {Event} evt
     */
    onEvent: function (name, evt) {
      switch (name) {
        // Try to close the socket when elements are removed
        case "htmx:beforeCleanupElement":
          var internalData = api.getInternalData(evt.target);

          if (internalData.webSocket) {
            internalData.webSocket.close();
          }
          return;

        // Try to create websockets when elements are processed
        case "htmx:beforeProcessNode":
          var parent = evt.target;

          forEach(
            queryAttributeOnThisOrChildren(parent, "ws-connect"),
            function (child) {
              ensureWebSocket(child);
            }
          );
          forEach(
            queryAttributeOnThisOrChildren(parent, "ws-send"),
            function (child) {
              ensureWebSocketSend(child);
            }
          );
      }
    },
  });

  function splitOnWhitespace(trigger) {
    return trigger.trim().split(/\s+/);
  }

  function getLegacyWebsocketURL(elt) {
    var legacySSEValue = api.getAttributeValue(elt, "hx-ws");
    if (legacySSEValue) {
      var values = splitOnWhitespace(legacySSEValue);
      for (var i = 0; i < values.length; i++) {
        var value = values[i].split(/:(.+)/);
        if (value[0] === "connect") {
          return value[1];
        }
      }
    }
  }

  /**
   * ensureWebSocket creates a new WebSocket on the designated element, using
   * the element's "ws-connect" attribute.
   * @param {HTMLElement} socketElt
   * @returns
   */
  function ensureWebSocket(socketElt) {
    // If the element containing the WebSocket connection no longer exists, then
    // do not connect/reconnect the WebSocket.
    if (!api.bodyContains(socketElt)) {
      return;
    }

    // Get the source straight from the element's value
    var wssSource = api.getAttributeValue(socketElt, "ws-connect");

    if (wssSource == null || wssSource === "") {
      var legacySource = getLegacyWebsocketURL(socketElt);
      if (legacySource == null) {
        return;
      } else {
        wssSource = legacySource;
      }
    }

    // Guarantee that the wssSource value is a fully qualified URL
    if (wssSource.indexOf("/") === 0) {
      var base_part =
        location.hostname + (location.port ? ":" + location.port : "");
      if (location.protocol === "https:") {
        wssSource = "wss://" + base_part + wssSource;
      } else if (location.protocol === "http:") {
        wssSource = "ws://" + base_part + wssSource;
      }
    }

    var socketWrapper = createWebsocketWrapper(socketElt, function () {
      return htmx.createWebSocket(wssSource);
    });

    socketWrapper.addEventListener("message", function (event) {
      if (maybeCloseWebSocketSource(socketElt)) {
        return;
      }

      var response = event.data;
      if (
        !api.triggerEvent(socketElt, "htmx:wsBeforeMessage", {
          message: response,
          socketWrapper: socketWrapper.publicInterface,
        })
      ) {
        return;
      }

      api.withExtensions(socketElt, function (extension) {
        response = extension.transformResponse(response, null, socketElt);
      });

      var settleInfo = api.makeSettleInfo(socketElt);
      var fragment = api.makeFragment(response);

      if (fragment.children.length) {
        var children = Array.from(fragment.children);
        for (var i = 0; i < children.length; i++) {
          api.oobSwap(
            api.getAttributeValue(children[i], "hx-swap-oob") || "true",
            children[i],
            settleInfo
          );
        }
      }

      api.settleImmediately(settleInfo.tasks);
      api.triggerEvent(socketElt, "htmx:wsAfterMessage", {
        message: response,
        socketWrapper: socketWrapper.publicInterface,
      });
    });

    // Put the WebSocket into the HTML Element's custom data.
    api.getInternalData(socketElt).webSocket = socketWrapper;
  }

  /**
   * @typedef {Object} WebSocketWrapper
   * @property {WebSocket} socket
   * @property {Array<{message: string, sendElt: Element}>} messageQueue
   * @property {number} retryCount
   * @property {(message: string, sendElt: Element) => void} sendImmediately sendImmediately sends message regardless of websocket connection state
   * @property {(message: string, sendElt: Element) => void} send
   * @property {(event: string, handler: Function) => void} addEventListener
   * @property {() => void} handleQueuedMessages
   * @property {() => void} init
   * @property {() => void} close
   */
  /**
   *
   * @param socketElt
   * @param socketFunc
   * @returns {WebSocketWrapper}
   */
  function createWebsocketWrapper(socketElt, socketFunc) {
    var wrapper = {
      socket: null,
      messageQueue: [],
      retryCount: 0,

      /** @type {Object<string, Function[]>} */
      events: {},

      addEventListener: function (event, handler) {
        if (this.socket) {
          this.socket.addEventListener(event, handler);
        }

        if (!this.events[event]) {
          this.events[event] = [];
        }

        this.events[event].push(handler);
      },

      sendImmediately: function (message, sendElt) {
        if (!this.socket) {
          api.triggerErrorEvent();
        }
        if (
          !sendElt ||
          api.triggerEvent(sendElt, "htmx:wsBeforeSend", {
            message: message,
            socketWrapper: this.publicInterface,
          })
        ) {
          this.socket.send(message);
          sendElt &&
            api.triggerEvent(sendElt, "htmx:wsAfterSend", {
              message: message,
              socketWrapper: this.publicInterface,
            });
        }
      },

      send: function (message, sendElt) {
        if (this.socket.readyState !== this.socket.OPEN) {
          this.messageQueue.push({ message: message, sendElt: sendElt });
        } else {
          this.sendImmediately(message, sendElt);
        }
      },

      handleQueuedMessages: function () {
        while (this.messageQueue.length > 0) {
          var queuedItem = this.messageQueue[0];
          if (this.socket.readyState === this.socket.OPEN) {
            this.sendImmediately(queuedItem.message, queuedItem.sendElt);
            this.messageQueue.shift();
          } else {
            break;
          }
        }
      },

      init: function () {
        if (this.socket && this.socket.readyState === this.socket.OPEN) {
          // Close discarded socket
          this.socket.close();
        }

        // Create a new WebSocket and event handlers
        /** @type {WebSocket} */
        var socket = socketFunc();

        // The event.type detail is added for interface conformance with the
        // other two lifecycle events (open and close) so a single handler method
        // can handle them polymorphically, if required.
        api.triggerEvent(socketElt, "htmx:wsConnecting", {
          event: { type: "connecting" },
        });

        this.socket = socket;

        socket.onopen = function (e) {
          wrapper.retryCount = 0;
          api.triggerEvent(socketElt, "htmx:wsOpen", {
            event: e,
            socketWrapper: wrapper.publicInterface,
          });
          wrapper.handleQueuedMessages();
        };

        socket.onclose = function (e) {
          // If socket should not be connected, stop further attempts to establish connection
          // If Abnormal Closure/Service Restart/Try Again Later, then set a timer to reconnect after a pause.
          if (
            !maybeCloseWebSocketSource(socketElt) &&
            [1006, 1012, 1013].indexOf(e.code) >= 0
          ) {
            var delay = getWebSocketReconnectDelay(wrapper.retryCount);
            setTimeout(function () {
              wrapper.retryCount += 1;
              wrapper.init();
            }, delay);
          }

          // Notify client code that connection has been closed. Client code can inspect `event` field
          // to determine whether closure has been valid or abnormal
          api.triggerEvent(socketElt, "htmx:wsClose", {
            event: e,
            socketWrapper: wrapper.publicInterface,
          });
        };

        socket.onerror = function (e) {
          api.triggerErrorEvent(socketElt, "htmx:wsError", {
            error: e,
            socketWrapper: wrapper,
          });
          maybeCloseWebSocketSource(socketElt);
        };

        var events = this.events;
        Object.keys(events).forEach(function (k) {
          events[k].forEach(function (e) {
            socket.addEventListener(k, e);
          });
        });
      },

      close: function () {
        this.socket.close();
      },
    };

    wrapper.init();

    wrapper.publicInterface = {
      send: wrapper.send.bind(wrapper),
      sendImmediately: wrapper.sendImmediately.bind(wrapper),
      queue: wrapper.messageQueue,
    };

    return wrapper;
  }

  /**
   * ensureWebSocketSend attaches trigger handles to elements with
   * "ws-send" attribute
   * @param {HTMLElement} elt
   */
  function ensureWebSocketSend(elt) {
    var legacyAttribute = api.getAttributeValue(elt, "hx-ws");
    if (legacyAttribute && legacyAttribute !== "send") {
      return;
    }

    var webSocketParent = api.getClosestMatch(elt, hasWebSocket);
    processWebSocketSend(webSocketParent, elt);
  }

  /**
   * hasWebSocket function checks if a node has webSocket instance attached
   * @param {HTMLElement} node
   * @returns {boolean}
   */
  function hasWebSocket(node) {
    return api.getInternalData(node).webSocket != null;
  }

  /**
   * processWebSocketSend adds event listeners to the <form> element so that
   * messages can be sent to the WebSocket server when the form is submitted.
   * @param {HTMLElement} socketElt
   * @param {HTMLElement} sendElt
   */
  function processWebSocketSend(socketElt, sendElt) {
    var nodeData = api.getInternalData(sendElt);
    var triggerSpecs = api.getTriggerSpecs(sendElt);
    triggerSpecs.forEach(function (ts) {
      api.addTriggerHandler(sendElt, ts, nodeData, function (elt, evt) {
        if (maybeCloseWebSocketSource(socketElt)) {
          return;
        }

        /** @type {WebSocketWrapper} */
        var socketWrapper = api.getInternalData(socketElt).webSocket;
        var headers = api.getHeaders(sendElt, api.getTarget(sendElt));
        var results = api.getInputValues(sendElt, "post");
        var errors = results.errors;
        var rawParameters = results.values;
        var expressionVars = api.getExpressionVars(sendElt);
        var allParameters = api.mergeObjects(rawParameters, expressionVars);
        var filteredParameters = api.filterValues(allParameters, sendElt);

        var sendConfig = {
          parameters: filteredParameters,
          unfilteredParameters: allParameters,
          headers: headers,
          errors: errors,

          triggeringEvent: evt,
          messageBody: undefined,
          socketWrapper: socketWrapper.publicInterface,
        };

        if (!api.triggerEvent(elt, "htmx:wsConfigSend", sendConfig)) {
          return;
        }

        if (errors && errors.length > 0) {
          api.triggerEvent(elt, "htmx:validation:halted", errors);
          return;
        }

        var body = sendConfig.messageBody;
        if (body === undefined) {
          var toSend = Object.assign({}, sendConfig.parameters);
          if (sendConfig.headers) toSend["HEADERS"] = headers;
          body = JSON.stringify(toSend);
        }

        socketWrapper.send(body, elt);

        if (evt && api.shouldCancel(evt, elt)) {
          evt.preventDefault();
        }
      });
    });
  }

  /**
   * getWebSocketReconnectDelay is the default easing function for WebSocket reconnects.
   * @param {number} retryCount // The number of retries that have already taken place
   * @returns {number}
   */
  function getWebSocketReconnectDelay(retryCount) {
    /** @type {"full-jitter" | ((retryCount:number) => number)} */
    var delay = htmx.config.wsReconnectDelay;
    if (typeof delay === "function") {
      return delay(retryCount);
    }
    if (delay === "full-jitter") {
      var exp = Math.min(retryCount, 6);
      var maxDelay = 1000 * Math.pow(2, exp);
      return maxDelay * Math.random();
    }

    logError(
      'htmx.config.wsReconnectDelay must either be a function or the string "full-jitter"'
    );
  }

  /**
   * maybeCloseWebSocketSource checks to the if the element that created the WebSocket
   * still exists in the DOM.  If NOT, then the WebSocket is closed and this function
   * returns TRUE.  If the element DOES EXIST, then no action is taken, and this function
   * returns FALSE.
   *
   * @param {*} elt
   * @returns
   */
  function maybeCloseWebSocketSource(elt) {
    if (!api.bodyContains(elt)) {
      api.getInternalData(elt).webSocket.close();
      return true;
    }
    return false;
  }

  /**
   * createWebSocket is the default method for creating new WebSocket objects.
   * it is hoisted into htmx.createWebSocket to be overridden by the user, if needed.
   *
   * @param {string} url
   * @returns WebSocket
   */
  function createWebSocket(url) {
    var sock = new WebSocket(url, []);
    sock.binaryType = htmx.config.wsBinaryType;
    return sock;
  }

  /**
   * queryAttributeOnThisOrChildren returns all nodes that contain the requested attributeName, INCLUDING THE PROVIDED ROOT ELEMENT.
   *
   * @param {HTMLElement} elt
   * @param {string} attributeName
   */
  function queryAttributeOnThisOrChildren(elt, attributeName) {
    var result = [];

    // If the parent element also contains the requested attribute, then add it to the results too.
    if (
      api.hasAttribute(elt, attributeName) ||
      api.hasAttribute(elt, "hx-ws")
    ) {
      result.push(elt);
    }

    // Search all child nodes that match the requested attribute
    elt
      .querySelectorAll(
        "[" +
          attributeName +
          "], [data-" +
          attributeName +
          "], [data-hx-ws], [hx-ws]"
      )
      .forEach(function (node) {
        result.push(node);
      });

    return result;
  }

  /**
   * @template T
   * @param {T[]} arr
   * @param {(T) => void} func
   */
  function forEach(arr, func) {
    if (arr) {
      for (var i = 0; i < arr.length; i++) {
        func(arr[i]);
      }
    }
  }
}
