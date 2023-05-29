package main

import (
	"context"
	"reflect"
	"sort"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	core "k8s.io/api/core/v1"
	gwapi "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func initPods(b *builder.Builder) *builder.Builder {
	return b.
		Watches(&source.Kind{Type: &core.Pod{}}, handler.EnqueueRequestsFromMapFunc(mapPod))
}

// mapPod creates reconciliation requests for the Gateways referenced
// by labels in the Node. Returns nothing if the object is not a
// core.Pod.
func mapPod(obj client.Object) []reconcile.Request {
	pod, ok := obj.(*core.Pod)
	if !ok {
		return nil
	}

	lbls := pod.GetLabels()
	name := types.NamespacedName{
		Namespace: lbls["gateway.envoyproxy.io/owning-gateway-namespace"],
		Name:      lbls["gateway.envoyproxy.io/owning-gateway-name"],
	}
	if name.Name == "" {
		return nil
	}

	return []reconcile.Request{
		{NamespacedName: name},
	}
}

func updateAddresses(ctx context.Context, gw *gwapi.Gateway, name types.NamespacedName, cl client.Client) (bool, error) {
	var pods core.PodList
	err := cl.List(ctx, &pods, client.MatchingLabels{
		"gateway.envoyproxy.io/owning-gateway-namespace": name.Namespace,
		"gateway.envoyproxy.io/owning-gateway-name":      name.Name,
	})
	if err != nil {
		return false, err
	}

	var addrs []gwapi.GatewayAddress
	for _, pod := range pods.Items {
		if !isPodConditionTrue(pod.Status.Conditions, core.PodReady) {
			continue
		}

		if pod.Status.HostIP != "" {
			addrs = append(addrs, gwapi.GatewayAddress{
				Type:  ptrTo(gwapi.AddressType(gwapi.IPAddressType)),
				Value: pod.Status.HostIP,
			})
		}
	}

	sort.Slice(addrs, func(i, j int) bool { return addrs[i].Value < addrs[j].Value })

	if reflect.DeepEqual(addrs, gw.Spec.Addresses) {
		return false, nil
	}

	gw.Spec.Addresses = addrs

	return true, nil
}

func isPodConditionTrue(conds []core.PodCondition, condType core.PodConditionType) bool {
	for _, cond := range conds {
		if cond.Type == condType {
			return cond.Status == core.ConditionTrue
		}
	}

	return false
}

func ptrTo[T any](v T) *T {
	return &v
}
