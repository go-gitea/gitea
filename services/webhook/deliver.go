// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/hostmatcher"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/proxy"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/gobwas/glob"
)

func newDefaultRequest(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (req *http.Request, body []byte, err error) {
	switch w.HTTPMethod {
	case "":
		log.Info("HTTP Method for %s webhook %s [ID: %d] is not set, defaulting to POST", w.Type, w.URL, w.ID)
		fallthrough
	case http.MethodPost:
		switch w.ContentType {
		case webhook_model.ContentTypeJSON:
			req, err = http.NewRequest("POST", w.URL, strings.NewReader(t.PayloadContent))
			if err != nil {
				return nil, nil, err
			}

			req.Header.Set("Content-Type", "application/json")
		case webhook_model.ContentTypeForm:
			forms := url.Values{
				"payload": []string{t.PayloadContent},
			}

			req, err = http.NewRequest("POST", w.URL, strings.NewReader(forms.Encode()))
			if err != nil {
				return nil, nil, err
			}

			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		default:
			return nil, nil, fmt.Errorf("invalid content type: %v", w.ContentType)
		}
	case http.MethodGet:
		u, err := url.Parse(w.URL)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid URL: %w", err)
		}
		vals := u.Query()
		vals["payload"] = []string{t.PayloadContent}
		u.RawQuery = vals.Encode()
		req, err = http.NewRequest("GET", u.String(), nil)
		if err != nil {
			return nil, nil, err
		}
	case http.MethodPut:
		switch w.Type {
		case webhook_module.MATRIX: // used when t.Version == 1
			txnID, err := getMatrixTxnID([]byte(t.PayloadContent))
			if err != nil {
				return nil, nil, err
			}
			url := fmt.Sprintf("%s/%s", w.URL, url.PathEscape(txnID))
			req, err = http.NewRequest("PUT", url, strings.NewReader(t.PayloadContent))
			if err != nil {
				return nil, nil, err
			}
		default:
			return nil, nil, fmt.Errorf("invalid http method: %v", w.HTTPMethod)
		}
	default:
		return nil, nil, fmt.Errorf("invalid http method: %v", w.HTTPMethod)
	}

	body = []byte(t.PayloadContent)
	return req, body, addDefaultHeaders(req, []byte(w.Secret), t, body)
}

func addDefaultHeaders(req *http.Request, secret []byte, t *webhook_model.HookTask, payloadContent []byte) error {
	var signatureSHA1 string
	var signatureSHA256 string
	if len(secret) > 0 {
		sig1 := hmac.New(sha1.New, secret)
		sig256 := hmac.New(sha256.New, secret)
		_, err := io.MultiWriter(sig1, sig256).Write(payloadContent)
		if err != nil {
			// this error should never happen, since the hashes are writing to []byte and always return a nil error.
			return fmt.Errorf("prepareWebhooks.sigWrite: %w", err)
		}
		signatureSHA1 = hex.EncodeToString(sig1.Sum(nil))
		signatureSHA256 = hex.EncodeToString(sig256.Sum(nil))
	}

	event := t.EventType.Event()
	eventType := string(t.EventType)
	req.Header.Add("X-Gitea-Delivery", t.UUID)
	req.Header.Add("X-Gitea-Event", event)
	req.Header.Add("X-Gitea-Event-Type", eventType)
	req.Header.Add("X-Gitea-Signature", signatureSHA256)
	req.Header.Add("X-Gogs-Delivery", t.UUID)
	req.Header.Add("X-Gogs-Event", event)
	req.Header.Add("X-Gogs-Event-Type", eventType)
	req.Header.Add("X-Gogs-Signature", signatureSHA256)
	req.Header.Add("X-Hub-Signature", "sha1="+signatureSHA1)
	req.Header.Add("X-Hub-Signature-256", "sha256="+signatureSHA256)
	req.Header["X-GitHub-Delivery"] = []string{t.UUID}
	req.Header["X-GitHub-Event"] = []string{event}
	req.Header["X-GitHub-Event-Type"] = []string{eventType}
	return nil
}

