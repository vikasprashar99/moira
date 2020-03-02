package events

import (
	"fmt"
	"testing"
	"time"

	moira2 "github.com/moira-alert/moira/internal/moira"

	"github.com/golang/mock/gomock"
	"github.com/op/go-logging"
	. "github.com/smartystreets/goconvey/convey"

	"github.com/moira-alert/moira/internal/database"
	"github.com/moira-alert/moira/internal/metrics"
	mock_moira_alert "github.com/moira-alert/moira/internal/mock/moira-alert"
	mock_scheduler "github.com/moira-alert/moira/internal/mock/scheduler"
	"github.com/moira-alert/moira/internal/notifier"
)

var notifierMetrics = metrics.ConfigureNotifierMetrics(metrics.NewDummyRegistry(), "notifier")

func TestEvent(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	scheduler := mock_scheduler.NewMockScheduler(mockCtrl)
	logger, _ := logging.GetLogger("Events")

	Convey("When event is TEST and subscription is disabled, should add new notification", t, func() {
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, logger, notifierMetrics),
		}
		event := moira2.NotificationEvent{
			State:          moira2.StateTEST,
			SubscriptionID: &subscription.ID,
		}
		dataBase.EXPECT().GetSubscription(*event.SubscriptionID).Times(1).Return(subscription, nil)
		dataBase.EXPECT().GetContact(contact.ID).Times(1).Return(contact, nil)
		notification := moira2.ScheduledNotification{
			Event: moira2.NotificationEvent{
				TriggerID:      event.TriggerID,
				State:          event.State,
				OldState:       event.OldState,
				Metric:         event.Metric,
				SubscriptionID: event.SubscriptionID,
			},
			SendFail:  0,
			Timestamp: time.Now().Unix(),
			Throttled: false,
			Contact:   contact,
		}
		dataBase.EXPECT().AddNotification(&notification)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})

	Convey("When event is TEST and has contactID", t, func() {
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: scheduler,
		}

		subID := "testSubscription"
		event := moira2.NotificationEvent{
			State:     moira2.StateTEST,
			OldState:  moira2.StateTEST,
			Metric:    "test.metric",
			ContactID: contact.ID,
		}
		dataBase.EXPECT().GetContact(event.ContactID).Times(1).Return(contact, nil)
		dataBase.EXPECT().GetContact(contact.ID).Times(1).Return(contact, nil)
		now := time.Now()
		notification := moira2.ScheduledNotification{
			Event: moira2.NotificationEvent{
				TriggerID:      "",
				State:          event.State,
				OldState:       event.OldState,
				Metric:         event.Metric,
				SubscriptionID: &subID,
			},
			SendFail:  0,
			Timestamp: now.Unix(),
			Throttled: false,
			Contact:   contact,
		}
		event2 := event
		event2.SubscriptionID = &subID
		scheduler.EXPECT().ScheduleNotification(gomock.Any(), event2, moira2.TriggerData{}, contact, notification.Plotting, false, 0).Return(&notification)
		dataBase.EXPECT().AddNotification(&notification)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestNoSubscription(t *testing.T) {
	Convey("When no subscription by event tags, should not call AddNotification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		logger, _ := logging.GetLogger("Events")

		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, logger, notifierMetrics),
		}

		event := moira2.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira2.StateOK,
			OldState:  moira2.StateWARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(triggerData.Tags).Times(1).Return(make([]*moira2.SubscriptionData, 0), nil)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestDisabledNotification(t *testing.T) {
	Convey("When subscription event tags is disabled, should not call AddNotification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		logger := mock_moira_alert.NewMockLogger(mockCtrl)

		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, logger, notifierMetrics),
		}

		event := moira2.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira2.StateOK,
			OldState:  moira2.StateWARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(triggerData.Tags).Times(1).Return([]*moira2.SubscriptionData{&disabledSubscription}, nil)

		logger.EXPECT().Debugf("Processing trigger id %s for metric %s == %f, %s -> %s", event.TriggerID, event.Metric, moira2.UseFloat64(event.Value), event.OldState, event.State)
		logger.EXPECT().Debugf("Getting subscriptions for tags %v", triggerData.Tags)
		logger.EXPECT().Debugf("Subscription %s is disabled", disabledSubscription.ID)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestSubscriptionsManagedToIgnoreEvents(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	logger := mock_moira_alert.NewMockLogger(mockCtrl)

	Convey("[TRUE] Do not send WARN notifications", t, func() {
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, logger, notifierMetrics),
		}

		event := moira2.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira2.StateOK,
			OldState:  moira2.StateWARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(triggerData.Tags).Times(1).Return([]*moira2.SubscriptionData{&subscriptionToIgnoreWarnings}, nil)

		logger.EXPECT().Debugf("Processing trigger id %s for metric %s == %f, %s -> %s", event.TriggerID, event.Metric, moira2.UseFloat64(event.Value), event.OldState, event.State)
		logger.EXPECT().Debugf("Getting subscriptions for tags %v", triggerData.Tags)
		logger.EXPECT().Debugf("Subscription %s is managed to ignore %s -> %s transitions", subscriptionToIgnoreWarnings.ID, event.OldState, event.State)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
	Convey("[TRUE] Send notifications when triggers degraded only", t, func() {
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, logger, notifierMetrics),
		}

		event := moira2.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira2.StateOK,
			OldState:  moira2.StateWARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(triggerData.Tags).Times(1).Return([]*moira2.SubscriptionData{&subscriptionToIgnoreRecoverings}, nil)

		logger.EXPECT().Debugf("Processing trigger id %s for metric %s == %f, %s -> %s", event.TriggerID, event.Metric, moira2.UseFloat64(event.Value), event.OldState, event.State)
		logger.EXPECT().Debugf("Getting subscriptions for tags %v", triggerData.Tags)
		logger.EXPECT().Debugf("Subscription %s is managed to ignore %s -> %s transitions", subscriptionToIgnoreRecoverings.ID, event.OldState, event.State)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
	Convey("[TRUE] Do not send WARN notifications & [TRUE] Send notifications when triggers degraded only", t, func() {
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, logger, notifierMetrics),
		}

		event := moira2.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira2.StateOK,
			OldState:  moira2.StateWARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(triggerData.Tags).Times(1).Return([]*moira2.SubscriptionData{&subscriptionToIgnoreWarningsAndRecoverings}, nil)

		logger.EXPECT().Debugf("Processing trigger id %s for metric %s == %f, %s -> %s", event.TriggerID, event.Metric, moira2.UseFloat64(event.Value), event.OldState, event.State)
		logger.EXPECT().Debugf("Getting subscriptions for tags %v", triggerData.Tags)
		logger.EXPECT().Debugf("Subscription %s is managed to ignore %s -> %s transitions", subscriptionToIgnoreWarningsAndRecoverings.ID, event.OldState, event.State)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestAddNotification(t *testing.T) {
	Convey("When good subscription, should add new notification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		logger, _ := logging.GetLogger("Events")
		scheduler := mock_scheduler.NewMockScheduler(mockCtrl)
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: scheduler,
		}

		event := moira2.NotificationEvent{
			Metric:         "generate.event.1",
			State:          moira2.StateOK,
			OldState:       moira2.StateWARN,
			TriggerID:      triggerData.ID,
			SubscriptionID: &subscription.ID,
		}
		emptyNotification := moira2.ScheduledNotification{}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(triggerData.Tags).Times(1).Return([]*moira2.SubscriptionData{&subscription}, nil)
		dataBase.EXPECT().GetContact(contact.ID).Times(1).Return(contact, nil)
		scheduler.EXPECT().ScheduleNotification(gomock.Any(), event, triggerData, contact, emptyNotification.Plotting, false, 0).Times(1).Return(&emptyNotification)
		dataBase.EXPECT().AddNotification(&emptyNotification).Times(1).Return(nil)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestAddOneNotificationByTwoSubscriptionsWithSame(t *testing.T) {
	Convey("When good subscription and create 2 same scheduled notifications, should add one new notification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		logger, _ := logging.GetLogger("Events")
		scheduler := mock_scheduler.NewMockScheduler(mockCtrl)
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: scheduler,
		}

		event := moira2.NotificationEvent{
			Metric:         "generate.event.1",
			State:          moira2.StateOK,
			OldState:       moira2.StateWARN,
			TriggerID:      triggerData.ID,
			SubscriptionID: &subscription.ID,
		}
		event2 := event
		event2.SubscriptionID = &subscription4.ID

		notification2 := moira2.ScheduledNotification{}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(triggerData.Tags).Times(1).Return([]*moira2.SubscriptionData{&subscription, &subscription4}, nil)
		dataBase.EXPECT().GetContact(contact.ID).Times(2).Return(contact, nil)

		scheduler.EXPECT().ScheduleNotification(gomock.Any(), event, triggerData, contact, notification2.Plotting, false, 0).Times(1).Return(&notification2)
		scheduler.EXPECT().ScheduleNotification(gomock.Any(), event2, triggerData, contact, notification2.Plotting, false, 0).Times(1).Return(&notification2)

		dataBase.EXPECT().AddNotification(&notification2).Times(1).Return(nil)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestFailReadContact(t *testing.T) {
	Convey("When read contact returns error, should not call AddNotification and not crashed", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		logger := mock_moira_alert.NewMockLogger(mockCtrl)
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, logger, notifierMetrics),
		}

		event := moira2.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira2.StateOK,
			OldState:  moira2.StateWARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(triggerData.Tags).Times(1).Return([]*moira2.SubscriptionData{&subscription}, nil)
		getContactError := fmt.Errorf("Can not get contact")
		dataBase.EXPECT().GetContact(contact.ID).Times(1).Return(moira2.ContactData{}, getContactError)

		logger.EXPECT().Debugf("Processing trigger id %s for metric %s == %f, %s -> %s", event.TriggerID, event.Metric, moira2.UseFloat64(event.Value), event.OldState, event.State)
		logger.EXPECT().Debugf("Getting subscriptions for tags %v", triggerData.Tags)
		logger.EXPECT().Warningf("Failed to get contact: %s, skip handling it, error: %v", contact.ID, getContactError)

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestEmptySubscriptions(t *testing.T) {
	Convey("When subscription is empty value object", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		logger := mock_moira_alert.NewMockLogger(mockCtrl)
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, logger, notifierMetrics),
		}

		event := moira2.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira2.StateOK,
			OldState:  moira2.StateWARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(triggerData.Tags).Times(1).Return([]*moira2.SubscriptionData{{ThrottlingEnabled: true}}, nil)

		logger.EXPECT().Debugf("Processing trigger id %s for metric %s == %f, %s -> %s", event.TriggerID, event.Metric, moira2.UseFloat64(event.Value), event.OldState, event.State)
		logger.EXPECT().Debugf("Getting subscriptions for tags %v", triggerData.Tags)
		logger.EXPECT().Debugf("Subscription %s is disabled", "")

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})

	Convey("When subscription is nil", t, func() {
		mockCtrl := gomock.NewController(t)
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		logger := mock_moira_alert.NewMockLogger(mockCtrl)
		worker := FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: notifier.NewScheduler(dataBase, logger, notifierMetrics),
		}

		event := moira2.NotificationEvent{
			Metric:    "generate.event.1",
			State:     moira2.StateOK,
			OldState:  moira2.StateWARN,
			TriggerID: triggerData.ID,
		}

		dataBase.EXPECT().GetTrigger(event.TriggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(triggerData.Tags).Times(1).Return([]*moira2.SubscriptionData{nil}, nil)

		logger.EXPECT().Debugf("Processing trigger id %s for metric %s == %f, %s -> %s", event.TriggerID, event.Metric, moira2.UseFloat64(event.Value), event.OldState, event.State)
		logger.EXPECT().Debugf("Getting subscriptions for tags %v", triggerData.Tags)
		logger.EXPECT().Debugf("Subscription is nil")

		err := worker.processEvent(event)
		So(err, ShouldBeEmpty)
	})
}

func TestGetNotificationSubscriptions(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	logger, _ := logging.GetLogger("Events")
	worker := FetchEventsWorker{
		Database:  dataBase,
		Logger:    logger,
		Metrics:   notifierMetrics,
		Scheduler: notifier.NewScheduler(dataBase, logger, notifierMetrics),
	}

	Convey("Error GetSubscription", t, func() {
		event := moira2.NotificationEvent{
			State:          moira2.StateTEST,
			SubscriptionID: &subscription.ID,
		}
		err := fmt.Errorf("Oppps")
		dataBase.EXPECT().GetSubscription(*event.SubscriptionID).Return(moira2.SubscriptionData{}, err)
		sub, expected := worker.getNotificationSubscriptions(event)
		So(sub, ShouldBeNil)
		So(expected, ShouldResemble, fmt.Errorf("error while read subscription %s: %s", *event.SubscriptionID, err.Error()))
	})

	Convey("Error GetContact", t, func() {
		event := moira2.NotificationEvent{
			State:     moira2.StateTEST,
			ContactID: "1233",
		}
		err := fmt.Errorf("Oppps")
		dataBase.EXPECT().GetContact(event.ContactID).Return(moira2.ContactData{}, err)
		sub, expected := worker.getNotificationSubscriptions(event)
		So(sub, ShouldBeNil)
		So(expected, ShouldResemble, fmt.Errorf("error while read contact %s: %s", event.ContactID, err.Error()))
	})

}

func TestGoRoutine(t *testing.T) {
	Convey("When good subscription, should add new notification", t, func() {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
		logger, _ := logging.GetLogger("Events")
		scheduler := mock_scheduler.NewMockScheduler(mockCtrl)

		worker := &FetchEventsWorker{
			Database:  dataBase,
			Logger:    logger,
			Metrics:   notifierMetrics,
			Scheduler: scheduler,
		}

		event := moira2.NotificationEvent{
			Metric:         "generate.event.1",
			State:          moira2.StateOK,
			OldState:       moira2.StateWARN,
			TriggerID:      triggerData.ID,
			SubscriptionID: &subscription.ID,
		}
		emptyNotification := moira2.ScheduledNotification{}
		shutdown := make(chan struct{})

		dataBase.EXPECT().FetchNotificationEvent().Return(moira2.NotificationEvent{}, fmt.Errorf("3433434")).Do(func(f ...interface{}) {
			dataBase.EXPECT().FetchNotificationEvent().Return(event, nil).Do(func(f ...interface{}) {
				dataBase.EXPECT().FetchNotificationEvent().AnyTimes().Return(moira2.NotificationEvent{}, database.ErrNil)
			})
		})
		dataBase.EXPECT().GetTrigger(event.TriggerID).Times(1).Return(trigger, nil)
		dataBase.EXPECT().GetTagsSubscriptions(triggerData.Tags).Times(1).Return([]*moira2.SubscriptionData{&subscription}, nil)
		dataBase.EXPECT().GetContact(contact.ID).Times(1).Return(contact, nil)
		scheduler.EXPECT().ScheduleNotification(gomock.Any(), event, triggerData, contact, emptyNotification.Plotting, false, 0).Times(1).Return(&emptyNotification)
		dataBase.EXPECT().AddNotification(&emptyNotification).Times(1).Return(nil).Do(func(f ...interface{}) { close(shutdown) })

		worker.Start()
		waitTestEnd(shutdown, worker)
	})
}

func waitTestEnd(shutdown chan struct{}, worker *FetchEventsWorker) {
	select {
	case <-shutdown:
		worker.Stop()
		break
	case <-time.After(time.Second * 10):
		close(shutdown)
		break
	}
}

var warnValue float64 = 10
var errorValue float64 = 20

var triggerData = moira2.TriggerData{
	ID:         "triggerID-0000000000001",
	Name:       "test trigger",
	Targets:    []string{"test.target.5"},
	WarnValue:  warnValue,
	ErrorValue: errorValue,
	Tags:       []string{"test-tag"},
}

var trigger = moira2.Trigger{
	ID:         "triggerID-0000000000001",
	Name:       "test trigger",
	Targets:    []string{"test.target.5"},
	WarnValue:  &warnValue,
	ErrorValue: &errorValue,
	Tags:       []string{"test-tag"},
}

var contact = moira2.ContactData{
	ID:    "ContactID-000000000000001",
	Type:  "email",
	Value: "mail1@example.com",
}

var subscription = moira2.SubscriptionData{
	ID:                "subscriptionID-00000000000001",
	Enabled:           true,
	Tags:              []string{"test-tag"},
	Contacts:          []string{contact.ID},
	ThrottlingEnabled: true,
}

var subscription4 = moira2.SubscriptionData{
	ID:                "subscriptionID-00000000000004",
	Enabled:           true,
	Tags:              []string{"test-tag"},
	Contacts:          []string{contact.ID},
	ThrottlingEnabled: true,
}

var disabledSubscription = moira2.SubscriptionData{
	ID:                "subscriptionID-00000000000002",
	Enabled:           false,
	Tags:              []string{"test-tag"},
	Contacts:          []string{contact.ID},
	ThrottlingEnabled: true,
}

var subscriptionToIgnoreWarnings = moira2.SubscriptionData{
	ID:                "subscriptionID-00000000000003",
	Enabled:           true,
	Tags:              []string{"test-tag"},
	Contacts:          []string{contact.ID},
	ThrottlingEnabled: true,
	IgnoreWarnings:    true,
}

var subscriptionToIgnoreRecoverings = moira2.SubscriptionData{
	ID:                "subscriptionID-00000000000003",
	Enabled:           true,
	Tags:              []string{"test-tag"},
	Contacts:          []string{contact.ID},
	ThrottlingEnabled: true,
	IgnoreRecoverings: true,
}

var subscriptionToIgnoreWarningsAndRecoverings = moira2.SubscriptionData{
	ID:                "subscriptionID-00000000000003",
	Enabled:           true,
	Tags:              []string{"test-tag"},
	Contacts:          []string{contact.ID},
	ThrottlingEnabled: true,
	IgnoreWarnings:    true,
	IgnoreRecoverings: true,
}