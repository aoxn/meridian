/*
Copyright 2023.

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
	"github.com/aoxn/meridian"
	"github.com/aoxn/meridian/internal/controller"
	"github.com/aoxn/meridian/internal/crds"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/sign"
	ravenv1beta1 "github.com/openyurtio/openyurt/pkg/apis/raven/v1beta1"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	//	_ "k8s.io/client-go/plugin/pkg/client/auth"

	meridianv1 "github.com/aoxn/meridian/api/v1"
	_ "github.com/aoxn/meridian/internal/cloud/alibaba"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	//+kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(meridianv1.AddToScheme(scheme))

	utilruntime.Must(ravenv1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	fmt.Println(meridian.Version)

	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":7080", "The address the metric service binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":7081", "The address the probe service binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	klog.InitFlags(flag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	ctrl.SetLogger(klog.NewKlogr())
	setDefaults()
	var (
		err           error
		webhookCrtDir = "/tmp/webhook-cert"
	)
	err = meridianv1.LoadConfig()
	if err != nil {
		klog.Errorf("load meridian config: %s", err.Error())
		klog.Infof("use default empty config to load meridian")
	}
	setServerCrt(webhookCrtDir)

	var restcfg = ctrl.GetConfigOrDie()
	klog.Info("auto register crds...")
	if err := crds.InitializeCRD(restcfg); err != nil {
		klog.Errorf("register tool failed with error: %s", err.Error())
		os.Exit(1)
	}

	options := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "2fe89953.meridian.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,

		WebhookServer: webhook.NewServer(webhook.Options{Port: 8443, CertDir: webhookCrtDir}),
	}
	mgr, err := ctrl.NewManager(restcfg, options)
	if err != nil {
		klog.Errorf("unable to start manager: %s", err.Error())
		os.Exit(1)
	}

	//if err = (&controller.MasterSetReconciler{
	//	Client: mgr.GetClient(),
	//	Scheme: mgr.GetScheme(),
	//}).SetupWithManager(mgr); err != nil {
	//	klog.Errorf("unable to create controller:%s", err.Error())
	//	os.Exit(1)
	//}
	err = controller.Add(mgr)
	if err != nil {
		klog.Errorf("unable to add controller:%s", err.Error())
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		klog.Errorf("unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		klog.Errorf("unable to set up ready check: %s", err.Error())
		os.Exit(1)
	}

	klog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Errorf("problem running manager:%s", err.Error())
		os.Exit(1)
	}
}

func setDefaults() {
}

func setServerCrt(dir string) {
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		panic(err)
	}
	ca := meridianv1.G.Config.Server.WebhookCA
	if len(ca) == 0 {
		klog.Infof("debug: %t", meridianv1.G.Debug)
		if !meridianv1.G.Debug {
			panic("webhook ca should not be empty online")
		} else {
			klog.Infof("empty tls.ca, " +
				"try to generate self signed cert")
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				panic(err.Error())
			}
			exist, err := tool.FileExist(filepath.Join(dir, "tls.crt"))
			if err != nil {
				klog.Errorf("lookup file with error: %s", err.Error())
			}
			if !exist {
				ca, err = sign.GenerateServerCert(dir)
				if err != nil {
					panic(err.Error())
				}
			}
		}
	} else {
		klog.Infof("using existing webhook crt: %d, %d, %d",
			len(meridianv1.G.Config.Server.WebhookCA),
			len(meridianv1.G.Config.Server.WebhookTLSCert),
			len(meridianv1.G.Config.Server.WebhookTLSKey),
		)
		for k, v := range map[string][]byte{
			"tls.ca":  meridianv1.G.Config.Server.WebhookCA,
			"tls.crt": meridianv1.G.Config.Server.WebhookTLSCert,
			"tls.key": meridianv1.G.Config.Server.WebhookTLSKey,
		} {
			err := os.WriteFile(path.Join(dir, k), v, 0755)
			if err != nil {
				panic(fmt.Sprintf("write webhook tls: %s", err.Error()))
			}
		}
	}
	meridianv1.G.WebhookCA = ca
}