// Deliver creates the [http.Request] (depending on the webhook type), sends it
// and records the status and response.
func Deliver(ctx context.Context, t *webhook_model.HookTask) error {
	w, err := webhook_model.GetWebhookByID(ctx, t.HookID)
	if err != nil {
		return err
	}

	defer func() {
		err := recover()
		if err == nil {
			return
		}
		// There was a panic whilst delivering a hook...
		log.Error("PANIC whilst trying to deliver webhook task[%d] to webhook %s Panic: %v\nStacktrace: %s", t.ID, w.URL, err, log.Stack(2))
	}()

	t.IsDelivered = true

	newRequest := webhookRequesters[w.Type]
	if t.PayloadVersion == 1 || newRequest == nil {
		newRequest = newDefaultRequest
	}

	req, body, err := newRequest(ctx, w, t)
	if err != nil {
		return fmt.Errorf("cannot create http request for webhook %s[%d %s]: %w", w.Type, w.ID, w.URL, err)
	}

	// Record delivery information.
	t.RequestInfo = &webhook_model.HookRequest{
		URL:        req.URL.String(),
		HTTPMethod: req.Method,
		Headers:    map[string]string{},
		Body:       string(body),
	}
	for k, vals := range req.Header {
		t.RequestInfo.Headers[k] = strings.Join(vals, ",")
	}

	// Add Authorization Header
	authorization, err := w.HeaderAuthorization()
	if err != nil {
		return fmt.Errorf("cannot get Authorization header for webhook %s[%d %s]: %w", w.Type, w.ID, w.URL, err)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
		t.RequestInfo.Headers["Authorization"] = "******"
	}

	t.ResponseInfo = &webhook_model.HookResponse{
		Headers: map[string]string{},
	}

	// OK We're now ready to attempt to deliver the task - we must double check that it
	// has not been delivered in the meantime
	updated, err := webhook_model.MarkTaskDelivered(ctx, t)
	if err != nil {
		log.Error("MarkTaskDelivered[%d]: %v", t.ID, err)
		return fmt.Errorf("unable to mark task[%d] delivered in the db: %w", t.ID, err)
	}
	if !updated {
		// This webhook task has already been attempted to be delivered or is in the process of being delivered
		log.Trace("Webhook Task[%d] already delivered", t.ID)
		return nil
	}

	// All code from this point will update the hook task
	defer func() {
		t.Delivered = timeutil.TimeStampNanoNow()
		if t.IsSucceed {
			log.Trace("Hook delivered: %s", t.UUID)
		} else if !w.IsActive {
			log.Trace("Hook delivery skipped as webhook is inactive: %s", t.UUID)
		} else {
			log.Trace("Hook delivery failed: %s", t.UUID)
		}

		if err := webhook_model.UpdateHookTask(ctx, t); err != nil {
			log.Error("UpdateHookTask [%d]: %v", t.ID, err)
		}

		// Update webhook last delivery status.
		if t.IsSucceed {
			w.LastStatus = webhook_module.HookStatusSucceed
		} else {
			w.LastStatus = webhook_module.HookStatusFail
		}
		if err = webhook_model.UpdateWebhookLastStatus(ctx, w); err != nil {
			log.Error("UpdateWebhookLastStatus: %v", err)
			return
		}
	}()

	if setting.DisableWebhooks {
		return fmt.Errorf("webhook task skipped (webhooks disabled): [%d]", t.ID)
	}

	if !w.IsActive {
		log.Trace("Webhook %s in Webhook Task[%d] is not active", w.URL, t.ID)
		return nil
	}

	resp, err := webhookHTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		t.ResponseInfo.Body = fmt.Sprintf("Delivery: %v", err)
		return fmt.Errorf("unable to deliver webhook task[%d] in %s due to error in http client: %w", t.ID, w.URL, err)
	}
	defer resp.Body.Close()

	// Status code is 20x can be seen as succeed.
	t.IsSucceed = resp.StatusCode/100 == 2
	t.ResponseInfo.Status = resp.StatusCode
	for k, vals := range resp.Header {
		t.ResponseInfo.Headers[k] = strings.Join(vals, ",")
	}

	p, err := io.ReadAll(resp.Body)
	if err != nil {
		t.ResponseInfo.Body = fmt.Sprintf("read body: %s", err)
		return fmt.Errorf("unable to deliver webhook task[%d] in %s as unable to read response body: %w", t.ID, w.URL, err)
	}
	t.ResponseInfo.Body = string(p)
	return nil
}

