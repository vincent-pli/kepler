/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package collector

import (
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

// updateNodeResourceUsage updates node resource usage with the total container resource usage
// The container metrics are for the kubernetes containers and system/OS processes
// TODO: verify if the cgroup metrics are also accounting for the OS, not only containers
func (c *Collector) updateNodeResourceUsage() {
	c.NodeMetrics.AddNodeResUsageFromContainerResUsage(c.ContainersMetrics)
}

// updateMeasuredNodeEnergy updates the node platfomr power consumption, i.e, the node total power consumption
func (c *Collector) updatePlatformEnergy() {
	nodePlatformEnergy := map[string]float64{}
	if c.acpiPowerMeter.IsPowerSupported() {
		nodePlatformEnergy, _ = c.acpiPowerMeter.GetEnergyFromHost()
	} else if model.IsNodePlatformPowerModelEnabled() {
		nodePlatformEnergy = model.GetEstimatedNodePlatformPower(c.NodeMetrics)
	}
	c.NodeMetrics.AddLastestPlatformEnergy(nodePlatformEnergy)
}

// updateMeasuredNodeEnergy updates each node component power consumption, i.e., the CPU core, uncore, package/socket and DRAM
func (c *Collector) updateNodeComponentsEnergy() {
	nodeComponentsEnergy := map[int]source.NodeComponentsEnergy{}
	if components.IsSystemCollectionSupported() {
		nodeComponentsEnergy = components.GetNodeComponentsEnergy()
	} else if model.IsNodeComponentPowerModelEnabled() {
		nodeComponentsEnergy = model.GetNodeComponentPowers(c.NodeMetrics)
	}
	c.NodeMetrics.AddNodeComponentsEnergy(nodeComponentsEnergy)
}

// updateNodeGPUEnergy updates each GPU power consumption. Right now we don't support other types of accelerators
func (c *Collector) updateNodeGPUEnergy() {
	if config.EnabledGPU {
		gpuEnergy := accelerator.GetGpuEnergyPerGPU()
		c.NodeMetrics.AddNodeGPUEnergy(gpuEnergy)
	}
}

// updateNodeAvgCPUFrequency updates the average CPU frequency in each core
func (c *Collector) updateNodeAvgCPUFrequency() {
	c.NodeCPUFrequency = c.acpiPowerMeter.GetCPUCoreFrequency()
}

// updateNodeEnergyMetrics updates the node energy consumption of each component
func (c *Collector) updateNodeEnergyMetrics() {
	c.updatePlatformEnergy()
	c.updateNodeComponentsEnergy()
	c.updateNodeAvgCPUFrequency()
	c.updateNodeGPUEnergy()
}
