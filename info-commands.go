//
// Copyright (c) 2015-2024 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.
//

package madmin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

//msgp:clearomitted
//msgp:tag json
//msgp:timezone utc
//go:generate msgp -file $GOFILE

// BackendType - represents different backend types.
type BackendType int

// Enum for different backend types.
const (
	Unknown BackendType = iota
	// Filesystem backend.
	FS
	// Multi disk Erasure (single, distributed) backend.
	Erasure
	// Gateway to other storage
	Gateway

	// Add your own backend.
)

// ItemState - represents the status of any item in offline,init,online state
type ItemState string

const (

	// ItemOffline indicates that the item is offline
	ItemOffline = ItemState("offline")
	// ItemInitializing indicates that the item is still in initialization phase
	ItemInitializing = ItemState("initializing")
	// ItemOnline indicates that the item is online
	ItemOnline = ItemState("online")
)

// StorageInfo - represents total capacity of underlying storage.
type StorageInfo struct {
	Disks []Disk

	// Backend type.
	Backend BackendInfo
}

// BackendInfo - contains info of the underlying backend
type BackendInfo struct {
	// Represents various backend types, currently on FS, Erasure and Gateway
	Type BackendType

	// Following fields are only meaningful if BackendType is Gateway.
	GatewayOnline bool

	// Following fields are only meaningful if BackendType is Erasure.
	OnlineDisks  BackendDisks // Online disks during server startup.
	OfflineDisks BackendDisks // Offline disks during server startup.

	// Following fields are only meaningful if BackendType is Erasure.
	StandardSCData     []int // Data disks for currently configured Standard storage class.
	StandardSCParities []int // Parity disks per pool for currently configured Standard storage class
	RRSCData           []int // Data disks for currently configured Reduced Redundancy storage class.
	RRSCParities       []int // Parity disks per pool for currently configured Reduced Redundancy storage class.

	// Adds number of erasure sets and drives per set.
	TotalSets    []int // Each index value corresponds to per pool
	DrivesPerSet []int // Each index value corresponds to per pool
}

// BackendDisks - represents the map of endpoint-disks.
type BackendDisks map[string]int

// Sum - Return the sum of the disks in the endpoint-disk map.
func (d1 BackendDisks) Sum() (sum int) {
	for _, count := range d1 {
		sum += count
	}
	return sum
}

// Merge - Reduces two endpoint-disk maps.
func (d1 BackendDisks) Merge(d2 BackendDisks) BackendDisks {
	if len(d2) == 0 {
		d2 = make(BackendDisks)
	}
	merged := make(BackendDisks)
	for i1, v1 := range d1 {
		if v2, ok := d2[i1]; ok {
			merged[i1] = v2 + v1
			continue
		}
		merged[i1] = v1
	}
	return merged
}

// StorageInfo - Connect to a minio server and call Storage Info Management API
// to fetch server's information represented by StorageInfo structure
func (adm *AdminClient) StorageInfo(ctx context.Context) (StorageInfo, error) {
	resp, err := adm.executeMethod(ctx, http.MethodGet, requestData{relPath: adminAPIPrefix + "/storageinfo"})
	defer closeResponse(resp)
	if err != nil {
		return StorageInfo{}, err
	}

	// Check response http status code
	if resp.StatusCode != http.StatusOK {
		return StorageInfo{}, httpRespToErrorResponse(resp)
	}

	// Unmarshal the server's json response
	var storageInfo StorageInfo
	if err = json.NewDecoder(resp.Body).Decode(&storageInfo); err != nil {
		return StorageInfo{}, err
	}

	return storageInfo, nil
}

