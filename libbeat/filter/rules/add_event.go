package rules

import (
	"fmt"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/filter"
)

type AddEvent struct {
	Cond *filter.Condition
}

type AddEventConfig struct {
	filter.ConditionConfig `config:",inline"`
}

func init() {
	if err := filter.RegisterPlugin("add_event", newAddEvent); err != nil {
		panic(err)
	}
}

func newAddEvent(c common.Config) (filter.FilterRule, error) {

	f := AddEvent{}

	if err := f.CheckConfig(c); err != nil {
		return nil, err
	}

	config := AddEventConfig{}

	err := c.Unpack(&config)
	if err != nil {
		return nil, fmt.Errorf("fail to unpack the add_event configuration: %s", err)
	}

	cond, err := filter.NewCondition(config.ConditionConfig)
	if err != nil {
		return nil, err
	}
	f.Cond = cond

	return &f, nil
}

func (f *AddEvent) CheckConfig(c common.Config) error {

	for _, field := range c.GetFields() {
		if !filter.AvailableCondition(field) {
			return fmt.Errorf("unexpected %s option in the add_event configuration", field)
		}
	}
	return nil
}

func (f *AddEvent) Filter(event common.MapStr) (common.MapStr, error) {

	/* If the condition is empty, none of the event will be added*/
	if f.Cond != nil && f.Cond.Check(event) {
		return event, nil
	}

	// return event=nil to delete the entire event
	return nil, nil
}

func (f AddEvent) String() string {
	if f.Cond != nil {
		return "add_event, condition=" + f.Cond.String()
	}
	return "add_event"
}
