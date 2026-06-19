{{/*
Nome base do chart.
*/}}
{{- define "foxbit-calc.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Nome completo (fullname) dos recursos.
*/}}
{{- define "foxbit-calc.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Identificador chart-version para a label helm.
*/}}
{{- define "foxbit-calc.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Labels comuns (recomendadas pelo Kubernetes).
*/}}
{{- define "foxbit-calc.labels" -}}
helm.sh/chart: {{ include "foxbit-calc.chart" . }}
{{ include "foxbit-calc.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Labels de seleção (estáveis — não incluem version).
*/}}
{{- define "foxbit-calc.selectorLabels" -}}
app.kubernetes.io/name: {{ include "foxbit-calc.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Tag da imagem: usa image.tag ou, se vazio, o appVersion do chart.
*/}}
{{- define "foxbit-calc.imageTag" -}}
{{- default .Chart.AppVersion .Values.image.tag -}}
{{- end -}}

{{/*
Endereço do Redis conforme o modo de cache:
  - internal: serviço master do subchart bitnami/redis
  - external: cache.external.address
*/}}
{{- define "foxbit-calc.redisAddress" -}}
{{- if eq .Values.cache.type "internal" -}}
{{- printf "%s-redis-master:6379" .Release.Name -}}
{{- else -}}
{{- .Values.cache.external.address -}}
{{- end -}}
{{- end -}}

{{/*
Nome do Secret de pull de imagem criado pelo chart (quando
imagePullSecret.create=true).
*/}}
{{- define "foxbit-calc.pullSecretName" -}}
{{- printf "%s-pull" (include "foxbit-calc.fullname" .) -}}
{{- end -}}

{{/*
Conteúdo .dockerconfigjson (base64) para o Secret de pull de imagem.
*/}}
{{- define "foxbit-calc.dockerconfigjson" -}}
{{- $reg := .Values.imagePullSecret.registry -}}
{{- $user := .Values.imagePullSecret.username -}}
{{- $pass := .Values.imagePullSecret.password -}}
{{- $auth := printf "%s:%s" $user $pass | b64enc -}}
{{- $cfg := dict "auths" (dict $reg (dict "username" $user "password" $pass "auth" $auth)) -}}
{{- $cfg | toJson | b64enc -}}
{{- end -}}

{{/*
Lista efetiva de imagePullSecrets: referências existentes (imagePullSecrets) mais
o Secret criado pelo chart (quando imagePullSecret.create=true).
*/}}
{{- define "foxbit-calc.imagePullSecrets" -}}
{{- $secrets := .Values.imagePullSecrets | default list -}}
{{- if .Values.imagePullSecret.create -}}
{{- $secrets = append $secrets (dict "name" (include "foxbit-calc.pullSecretName" .)) -}}
{{- end -}}
{{- toYaml $secrets -}}
{{- end -}}

{{/*
Valida a configuração do Secret de pull de imagem.
*/}}
{{- define "foxbit-calc.validateImagePullSecret" -}}
{{- if .Values.imagePullSecret.create -}}
{{- if or (not .Values.imagePullSecret.registry) (not .Values.imagePullSecret.username) (not .Values.imagePullSecret.password) -}}
{{- fail "imagePullSecret.create=true exige imagePullSecret.registry, .username e .password" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Nome do Secret que guarda a senha do Redis externo (gerado pelo chart) quando
uma senha em texto puro é fornecida sem existingSecret.
*/}}
{{- define "foxbit-calc.redisSecretName" -}}
{{- if .Values.cache.external.existingSecret -}}
{{- .Values.cache.external.existingSecret -}}
{{- else -}}
{{- printf "%s-redis-external" (include "foxbit-calc.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
Valida a coerência da configuração de cache, abortando o template com uma
mensagem clara em caso de configuração inconsistente.
*/}}
{{- define "foxbit-calc.validateCache" -}}
{{- if .Values.cache.enabled -}}
{{- if eq .Values.cache.type "internal" -}}
{{- if not .Values.redis.enabled -}}
{{- fail "cache.enabled=true com cache.type=internal exige redis.enabled=true (subchart bitnami/redis)" -}}
{{- end -}}
{{- else if eq .Values.cache.type "external" -}}
{{- if not .Values.cache.external.address -}}
{{- fail "cache.type=external exige cache.external.address" -}}
{{- end -}}
{{- else -}}
{{- fail (printf "cache.type inválido: %q (use 'internal' ou 'external')" .Values.cache.type) -}}
{{- end -}}
{{- end -}}
{{- end -}}
