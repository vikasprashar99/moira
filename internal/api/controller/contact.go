package controller

import (
	"bytes"
	"fmt"
	"time"

	moira2 "github.com/moira-alert/moira/internal/moira"

	"github.com/go-graphite/carbonapi/date"
	"github.com/gofrs/uuid"

	"github.com/moira-alert/moira/internal/api"
	"github.com/moira-alert/moira/internal/api/dto"
	"github.com/moira-alert/moira/internal/database"
)

// GetAllContacts gets all moira contacts
func GetAllContacts(database moira2.Database) (*dto.ContactList, *api.ErrorResponse) {
	contacts, err := database.GetAllContacts()
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	contactsList := dto.ContactList{
		List: contacts,
	}
	return &contactsList, nil
}

// CreateContact creates new notification contact for current user
func CreateContact(dataBase moira2.Database, contact *dto.Contact, userLogin string) *api.ErrorResponse {
	contactData := moira2.ContactData{
		ID:    contact.ID,
		User:  userLogin,
		Type:  contact.Type,
		Value: contact.Value,
	}
	if contactData.ID == "" {
		uuid4, err := uuid.NewV4()
		if err != nil {
			return api.ErrorInternalServer(err)
		}
		contactData.ID = uuid4.String()
	} else {
		exists, err := isContactExists(dataBase, contactData.ID)
		if err != nil {
			return api.ErrorInternalServer(err)
		}
		if exists {
			return api.ErrorInvalidRequest(fmt.Errorf("contact with this ID already exists"))
		}
	}

	if err := dataBase.SaveContact(&contactData); err != nil {
		return api.ErrorInternalServer(err)
	}
	contact.User = userLogin
	contact.ID = contactData.ID
	return nil
}

// UpdateContact updates notification contact for current user
func UpdateContact(dataBase moira2.Database, contactDTO dto.Contact, contactData moira2.ContactData) (dto.Contact, *api.ErrorResponse) {
	contactData.Type = contactDTO.Type
	contactData.Value = contactDTO.Value
	if err := dataBase.SaveContact(&contactData); err != nil {
		return contactDTO, api.ErrorInternalServer(err)
	}
	contactDTO.User = contactData.User
	contactDTO.ID = contactData.ID
	return contactDTO, nil
}

// RemoveContact deletes notification contact for current user and remove contactID from all subscriptions
func RemoveContact(database moira2.Database, contactID string, userLogin string) *api.ErrorResponse {
	subscriptionIDs, err := database.GetUserSubscriptionIDs(userLogin)
	if err != nil {
		return api.ErrorInternalServer(err)
	}

	subscriptions, err := database.GetSubscriptions(subscriptionIDs)
	if err != nil {
		return api.ErrorInternalServer(err)
	}

	subscriptionsWithDeletingContact := make([]*moira2.SubscriptionData, 0)

	for _, subscription := range subscriptions {
		if subscription == nil {
			continue
		}
		for i, contact := range subscription.Contacts {
			if contact == contactID {
				subscription.Contacts = append(subscription.Contacts[:i], subscription.Contacts[i+1:]...)
				subscriptionsWithDeletingContact = append(subscriptionsWithDeletingContact, subscription)
				break
			}
		}
	}

	if len(subscriptionsWithDeletingContact) > 0 {
		errBuffer := bytes.NewBuffer([]byte("this contact is being used in following subscriptions: "))
		for subInd, subscription := range subscriptionsWithDeletingContact {
			errBuffer.WriteString(subscription.ID)
			errBuffer.WriteString(" (tags: ")
			for tagInd := range subscription.Tags {
				errBuffer.WriteString(subscription.Tags[tagInd])
				if tagInd != len(subscription.Tags)-1 {
					errBuffer.WriteString(", ")
				}
			}
			errBuffer.WriteString(")")
			if subInd != len(subscriptionsWithDeletingContact)-1 {
				errBuffer.WriteString(", ")
			}
		}
		return api.ErrorInvalidRequest(fmt.Errorf(errBuffer.String()))
	}

	if err := database.RemoveContact(contactID); err != nil {
		return api.ErrorInternalServer(err)
	}

	return nil
}

// SendTestContactNotification push test notification to verify the correct contact settings
func SendTestContactNotification(dataBase moira2.Database, contactID string) *api.ErrorResponse {
	var value float64 = 1
	eventData := &moira2.NotificationEvent{
		ContactID: contactID,
		Metric:    "Test.metric.value",
		Value:     &value,
		OldState:  moira2.StateTEST,
		State:     moira2.StateTEST,
		Timestamp: date.DateParamToEpoch("now", "", time.Now().Add(-24*time.Hour).Unix(), time.UTC),
	}
	if err := dataBase.PushNotificationEvent(eventData, false); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}

// CheckUserPermissionsForContact checks contact for existence and permissions for given user
func CheckUserPermissionsForContact(dataBase moira2.Database, contactID string, userLogin string) (moira2.ContactData, *api.ErrorResponse) {
	contactData, err := dataBase.GetContact(contactID)
	if err != nil {
		if err == database.ErrNil {
			return contactData, api.ErrorNotFound(fmt.Sprintf("contact with ID '%s' does not exists", contactID))
		}
		return contactData, api.ErrorInternalServer(err)
	}
	if contactData.User != userLogin {
		return contactData, api.ErrorForbidden("you are not permitted")
	}
	return contactData, nil
}

func isContactExists(dataBase moira2.Database, contactID string) (bool, error) {
	_, err := dataBase.GetContact(contactID)
	if err == database.ErrNil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}