// BucketUsageInfo - bucket usage info provides
// - total size of the bucket
// - total objects in a bucket
// - object size histogram per bucket
type BucketUsageInfo struct {
	Size                    uint64 `json:"size"`
	ReplicationPendingSize  uint64 `json:"objectsPendingReplicationTotalSize"`
	ReplicationFailedSize   uint64 `json:"objectsFailedReplicationTotalSize"`
	ReplicatedSize          uint64 `json:"objectsReplicatedTotalSize"`
	ReplicaSize             uint64 `json:"objectReplicaTotalSize"`
	ReplicationPendingCount uint64 `json:"objectsPendingReplicationCount"`
	ReplicationFailedCount  uint64 `json:"objectsFailedReplicationCount"`

	VersionsCount           uint64            `json:"versionsCount"`
	ObjectsCount            uint64            `json:"objectsCount"`
	DeleteMarkersCount      uint64            `json:"deleteMarkersCount"`
	ObjectSizesHistogram    map[string]uint64 `json:"objectsSizesHistogram"`
	ObjectVersionsHistogram map[string]uint64 `json:"objectsVersionsHistogram"`
}

// DataUsageInfo represents data usage stats of the underlying Object API
type DataUsageInfo struct {
	// LastUpdate is the timestamp of when the data usage info was last updated.
	// This does not indicate a full scan.
	LastUpdate time.Time `json:"lastUpdate"`

	// Objects total count across all buckets
	ObjectsTotalCount uint64 `json:"objectsCount"`

	// Objects total size across all buckets
	ObjectsTotalSize uint64 `json:"objectsTotalSize"`

	// Total Size for objects that have not yet been replicated
	ReplicationPendingSize uint64 `json:"objectsPendingReplicationTotalSize"`

	// Total size for objects that have witness one or more failures and will be retried
	ReplicationFailedSize uint64 `json:"objectsFailedReplicationTotalSize"`

	// Total size for objects that have been replicated to destination
	ReplicatedSize uint64 `json:"objectsReplicatedTotalSize"`

	// Total size for objects that are replicas
	ReplicaSize uint64 `json:"objectsReplicaTotalSize"`

	// Total number of objects pending replication
	ReplicationPendingCount uint64 `json:"objectsPendingReplicationCount"`

	// Total number of objects that failed replication
	ReplicationFailedCount uint64 `json:"objectsFailedReplicationCount"`

	// Total number of buckets in this cluster
	BucketsCount uint64 `json:"bucketsCount"`

	// Buckets usage info provides following information across all buckets
	// - total size of the bucket
	// - total objects in a bucket
	// - object size histogram per bucket
	BucketsUsage map[string]BucketUsageInfo `json:"bucketsUsageInfo"`

	// TierStats holds per-tier stats like bytes tiered, etc.
	TierStats map[string]TierStats `json:"tierStats"`

	// Server capacity related data
	TotalCapacity     uint64 `json:"capacity"`
	TotalFreeCapacity uint64 `json:"freeCapacity"`
	TotalUsedCapacity uint64 `json:"usedCapacity"`
}

// DataUsageInfo - returns data usage of the current object API
func (adm *AdminClient) DataUsageInfo(ctx context.Context) (DataUsageInfo, error) {
	values := make(url.Values)
	values.Set("capacity", "true") // We can make this configurable in future but for now its fine.

	resp, err := adm.executeMethod(ctx, http.MethodGet, requestData{
		relPath:     adminAPIPrefix + "/datausageinfo",
		queryValues: values,
	})
	defer closeResponse(resp)
	if err != nil {
		return DataUsageInfo{}, err
	}

	// Check response http status code
	if resp.StatusCode != http.StatusOK {
		return DataUsageInfo{}, httpRespToErrorResponse(resp)
	}

	// Unmarshal the server's json response
	var dataUsageInfo DataUsageInfo
	if err = json.NewDecoder(resp.Body).Decode(&dataUsageInfo); err != nil {
		return DataUsageInfo{}, err
	}

	return dataUsageInfo, nil
}

