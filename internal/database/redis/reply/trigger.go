package reply

import (
	"encoding/json"
	"fmt"
	"strconv"

	moira2 "github.com/moira-alert/moira/internal/moira"

	"github.com/gomodule/redigo/redis"
	"github.com/moira-alert/moira/internal/database"
)

// Duty hack for moira.Trigger TTL int64 and stored trigger TTL string compatibility
type triggerStorageElement struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	Desc             *string              `json:"desc,omitempty"`
	Targets          []string             `json:"targets"`
	WarnValue        *float64             `json:"warn_value"`
	ErrorValue       *float64             `json:"error_value"`
	TriggerType      string               `json:"trigger_type,omitempty"`
	Tags             []string             `json:"tags"`
	TTLState         *moira2.TTLState     `json:"ttl_state,omitempty"`
	Schedule         *moira2.ScheduleData `json:"sched,omitempty"`
	Expression       *string              `json:"expr,omitempty"`
	PythonExpression *string              `json:"expression,omitempty"`
	Patterns         []string             `json:"patterns"`
	TTL              string               `json:"ttl,omitempty"`
	IsRemote         bool                 `json:"is_remote"`
	MuteNewMetrics   bool                 `json:"mute_new_metrics,omitempty"`
}

func (storageElement *triggerStorageElement) toTrigger() moira2.Trigger {
	return moira2.Trigger{
		ID:               storageElement.ID,
		Name:             storageElement.Name,
		Desc:             storageElement.Desc,
		Targets:          storageElement.Targets,
		WarnValue:        storageElement.WarnValue,
		ErrorValue:       storageElement.ErrorValue,
		TriggerType:      storageElement.TriggerType,
		Tags:             storageElement.Tags,
		TTLState:         storageElement.TTLState,
		Schedule:         storageElement.Schedule,
		Expression:       storageElement.Expression,
		PythonExpression: storageElement.PythonExpression,
		Patterns:         storageElement.Patterns,
		TTL:              getTriggerTTL(storageElement.TTL),
		IsRemote:         storageElement.IsRemote,
		MuteNewMetrics:   storageElement.MuteNewMetrics,
	}
}

func toTriggerStorageElement(trigger *moira2.Trigger, triggerID string) *triggerStorageElement {
	return &triggerStorageElement{
		ID:               triggerID,
		Name:             trigger.Name,
		Desc:             trigger.Desc,
		Targets:          trigger.Targets,
		WarnValue:        trigger.WarnValue,
		ErrorValue:       trigger.ErrorValue,
		TriggerType:      trigger.TriggerType,
		Tags:             trigger.Tags,
		TTLState:         trigger.TTLState,
		Schedule:         trigger.Schedule,
		Expression:       trigger.Expression,
		PythonExpression: trigger.PythonExpression,
		Patterns:         trigger.Patterns,
		TTL:              getTriggerTTLString(trigger.TTL),
		IsRemote:         trigger.IsRemote,
		MuteNewMetrics:   trigger.MuteNewMetrics,
	}
}

func getTriggerTTL(ttlString string) int64 {
	if ttlString == "" {
		return 0
	}
	ttl, _ := strconv.ParseInt(ttlString, 10, 64)
	return ttl
}

func getTriggerTTLString(ttl int64) string {
	return fmt.Sprintf("%v", ttl)
}

// Trigger converts redis DB reply to moira.Trigger object
func Trigger(rep interface{}, err error) (moira2.Trigger, error) {
	bytes, err := redis.Bytes(rep, err)
	if err != nil {
		if err == redis.ErrNil {
			return moira2.Trigger{}, database.ErrNil
		}
		return moira2.Trigger{}, fmt.Errorf("failed to read trigger: %s", err.Error())
	}
	triggerSE := &triggerStorageElement{}
	err = json.Unmarshal(bytes, triggerSE)
	if err != nil {
		return moira2.Trigger{}, fmt.Errorf("failed to parse trigger json %s: %s", string(bytes), err.Error())
	}

	trigger := triggerSE.toTrigger()
	return trigger, nil
}

// GetTriggerBytes marshal moira.Trigger to bytes array
func GetTriggerBytes(triggerID string, trigger *moira2.Trigger) ([]byte, error) {
	triggerSE := toTriggerStorageElement(trigger, triggerID)
	bytes, err := json.Marshal(triggerSE)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trigger: %s", err.Error())
	}
	return bytes, nil
}