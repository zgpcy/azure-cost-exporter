{{/*
Expand the name of the chart.
*/}}
{{- define "azure-cost-exporter.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "azure-cost-exporter.fullname" -}}
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
{{- define "azure-cost-exporter.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "azure-cost-exporter.labels" -}}
helm.sh/chart: {{ include "azure-cost-exporter.chart" . }}
{{ include "azure-cost-exporter.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "azure-cost-exporter.selectorLabels" -}}
app.kubernetes.io/name: {{ include "azure-cost-exporter.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "azure-cost-exporter.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "azure-cost-exporter.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the secret to use for Azure credentials
*/}}
{{- define "azure-cost-exporter.secretName" -}}
{{- if .Values.existingSecret }}
{{- .Values.existingSecret }}
{{- else }}
{{- include "azure-cost-exporter.fullname" . }}
{{- end }}
{{- end }}

{{/*
Validate required values
*/}}
{{- define "azure-cost-exporter.validateValues" -}}
{{- /* Validate subscriptions are configured */ -}}
{{- if not .Values.config.subscriptions }}
{{- fail "config.subscriptions is required. Please provide at least one Azure subscription." }}
{{- end }}

{{- /* Validate authentication configuration */ -}}
{{- if .Values.managedIdentity.enabled }}
  {{- /* Using Managed Identity - validate identity configuration */ -}}
  {{- if or .Values.existingSecret .Values.azure.clientId .Values.azure.clientSecret .Values.azure.tenantId }}
  {{- fail "When managedIdentity.enabled=true, you cannot also provide Azure credentials (existingSecret or azure.clientId/clientSecret/tenantId). Choose either Managed Identity OR credentials, not both." }}
  {{- end }}

  {{- /* Validate clientId is provided for workload-identity and user-assigned */ -}}
  {{- if or (eq .Values.managedIdentity.type "workload-identity") (eq .Values.managedIdentity.type "user-assigned") }}
    {{- if not .Values.managedIdentity.clientId }}
    {{- fail (printf "managedIdentity.clientId is required when using type '%s'. Please provide the Managed Identity Client ID." .Values.managedIdentity.type) }}
    {{- end }}
  {{- end }}

  {{- /* Validate type is one of the supported values */ -}}
  {{- if not (or (eq .Values.managedIdentity.type "workload-identity") (eq .Values.managedIdentity.type "user-assigned") (eq .Values.managedIdentity.type "system-assigned")) }}
  {{- fail (printf "Invalid managedIdentity.type '%s'. Must be one of: workload-identity, user-assigned, system-assigned" .Values.managedIdentity.type) }}
  {{- end }}
{{- else }}
  {{- /* Using Service Principal credentials - validate credentials are provided */ -}}
  {{- if and (not .Values.existingSecret) (or (not .Values.azure.clientId) (not .Values.azure.clientSecret) (not .Values.azure.tenantId)) }}
  {{- fail "Azure credentials are required when managedIdentity.enabled=false. Either set managedIdentity.enabled=true OR provide existingSecret OR set azure.clientId, azure.clientSecret, and azure.tenantId." }}
  {{- end }}
{{- end }}
{{- end }}
