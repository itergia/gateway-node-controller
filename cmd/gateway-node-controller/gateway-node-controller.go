package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	core "k8s.io/api/core/v1"
	gwapi "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func init() {
	config.RegisterFlags(nil)
}

func main() {
	flag.Parse()

	logf.SetLogger(zap.New())
	log := logf.Log.WithName("gateway-node-controller")
	ctx := logf.IntoContext(signals.SetupSignalHandler(), log)

	if err := run(ctx); err != nil {
		log.Error(err, "running the controller")
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	sch := runtime.NewScheme()
	if err := core.AddToScheme(sch); err != nil {
		return err
	}
	if err := gwapi.AddToScheme(sch); err != nil {
		return err
	}

	mgr, err := manager.New(cfg, manager.Options{Scheme: sch})
	if err != nil {
		return err
	}

	b := builder.
		ControllerManagedBy(mgr).
		For(&gwapi.Gateway{})
	b = initNodes(b)

	if err := b.Complete(&gatewayReconciler{}); err != nil {
		return err
	}

	// Blocks until the context is canceled.
	return mgr.Start(ctx)
}

type gatewayReconciler struct {
	cl client.Client
}

func (a *gatewayReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	var gw gwapi.Gateway
	err := a.cl.Get(ctx, req.NamespacedName, &gw)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if gw.ObjectMeta.Labels[controllerGatewayNodeKey] != "true" {
		// Not enabled for this controller.
		return reconcile.Result{}, nil
	}

	var updated bool
	if upd, err := updateAddresses(ctx, &gw, req.NamespacedName, a.cl); err != nil {
		return reconcile.Result{}, err
	} else {
		updated = updated || upd
	}

	if !updated {
		log.Info("No updates to Gateway", "name", req.NamespacedName)
		return reconcile.Result{}, nil
	}

	if err := updateRevisionAnnotation(&gw); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Updating Gateway", "name", req.NamespacedName, "addresses", gw.Spec.Addresses)

	return reconcile.Result{}, a.cl.Update(ctx, &gw)
}

const controllerGatewayNodeKey = "controller.itergia.com/gateway-node"

func updateRevisionAnnotation(gw *gwapi.Gateway) error {
	s := gw.ObjectMeta.Annotations[gatewayNodeControllerRevisionAnnotation]
	if s == "" {
		s = "0"
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("parsing annotation %q: %w", gatewayNodeControllerRevisionAnnotation, err)
	}

	gw.ObjectMeta.Annotations[gatewayNodeControllerRevisionAnnotation] = fmt.Sprint(i + 1)

	return nil
}

const gatewayNodeControllerRevisionAnnotation = "gateway-node-controller.itergia.com/revision"

func (a *gatewayReconciler) InjectClient(cl client.Client) error {
	a.cl = cl
	return nil
}
