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

/*

This file is a main file of cgroup module containing
- init
- TryInitStatReaders
- GetStandardStat

*/

package cgroup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

const (
	sliceSuffix = ".slice"
	scopeSuffix = ".scope"
)

var (
	baseCGroupPath    string = "/sys/fs/cgroup"
	KubePodCGroupPath string = "/sys/fs/cgroup/kubepods.slice"
	SystemCGroupPath  string = "/sys/fs/cgroup/system.slice"
)

type SliceHandler struct {
	statReaders   map[string][]StatReader
	CPUTopPath    string
	MemoryTopPath string
	IOTopPath     string
}

var SliceHandlerInstance *SliceHandler

func SetSliceHandler() {
	SliceHandlerInstance = InitSliceHandler()
}

func (s *SliceHandler) Init() {
	s.statReaders = make(map[string][]StatReader)
}

func (s *SliceHandler) GetStatReaders() map[string][]StatReader {
	return s.statReaders
}

func (s *SliceHandler) SetStatReaders(containerID string, statReaders []StatReader) {
	s.statReaders[containerID] = statReaders
}

func (s *SliceHandler) GetCPUTopPath() string {
	return s.CPUTopPath
}

func (s *SliceHandler) GetMemoryTopPath() string {
	return s.MemoryTopPath
}

func (s *SliceHandler) GetIOTopPath() string {
	return s.IOTopPath
}

func (s *SliceHandler) GetStats(containerID string) map[string]interface{} {
	if readers, exists := s.statReaders[containerID]; exists {
		values := make(map[string]interface{})
		for _, reader := range readers {
			newValues := reader.Read()
			for k, v := range newValues {
				values[k] = v
			}
		}
		return values
	}
	return map[string]interface{}{}
}

func InitSliceHandler() *SliceHandler {
	var handler *SliceHandler
	if _, err := os.Stat(KubePodCGroupPath); err == nil {
		handler = &SliceHandler{
			CPUTopPath:    KubePodCGroupPath,
			MemoryTopPath: KubePodCGroupPath,
			IOTopPath:     KubePodCGroupPath,
		}
	} else if _, err := os.Stat(SystemCGroupPath); err == nil {
		handler = &SliceHandler{
			CPUTopPath:    SystemCGroupPath,
			MemoryTopPath: SystemCGroupPath,
			IOTopPath:     SystemCGroupPath,
		}
	} else {
		handler = &SliceHandler{
			CPUTopPath:    filepath.Join(baseCGroupPath, "cpu"),
			MemoryTopPath: filepath.Join(baseCGroupPath, "memory"),
			IOTopPath:     filepath.Join(baseCGroupPath, "blkio"),
		}
	}
	handler.Init()
	klog.V(3).Infof("InitSliceHandler: %v", handler)
	return handler
}

func TryInitStatReaders(containerID string) {
	statReaders := SliceHandlerInstance.GetStatReaders()
	if _, exists := statReaders[containerID]; !exists {
		cpuTopPath := SliceHandlerInstance.GetCPUTopPath()
		memoryTopPath := SliceHandlerInstance.GetMemoryTopPath()
		ioTopPath := SliceHandlerInstance.GetIOTopPath()
		containerCPUPath := SearchByContainerID(cpuTopPath, containerID)
		containerMemoryPath := strings.Replace(containerCPUPath, cpuTopPath, memoryTopPath, 1)
		containerIOPath := strings.Replace(containerCPUPath, cpuTopPath, ioTopPath, 1)
		statReaders[containerID] = []StatReader{
			CPUStatReader{Path: containerCPUPath},
			MemoryStatReader{Path: containerMemoryPath},
			IOStatReader{Path: containerIOPath},
		}
	}
}

func GetStandardStat(containerID string) map[string]interface{} {
	stats := SliceHandlerInstance.GetStats(containerID)
	return convertToStandard(stats)
}

func searchBySuffix(topFolder, suffix string) string {
	found := filepath.Walk(topFolder,
		func(path string, info os.FileInfo, err error) error {
			if path == topFolder {
				return nil
			}
			if strings.HasSuffix(path, suffix) {
				return errors.New(path)
			}
			return nil
		})
	if found != nil {
		return found.Error()
	}
	return ""
}

func findContainerScope(path string) string {
	// Search a scope file, if we do, return it, the file might be
	// /sys/fs/cgroup/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-podd4f512ba_d18f_41ef_a3a7_0d169174b9ff.slice/crio-12b270d3b4ec20ad1fe09889bbf7faa153309d691adbaa660b110289a41f6434.scope
	scopePath := searchBySuffix(path, scopeSuffix)
	if scopePath != "" {
		klog.V(5).Infof("found scope file path %v", scopePath)
		return scopePath
	}

	// let's search subfolder with .slice
	slicePath := searchBySuffix(path, sliceSuffix)
	if slicePath == "" {
		return ""
	}
	return findContainerScope(slicePath)
}

func findExampleContainerID(slice *SliceHandler) string {
	topPath := slice.GetCPUTopPath()
	containerScopePath := findContainerScope(topPath)

	// if we are not able to find scope path, this means
	// availableMetrics is likely to be empty, we should
	// put log to warn user and return fast
	if containerScopePath == "" {
		klog.Infof("Not able to find any valid .scope file in %v, this likely cause all cgroup metrics to be 0", topPath)
		return ""
	}

	pathSplits := strings.Split(containerScopePath, "/")
	fileName := pathSplits[len(pathSplits)-1]
	scopeSplit := strings.Split(fileName, ".scope")[0]
	partSplits := strings.Split(scopeSplit, "-")
	return partSplits[len(partSplits)-1]
}

func GetAvailableCgroupMetrics() []string {
	var availableMetrics []string
	containerID := findExampleContainerID(SliceHandlerInstance)
	TryInitStatReaders(containerID)
	stats := GetStandardStat(containerID)
	for metric := range stats {
		availableMetrics = append(availableMetrics, metric)
	}
	return availableMetrics
}
