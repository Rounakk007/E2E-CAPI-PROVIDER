#!/usr/bin/env bash
# setup-cni.sh — Creates the ConfigMap and ClusterResourceSet for automatic
# CNI installation via CAPI's ClusterResourceSet controller.
#
# Supported CNIs: calico, cilium, flannel
#
# Usage:
#   bash hack/setup-cni.sh calico          # default: v3.28.0
#   bash hack/setup-cni.sh calico v3.27.0
#   bash hack/setup-cni.sh cilium          # default: v1.15.0
#   bash hack/setup-cni.sh flannel         # default: v0.25.0
#
# After running, label a cluster to use the CNI:
#   kubectl label cluster <name> cni=calico
#   kubectl label cluster <name> cni=cilium
#   kubectl label cluster <name> cni=flannel

set -euo pipefail

# CNI_NAME and CNI_VERSION can be set as environment variables or passed as args.
# Args take precedence over env vars; env vars take precedence over defaults.
CNI="${1:-${CNI_NAME:-calico}}"
NAMESPACE="default"
TMPFILE=$(mktemp /tmp/cni-XXXXXX.yaml)

case "${CNI}" in
  calico)
    VERSION="${2:-${CNI_VERSION:-v3.28.0}}"
    URL="https://raw.githubusercontent.com/projectcalico/calico/${VERSION}/manifests/calico.yaml"
    ;;
  cilium)
    VERSION="${2:-${CNI_VERSION:-v1.15.0}}"
    URL="https://raw.githubusercontent.com/cilium/cilium/${VERSION}/install/kubernetes/quick-install.yaml"
    ;;
  flannel)
    VERSION="${2:-${CNI_VERSION:-v0.25.0}}"
    URL="https://raw.githubusercontent.com/flannel-io/flannel/${VERSION}/Documentation/kube-flannel.yml"
    ;;
  *)
    echo "ERROR: Unsupported CNI '${CNI}'. Supported: calico, cilium, flannel"
    exit 1
    ;;
esac

echo "==> Downloading ${CNI} ${VERSION}..."
curl -fsSL "${URL}" -o "${TMPFILE}"
echo "    Downloaded $(wc -l < "${TMPFILE}") lines"

CONFIGMAP_NAME="${CNI}-cni"
CRS_NAME="${CNI}-cni"

echo "==> Creating ConfigMap ${CONFIGMAP_NAME} in namespace ${NAMESPACE}..."
kubectl create configmap "${CONFIGMAP_NAME}" \
  --from-file=cni.yaml="${TMPFILE}" \
  --namespace="${NAMESPACE}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> Creating ClusterResourceSet ${CRS_NAME}..."
cat <<EOF | kubectl apply -f -
apiVersion: addons.cluster.x-k8s.io/v1beta1
kind: ClusterResourceSet
metadata:
  name: ${CRS_NAME}
  namespace: ${NAMESPACE}
spec:
  strategy: ApplyOnce
  clusterSelector:
    matchLabels:
      cni: ${CNI}
  resources:
    - name: ${CONFIGMAP_NAME}
      kind: ConfigMap
EOF

echo ""
echo "==> Verifying..."
kubectl get clusterresourceset "${CRS_NAME}" -n "${NAMESPACE}"
kubectl get configmap "${CONFIGMAP_NAME}" -n "${NAMESPACE}"

echo ""
echo "Done. Label a cluster to install ${CNI} automatically:"
echo "  kubectl label cluster <name> cni=${CNI}"
echo ""
echo "Or set it in the cluster template:"
echo "  labels:"
echo "    cni: ${CNI}"

rm -f "${TMPFILE}"
