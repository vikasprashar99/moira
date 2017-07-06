package notifier

import (
	"fmt"
	"github.com/moira-alert/moira-alert"
	graphite "github.com/moira-alert/moira-alert/metrics/graphite/go-metrics"
	"github.com/moira-alert/moira-alert/notifier/scheduler"
	"sync"
	"time"
)

type Notifier struct {
	waitGroup sync.WaitGroup
	senders   map[string]chan NotificationPackage
	logger    moira_alert.Logger
	database  moira_alert.Database
	scheduler scheduler.Schedule
	config    Config
}

func Init(database moira_alert.Database, logger moira_alert.Logger, config Config) Notifier {
	return Notifier{
		senders:   make(map[string]chan NotificationPackage),
		logger:    logger,
		database:  database,
		scheduler: scheduler.Init(database, logger),
		config:    config,
	}
}

func (notifier *Notifier) Send(pkg *NotificationPackage, waitGroup *sync.WaitGroup) {
	ch, found := notifier.senders[pkg.Contact.Type]
	if !found {
		notifier.resend(pkg, fmt.Sprintf("Unknown contact type [%s]", pkg))
		return
	}
	waitGroup.Add(1)
	go func(pkg *NotificationPackage) {
		defer waitGroup.Done()
		notifier.logger.Debugf("Start sending %s", pkg)
		select {
		case ch <- *pkg:
			break
		case <-time.After(notifier.config.SendingTimeout):
			notifier.resend(pkg, fmt.Sprintf("Timeout sending %s", pkg))
			break
		}
	}(pkg)
}

func (notifier *Notifier) GetSendersHash() map[string]bool {
	hash := make(map[string]bool)
	for key, _ := range notifier.senders {
		hash[key] = true
	}
	return hash
}

func (notifier *Notifier) resend(pkg *NotificationPackage, reason string) {
	if pkg.DontResend {
		return
	}
	graphite.NotifierMetric.SendingFailed.Mark(1)
	if metric, found := graphite.NotifierMetric.SendersFailedMetrics[pkg.Contact.Type]; found {
		metric.Mark(1)
	}
	notifier.logger.Warningf("Can't send message after %d try: %s. Retry again after 1 min", pkg.FailCount, reason)
	if time.Duration(pkg.FailCount)*time.Minute > notifier.config.ResendingTimeout {
		notifier.logger.Error("Stop resending. Notification interval is timed out")
	} else {
		for _, event := range pkg.Events {
			notification := notifier.scheduler.ScheduleNotification(event, pkg.Trigger, pkg.Contact, pkg.Throttled, pkg.FailCount+1)
			if err := notifier.database.AddNotification(notification); err != nil {
				notifier.logger.Errorf("Failed to save scheduled notification: %s", err)
			}
		}
	}
}

func (notifier *Notifier) run(sender moira_alert.Sender, ch chan NotificationPackage) {
	defer notifier.waitGroup.Done()
	for pkg := range ch {
		err := sender.SendEvents(pkg.Events, pkg.Contact, pkg.Trigger, pkg.Throttled)
		if err == nil {
			graphite.NotifierMetric.SendersOkMetrics[pkg.Contact.Type].Mark(1)
		} else {
			notifier.resend(&pkg, err.Error())
		}
	}
}