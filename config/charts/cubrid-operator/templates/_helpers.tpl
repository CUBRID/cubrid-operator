# common labels
{{- define "common.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: webhook
app.kubernetes.io/created-by: {{ .Chart.Name }}
app.kubernetes.io/part-of: {{ .Chart.Name }}
app.kubernetes.io/managed-by: Helm
{{- end }}

# cubrid-operator name
{{- define "cubrid-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "cubrid-operator.fullname" -}}
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
Create the name of the service account to use
*/}}
{{- define "cubrid-operator.serviceAccountName" -}}
{{- if .Values.operator.enabled }}
{{- default (printf "%s-sa" (include "cubrid-operator.fullname" .)) .Values.operator.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.operator.serviceAccount.name }}
{{- end }}
{{- end }}

# clusterRole for operator
{{- define "cubrid-operator.clusterRoleName" -}}
{{- if .Values.operator.enabled }}
{{- default (printf "%s-role" (include "cubrid-operator.fullname" .))  .Values.operator.clusterRoleName.name }}
{{- else }}
{{- default "default" .Values.operator.clusterRoleName.name }}
{{- end }}
{{- end }}

# clusterRoleBinding for operator
{{- define "cubrid-operator.clusterRoleBindingName" -}}
{{- if .Values.operator.enabled }}
{{- default (printf "%s-rolebinding" (include "cubrid-operator.fullname" .))  .Values.operator.clusterRoleBindingName.name }}
{{- else }}
{{- default "default" .Values.operator.clusterRoleBindingName.name }}
{{- end }}
{{- end }}

# Role for elction of operator
{{- define "cubrid-operator.electionRoleName" -}}
{{- if .Values.operator.enabled }}
{{- default (printf "%s-electRole" (include "cubrid-operator.fullname" .))  .Values.operator.electionRoleName.name }}
{{- else }}
{{- default "default" .Values.operator.electionRoleName.name }}
{{- end }}
{{- end }}

# RoleBinding for election of operator
{{- define "cubrid-operator.electionRoleBindingName" -}}
{{- if .Values.operator.enabled }}
{{- default (printf "%s-electRolebinding" (include "cubrid-operator.fullname" .))  .Values.operator.electionRoleBindingName.name }}
{{- else }}
{{- default "default" .Values.operator.electionRoleBindingName.name }}
{{- end }}
{{- end }}

# ClusgterRole for cubriddb-editor of operator
{{- define "cubrid-operator.cubriddbEditClusterRoleName" -}}
{{- if .Values.operator.enabled }}
{{- default (printf "%s-cubriddb-editor-ClusterRole" (include "cubrid-operator.fullname" .))  .Values.operator.cubriddbEditClusterRoleName.name }}
{{- else }}
{{- default "default" .Values.operator.cubriddbEditClusterRoleName.name }}
{{- end }}
{{- end }}

# ClusgterRole for cubriddb-viewer of operator
{{- define "cubrid-operator.cubriddbViewClusterRoleName" -}}
{{- if .Values.operator.enabled }}
{{- default (printf "%s-cubriddb-viewer-ClusterRole" (include "cubrid-operator.fullname" .))  .Values.operator.cubriddbViewClusterRoleName.name }}
{{- else }}
{{- default "default" .Values.operator.cubriddbViewClusterRoleName.name }}
{{- end }}
{{- end }}

# ClusgterRole for backupdb-editor of operator
{{- define "cubrid-operator.backupdbEditClusterRoleName" -}}
{{- if .Values.operator.enabled }}
{{- default (printf "%s-backupdb-editor-ClusterRole" (include "cubrid-operator.fullname" .))  .Values.operator.backupdbEditClusterRoleName.name }}
{{- else }}
{{- default "default" .Values.operator.backupdbEditClusterRoleName.name }}
{{- end }}
{{- end }}

# ClusgterRole for backupdb-viewer of operator
{{- define "cubrid-operator.backupdbViewClusterRoleName" -}}
{{- if .Values.operator.enabled }}
{{- default (printf "%s-backupdb-viewer-ClusterRole" (include "cubrid-operator.fullname" .))  .Values.operator.backupdbViewClusterRoleName.name }}
{{- else }}
{{- default "default" .Values.operator.backupdbViewClusterRoleName.name }}
{{- end }}
{{- end }}



# Webhook
# serviceAccount for webhook
{{- define "cubrid-operator-webhook.serviceAccountName" -}}
{{- if .Values.webhook.enabled }}
{{- default (printf "%s-webhook-sa" (include "cubrid-operator.fullname" .))  .Values.webhook.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.webhook.serviceAccount.name }}
{{- end }}
{{- end }}


# clusterRole for webhook
{{- define "cubrid-operator-webhook.clusterRoleName" -}}
{{- if .Values.webhook.enabled }}
{{- default (printf "%s-webhook-role" (include "cubrid-operator.fullname" .))  .Values.webhook.clusterRoleName.name }}
{{- else }}
{{- default "default" .Values.webhook.clusterRoleName.name }}
{{- end }}
{{- end }}

# clusterRoleBinding for webhook
{{- define "cubrid-operator-webhook.clusterRoleBindingName" -}}
{{- if .Values.webhook.enabled }}
{{- default (printf "%s-webhook-rolebinding" (include "cubrid-operator.fullname" .))  .Values.webhook.clusterRoleBindingName.name }}
{{- else }}
{{- default "default" .Values.webhook.clusterRoleBindingName.name }}
{{- end }}
{{- end }}

# service for webhook
{{- define "cubrid-operator-webhook.serviceName" -}}
{{- printf "%s-webhook-service" (include "cubrid-operator.fullname" .) -}}
{{- end -}}
