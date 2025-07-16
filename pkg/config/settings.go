package settings

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	CubridDefaultImage        = "cubrid/cubrid:latest"
	InitContainerDefaultImage = "busybox:latest"
	DefaultCUBRIDPath         = "/home/cubrid/CUBRID"
	HATemplateFilePath        = "share/scripts/operator_conf.sh"
	StorageType_conf          = "conf-storage-type"
	ConfStorageVolumeName     = "conf-storage"
	ConfMountPath             = "/home/cubrid/CUBRID/conf"
	StorageType_Database      = "database-storage-type"
	DatabaseStorageVolumeName = "database-storage"
	DatabaseMountPath         = "/home/cubrid/CUBRID/databases"
	StorageType_backup        = "backup-storage-type"
	BackupDBStorageVolumeName = "backupdb-storage"
	BackupDBMountPath         = "/home/cubrid/CUBRID/backupdb"
	StorageType_Logs          = "logs-storage-type"
	LogsStorageVolumeName     = "logs-storage"
	LogsMountPath             = "/home/cubrid/CUBRID/log"

	ConfBackupVolumeName = "conf-backup"
	ConfBackupMountPath  = "/mnt/conf"
	LogsBackupVolumeName = "logs-backup"
	LogsBackupMountPath  = "/mnt/log"
	DBBackupVolumeName   = "databases-backup"
	DBBackupMountPath    = "/mnt/databases"
	Default_Volume_Size  = "2Gi"
	Volume_Size_10Gi     = "10Gi"
	Volume_Size_50Gi     = "50Gi"
	Volume_Size_100Gi    = "100Gi"

	DefaultStorageClassName = "standard" // needs to be changed to standard

	// initcontainer
	InitCopyConfContainerName     = "init-copy-conf"
	InitRecoveryConfContainerName = "init-recovery-conf"
	InitCopyConfCommand           = "cp -rn /home/cubrid/CUBRID/conf/* /mnt/conf"
	InitRecoveryConfCommand       = "cp -rn /mnt/conf/* /home/cubrid/CUBRID/conf/ && " +
		"find /home/cubrid/CUBRID/conf -type f -not -path '*/lost+found/*' -exec chmod 644 {} \\; && " +
		"find /home/cubrid/CUBRID/conf -type d -not -path '*/lost+found*' -exec chmod 755 {} \\; && " +
		"find /home/cubrid/CUBRID/conf -type f -not -path '*/lost+found/*' -exec chown 1000:1000 {} \\; && " +
		"find /home/cubrid/CUBRID/conf -type d -not -path '*/lost+found*' -exec chown 1000:1000 {} \\; && " +
		"find /home/cubrid/CUBRID/databases -type f -not -path '*/lost+found/*' -exec chmod 644 {} \\; && " +
		"find /home/cubrid/CUBRID/databases -type d -not -path '*/lost+found*' -exec chmod 755 {} \\; && " +
		"find /home/cubrid/CUBRID/databases -type f -not -path '*/lost+found/*' -exec chown 1000:1000 {} \\; && " +
		"find /home/cubrid/CUBRID/databases -type d -not -path '*/lost+found*' -exec chown 1000:1000 {} \\; && " +
		"find /home/cubrid/CUBRID/backupdb -type f -not -path '*/lost+found/*' -exec chmod 644 {} \\; && " +
		"find /home/cubrid/CUBRID/backupdb -type d -not -path '*/lost+found*' -exec chmod 755 {} \\; && " +
		"find /home/cubrid/CUBRID/backupdb -type f -not -path '*/lost+found/*' -exec chown 1000:1000 {} \\; && " +
		"find /home/cubrid/CUBRID/backupdb -type d -not -path '*/lost+found*' -exec chown 1000:1000 {} \\; && " +
		"find /home/cubrid/CUBRID/log -type f -not -path '*/lost+found/*' -exec chmod 644 {} \\; && " +
		"find /home/cubrid/CUBRID/log -type d -not -path '*/lost+found*' -exec chmod 755 {} \\; && " +
		"find /home/cubrid/CUBRID/log -type f -not -path '*/lost+found/*' -exec chown 1000:1000 {} \\; && " +
		"find /home/cubrid/CUBRID/log -type d -not -path '*/lost+found*' -exec chown 1000:1000 {} \\;"

	// user
	CubridUser  = int64(1000)
	CubridGroup = int64(1000)

	HA_MASTER_SLAVE_TYPE = "master-slave"
	HA_REPLICA_TYPE      = "replica"

	SELECTOR_SUFFIX = "-group"

	// CMS Service
	SVC_NAME_SUFFIX         = "headless"
	SVC_CMS_NODEPORT        = "%s-%s-cms-%d" // <cubriddb-name>-<namespace>-cms-<pod-index>
	SVC_CMS_START_NODE_PORT = 31000

	// CMS Service Types
	CMSServiceTypeNodePort   = "NodePort"
	CMSServiceTypeIngress    = "Ingress"
	SVC_TYPE_NODE_PORT       = (corev1.ServiceTypeClusterIP)
	SVC_TYPE_CLUSTER_IP      = (corev1.ServiceTypeClusterIP)
	SVC_BR_NAME_QUERY_EDITOR = "cubrid-query-editor"
	SVC_BR_NAME_BROKER1      = "cubrid-broker1"
	SVC_HEADLESS_DNS         = "%s.%s"
	SVC_HEADLESS_PORT_NAME   = "cubriddb-headless"
	SVC_HA_PORT              = 59901 // Default port for HA service
	SVC_CMS_PORT_NAME        = "cubrid-cms-port"

	// Ingress
	INGRESS_CMS_SVC_NAME     = "%s-%s-cms-svc"        // <pod-name>-<namespace>-cms-svc
	INGRESS_CMS_INGRESS_NAME = "%s-%s-cms-ingress"    // <cubridDB-name>-<namespace>-cms-ingress
	INGRESS_HOST_NAME        = "%s.%s.cms.cubrid.com" // <pod-name>.<namespace>.cms.cubrid.com

	ADD_REPLICALINK    = 0
	DELETE_REPLICALINK = 1

	// BackupDB
	BackupDB_Schedule         = "0 0 * * 0"
	BackupdbArgs              = "start demodb 0 7"
	BackupDB_Script_File_Path = "/home/cubrid/CUBRID/share/scripts/backupdb.sh"
	BackupDB_StorageType      = StorageType_backup
	BackupDB_COMPLETED        = "Completed"
	BackupDB_FAILED           = "Failed"

	// HA Mode
	HAMODE_ON         = "ON"
	HAMODE_OFF        = "OFF"
	HAMODE_STANDALONE = "STANDALONE"
	HAMODE_MASTER     = "master"
	HAMODE_SLAVE      = "slave"
	HAMODE_REPLICA    = "replica"
	HAMODE_UNKONW     = "Unkonw"

	// Cubrid Manager Server
	CMS_HTTPS_URL = "https://%s:%d/cm_api"
	CMS_ID        = "cm_info"
	CMS_PW        = "cm_inf0pw"
	CMS_PORT      = 8001
	CMS_VERSION   = "11.4"

	// CMS API (task name)
	CMS_CMD_LOGIN     = "login"
	CMS_CMD_HA_STATUS = "ha_status"

	POD_DNS_FULL_NAME  = "%s.%s.%s.svc.cluster.local" // <pod-name>.<headless-service-name>.<namespace>.<base-domain>
	POD_DNS_SHORT_NAME = "%s.%s"                      // <pod-name>.<headless-service-name>

	// NodePortRangeMin is the minimum value for NodePort
	NodePortRangeMin = 30000
	// NodePortRangeMax is the maximum value for NodePort
	NodePortRangeMax = 32767
)
