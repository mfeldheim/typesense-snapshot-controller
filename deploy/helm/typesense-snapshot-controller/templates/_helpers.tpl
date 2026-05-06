{{/*
Expand the name of the chart.
*/}}
{{- define "typesense-snapshot-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Full name: release + chart name, truncated to 63 chars.
*/}}
{{- define "typesense-snapshot-controller.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "typesense-snapshot-controller.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "typesense-snapshot-controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "typesense-snapshot-controller.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "typesense-snapshot-controller.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | quote }}
app.kubernetes.io/name: {{ include "typesense-snapshot-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels (stable — not chart-version-dependent).
*/}}
{{- define "typesense-snapshot-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "typesense-snapshot-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