// ErasureSetInfo provides information per erasure set
type ErasureSetInfo struct {
	ID                 int      `json:"id"`
	RawUsage           uint64   `json:"rawUsage"`
	RawCapacity        uint64   `json:"rawCapacity"`
	Usage              uint64   `json:"usage"`
	ObjectsCount       uint64   `json:"objectsCount"`
	VersionsCount      uint64   `json:"versionsCount"`
	DeleteMarkersCount uint64   `json:"deleteMarkersCount"`
	HealDisks          int      `json:"healDisks"`
	OnlineDisks        int      `json:"onlineDisks,omitempty"`
	OfflineDisks       int      `json:"offlineDisks,omitempty"`
	Nodes              []string `json:"nodes,omitempty"`
}

// InfoMessage container to hold server admin related information.
type InfoMessage struct {
	Mode          string             `json:"mode,omitempty"`
	Domain        []string           `json:"domain,omitempty"`
	Region        string             `json:"region,omitempty"`
	SQSARN        []string           `json:"sqsARN,omitempty"`
	DeploymentID  string             `json:"deploymentID,omitempty"`
	Buckets       Buckets            `json:"buckets,omitempty"`
	Objects       Objects            `json:"objects,omitempty"`
	Versions      Versions           `json:"versions,omitempty"`
	DeleteMarkers DeleteMarkers      `json:"deletemarkers,omitempty"`
	Usage         Usage              `json:"usage,omitempty"`
	Services      Services           `json:"services,omitempty"`
	Backend       ErasureBackend     `json:"backend,omitempty"`
	Servers       []ServerProperties `json:"servers,omitempty"`

	Pools map[int]map[int]ErasureSetInfo `json:"pools,omitempty"`
}

func (info InfoMessage) BackendType() BackendType {
	// MinIO server type default
	switch info.Backend.Type {
	case "Erasure":
		return Erasure
	case "FS":
		return FS
	default:
		return Unknown
	}
}

func (info InfoMessage) StandardParity() int {
	switch info.BackendType() {
	case Erasure:
		return info.Backend.StandardSCParity
	default:
		return -1
	}
}

// Services contains different services information
type Services struct {
	KMS           KMS                           `json:"kms,omitempty"` // deprecated july 2023
	KMSStatus     []KMS                         `json:"kmsStatus,omitempty"`
	LDAP          LDAP                          `json:"ldap,omitempty"`
	Logger        []Logger                      `json:"logger,omitempty"`
	Audit         []Audit                       `json:"audit,omitempty"`
	Notifications []map[string][]TargetIDStatus `json:"notifications,omitempty"`
}

// ListNotificationARNs return a list of configured notification ARNs
func (s Services) ListNotificationARNs() (arns []ARN) {
	for _, notify := range s.Notifications {
		for targetType, targetStatuses := range notify {
			for _, targetStatus := range targetStatuses {
				for targetID := range targetStatus {
					arns = append(arns, ARN{
						Type:     "sqs",
						ID:       targetID,
						Resource: targetType,
					})
				}
			}
		}
	}
	return arns
}

// Buckets contains the number of buckets
type Buckets struct {
	Count uint64 `json:"count"`
	Error string `json:"error,omitempty"`
}

// Objects contains the number of objects
type Objects struct {
	Count uint64 `json:"count"`
	Error string `json:"error,omitempty"`
}

// Versions contains the number of versions
type Versions struct {
	Count uint64 `json:"count"`
	Error string `json:"error,omitempty"`
}

// DeleteMarkers contains the number of delete markers
type DeleteMarkers struct {
	Count uint64 `json:"count"`
	Error string `json:"error,omitempty"`
}

// Usage contains the total size used
type Usage struct {
	Size  uint64 `json:"size"`
	Error string `json:"error,omitempty"`
}

// TierStats contains per-tier statistics like total size, number of
// objects/versions transitioned, etc.
type TierStats struct {
	TotalSize   uint64 `json:"totalSize"`
	NumVersions int    `json:"numVersions"`
	NumObjects  int    `json:"numObjects"`
}

