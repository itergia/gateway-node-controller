package main

import (
	"context"
	"reflect"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	core "k8s.io/api/core/v1"
	gwapi "sigs.k8s.io/gateway-api/apis/v1beta1"
)

const gatewayNodeKeyPrefix = "gateway-node.k8s.itergia.com/"

func initNodes(b *builder.Builder) *builder.Builder {
	return b.Watches(&source.Kind{Type: &core.Node{}}, handler.EnqueueRequestsFromMapFunc(mapNode))
}

// mapNode creates reconciliation requests for the Gateways referenced
// by gateway-node labels in the Node. Returns nothing if the object
// is not a core.Node.
func mapNode(obj client.Object) []reconcile.Request {
	node, ok := obj.(*core.Node)
	if !ok {
		return nil
	}

	var reqs []reconcile.Request
	for k, v := range node.GetLabels() {
		if strings.HasPrefix(k, gatewayNodeKeyPrefix) {
			if v != "" {
				continue
			}

			ss := strings.SplitN(k[len(gatewayNodeKeyPrefix):], "/", 1)
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: ss[0],
				Name:      ss[1],
			}})
		}
	}

	return reqs
}

func updateAddresses(ctx context.Context, gw *gwapi.Gateway, name types.NamespacedName, cl client.Client) (bool, error) {
	var nodes core.NodeList
	err := cl.List(ctx, &nodes, client.MatchingLabels{gatewayNodeKeyPrefix + name.String(): ""})
	if err != nil {
		return false, err
	}

	var addrs []gwapi.GatewayAddress
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == "InternalIP" {
				addrs = append(addrs, gwapi.GatewayAddress{Value: addr.Address})
			}
		}
	}

	sort.Slice(addrs, func(i, j int) bool { return addrs[i].Value < addrs[j].Value })

	if reflect.DeepEqual(addrs, gw.Spec.Addresses) {
		return false, nil
	}

	gw.Spec.Addresses = addrs

	return true, nil
}
