// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"net/http"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/SherClockHolmes/webpush-go"
)

// WebPushSubscription represents a HTML5 Web Push Subscription used for background notifications.
type WebPushSubscription struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"INDEX"`

	Endpoint string
	Auth     string
	P256DH   string

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
}

// CreateWebPushSubscription creates a row on the push subscriptions table.
func CreateWebPushSubscription(userID int64, subscription *structs.NotificationWebPushSubscription) error {
	return createWebPushSubscription(x, userID, subscription)
}

func createWebPushSubscription(e Engine, userID int64, subscription *structs.NotificationWebPushSubscription) error {
	webPushSubscription := &WebPushSubscription{
		UserID:   userID,
		Endpoint: subscription.Endpoint,
		Auth:     subscription.Auth,
		P256DH:   subscription.P256DH,
	}

	_, err := e.Insert(webPushSubscription)
	return err
}

// GetWebPushSubscriptionsByUserID gets all the Web Push subscriptions for a given user
func GetWebPushSubscriptionsByUserID(userID int64) ([]*WebPushSubscription, error) {
	return getWebPushSubscriptionsByUserID(x, userID)
}

func getWebPushSubscriptionsByUserID(e Engine, userID int64) ([]*WebPushSubscription, error) {
	subscriptions := make([]*WebPushSubscription, 0)
	err := e.Where("user_id = ?", userID).Find(&subscriptions)
	return subscriptions, err
}

// DeleteWebPushSubscription deletes a given Web Push subscription by ID
func DeleteWebPushSubscription(subscriptionID int64) error {
	return deleteWebPushSubscription(x, subscriptionID)
}

func deleteWebPushSubscription(e Engine, subscriptionID int64) error {
	_, err := e.Delete(&WebPushSubscription{ID: subscriptionID})
	return err
}

// SendWebPushNotificationToUser sends a background Web Push notification to any of the user's
// enrolled browsers.
// It will also remove any failed (expired) subscriptions.
func SendWebPushNotificationToUser(userID int64, payload *structs.NotificationPayload) error {
	userSubscriptions, err := GetWebPushSubscriptionsByUserID(userID)
	if err != nil {
		return err
	}

	for _, userSubscription := range userSubscriptions {
		subscription := &structs.NotificationWebPushSubscription{
			Endpoint: userSubscription.Endpoint,
			Auth:     userSubscription.Auth,
			P256DH:   userSubscription.P256DH,
		}
		resp, err := SendWebPushNotification(subscription, payload)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			// This is a bad subscription. It may have expired (410 Gone).
			err = DeleteWebPushSubscription(userSubscription.ID)
			if err != nil {
				log.Error("could not delete Web Push subscription: %v", err)
				return err
			}
		}
	}
	return nil
}

// SendWebPushNotification sends a background Web Push notification to any of the user's
// enrolled browsers.
// The HTTP status code indicates success. err is for internal problems.
func SendWebPushNotification(subscription *structs.NotificationWebPushSubscription, payload *structs.NotificationPayload) (*http.Response, error) {
	webPushSubscription := &webpush.Subscription{
		Endpoint: subscription.Endpoint,
		Keys: webpush.Keys{
			Auth:   subscription.Auth,
			P256dh: subscription.P256DH,
		},
	}

	pushPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := webpush.SendNotification(pushPayload, webPushSubscription, &webpush.Options{
		VAPIDPublicKey:  setting.WebPushPublicKey,
		VAPIDPrivateKey: setting.WebPushPrivateKey,
		TTL:             30,
	})
	if err != nil {
		return resp, err
	}

	defer resp.Body.Close()
	return resp, nil
}