// KMS contains KMS status information
type KMS struct {
	Status   string `json:"status,omitempty"`
	Encrypt  string `json:"encrypt,omitempty"`
	Decrypt  string `json:"decrypt,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Version  string `json:"version,omitempty"`
}

// LDAP contains ldap status
type LDAP struct {
	Status string `json:"status,omitempty"`
}

// Status of endpoint
type Status struct {
	Status string `json:"status,omitempty"`
}

// Audit contains audit logger status
type Audit map[string]Status

// Logger contains logger status
type Logger map[string]Status

// TargetIDStatus containsid and status
type TargetIDStatus map[string]Status

//msgp:replace backendType with:string

// backendType - indicates the type of backend storage
type backendType string

const (
	// FsType - Backend is FS Type
	FsType = backendType("FS")
	// ErasureType - Backend is Erasure type
	ErasureType = backendType("Erasure")
)

// FSBackend contains specific FS storage information
type FSBackend struct {
	Type backendType `json:"backendType"`
}

// ErasureBackend contains specific erasure storage information
type ErasureBackend struct {
	Type         backendType `json:"backendType"`
	OnlineDisks  int         `json:"onlineDisks"`
	OfflineDisks int         `json:"offlineDisks"`
	// Parity disks for currently configured Standard storage class.
	StandardSCParity int `json:"standardSCParity"`
	// Parity disks for currently configured Reduced Redundancy storage class.
	RRSCParity int `json:"rrSCParity"`

	// Per pool information
	TotalSets    []int `json:"totalSets"`
	DrivesPerSet []int `json:"totalDrivesPerSet"`
}

// ServerProperties holds server information
type ServerProperties struct {
	State               string            `json:"state,omitempty"`
	Endpoint            string            `json:"endpoint,omitempty"`
	Scheme              string            `json:"scheme,omitempty"`
	Uptime              int64             `json:"uptime,omitempty"`
	Version             string            `json:"version,omitempty"`
	CommitID            string            `json:"commitID,omitempty"`
	Network             map[string]string `json:"network,omitempty"`
	Disks               []Disk            `json:"drives,omitempty"`
	PoolNumber          int               `json:"poolNumber,omitempty"` // Only set if len(PoolNumbers) == 1
	PoolNumbers         []int             `json:"poolNumbers,omitempty"`
	MemStats            MemStats          `json:"mem_stats"`
	GoMaxProcs          int               `json:"go_max_procs,omitempty"`
	NumCPU              int               `json:"num_cpu,omitempty"`
	RuntimeVersion      string            `json:"runtime_version,omitempty"`
	GCStats             *GCStats          `json:"gc_stats,omitempty"`
	MinioEnvVars        map[string]string `json:"minio_env_vars,omitempty"`
	Edition             string            `json:"edition"`
	License             *LicenseInfo      `json:"license,omitempty"`
	IsLeader            bool              `json:"is_leader"`
	ILMExpiryInProgress bool              `json:"ilm_expiry_in_progress"`
}

// MemStats is strip down version of runtime.MemStats containing memory stats of MinIO server.
type MemStats struct {
	Alloc      uint64
	TotalAlloc uint64
	Mallocs    uint64
	Frees      uint64
	HeapAlloc  uint64
}

// GCStats collect information about recent garbage collections.
type GCStats struct {
	LastGC     time.Time       `json:"last_gc"`     // time of last collection
	NumGC      int64           `json:"num_gc"`      // number of garbage collections
	PauseTotal time.Duration   `json:"pause_total"` // total pause for all collections
	Pause      []time.Duration `json:"pause"`       // pause history, most recent first
	PauseEnd   []time.Time     `json:"pause_end"`   // pause end times history, most recent first
}

// DiskMetrics has the information about XL Storage APIs
// the number of calls of each API and the moving average of
// the duration, in nanosecond, of each API.
type DiskMetrics struct {
	LastMinute map[string]TimedAction `json:"lastMinute,omitempty"`
	APICalls   map[string]uint64      `json:"apiCalls,omitempty"`

	// TotalTokens set per drive max concurrent I/O.
	TotalTokens uint32 `json:"totalTokens,omitempty"` // Deprecated (unused)

	// TotalWaiting the amount of concurrent I/O waiting on disk
	TotalWaiting uint32 `json:"totalWaiting,omitempty"`

	// Captures all data availability errors such as
	// permission denied, faulty disk and timeout errors.
	TotalErrorsAvailability uint64 `json:"totalErrorsAvailability,omitempty"`

	// Captures all timeout only errors
	TotalErrorsTimeout uint64 `json:"totalErrorsTimeout,omitempty"`

	// Total writes on disk (could be empty if the feature
	// is not enabled on the server)
	TotalWrites uint64 `json:"totalWrites,omitempty"`

	// Total deletes on disk (could be empty if the feature
	// is not enabled on the server)
	TotalDeletes uint64 `json:"totalDeletes,omitempty"`
}

// CacheStats drive cache stats
type CacheStats struct {
	Capacity   int64 `json:"capacity"`
	Used       int64 `json:"used"`
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	DelHits    int64 `json:"delHits"`
	DelMisses  int64 `json:"delMisses"`
	Collisions int64 `json:"collisions"`
}

// Disk holds Disk information
type Disk struct {
	Endpoint        string       `json:"endpoint,omitempty"`
	RootDisk        bool         `json:"rootDisk,omitempty"`
	DrivePath       string       `json:"path,omitempty"`
	Healing         bool         `json:"healing,omitempty"`
	Scanning        bool         `json:"scanning,omitempty"`
	State           string       `json:"state,omitempty"`
	UUID            string       `json:"uuid,omitempty"`
	Major           uint32       `json:"major"`
	Minor           uint32       `json:"minor"`
	Model           string       `json:"model,omitempty"`
	TotalSpace      uint64       `json:"totalspace,omitempty"`
	UsedSpace       uint64       `json:"usedspace,omitempty"`
	AvailableSpace  uint64       `json:"availspace,omitempty"`
	ReadThroughput  float64      `json:"readthroughput,omitempty"`
	WriteThroughPut float64      `json:"writethroughput,omitempty"`
	ReadLatency     float64      `json:"readlatency,omitempty"`
	WriteLatency    float64      `json:"writelatency,omitempty"`
	Utilization     float64      `json:"utilization,omitempty"`
	Metrics         *DiskMetrics `json:"metrics,omitempty"`
	HealInfo        *HealingDisk `json:"heal_info,omitempty"`
	UsedInodes      uint64       `json:"used_inodes"`
	FreeInodes      uint64       `json:"free_inodes,omitempty"`
	Local           bool         `json:"local,omitempty"`
	Cache           *CacheStats  `json:"cacheStats,omitempty"`

	// Indexes, will be -1 until assigned a set.
	PoolIndex int `json:"pool_index"`
	SetIndex  int `json:"set_index"`
	DiskIndex int `json:"disk_index"`
}

// ServerInfoOpts ask for additional data from the server
type ServerInfoOpts struct {
	Metrics bool
}

// WithDriveMetrics asks server to return additional metrics per drive
func WithDriveMetrics(metrics bool) func(*ServerInfoOpts) {
	return func(opts *ServerInfoOpts) {
		opts.Metrics = metrics
	}
}

// ServerInfo - Connect to a minio server and call Server Admin Info Management API
// to fetch server's information represented by infoMessage structure
func (adm *AdminClient) ServerInfo(ctx context.Context, options ...func(*ServerInfoOpts)) (InfoMessage, error) {
	srvOpts := &ServerInfoOpts{}

	for _, o := range options {
		o(srvOpts)
	}

	values := make(url.Values)
	values.Set("metrics", strconv.FormatBool(srvOpts.Metrics))

	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/info",
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return InfoMessage{}, err
	}

	// Check response http status code
	if resp.StatusCode != http.StatusOK {
		return InfoMessage{}, httpRespToErrorResponse(resp)
	}

	// Unmarshal the server's json response
	var message InfoMessage
	if err = json.NewDecoder(resp.Body).Decode(&message); err != nil {
		return InfoMessage{}, err
	}

	return message, nil
}
