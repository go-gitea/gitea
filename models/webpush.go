// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
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
func CreateWebPushSubscription(userID int64, subscription *webpush.Subscription) error {
	return createWebPushSubscription(x, userID, subscription)
}

func createWebPushSubscription(e Engine, userID int64, subscription *webpush.Subscription) error {
	webPushSubscription := &WebPushSubscription{
		UserID:   userID,
		Endpoint: subscription.Endpoint,
		Auth:     subscription.Keys.Auth,
		P256DH:   subscription.Keys.P256dh,
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
func DeleteWebPushSubscription(subscriptionID int64) ([]*WebPushSubscription, error) {
	return deleteWebPushSubscription(x, subscriptionID)
}

func deleteWebPushSubscription(e Engine, subscriptionID int64) ([]*WebPushSubscription, error) {
	subscriptions := make([]*WebPushSubscription, 0)
	_, err := e.Delete(&WebPushSubscription{ID: subscriptionID})
	return subscriptions, err
}
