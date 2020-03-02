package pushover

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	moira2 "github.com/moira-alert/moira/internal/moira"

	"github.com/gregdel/pushover"
	"github.com/moira-alert/moira/internal/logging/go-logging"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSender_Init(t *testing.T) {
	logger, _ := logging.ConfigureLog("stdout", "debug", "test")
	Convey("Empty map", t, func() {
		sender := Sender{}
		err := sender.Init(map[string]string{}, logger, nil, "")
		So(err, ShouldResemble, fmt.Errorf("can not read pushover api_token from config"))
		So(sender, ShouldResemble, Sender{})
	})

	Convey("Settings has api_token", t, func() {
		sender := Sender{}
		err := sender.Init(map[string]string{"api_token": "123"}, logger, nil, "")
		So(err, ShouldBeNil)
		So(sender, ShouldResemble, Sender{apiToken: "123", client: pushover.New("123"), logger: logger})
	})

	Convey("Settings has all data", t, func() {
		sender := Sender{}
		location, _ := time.LoadLocation("UTC")
		err := sender.Init(map[string]string{"api_token": "123", "front_uri": "321"}, logger, location, "")
		So(err, ShouldBeNil)
		So(sender, ShouldResemble, Sender{apiToken: "123", client: pushover.New("123"), frontURI: "321", logger: logger, location: location})
	})
}

func TestGetPushoverPriority(t *testing.T) {
	sender := Sender{}
	Convey("All events has OK state", t, func() {
		priority := sender.getMessagePriority([]moira2.NotificationEvent{{State: moira2.StateOK}, {State: moira2.StateOK}, {State: moira2.StateOK}})
		So(priority, ShouldResemble, pushover.PriorityNormal)
	})

	Convey("One of events has WARN state", t, func() {
		priority := sender.getMessagePriority([]moira2.NotificationEvent{{State: moira2.StateOK}, {State: moira2.StateWARN}, {State: moira2.StateOK}})
		So(priority, ShouldResemble, pushover.PriorityHigh)
	})

	Convey("One of events has NODATA state", t, func() {
		priority := sender.getMessagePriority([]moira2.NotificationEvent{{State: moira2.StateOK}, {State: moira2.StateNODATA}, {State: moira2.StateOK}})
		So(priority, ShouldResemble, pushover.PriorityHigh)
	})

	Convey("One of events has ERROR state", t, func() {
		priority := sender.getMessagePriority([]moira2.NotificationEvent{{State: moira2.StateOK}, {State: moira2.StateERROR}, {State: moira2.StateOK}})
		So(priority, ShouldResemble, pushover.PriorityEmergency)
	})

	Convey("One of events has EXCEPTION state", t, func() {
		priority := sender.getMessagePriority([]moira2.NotificationEvent{{State: moira2.StateOK}, {State: moira2.StateEXCEPTION}, {State: moira2.StateOK}})
		So(priority, ShouldResemble, pushover.PriorityEmergency)
	})

	Convey("Events has WARN and ERROR states", t, func() {
		priority := sender.getMessagePriority([]moira2.NotificationEvent{{State: moira2.StateOK}, {State: moira2.StateWARN}, {State: moira2.StateERROR}})
		So(priority, ShouldResemble, pushover.PriorityEmergency)
	})

	Convey("Events has ERROR and WARN states", t, func() {
		priority := sender.getMessagePriority([]moira2.NotificationEvent{{State: moira2.StateOK}, {State: moira2.StateERROR}, {State: moira2.StateWARN}})
		So(priority, ShouldResemble, pushover.PriorityEmergency)
	})
}

