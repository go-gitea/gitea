package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/gorilla/websocket"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const (
	connectionInitMsg      = "connection_init"      // Client -> Server
	connectionTerminateMsg = "connection_terminate" // Client -> Server
	startMsg               = "start"                // Client -> Server
	stopMsg                = "stop"                 // Client -> Server
	connectionAckMsg       = "connection_ack"       // Server -> Client
	connectionErrorMsg     = "connection_error"     // Server -> Client
	dataMsg                = "data"                 // Server -> Client
	errorMsg               = "error"                // Server -> Client
	completeMsg            = "complete"             // Server -> Client
	connectionKeepAliveMsg = "ka"                   // Server -> Client
)

type (
	Websocket struct {
		Upgrader              websocket.Upgrader
		InitFunc              WebsocketInitFunc
		KeepAlivePingInterval time.Duration
	}
	wsConnection struct {
		Websocket
		ctx             context.Context
		conn            *websocket.Conn
		active          map[string]context.CancelFunc
		mu              sync.Mutex
		keepAliveTicker *time.Ticker
		exec            graphql.GraphExecutor

		initPayload InitPayload
	}
	operationMessage struct {
		Payload json.RawMessage `json:"payload,omitempty"`
		ID      string          `json:"id,omitempty"`
		Type    string          `json:"type"`
	}
	WebsocketInitFunc func(ctx context.Context, initPayload InitPayload) (context.Context, error)
)

var _ graphql.Transport = Websocket{}

func (t Websocket) Supports(r *http.Request) bool {
	return r.Header.Get("Upgrade") != ""
}

func (t Websocket) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor) {
	ws, err := t.Upgrader.Upgrade(w, r, http.Header{
		"Sec-Websocket-Protocol": []string{"graphql-ws"},
	})
	if err != nil {
		log.Printf("unable to upgrade %T to websocket %s: ", w, err.Error())
		SendErrorf(w, http.StatusBadRequest, "unable to upgrade")
		return
	}

	conn := wsConnection{
		active:    map[string]context.CancelFunc{},
		conn:      ws,
		ctx:       r.Context(),
		exec:      exec,
		Websocket: t,
	}

	if !conn.init() {
		return
	}

	conn.run()
}

func (c *wsConnection) init() bool {
	message := c.readOp()
	if message == nil {
		c.close(websocket.CloseProtocolError, "decoding error")
		return false
	}

	switch message.Type {
	case connectionInitMsg:
		if len(message.Payload) > 0 {
			c.initPayload = make(InitPayload)
			err := json.Unmarshal(message.Payload, &c.initPayload)
			if err != nil {
				return false
			}
		}

		if c.InitFunc != nil {
			ctx, err := c.InitFunc(c.ctx, c.initPayload)
			if err != nil {
				c.sendConnectionError(err.Error())
				c.close(websocket.CloseNormalClosure, "terminated")
				return false
			}
			c.ctx = ctx
		}

		c.write(&operationMessage{Type: connectionAckMsg})
		c.write(&operationMessage{Type: connectionKeepAliveMsg})
	case connectionTerminateMsg:
		c.close(websocket.CloseNormalClosure, "terminated")
		return false
	default:
		c.sendConnectionError("unexpected message %s", message.Type)
		c.close(websocket.CloseProtocolError, "unexpected message")
		return false
	}

	return true
}

func (c *wsConnection) write(msg *operationMessage) {
	c.mu.Lock()
	c.conn.WriteJSON(msg)
	c.mu.Unlock()
}

