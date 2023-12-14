{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "nodejs.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "nodejs.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "nodejs.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create the name of the service account
*/}}
{{- define "nodejs.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "nodejs.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "nodejs.serviceAccountNameTest" -}}
{{- if .Values.serviceAccount.create }}
{{- default (print (include "nodejs.fullname" .) "-test") .Values.serviceAccount.nameTest }}
{{- else }}
{{- default "default" .Values.serviceAccount.nameTest }}
{{- end }}
{{- end }}

{{/*
Allow the release namespace to be overridden for multi-namespace deployments in combined charts
*/}}
{{- define "nodejs.namespace" -}}
{{- if .Values.namespaceOverride }}
{{- .Values.namespaceOverride }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "nodejs.labels" -}}
helm.sh/chart: {{ include "nodejs.chart" . }}
{{ include "nodejs.selectorLabels" . }}
{{- if or .Chart.AppVersion .Values.nodejs.image.tag }}
app.kubernetes.io/version: {{ mustRegexReplaceAllLiteral "@sha.*" .Values.nodejs.image.tag "" | default .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.extraLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "nodejs.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nodejs.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "nodejs.imageRenderer.labels" -}}
helm.sh/chart: {{ include "nodejs.chart" . }}
{{ include "nodejs.imageRenderer.selectorLabels" . }}
{{- if or .Chart.AppVersion .Values.image.tag }}
app.kubernetes.io/version: {{ mustRegexReplaceAllLiteral "@sha.*" .Values.image.tag "" | default .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels ImageRenderer
*/}}
{{- define "nodejs.imageRenderer.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nodejs.name" . }}-image-renderer
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Looks if there's an existing secret and reuse its password. If not it generates
new password and use it.
*/}}
{{- define "nodejs.password" -}}
{{- $secret := (lookup "v1" "Secret" (include "nodejs.namespace" .) (include "nodejs.fullname" .) ) }}
{{- if $secret }}
{{- index $secret "data" "admin-password" }}
{{- else }}
{{- (randAlphaNum 40) | b64enc | quote }}
{{- end }}
{{- end }}

{{/*
Return the appropriate apiVersion for rbac.
*/}}
{{- define "nodejs.rbac.apiVersion" -}}
{{- if $.Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1" }}
{{- print "rbac.authorization.k8s.io/v1" }}
{{- else }}
{{- print "rbac.authorization.k8s.io/v1beta1" }}
{{- end }}
{{- end }}

{{/*
Return the appropriate apiVersion for ingress.
*/}}
{{- define "nodejs.ingress.apiVersion" -}}
{{- if and ($.Capabilities.APIVersions.Has "networking.k8s.io/v1") (semverCompare ">= 1.19-0" .Capabilities.KubeVersion.Version) }}
{{- print "networking.k8s.io/v1" }}
{{- else if $.Capabilities.APIVersions.Has "networking.k8s.io/v1beta1" }}
{{- print "networking.k8s.io/v1beta1" }}
{{- else }}
{{- print "extensions/v1beta1" }}
{{- end }}
{{- end }}

{{/*
Return the appropriate apiVersion for Horizontal Pod Autoscaler.
*/}}
{{- define "nodejs.hpa.apiVersion" -}}
{{- if $.Capabilities.APIVersions.Has "autoscaling/v2/HorizontalPodAutoscaler" }}
{{- print "autoscaling/v2" }}
{{- else if $.Capabilities.APIVersions.Has "autoscaling/v2beta2/HorizontalPodAutoscaler" }}
{{- print "autoscaling/v2beta2" }}
{{- else }}
{{- print "autoscaling/v2beta1" }}
{{- end }}
{{- end }}

{{/*
Return the appropriate apiVersion for podDisruptionBudget.
*/}}
{{- define "nodejs.podDisruptionBudget.apiVersion" -}}
{{- if $.Values.podDisruptionBudget.apiVersion }}
{{- print $.Values.podDisruptionBudget.apiVersion }}
{{- else if $.Capabilities.APIVersions.Has "policy/v1/PodDisruptionBudget" }}
{{- print "policy/v1" }}
{{- else }}
{{- print "policy/v1beta1" }}
{{- end }}
{{- end }}

{{/*
Return if ingress is stable.
*/}}
{{- define "nodejs.ingress.isStable" -}}
{{- eq (include "nodejs.ingress.apiVersion" .) "networking.k8s.io/v1" }}
{{- end }}

{{/*
Return if ingress supports ingressClassName.
*/}}
{{- define "nodejs.ingress.supportsIngressClassName" -}}
{{- or (eq (include "nodejs.ingress.isStable" .) "true") (and (eq (include "nodejs.ingress.apiVersion" .) "networking.k8s.io/v1beta1") (semverCompare ">= 1.18-0" .Capabilities.KubeVersion.Version)) }}
{{- end }}

{{/*
Return if ingress supports pathType.
*/}}
{{- define "nodejs.ingress.supportsPathType" -}}
{{- or (eq (include "nodejs.ingress.isStable" .) "true") (and (eq (include "nodejs.ingress.apiVersion" .) "networking.k8s.io/v1beta1") (semverCompare ">= 1.18-0" .Capabilities.KubeVersion.Version)) }}
{{- end }}

{{/*
Formats imagePullSecrets. Input is (dict "root" . "imagePullSecrets" .{specific imagePullSecrets})
*/}}
{{- define "nodejs.imagePullSecrets" -}}
{{- $root := .root }}
{{- range (concat .root.Values.global.imagePullSecrets .imagePullSecrets) }}
{{- if eq (typeOf .) "map[string]interface {}" }}
- {{ toYaml (dict "name" (tpl .name $root)) | trim }}
{{- else }}
- name: {{ tpl . $root }}
{{- end }}
{{- end }}
{{- end }}
