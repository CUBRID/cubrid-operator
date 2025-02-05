package settings

const (
	CubridDefaultImage        = "cubrid/cubrid:latest"
	DefaultCUBRIDPath         = "/home/cubrid/CUBRID"
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

	DefaultStorageClassName = "longhorn" // standard로 변경이 필요함

	// initcontainer
	InitCopyConfContainerName     = "init-copy-conf"
	InitRecoveryConfContainerName = "init-recovery-conf"
	InitCopyConfCommand           = "cp -rn /home/cubrid/CUBRID/conf/* /mnt/conf"
	InitRecoveryConfCommand       = "cp -rn /mnt/conf/* /home/cubrid/CUBRID/conf/ && " +
		"chown -R 1000:1000 /home/cubrid/CUBRID/conf && " +
		"chown -R 1000:1000 /home/cubrid/CUBRID/databases && " +
		"chown -R 1000:1000 /home/cubrid/CUBRID/backupdb && " +
		"chown -R 1000:1000 /home/cubrid/CUBRID/log"
	BusyBoxImage = "busybox"

	// user
	CubridUser  = int64(1000)
	CubridGroup = int64(1000)
	RootUser    = int64(0)

	HA_MASTER_SLAVE_TYPE = "master-slave"
	HA_REPLICA_TYPE      = "replica"
	SVC_SUFFIX           = "-int"
	SELECTOR_SUFFIX      = "-group"

	ADD_REPLICALINK    = 0
	DELETE_REPLICALINK = 1

	// BackupDB
	BackupDB_Script_File_Path = "/home/cubrid/CUBRID/share/scripts/backupdb.sh"
	BackupDB_StorageType      = StorageType_backup
	BackupDB_IDLE             = "Idle"
	BackupDB_PENDING          = "Pending"
	BackupDB_INPROGRESS       = "InProgress"
	BackupDB_COMPLETED        = "Completed"
	BackupDB_FAILED           = "Failed"

	// HA Mode
	HAMODE_ON         = "ON"
	HAMODE_OFF        = "OFF"
	HAMODE_STANDALONE = "STANDALONE"
	HAMODE_MASTER     = "Master"
	HAMODE_SLAVE      = "Slave"
	HAMODE_REPLICA    = "Replica"
	HAMODE_UNKONW     = "Unkonw"

	// Cubrid Manager
	CMS_HTTPS_URL = "https://%s:%d/cm_api"
	CMS_ID        = "testcm"  // 반영할때는 cms_cr
	CMS_PW        = "testpwd" // 반영할때는 cms_pwd
	CMS_PORT      = 8001
	CMS_VERSION   = "11.4"

	// CMS API (task name)
	CMS_CMD_LOGIN     = "login"
	CMS_CMD_HA_STATUS = "ha_status"

	POD_DNS_FULL_NAME  = "%s.%s.%s.svc.cluster.local" // <pod-name>.<headless-service-name>.<namespace>.<base-domain>
	POD_DNS_SHORT_NAME = "%s.%s"                      // <pod-name>.<headless-service-name>
)

var (
	/*********************************
		 Configuration cubrid.conf
	**********************************/

	// ha_mode templete
	HaModeTemplate = `
		if grep -q '^[#[:space:]]*ha_mode[[:space:]]*=' %s/conf/cubrid.conf; then
			sed -i "s/^[#[:space:]]*ha_mode[[:space:]]*=.*/ha_mode=on/" %s/conf/cubrid.conf
		else
			echo "ha_mode=on" >> %s/conf/cubrid.conf
		fi
	`

	// ha_mode templete
	HaReplicaModeTemplate = `
		if grep -q '^[#[:space:]]*ha_mode[[:space:]]*=' %s/conf/cubrid.conf; then
			sed -i "s/^[#[:space:]]*ha_mode[[:space:]]*=.*/ha_mode=replica/" %s/conf/cubrid.conf
		else
			echo "ha_mode=replica" >> %s/conf/cubrid.conf
		fi
	`

	// log templete
	HaLogMaxArchivesTemplate = `
		sed -i 's/^[:space:]*log_max_archives[[:space:]]*==[[:space:]]*[0-9]*/log_max_archives=900/' %s/conf/cubrid.conf

		if grep -q '^[#[:space:]]*force_remove_log_archives[[:space:]]*=' %s/conf/cubrid.conf; then
			sed -i "s/^[#[:space:]]*force_remove_log_archives[[:space:]]*=.*/force_remove_log_archives=no/" %s/conf/cubrid.conf
		else
			echo "force_remove_log_archives=no" >> %s/conf/cubrid.conf
		fi
	`

	/*********************************
		 Configuration cubrid_ha.conf
	**********************************/
	// Common templete
	HaCommonConfigTemplate = `
		sed -i 's/^[#[:space:]]*\[\s*common\s*\]/[common]/' %s/conf/cubrid_ha.conf
		sed -i 's/^[#[:space:]]*ha_port_id[[:space:]]*=/ha_port_id=/' %s/conf/cubrid_ha.conf
		sed -i 's/^[#[:space:]]*ha_db_list[[:space:]]*=/ha_db_list=/' %s/conf/cubrid_ha.conf
		sed -i 's/^[#[:space:]]*ha_apply_max_mem_size[[:space:]]*=/ha_apply_max_mem_size=/' %s/conf/cubrid_ha.conf
		sed -i 's/^[#[:space:]]*ha_copy_log_max_archives[[:space:]]*=/ha_copy_log_max_archives=/' %s/conf/cubrid_ha.conf
	`

	// ha_node_list templete
	HaNodeListTemplate = `
		if grep -q '^[#[:space:]]*ha_node_list[[:space:]]*=' %s/conf/cubrid_ha.conf; then
			sed -i "s/^[#[:space:]]*ha_node_list[[:space:]]*=.*/ha_node_list=%s/" %s/conf/cubrid_ha.conf
		else
			echo "ha_node_list=%s" >> %s/conf/cubrid_ha.conf
		fi
	`
	// ha_copy_sync_mode templete
	HaSyncModeTemplate = `
		if grep -q '^[#[:space:]]*ha_copy_sync_mode[[:space:]]*=' %s/conf/cubrid_ha.conf; then
			sed -i "s/^[#[:space:]]*ha_copy_sync_mode[[:space:]]*=.*/ha_copy_sync_mode=%s/" %s/conf/cubrid_ha.conf
		else
			echo "ha_copy_sync_mode=%s" >> %s/conf/cubrid_ha.conf
		fi
	`

	// ha_replica_list templete
	HaReplicaListTemplate = `
		if grep -q '^[#[:space:]]*ha_replica_list[[:space:]]*=' %s/conf/cubrid_ha.conf; then
			sed -i "s/^[#[:space:]]*ha_replica_list[[:space:]]*=.*/ha_replica_list=%s/" %s/conf/cubrid_ha.conf
		else
			echo "ha_replica_list=%s" >> %s/conf/cubrid_ha.conf
		fi
	`
	// delete ha_copy_sync_mode
	DelReplicaListTemplate = "sed -i '/^[#[:space:]]*ha_replica_list[[:space:]]*=.*/d' %s/conf/cubrid_ha.conf"

	BackupdbArgs = []string{"start", "demodb", "0", "7"}
)
