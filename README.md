# Gateway Node Controller

A [Kubernetes](https://kubernetes.io/) controller for the [Gateway API](https://gateway-api.sigs.k8s.io/).
It builds the [`Gateway.spec.addresses`](https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io/v1beta1.Gateway) field from the internal IP addresses of Nodes.
This is useful when a Kubernets cluster operates on bare metal, without an external load balancer.

## Configuration

* Only `Gateway` resources matching the label `controller.itergia.com/gateway-node=true` are updated.
* Nodes with the label `gateway-node.k8s.itergia.com/$namespace.$name` will be added to that named `Gateway`.
  The label value is ignored.

## Usage

To deploy the controller in the `gateway-system` namespace (created by the Envoy Gateway deployment):

```sh
kubectl apply -f https://github.com/itergia/gateway-node-controller/raw/main/install.yaml
```

The built [controller container](https://hub.docker.com/r/githubtommie/gateway-node-controller) is available at Docker Hub.
