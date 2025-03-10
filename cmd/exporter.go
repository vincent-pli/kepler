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

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/manager"
	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	kversion "github.com/sustainable-computing-io/kepler/pkg/version"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"

	"k8s.io/klog/v2"
)

const (
	// to change these msg, you also need to update the e2e test
	finishingMsg = "Exiting..."
	startedMsg   = "Started Kepler in %s"
)

var (
	address                      = flag.String("address", "0.0.0.0:8888", "bind address")
	metricsPath                  = flag.String("metrics-path", "/metrics", "metrics path")
	enableGPU                    = flag.Bool("enable-gpu", false, "whether enable gpu (need to have libnvidia-ml installed)")
	modelServerEndpoint          = flag.String("model-server-endpoint", "", "model server endpoint")
	enabledEBPFCgroupID          = flag.Bool("enable-cgroup-id", true, "whether enable eBPF to collect cgroup id (must have kernel version >= 4.18 and cGroup v2)")
	exposeHardwareCounterMetrics = flag.Bool("expose-hardware-counter-metrics", true, "whether expose hardware counter as prometheus metrics")
	cpuProfile                   = flag.String("cpuprofile", "", "dump cpu profile to a file")
)

func healthProbe(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`ok`))
	if err != nil {
		klog.Fatalf("%s", fmt.Sprintf("failed to write response: %v", err))
	}
}

func finalizing() {
	exitCode := 10
	klog.Infoln(finishingMsg)
	klog.FlushAndExit(klog.ExitFlushTimeout, exitCode)
}

func main() {
	start := time.Now()
	defer finalizing()
	klog.InitFlags(nil)
	flag.Parse()

	klog.Infof("Kepler running on version: %s", kversion.Version)
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err == nil {
			profErr := pprof.StartCPUProfile(f)
			if profErr == nil {
				sigs := make(chan os.Signal, 1)
				signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
				go func() {
					<-sigs
					fmt.Println("exiting...")
					pprof.StopCPUProfile()
					os.Exit(0)
				}()
			}
		}
	}

	config.SetEnabledEBPFCgroupID(*enabledEBPFCgroupID)
	config.SetEnabledHardwareCounterMetrics(*exposeHardwareCounterMetrics)
	config.SetEnabledGPU(*enableGPU)

	cgroup.SetSliceHandler()

	if modelServerEndpoint != nil {
		klog.Infof("Initializing the Model Server")
		config.SetModelServerEndpoint(*modelServerEndpoint)
		model.InitEstimateFunctions(collector_metric.ContainerMetricNames, collector_metric.NodeMetadataNames, collector_metric.NodeMetadataValues)
	}

	collector_metric.InitAvailableParamAndMetrics()

	if *enableGPU {
		klog.Infof("Initializing the GPU collector")
		err := accelerator.Init()
		if err == nil {
			defer accelerator.Shutdown()
		}
	}

	m := manager.New()
	prometheus.MustRegister(version.NewCollector("kepler_exporter"))
	prometheus.MustRegister(m.PrometheusCollector)
	defer m.MetricCollector.Destroy()
	defer components.StopPower()

	// starting a new gorotine to collect data and report metrics
	if err := m.Start(); err != nil {
		klog.Infof("%s", fmt.Sprintf("failed to start : %v", err))
	}

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/healthz", healthProbe)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
                        <head><title>Energy Stats Exporter</title></head>
                        <body>
                        <h1>Energy Stats Exporter</h1>
                        <p><a href="` + *metricsPath + `">Metrics</a></p>
                        </body>
                        </html>`))
		if err != nil {
			klog.Fatalf("%s", fmt.Sprintf("failed to write response: %v", err))
		}
	})

	ch := make(chan error)
	go func() {
		ch <- http.ListenAndServe(*address, nil)
	}()

	klog.Infof(startedMsg, time.Since(start))
	klog.Flush() // force flush to parse the start msg in the e2e test
	err := <-ch
	klog.Fatalf("%s", fmt.Sprintf("failed to bind on %s: %v", *address, err))
}
