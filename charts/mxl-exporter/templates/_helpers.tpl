{{/*
Chart name, overridable with nameOverride.
*/}}
{{- define "mxl-exporter.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Fully qualified app name. Truncated at 63 characters, because some Kubernetes
name fields are limited to that.
*/}}
{{- define "mxl-exporter.fullname" -}}
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

{{- define "mxl-exporter.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "mxl-exporter.labels" -}}
helm.sh/chart: {{ include "mxl-exporter.chart" . }}
{{ include "mxl-exporter.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "mxl-exporter.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mxl-exporter.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Renders a map as annotations or labels. Kubernetes rejects any value that is
not a string, so every value is quoted. Call it with the map as the context.
*/}}
{{- define "mxl-exporter.stringMap" -}}
{{- range $key, $value := . }}
{{ $key | quote }}: {{ $value | quote }}
{{- end }}
{{- end }}

{{- define "mxl-exporter.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "mxl-exporter.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Container image reference.
*/}}
{{- define "mxl-exporter.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) }}
{{- end }}

{{/*
Command line arguments for the exporter.
*/}}
{{- define "mxl-exporter.args" -}}
- --listen={{ .Values.exporter.listen.address }}:{{ .Values.exporter.listen.port }}
{{- if and .Values.shm.enabled .Values.shm.search }}
- --search={{ .Values.shm.mountPath }}
{{- end }}
{{- range .Values.exporter.search }}
- --search={{ . }}
{{- end }}
{{- range .Values.exporter.domains }}
- --domain={{ . }}
{{- end }}
{{- with .Values.exporter.lifetime.default }}
- --default-lifetime={{ . }}
{{- end }}
{{- with .Values.exporter.lifetime.filesystem }}
- --fs-lifetime={{ . }}
{{- end }}
{{- with .Values.exporter.lifetime.domain }}
- --domain-lifetime={{ . }}
{{- end }}
{{- with .Values.exporter.lifetime.flow }}
- --flow-lifetime={{ . }}
{{- end }}
{{- with .Values.exporter.extraArgs }}
{{- toYaml . | nindent 0 }}
{{- end }}
{{- end }}
