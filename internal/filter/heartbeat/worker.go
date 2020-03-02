package heartbeat

import (
	"time"

	moira2 "github.com/moira-alert/moira/internal/moira"

	"gopkg.in/tomb.v2"

	"github.com/moira-alert/moira/internal/metrics"
)

// Worker is heartbeat worker realization
type Worker struct {
	database moira2.Database
	metrics  *metrics.FilterMetrics
	logger   moira2.Logger
	tomb     tomb.Tomb
}

// NewHeartbeatWorker creates new worker
func NewHeartbeatWorker(database moira2.Database, metrics *metrics.FilterMetrics, logger moira2.Logger) *Worker {
	return &Worker{
		database: database,
		metrics:  metrics,
		logger:   logger,
	}
}

// Start every 5 second takes TotalMetricsReceived metrics and save it to database, for self-checking
func (worker *Worker) Start() {
	worker.tomb.Go(func() error {
		count := worker.metrics.TotalMetricsReceived.Count()
		checkTicker := time.NewTicker(time.Second * 5)
		for {
			select {
			case <-worker.tomb.Dying():
				worker.logger.Info("Moira Filter Heartbeat stopped")
				return nil
			case <-checkTicker.C:
				newCount := worker.metrics.TotalMetricsReceived.Count()
				if newCount != count {
					worker.logger.Debugf("Heartbeat was updated: %v -> %v", count, newCount)
					if err := worker.database.UpdateMetricsHeartbeat(); err != nil {
						worker.logger.Infof("Save state failed: %s", err.Error())
					} else {
						count = newCount
					}
				}
			}
		}
	})
	worker.logger.Info("Moira Filter Heartbeat started")
}

// Stop heartbeat worker
func (worker *Worker) Stop() error {
	worker.tomb.Kill(nil)
	return worker.tomb.Wait()
}