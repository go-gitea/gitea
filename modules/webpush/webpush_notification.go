// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webpush

import (
	"encoding/json"

	"code.gitea.io/gitea/modules/setting"
	"github.com/SherClockHolmes/webpush-go"
)

// NotificationPayload marks a JSON payload sent in a push event to the JS service worker.
// This is used for background notifications.
type NotificationPayload struct {
	Title string `json:"title"`
	Text  string `json:"text"`
	URL   string `json:"url"`
}

// SendWebPushNotification sends a background Web Push notification to any of the user's
// enrolled browsers.
func SendWebPushNotification(userID int64, payload *NotificationPayload) error {
	subscriptionJSON := `{"endpoint":"https://updates.push.services.mozilla.com/wpush/v2/gAAAAABef9kAcjpOcA08JjgxxNAtS-bcR63Q8Rb2DWvjjGHJvyR3FeUc6ILDSFiuMXU6MVEdNLATfwkVQDrrwk2_ZZLawyqHQ04SpWDTiJyaPmc-izAScxmMfhkwaHE2QJ4iwaekANIUS7E0cPCbKCeKQelYXz7OsGdI_9CGp7HPW2mDSUlnTDE","keys":{"auth":"ZoXN3QIExEC1FG0ZeeyRMg","p256dh":"BFWEbLR4Yd2Jai0R-xIBlKE66U6_LXV9m33qTCRu51TVj0rIMeA4B9juluGFUxKIDYQhOtfrvsGyD0BMX33Tenc"}}`
	s := &webpush.Subscription{}
	json.Unmarshal([]byte(subscriptionJSON), s)

	pushPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := webpush.SendNotification(pushPayload, s, &webpush.Options{
		VAPIDPublicKey:  setting.WebPushPublicKey,
		VAPIDPrivateKey: setting.WebPushPrivateKey,
		TTL:             30,
	})
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	return nil
}
