package client

import (
	"errors"
	"math/rand"
)

var (
	ErrNoInstances = errors.New("kvcache: no instances available")
)

type InstancePicker struct {
	registry *InstanceRegistry
	node     string
	threshold float64
}

func NewInstancePicker(registry *InstanceRegistry, node string, threshold float64) *InstancePicker {
	return &InstancePicker{
		registry:  registry,
		node:      node,
		threshold: threshold,
	}
}

func (p *InstancePicker) Pick() (*InstanceInfo, error) {
	local := p.registry.GetLocalInstances()
	if inst := p.randomHealthy(local); inst != nil {
		return inst, nil
	}

	remote := p.registry.GetRemoteInstances()
	if inst := p.randomHealthy(remote); inst != nil {
		return inst, nil
	}

	if len(local) > 0 {
		return local[rand.Intn(len(local))], nil
	}

	all := p.registry.GetActiveInstances()
	if len(all) > 0 {
		for _, inst := range all {
			return inst, nil
		}
	}

	return nil, ErrNoInstances
}

func (p *InstancePicker) randomHealthy(instances []*InstanceInfo) *InstanceInfo {
	if len(instances) == 0 {
		return nil
	}

	var healthy []*InstanceInfo
	for _, inst := range instances {
		if inst.UsagePercent() < p.threshold {
			healthy = append(healthy, inst)
		}
	}

	if len(healthy) > 0 {
		return healthy[rand.Intn(len(healthy))]
	}
	return nil
}