var (
	webhookHTTPClient *http.Client
	once              sync.Once
	hostMatchers      []glob.Glob
)

func webhookProxy(allowList *hostmatcher.HostMatchList) func(req *http.Request) (*url.URL, error) {
	if setting.Webhook.ProxyURL == "" {
		return proxy.Proxy()
	}

	once.Do(func() {
		for _, h := range setting.Webhook.ProxyHosts {
			if g, err := glob.Compile(h); err == nil {
				hostMatchers = append(hostMatchers, g)
			} else {
				log.Error("glob.Compile %s failed: %v", h, err)
			}
		}
	})

	return func(req *http.Request) (*url.URL, error) {
		for _, v := range hostMatchers {
			if v.Match(req.URL.Host) {
				if !allowList.MatchHostName(req.URL.Host) {
					return nil, fmt.Errorf("webhook can only call allowed HTTP servers (check your %s setting), deny '%s'", allowList.SettingKeyHint, req.URL.Host)
				}
				return http.ProxyURL(setting.Webhook.ProxyURLFixed)(req)
			}
		}
		return http.ProxyFromEnvironment(req)
	}
}

// Init starts the hooks delivery thread
func Init() error {
	timeout := time.Duration(setting.Webhook.DeliverTimeout) * time.Second

	allowedHostListValue := setting.Webhook.AllowedHostList
	if allowedHostListValue == "" {
		allowedHostListValue = hostmatcher.MatchBuiltinExternal
	}
	allowedHostMatcher := hostmatcher.ParseHostMatchList("webhook.ALLOWED_HOST_LIST", allowedHostListValue)

	webhookHTTPClient = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: setting.Webhook.SkipTLSVerify},
			Proxy:           webhookProxy(allowedHostMatcher),
			DialContext:     hostmatcher.NewDialContext("webhook", allowedHostMatcher, nil, setting.Webhook.ProxyURLFixed),
		},
	}

	hookQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "webhook_sender", handler)
	if hookQueue == nil {
		return fmt.Errorf("unable to create webhook_sender queue")
	}
	go graceful.GetManager().RunWithCancel(hookQueue)

	go graceful.GetManager().RunWithShutdownContext(populateWebhookSendingQueue)

	return nil
}

func populateWebhookSendingQueue(ctx context.Context) {
	ctx, _, finished := process.GetManager().AddContext(ctx, "Webhook: Populate sending queue")
	defer finished()

	lowerID := int64(0)
	for {
		taskIDs, err := webhook_model.FindUndeliveredHookTaskIDs(ctx, lowerID)
		if err != nil {
			log.Error("Unable to populate webhook queue as FindUndeliveredHookTaskIDs failed: %v", err)
			return
		}
		if len(taskIDs) == 0 {
			return
		}
		lowerID = taskIDs[len(taskIDs)-1]

		for _, taskID := range taskIDs {
			select {
			case <-ctx.Done():
				log.Warn("Shutdown before Webhook Sending queue finishing being populated")
				return
			default:
			}
			if err := enqueueHookTask(taskID); err != nil {
				log.Error("Unable to push HookTask[%d] to the Webhook Sending queue: %v", taskID, err)
			}
		}
	}
}
