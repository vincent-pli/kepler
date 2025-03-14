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
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	accelerator_source "github.com/sustainable-computing-io/kepler/pkg/power/accelerator/source"
	"k8s.io/klog/v2"
)

var (
	// lastUtilizationTimestamp represents the CPU timestamp in microseconds at which utilization samples were last read
	lastUtilizationTimestamp time.Time = time.Now()
)

// updateBPFMetrics reads the BPF tables with process/pid/cgroupid metrics (CPU time, available HW counters)
func (c *Collector) updateAcceleratorMetrics() {
	var err error
	var processesUtilization map[uint32]accelerator_source.ProcessUtilizationSample
	// calculate the gpu's processes energy consumption for each gpu
	for _, device := range accelerator.GetGpus() {
		if processesUtilization, err = accelerator.GetProcessResourceUtilizationPerDevice(device, time.Since(lastUtilizationTimestamp)); err != nil {
			klog.V(2).Infoln(err)
		}

		var containerID string
		for pid, processUtilization := range processesUtilization {
			if containerID, err = cgroup.GetContainerIDFromPID(uint64(pid)); err != nil {
				klog.V(5).Infof("failed to resolve container for Pid %v: %v, set containerID=%s", pid, err, c.systemProcessName)
				containerID = c.systemProcessName
			}

			c.createContainersMetricsIfNotExist(containerID, 0, uint64(pid), false)

			if err = c.ContainersMetrics[containerID].CounterStats[config.GPUSMUtilization].AddNewCurr(uint64(processUtilization.SmUtil)); err != nil {
				klog.V(5).Infoln(err)
			}
			if err = c.ContainersMetrics[containerID].CounterStats[config.GPUMemUtilization].AddNewCurr(uint64(processUtilization.MemUtil)); err != nil {
				klog.V(5).Infoln(err)
			}
		}
	}

	lastUtilizationTimestamp = time.Now()
}