func (c *wsConnection) run() {
	// We create a cancellation that will shutdown the keep-alive when we leave
	// this function.
	ctx, cancel := context.WithCancel(c.ctx)
	defer func() {
		cancel()
		c.close(websocket.CloseAbnormalClosure, "unexpected closure")
	}()

	// Create a timer that will fire every interval to keep the connection alive.
	if c.KeepAlivePingInterval != 0 {
		c.mu.Lock()
		c.keepAliveTicker = time.NewTicker(c.KeepAlivePingInterval)
		c.mu.Unlock()

		go c.keepAlive(ctx)
	}

	for {
		start := graphql.Now()
		message := c.readOp()
		if message == nil {
			return
		}

		switch message.Type {
		case startMsg:
			c.subscribe(start, message)
		case stopMsg:
			c.mu.Lock()
			closer := c.active[message.ID]
			c.mu.Unlock()
			if closer != nil {
				closer()
			}
		case connectionTerminateMsg:
			c.close(websocket.CloseNormalClosure, "terminated")
			return
		default:
			c.sendConnectionError("unexpected message %s", message.Type)
			c.close(websocket.CloseProtocolError, "unexpected message")
			return
		}
	}
}

func (c *wsConnection) keepAlive(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.keepAliveTicker.Stop()
			return
		case <-c.keepAliveTicker.C:
			c.write(&operationMessage{Type: connectionKeepAliveMsg})
		}
	}
}

func (c *wsConnection) subscribe(start time.Time, message *operationMessage) {
	ctx := graphql.StartOperationTrace(c.ctx)
	var params *graphql.RawParams
	if err := jsonDecode(bytes.NewReader(message.Payload), &params); err != nil {
		c.sendError(message.ID, &gqlerror.Error{Message: "invalid json"})
		c.complete(message.ID)
		return
	}

	params.ReadTime = graphql.TraceTiming{
		Start: start,
		End:   graphql.Now(),
	}

	rc, err := c.exec.CreateOperationContext(ctx, params)
	if err != nil {
		resp := c.exec.DispatchError(graphql.WithOperationContext(ctx, rc), err)
		switch errcode.GetErrorKind(err) {
		case errcode.KindProtocol:
			c.sendError(message.ID, resp.Errors...)
		default:
			c.sendResponse(message.ID, &graphql.Response{Errors: err})
		}

		c.complete(message.ID)
		return
	}

	ctx = graphql.WithOperationContext(ctx, rc)

	if c.initPayload != nil {
		ctx = withInitPayload(ctx, c.initPayload)
	}

	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.active[message.ID] = cancel
	c.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				userErr := rc.Recover(ctx, r)
				c.sendError(message.ID, &gqlerror.Error{Message: userErr.Error()})
			}
		}()
		responses, ctx := c.exec.DispatchOperation(ctx, rc)
		for {
			response := responses(ctx)
			if response == nil {
				break
			}

			c.sendResponse(message.ID, response)
		}
		c.complete(message.ID)

		c.mu.Lock()
		delete(c.active, message.ID)
		c.mu.Unlock()
		cancel()
	}()
}

func (c *wsConnection) sendResponse(id string, response *graphql.Response) {
	b, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	c.write(&operationMessage{
		Payload: b,
		ID:      id,
		Type:    dataMsg,
	})
}

func (c *wsConnection) complete(id string) {
	c.write(&operationMessage{ID: id, Type: completeMsg})
}

func (c *wsConnection) sendError(id string, errors ...*gqlerror.Error) {
	errs := make([]error, len(errors))
	for i, err := range errors {
		errs[i] = err
	}
	b, err := json.Marshal(errs)
	if err != nil {
		panic(err)
	}
	c.write(&operationMessage{Type: errorMsg, ID: id, Payload: b})
}

func (c *wsConnection) sendConnectionError(format string, args ...interface{}) {
	b, err := json.Marshal(&gqlerror.Error{Message: fmt.Sprintf(format, args...)})
	if err != nil {
		panic(err)
	}

	c.write(&operationMessage{Type: connectionErrorMsg, Payload: b})
}

func (c *wsConnection) readOp() *operationMessage {
	_, r, err := c.conn.NextReader()
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
		return nil
	} else if err != nil {
		c.sendConnectionError("invalid json: %T %s", err, err.Error())
		return nil
	}
	message := operationMessage{}
	if err := jsonDecode(r, &message); err != nil {
		c.sendConnectionError("invalid json")
		return nil
	}

	return &message
}

func (c *wsConnection) close(closeCode int, message string) {
	c.mu.Lock()
	_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, message))
	c.mu.Unlock()
	_ = c.conn.Close()
}
