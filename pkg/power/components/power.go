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

package components

import (
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

type powerInterface interface {
	// GetEnergyFromDram returns mJ in DRAM
	GetEnergyFromDram() (uint64, error)
	// GetEnergyFromDram returns mJ in CPU cores
	GetEnergyFromCore() (uint64, error)
	// GetEnergyFromDram returns mJ in uncore (i.e. iGPU)
	GetEnergyFromUncore() (uint64, error)
	// GetEnergyFromDram returns mJ in package
	GetEnergyFromPackage() (uint64, error)
	// GetNodeComponentsEnergy returns set of mJ per RAPL components
	GetNodeComponentsEnergy() map[int]source.NodeComponentsEnergy
	// StopPower stops the collection
	StopPower()
	// IsSystemCollectionSupported returns if it is possible to use this collector
	IsSystemCollectionSupported() bool
}

var (
	dummyImpl                = &source.PowerDummy{}
	sysfsImpl                = &source.PowerSysfs{}
	msrImpl                  = &source.PowerMSR{}
	powerImpl powerInterface = sysfsImpl
	useMSR                   = false // it looks MSR on kvm or hyper-v is not working
)

func init() {
	if sysfsImpl.IsSystemCollectionSupported() /*&& false*/ {
		klog.V(1).Infoln("use sysfs to obtain power")
		powerImpl = sysfsImpl
	} else {
		if msrImpl.IsSystemCollectionSupported() && useMSR {
			klog.V(1).Infoln("use MSR to obtain power")
			powerImpl = msrImpl
		} else {
			klog.V(1).Infoln("power not supported")
			powerImpl = dummyImpl
		}
	}
}

func GetEnergyFromDram() (uint64, error) {
	return powerImpl.GetEnergyFromDram()
}

func GetEnergyFromCore() (uint64, error) {
	return powerImpl.GetEnergyFromCore()
}

func GetEnergyFromUncore() (uint64, error) {
	return powerImpl.GetEnergyFromUncore()
}

func GetEnergyFromPackage() (uint64, error) {
	return powerImpl.GetEnergyFromPackage()
}

func GetNodeComponentsEnergy() map[int]source.NodeComponentsEnergy {
	return powerImpl.GetNodeComponentsEnergy()
}

func IsSystemCollectionSupported() bool {
	return powerImpl.IsSystemCollectionSupported()
}

func StopPower() {
	powerImpl.StopPower()
}