func TestBuildMoiraMessage(t *testing.T) {
	location, _ := time.LoadLocation("UTC")
	sender := Sender{location: location}
	value := float64(123)

	Convey("Build Moira Message tests", t, func() {
		event := moira2.NotificationEvent{
			Value:     &value,
			Timestamp: 150000000,
			Metric:    "Metric",
			OldState:  moira2.StateOK,
			State:     moira2.StateNODATA,
		}

		Convey("Print moira message with one event", func() {
			actual := sender.buildMessage([]moira2.NotificationEvent{event}, false)
			expected := "02:40: Metric = 123 (OK to NODATA)\n"
			So(actual, ShouldResemble, expected)
		})

		Convey("Print moira message with one event and message", func() {
			var interval int64 = 24
			event.MessageEventInfo = &moira2.EventInfo{Interval: &interval}
			actual := sender.buildMessage([]moira2.NotificationEvent{event}, false)
			expected := "02:40: Metric = 123 (OK to NODATA). This metric has been in bad state for more than 24 hours - please, fix.\n"
			So(actual, ShouldResemble, expected)
		})

		Convey("Print moira message with one event and throttled", func() {
			actual := sender.buildMessage([]moira2.NotificationEvent{event}, true)
			expected := `02:40: Metric = 123 (OK to NODATA)

Please, fix your system or tune this trigger to generate less events.`
			So(actual, ShouldResemble, expected)
		})

		Convey("Print moira message with 6 events", func() {
			actual := sender.buildMessage([]moira2.NotificationEvent{event, event, event, event, event, event}, false)
			expected := `02:40: Metric = 123 (OK to NODATA)
02:40: Metric = 123 (OK to NODATA)
02:40: Metric = 123 (OK to NODATA)
02:40: Metric = 123 (OK to NODATA)
02:40: Metric = 123 (OK to NODATA)

...and 1 more events.`
			So(actual, ShouldResemble, expected)
		})
	})
}

func TestBuildTitle(t *testing.T) {
	sender := Sender{}
	Convey("Build title with three events with max ERROR state and two tags", t, func() {
		title := sender.buildTitle([]moira2.NotificationEvent{{State: moira2.StateERROR}, {State: moira2.StateWARN}, {State: moira2.StateWARN}, {State: moira2.StateOK}}, moira2.TriggerData{Tags: []string{"tag1", "tag2"}, Name: "Name"})
		So(title, ShouldResemble, "ERROR Name [tag1][tag2] (4)")
	})
	Convey("Build title with three events with max ERROR state empty trigger", t, func() {
		title := sender.buildTitle([]moira2.NotificationEvent{{State: moira2.StateERROR}, {State: moira2.StateWARN}, {State: moira2.StateWARN}, {State: moira2.StateOK}}, moira2.TriggerData{})
		So(title, ShouldResemble, "ERROR   (4)")
	})
	Convey("Build title that exceeds the title limit", t, func() {
		var reallyLongTag string
		for i := 0; i < 30; i++ {
			reallyLongTag = reallyLongTag + "randomstring"
		}
		title := sender.buildTitle([]moira2.NotificationEvent{{State: moira2.StateERROR}, {State: moira2.StateWARN}, {State: moira2.StateWARN}, {State: moira2.StateOK}}, moira2.TriggerData{Tags: []string{"tag1", "tag2", "tag3", reallyLongTag, "tag4"}, Name: "Name"})
		So(title, ShouldResemble, "ERROR Name [tag1][tag2][tag3].... (4)")
	})
}

func TestMakePushoverMessage(t *testing.T) {
	location, _ := time.LoadLocation("UTC")
	logger, _ := logging.ConfigureLog("stdout", "debug", "test")

	value := float64(123)
	sender := Sender{
		frontURI: "https://my-moira.com",
		location: location,
		logger:   logger,
	}
	Convey("Just build PushoverMessage", t, func() {
		event := []moira2.NotificationEvent{{
			Value:     &value,
			Timestamp: 150000000,
			Metric:    "Metric",
			OldState:  moira2.StateOK,
			State:     moira2.StateERROR,
		},
		}
		trigger := moira2.TriggerData{
			ID:   "SomeID",
			Name: "TriggerName",
			Tags: []string{"tag1", "tag2"},
		}
		contact := moira2.ContactData{
			Value: "123",
		}
		expected := &pushover.Message{
			Timestamp: 150000000,
			Retry:     5 * time.Minute,
			Expire:    time.Hour,
			URL:       "https://my-moira.com/trigger/SomeID",
			Priority:  pushover.PriorityEmergency,
			Title:     "ERROR TriggerName [tag1][tag2] (1)",
			Message:   "02:40: Metric = 123 (OK to ERROR)\n",
		}
		expected.AddAttachment(bytes.NewReader([]byte{1, 0, 1}))
		So(sender.makePushoverMessage(event, contact, trigger, []byte{1, 0, 1}, false), ShouldResemble, expected)
	})
}