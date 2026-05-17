{{- define "argus-common.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "argus-common.fullname" -}}
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

{{- define "argus-common.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "argus-common.labels" -}}
helm.sh/chart: {{ include "argus-common.chart" . }}
{{ include "argus-common.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "argus-common.selectorLabels" -}}
app.kubernetes.io/name: {{ include "argus-common.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "argus-common.httpGetProbe" -}}
httpGet:
  path: {{ .path | default "/health" }}
  port: {{ .port | default "http" }}
initialDelaySeconds: {{ .initialDelay | default 10 }}
periodSeconds: {{ .period | default 10 }}
timeoutSeconds: {{ .timeout | default 3 }}
failureThreshold: {{ .failureThreshold | default 3 }}
{{- end }}

{{- define "argus-common.tcpSocketProbe" -}}
tcpSocket:
  port: {{ .port | default "http" }}
initialDelaySeconds: {{ .initialDelay | default 10 }}
periodSeconds: {{ .period | default 10 }}
{{- end }}

{{- define "argus-common.imageSpec" -}}
image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
imagePullPolicy: {{ .Values.image.pullPolicy }}
{{- end }}

{{- define "argus-common.podSecurityContext" -}}
{{- if .Values.podSecurityContext }}
securityContext:
  {{- toYaml .Values.podSecurityContext | nindent 2 }}
{{- end }}
{{- end }}

{{- define "argus-common.containerSecurityContext" -}}
{{- if .Values.securityContext }}
securityContext:
  {{- toYaml .Values.securityContext | nindent 2 }}
{{- end }}
{{- end }}

{{- define "argus-common.resources" -}}
resources:
  {{- toYaml .Values.resources | nindent 2 }}
{{- end }}

{{- define "argus-common.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "argus-common.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
