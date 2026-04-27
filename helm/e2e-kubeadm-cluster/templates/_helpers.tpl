{{/*
k0s.resolvedVersion returns the full k0s version string (e.g. "v1.33.10+k0s.0").

Resolution order:
  1. If .Values.k0s.version is set and non-empty, use it as-is (explicit override).
  2. Otherwise, extract the major.minor from .Values.kubernetesVersion and look it
     up in .Values.k0sVersionMap.  If no entry is found the template fails with a
     clear message telling the operator to add a mapping.

This means a user can write kubernetesVersion: v1.33.0 (or v1.33.99) and still
get a real release: the patch component of kubernetesVersion is intentionally
ignored when auto-resolving.
*/}}
{{- define "k0s.resolvedVersion" -}}
{{- if and .Values.k0s.version (ne .Values.k0s.version "") -}}
{{ .Values.k0s.version | trim }}
{{- else -}}
{{- $raw := .Values.kubernetesVersion | trimPrefix "v" -}}
{{- $parts := splitList "." $raw -}}
{{- if lt (len $parts) 2 -}}
{{ fail (printf "kubernetesVersion %q is not in vX.Y or vX.Y.Z format" .Values.kubernetesVersion) }}
{{- end -}}
{{- $minor := printf "%s.%s" (index $parts 0) (index $parts 1) -}}
{{- if not (hasKey .Values.k0sVersionMap $minor) -}}
{{ fail (printf "No k0s version mapping found for Kubernetes %s. Add an entry to k0sVersionMap in values.yaml (e.g. \"%s\": \"v%s.0+k0s.0\")." $minor $minor $minor) }}
{{- end -}}
{{ index .Values.k0sVersionMap $minor | trim }}
{{- end -}}
{{- end -}}


{{/*
k8s.resolvedVersion returns the bare Kubernetes version (e.g. "v1.33.10") by
stripping the "+k0s.N" suffix from the resolved k0s version.  Use this for
fields that expect a plain Kubernetes version (e.g. MachineDeployment.spec.template.spec.version).
*/}}
{{- define "k8s.resolvedVersion" -}}
{{- include "k0s.resolvedVersion" . | trim | regexReplaceAll "\\+k0s\\..*$" "" -}}
{{- end -}}
