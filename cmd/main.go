/*
Copyright 2024.

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
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spf13/cobra"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	"github.com/cubrid/cubrid-operator/internal/controller"
	"github.com/cubrid/cubrid-operator/pkg/certmanager"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	// Operator flags
	operatorMetricsAddr  string
	operatorProbeAddr    string
	enableLeaderElection bool

	// Webhook flags
	webhookMetricsAddr     string
	webhookProbeAddr       string
	webhookPort            int
	webhookServiceName     string
	webhookNamespace       string
	webhookCertDir         string
	webhookCertManagerType string
	webhookSecretName      string
	webhookMutatingName    string
	webhookValidatingName  string
	webhookDeploymentName  string
)

func init() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(cubridv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	// Operator flags
	rootCmd.PersistentFlags().StringVar(&operatorMetricsAddr, "metrics-bind-address", ":8080", "The address the operator metric endpoint binds to.")
	rootCmd.PersistentFlags().StringVar(&operatorProbeAddr, "health-probe-bind-address", ":8081", "The address the operator probe endpoint binds to.")
	rootCmd.PersistentFlags().BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	// Webhook flags
	webhookCmd.Flags().StringVar(&webhookMetricsAddr, "metrics-bind-address", ":8082", "The address the webhook metric endpoint binds to.")
	webhookCmd.Flags().StringVar(&webhookProbeAddr, "health-probe-bind-address", ":8083", "The address the webhook probe endpoint binds to.")
	webhookCmd.Flags().IntVar(&webhookPort, "webhook-port", 8443, "The port the webhook server serves on.")
	webhookCmd.Flags().StringVar(&webhookServiceName, "webhook-service-name", "", "The name of the webhook service")
	webhookCmd.Flags().StringVar(&webhookNamespace, "webhook-namespace", "cubrid", "The namespace where the webhook server is deployed")
	webhookCmd.Flags().StringVar(&webhookCertDir, "webhook-cert-dir", certmanager.DefaultCertificateDir, "The directory where certificates are stored")
	webhookCmd.Flags().StringVar(&webhookCertManagerType, "webhook-cert-manager-type", "internal", "The type of cert-manager to use (internal or external)")
	webhookCmd.Flags().StringVar(&webhookSecretName, "webhook-secret-name", "", "The name of the secret used by the webhook server")
	webhookCmd.Flags().StringVar(&webhookMutatingName, "webhook-mutating-name", "", "The name of the webhook mutating")
	webhookCmd.Flags().StringVar(&webhookValidatingName, "webhook-validating-name", "", "The name of the webhook validating")
	webhookCmd.Flags().StringVar(&webhookDeploymentName, "webhook-deployment-name", "cubrid-operator-webhook-dep", "The name of the webhook deployment")
}

var rootCmd = &cobra.Command{
	Use:   "controller",
	Short: "Run the controller",
	// Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		opts := zap.Options{
			Development: true,
		}
		opts.BindFlags(flag.CommandLine)

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: operatorMetricsAddr,
			},
			HealthProbeBindAddress: operatorProbeAddr,
			LeaderElection:         enableLeaderElection,
			LeaderElectionID:       "54dd1e7c.cubrid.com",
		})
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			os.Exit(1)
		}

		if err = (&controller.CubridDBReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
			Config: mgr.GetConfig(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "CubridDB")
			os.Exit(1)
		}

		if err = controller.NewBackupDBReconciler(mgr).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "BackupDB")
			os.Exit(1)
		}

		if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
			setupLog.Error(err, "unable to set up health check")
			os.Exit(1)
		}
		if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
			setupLog.Error(err, "unable to set up ready check")
			os.Exit(1)
		}

		setupLog.Info("starting manager !!!!!!")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	},
}

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "CubridDB operator webhook server.",
	Long:  `Provides validation and inmutability checks for CubridDB resources.`,
	Run: func(cmd *cobra.Command, args []string) {
		setupLog.Info("Starting webhook server!!")

		// Create Kubernetes client
		config := ctrl.GetConfigOrDie()

		if webhookCertManagerType == "external" {
			// When using external cert-manager, only check certificate path
			setupLog.Info("External cert-manager")
			if webhookCertDir == "" {
				webhookCertDir = certmanager.DefaultCertificateDir
			}

			// Check if certificate files exist
			certPath := filepath.Join(webhookCertDir, "tls.crt")
			keyPath := filepath.Join(webhookCertDir, "tls.key")
			if _, err := os.Stat(certPath); os.IsNotExist(err) {
				setupLog.Error(err, "Certificate file not found", "path", certPath)
				os.Exit(1)
			}
			if _, err := os.Stat(keyPath); os.IsNotExist(err) {
				setupLog.Error(err, "Key file not found", "path", keyPath)
				os.Exit(1)
			}
		} else {
			setupLog.Info("Internal cert-manager")
			if webhookCertDir == "" {
				setupLog.Info("Internal cert-manager: certDir is empty")
				webhookCertDir = certmanager.DefaultCertificateDir
			}

			setupLog.Info("Webhook configuration",
				"webhookServiceName", webhookServiceName,
				"webhookNamespace", webhookNamespace,
				"webhookCertDir", webhookCertDir,
				"webhookCertManagerType", webhookCertManagerType,
				"webhookSecretName", webhookSecretName,
				"webhookPort", webhookPort,
				"webhookMetricsAddr", webhookMetricsAddr,
				"webhookProbeAddr", webhookProbeAddr)

			// When using internal cert-manager, create and start cert-manager manager
			certManager, err := certmanager.NewManager(config, certmanager.Config{
				WebhookServiceName:     webhookServiceName,
				WebhookNamespace:       webhookNamespace,
				WebhookCertDir:         webhookCertDir,
				WebhookCertManagerType: certmanager.CertManagerType(webhookCertManagerType),
				WebhookSecretName:      webhookSecretName,
				WebhookMutatingName:    webhookMutatingName,
				WebhookValidatingName:  webhookValidatingName,
				WebhookDeploymentName:  webhookDeploymentName,
			})
			if err != nil {
				setupLog.Error(err, "Unable to create cert-manager")
				os.Exit(1)
			}

			// Start cert-manager
			if err := certManager.Start(); err != nil {
				setupLog.Error(err, "Unable to start cert-manager")
				os.Exit(1)
			}

			certPath := filepath.Join(webhookCertDir, "tls.crt")
			keyPath := filepath.Join(webhookCertDir, "tls.key")
			caPath := filepath.Join(webhookCertDir, "ca.crt")

			setupLog.Info("Waiting for certificates to be available...")
			const maxRetries = 60
			for i := 0; i < maxRetries; i++ {
				if certInfo, err := os.Stat(certPath); err == nil {
					if keyInfo, err := os.Stat(keyPath); err == nil {
						if caInfo, err := os.Stat(caPath); err == nil {
							if certInfo.Size() > 0 && keyInfo.Size() > 0 && caInfo.Size() > 0 {
								setupLog.Info("All certificates are available and have content")
								break
							}
						}
					}
				}
				if i == maxRetries-1 {
					setupLog.Error(nil, "Timeout waiting for certificates")
					os.Exit(1)
				}
				setupLog.Info("Waiting for certificate content...")
				time.Sleep(time.Second)
			}

			setupLog.Info("All certificates verified successfully")
		}

		// Configure to use certificates created by cert-manager
		mgr, err := ctrl.NewManager(config, ctrl.Options{
			HealthProbeBindAddress: webhookProbeAddr,
			Metrics: metricsserver.Options{
				BindAddress: webhookMetricsAddr,
			},
			WebhookServer: webhook.NewServer(webhook.Options{
				Port:    webhookPort,
				CertDir: webhookCertDir,
				TLSOpts: []func(*tls.Config){
					func(cfg *tls.Config) {
						// Check if certificate file exists
						certPath := filepath.Join(webhookCertDir, "tls.crt")
						if _, err := os.Stat(certPath); os.IsNotExist(err) {
							setupLog.Error(err, "Certificate file not found", "path", certPath)
							os.Exit(1)
						}
						keyPath := filepath.Join(webhookCertDir, "tls.key")
						if _, err := os.Stat(keyPath); os.IsNotExist(err) {
							setupLog.Error(err, "Key file not found", "path", keyPath)
							os.Exit(1)
						}
						// Load certificate
						cert, err := tls.LoadX509KeyPair(certPath, keyPath)
						if err != nil {
							setupLog.Error(err, "Failed to load certificate")
							os.Exit(1)
						}
						cfg.Certificates = []tls.Certificate{cert}
					},
				},
			}),
		})
		if err != nil {
			setupLog.Error(err, "Unable to start manager")
			os.Exit(1)
		}

		if err := cubridv1.AddToScheme(mgr.GetScheme()); err != nil {
			fmt.Println("Unable to add CubridDB to scheme:", err)
			os.Exit(1)
		}

		if err = (&cubridv1.CubridDB{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create webhook", "webhook", "CubirdDB")
			os.Exit(1)
		}

		if err = (&cubridv1.BackupDB{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create webhook", "webhook", "BackupDB")
			os.Exit(1)
		}

		if err := mgr.AddHealthzCheck("webhook-healthz", healthz.Ping); err != nil {
			setupLog.Error(err, "unable to set up health check")
			os.Exit(1)
		}

		// Add certificate check to readiness probe
		if err := mgr.AddReadyzCheck("webhook-cert-check", func(_ *http.Request) error {
			certPath := filepath.Join(webhookCertDir, "tls.crt")
			if _, err := os.Stat(certPath); os.IsNotExist(err) {
				return fmt.Errorf("certificate file not found: %s", certPath)
			}

			keyPath := filepath.Join(webhookCertDir, "tls.key")
			if _, err := os.Stat(keyPath); os.IsNotExist(err) {
				return fmt.Errorf("key file not found: %s", keyPath)
			}

			caPath := filepath.Join(webhookCertDir, "ca.crt")
			if _, err := os.Stat(caPath); os.IsNotExist(err) {
				return fmt.Errorf("CA certificate file not found: %s", caPath)
			}

			setupLog.Info("successfully checked webhook-cert-check")
			return nil
		}); err != nil {
			setupLog.Error(err, "Unable to add ready check")
			os.Exit(1)
		}

		setupLog.Info("Starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "Error running manager")
			os.Exit(3)
		}
	},
}

func main() {
	rootCmd.AddCommand(webhookCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		setupLog.Error(err, "Error main")
		os.Exit(1)
	}
}
