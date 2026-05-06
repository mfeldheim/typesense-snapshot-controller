package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1alpha1 "github.com/opentelekomcloud/typesense-snapshot-controller/api/v1alpha1"
	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/config"
	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/controller"
	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/cooldown"
	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/discovery"
	"github.com/opentelekomcloud/typesense-snapshot-controller/internal/runner"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

func main() {
	root := &cobra.Command{
		Use:   "typesense-snapshot-controller",
		Short: "Typesense snapshot automation",
	}
	root.AddCommand(newRunCmd())
	root.AddCommand(newServeCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Take a single snapshot and exit (CronJob mode)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromEnv()

			log, err := zap.NewProduction()
			if err != nil {
				return err
			}
			defer log.Sync() //nolint:errcheck

			rc, err := ctrl.GetConfig()
			if err != nil {
				return fmt.Errorf("get kubeconfig: %w", err)
			}
			k8s, err := kubernetes.NewForConfig(rc)
			if err != nil {
				return fmt.Errorf("build k8s client: %w", err)
			}

			apiKey, err := getAPIKey(cmd.Context(), k8s, cfg)
			if err != nil {
				return err
			}

			disc := discovery.New(k8s, cfg.Namespace, cfg.PodLabelSelector, cfg.TypesensePort, apiKey, cfg.SnapshotTimeout, log)
			cd := cooldown.New(k8s, cfg.Namespace, cfg.CooldownConfigMap, cfg.CooldownDuration)
			r := runner.New(disc, cd, cfg.TypesensePort, apiKey, cfg.SnapshotPath, cfg.SnapshotTimeout, log)

			return r.Run(cmd.Context(), runner.TriggerScheduled)
		},
	}
}

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the event-driven snapshot controller (Deployment mode)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromEnv()

			ctrl.SetLogger(ctrlzap.New())

			log, err := zap.NewProduction()
			if err != nil {
				return err
			}
			defer log.Sync() //nolint:errcheck

			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
				Scheme: scheme,
				Metrics: server.Options{
					BindAddress: cfg.MetricsAddr,
				},
				HealthProbeBindAddress:  cfg.HealthProbeAddr,
				LeaderElection:          cfg.LeaderElection,
				LeaderElectionID:        cfg.LeaderElectionID,
				LeaderElectionNamespace: cfg.Namespace,
			})
			if err != nil {
				return fmt.Errorf("create manager: %w", err)
			}

			if err := mgr.AddHealthzCheck("healthz", func(_ *http.Request) error { return nil }); err != nil {
				return fmt.Errorf("add healthz: %w", err)
			}
			if err := mgr.AddReadyzCheck("readyz", func(_ *http.Request) error { return nil }); err != nil {
				return fmt.Errorf("add readyz: %w", err)
			}

			k8s, err := kubernetes.NewForConfig(mgr.GetConfig())
			if err != nil {
				return fmt.Errorf("build k8s client: %w", err)
			}

			apiKey, err := getAPIKey(cmd.Context(), k8s, cfg)
			if err != nil {
				return err
			}

			disc := discovery.New(k8s, cfg.Namespace, cfg.PodLabelSelector, cfg.TypesensePort, apiKey, cfg.SnapshotTimeout, log)
			cd := cooldown.New(k8s, cfg.Namespace, cfg.CooldownConfigMap, cfg.CooldownDuration)
			r := runner.New(disc, cd, cfg.TypesensePort, apiKey, cfg.SnapshotPath, cfg.SnapshotTimeout, log)

			if err := controller.Setup(mgr, cfg, r, log); err != nil {
				return fmt.Errorf("setup controllers: %w", err)
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGTERM, syscall.SIGINT)
			defer stop()
			return mgr.Start(ctx)
		},
	}
}

func getAPIKey(ctx context.Context, k8s kubernetes.Interface, cfg config.Config) (string, error) {
	secret, err := k8s.CoreV1().Secrets(cfg.Namespace).Get(ctx, cfg.SecretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get secret %s/%s: %w", cfg.Namespace, cfg.SecretName, err)
	}
	val, ok := secret.Data[cfg.SecretKey]
	if !ok {
		return "", fmt.Errorf("secret %s has no key %q", cfg.SecretName, cfg.SecretKey)
	}
	return string(val), nil
}
