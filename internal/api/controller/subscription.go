package controller

import (
	"fmt"
	"time"

	moira2 "github.com/moira-alert/moira/internal/moira"

	"github.com/go-graphite/carbonapi/date"
	"github.com/gofrs/uuid"

	"github.com/moira-alert/moira/internal/api"
	"github.com/moira-alert/moira/internal/api/dto"
	"github.com/moira-alert/moira/internal/database"
)

// GetUserSubscriptions get all user subscriptions
func GetUserSubscriptions(database moira2.Database, userLogin string) (*dto.SubscriptionList, *api.ErrorResponse) {
	subscriptionIDs, err := database.GetUserSubscriptionIDs(userLogin)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	subscriptions, err := database.GetSubscriptions(subscriptionIDs)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	subscriptionsList := &dto.SubscriptionList{
		List: make([]moira2.SubscriptionData, 0),
	}
	for _, subscription := range subscriptions {
		if subscription != nil {
			subscriptionsList.List = append(subscriptionsList.List, *subscription)
		}
	}
	return subscriptionsList, nil
}

// CreateSubscription create or update subscription
func CreateSubscription(dataBase moira2.Database, userLogin string, subscription *dto.Subscription) *api.ErrorResponse {
	if subscription.ID == "" {
		uuid4, err := uuid.NewV4()
		if err != nil {
			return api.ErrorInternalServer(err)
		}
		subscription.ID = uuid4.String()
	} else {
		exists, err := isSubscriptionExists(dataBase, subscription.ID)
		if err != nil {
			return api.ErrorInternalServer(err)
		}
		if exists {
			return api.ErrorInvalidRequest(fmt.Errorf("subscription with this ID already exists"))
		}
	}

	subscription.User = userLogin
	data := moira2.SubscriptionData(*subscription)
	if err := dataBase.SaveSubscription(&data); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// UpdateSubscription updates existing subscription
func UpdateSubscription(dataBase moira2.Database, subscriptionID string, userLogin string, subscription *dto.Subscription) *api.ErrorResponse {
	subscription.ID = subscriptionID
	subscription.User = userLogin
	data := moira2.SubscriptionData(*subscription)
	if err := dataBase.SaveSubscription(&data); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// RemoveSubscription deletes subscription
func RemoveSubscription(database moira2.Database, subscriptionID string) *api.ErrorResponse {
	if err := database.RemoveSubscription(subscriptionID); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// SendTestNotification push test notification to verify the correct notification settings
func SendTestNotification(database moira2.Database, subscriptionID string) *api.ErrorResponse {
	var value float64 = 1
	eventData := &moira2.NotificationEvent{
		SubscriptionID: &subscriptionID,
		Metric:         "Test.metric.value",
		Value:          &value,
		OldState:       moira2.StateTEST,
		State:          moira2.StateTEST,
		Timestamp:      date.DateParamToEpoch("now", "", time.Now().Add(-24*time.Hour).Unix(), time.UTC),
	}

	if err := database.PushNotificationEvent(eventData, false); err != nil {
		return api.ErrorInternalServer(err)
	}

	return nil
}

// CheckUserPermissionsForSubscription checks subscription for existence and permissions for given user
func CheckUserPermissionsForSubscription(dataBase moira2.Database, subscriptionID string, userLogin string) (moira2.SubscriptionData, *api.ErrorResponse) {
	subscription, err := dataBase.GetSubscription(subscriptionID)
	if err != nil {
		if err == database.ErrNil {
			return subscription, api.ErrorNotFound(fmt.Sprintf("subscription with ID '%s' does not exists", subscriptionID))
		}
		return subscription, api.ErrorInternalServer(err)
	}
	if subscription.User != userLogin {
		return subscription, api.ErrorForbidden("you are not permitted")
	}
	return subscription, nil
}

func isSubscriptionExists(dataBase moira2.Database, subscriptionID string) (bool, error) {
	_, err := dataBase.GetSubscription(subscriptionID)
	if err == database.ErrNil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}