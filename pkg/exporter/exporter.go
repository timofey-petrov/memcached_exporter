// Copyright 2020 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exporter

import (
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	helpmemcache "help/memcache"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	Namespace           = "memcached"
	subsystemLruCrawler = "lru_crawler"
	subsystemSlab       = "slab"
)

var errKeyNotFound = errors.New("key not found")

// Exporter collects metrics from a memcached server.
type Exporter struct {
	address   string
	timeout   time.Duration
	logger    *slog.Logger
	tlsConfig *tls.Config
	username  string
	password  string

	up                       *prometheus.Desc
	uptime                   *prometheus.Desc
	time                     *prometheus.Desc
	version                  *prometheus.Desc
	rusageUser               *prometheus.Desc
	rusageSystem             *prometheus.Desc
	bytesRead                *prometheus.Desc
	bytesWritten             *prometheus.Desc
	currentConnections       *prometheus.Desc
	maxConnections           *prometheus.Desc
	connectionsTotal         *prometheus.Desc
	rejectedConnections      *prometheus.Desc
	connsYieldedTotal        *prometheus.Desc
	listenerDisabledTotal    *prometheus.Desc
	currentBytes             *prometheus.Desc
	limitBytes               *prometheus.Desc
	commands                 *prometheus.Desc
	items                    *prometheus.Desc
	itemsTotal               *prometheus.Desc
	evictions                *prometheus.Desc
	reclaimed                *prometheus.Desc
	itemStoreTooLarge        *prometheus.Desc
	itemStoreNoMemory        *prometheus.Desc
	lruCrawlerEnabled        *prometheus.Desc
	lruCrawlerSleep          *prometheus.Desc
	lruCrawlerMaxItems       *prometheus.Desc
	lruMaintainerThread      *prometheus.Desc
	lruHotPercent            *prometheus.Desc
	lruWarmPercent           *prometheus.Desc
	lruHotMaxAgeFactor       *prometheus.Desc
	lruWarmMaxAgeFactor      *prometheus.Desc
	lruCrawlerStarts         *prometheus.Desc
	lruCrawlerReclaimed      *prometheus.Desc
	lruCrawlerItemsChecked   *prometheus.Desc
	lruCrawlerMovesToCold    *prometheus.Desc
	lruCrawlerMovesToWarm    *prometheus.Desc
	lruCrawlerMovesWithinLru *prometheus.Desc
	directReclaims           *prometheus.Desc
	malloced                 *prometheus.Desc
	itemsNumber              *prometheus.Desc
	itemsAge                 *prometheus.Desc
	itemsCrawlerReclaimed    *prometheus.Desc
	itemsEvicted             *prometheus.Desc
	itemsEvictedNonzero      *prometheus.Desc
	itemsEvictedTime         *prometheus.Desc
	itemsEvictedUnfetched    *prometheus.Desc
	itemsExpiredUnfetched    *prometheus.Desc
	itemsOutofmemory         *prometheus.Desc
	itemsReclaimed           *prometheus.Desc
	itemsTailrepairs         *prometheus.Desc
	itemsMovesToCold         *prometheus.Desc
	itemsMovesToWarm         *prometheus.Desc
	itemsMovesWithinLru      *prometheus.Desc
	itemsHot                 *prometheus.Desc
	itemsWarm                *prometheus.Desc
	itemsCold                *prometheus.Desc
	itemsTemporary           *prometheus.Desc
	itemsAgeOldestHot        *prometheus.Desc
	itemsAgeOldestWarm       *prometheus.Desc
	itemsLruHits             *prometheus.Desc
	slabsChunkSize           *prometheus.Desc
	slabsChunksPerPage       *prometheus.Desc
	slabsCurrentPages        *prometheus.Desc
	slabsCurrentChunks       *prometheus.Desc
	slabsChunksUsed          *prometheus.Desc
	slabsChunksFree          *prometheus.Desc
	slabsChunksFreeEnd       *prometheus.Desc
	slabsMemRequested        *prometheus.Desc
	slabsCommands            *prometheus.Desc
	extstoreCompactLost      *prometheus.Desc
	extstoreCompactRescues   *prometheus.Desc
	extstoreCompactSkipped   *prometheus.Desc
	extstorePageAllocs       *prometheus.Desc
	extstorePageEvictions    *prometheus.Desc
	extstorePageReclaims     *prometheus.Desc
	extstorePagesFree        *prometheus.Desc
	extstorePagesUsed        *prometheus.Desc
	extstoreObjectsEvicted   *prometheus.Desc
	extstoreObjectsRead      *prometheus.Desc
	extstoreObjectsWritten   *prometheus.Desc
	extstoreObjectsUsed      *prometheus.Desc
	extstoreBytesEvicted     *prometheus.Desc
	extstoreBytesWritten     *prometheus.Desc
	extstoreBytesRead        *prometheus.Desc
	extstoreBytesUsed        *prometheus.Desc
	extstoreBytesLimit       *prometheus.Desc
	extstoreBytesFragmented  *prometheus.Desc
	extstoreIOQueueDepth     *prometheus.Desc
	acceptingConnections     *prometheus.Desc
}

// New returns an initialized exporter.
func New(server string, timeout time.Duration, logger *slog.Logger, tlsConfig *tls.Config, username, password string) *Exporter {
	return &Exporter{
		address:   server,
		timeout:   timeout,
		logger:    logger,
		tlsConfig: tlsConfig,
		username:  username,
		password:  password,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "up"),
			"Could the memcached server be reached.",
			nil,
			nil,
		),
		uptime: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "uptime_seconds"),
			"Number of seconds since the server started.",
			nil,
			nil,
		),
		time: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "time_seconds"),
			"current UNIX time according to the server.",
			nil,
			nil,
		),
		version: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "version"),
			"The version of this memcached server.",
			[]string{"version"},
			nil,
		),
		rusageUser: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "process_user_cpu_seconds_total"),
			"Accumulated user time for this process.",
			nil,
			nil,
		),
		rusageSystem: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "process_system_cpu_seconds_total"),
			"Accumulated system time for this process.",
			nil,
			nil,
		),
		bytesRead: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "read_bytes_total"),
			"Total number of bytes read by this server from network.",
			nil,
			nil,
		),
		bytesWritten: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "written_bytes_total"),
			"Total number of bytes sent by this server to network.",
			nil,
			nil,
		),
		currentConnections: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "current_connections"),
			"Current number of open connections.",
			nil,
			nil,
		),
		maxConnections: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "max_connections"),
			"Maximum number of clients allowed.",
			nil,
			nil,
		),
		connectionsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "connections_total"),
			"Total number of connections opened since the server started running.",
			nil,
			nil,
		),
		rejectedConnections: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "connections_rejected_total"),
			"Total number of connections rejected due to hitting the memcached's -c limit in maxconns_fast mode.",
			nil,
			nil,
		),
		connsYieldedTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "connections_yielded_total"),
			"Total number of connections yielded running due to hitting the memcached's -R limit.",
			nil,
			nil,
		),
		listenerDisabledTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "connections_listener_disabled_total"),
			"Number of times that memcached has hit its connections limit and disabled its listener.",
			nil,
			nil,
		),
		currentBytes: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "current_bytes"),
			"Current number of bytes used to store items.",
			nil,
			nil,
		),
		limitBytes: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "limit_bytes"),
			"Number of bytes this server is allowed to use for storage.",
			nil,
			nil,
		),
		commands: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "commands_total"),
			"Total number of all requests broken down by command (get, set, etc.) and status.",
			[]string{"command", "status"},
			nil,
		),
		items: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "current_items"),
			"Current number of items stored by this instance.",
			nil,
			nil,
		),
		itemsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "items_total"),
			"Total number of items stored during the life of this instance.",
			nil,
			nil,
		),
		evictions: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "items_evicted_total"),
			"Total number of valid items removed from cache to free memory for new items.",
			nil,
			nil,
		),
		reclaimed: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "items_reclaimed_total"),
			"Total number of times an entry was stored using memory from an expired entry.",
			nil,
			nil,
		),
		itemStoreTooLarge: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "item_too_large_total"),
			"The number of times an item exceeded the max-item-size when being stored.",
			nil,
			nil,
		),
		itemStoreNoMemory: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "item_no_memory_total"),
			"The number of times an item could not be stored due to no more memory.",
			nil,
			nil,
		),
		directReclaims: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "direct_reclaims_total"),
			"Times worker threads had to directly reclaim or evict items.",
			nil,
			nil,
		),
		lruCrawlerEnabled: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "enabled"),
			"Whether the LRU crawler is enabled.",
			nil,
			nil,
		),
		lruCrawlerSleep: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "sleep"),
			"Microseconds to sleep between LRU crawls.",
			nil,
			nil,
		),
		lruCrawlerMaxItems: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "to_crawl"),
			"Max items to crawl per slab per run.",
			nil,
			nil,
		),
		lruMaintainerThread: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "maintainer_thread"),
			"Split LRU mode and background threads.",
			nil,
			nil,
		),
		lruHotPercent: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "hot_percent"),
			"Percent of slab memory reserved for HOT LRU.",
			nil,
			nil,
		),
		lruWarmPercent: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "warm_percent"),
			"Percent of slab memory reserved for WARM LRU.",
			nil,
			nil,
		),
		lruHotMaxAgeFactor: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "hot_max_factor"),
			"Set idle age of HOT LRU to COLD age * this",
			nil,
			nil,
		),
		lruWarmMaxAgeFactor: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "warm_max_factor"),
			"Set idle age of WARM LRU to COLD age * this",
			nil,
			nil,
		),
		lruCrawlerStarts: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "starts_total"),
			"Times an LRU crawler was started.",
			nil,
			nil,
		),
		lruCrawlerReclaimed: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "reclaimed_total"),
			"Total items freed by LRU Crawler.",
			nil,
			nil,
		),
		lruCrawlerItemsChecked: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "items_checked_total"),
			"Total items examined by LRU Crawler.",
			nil,
			nil,
		),
		lruCrawlerMovesToCold: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "moves_to_cold_total"),
			"Total number of items moved from HOT/WARM to COLD LRU's.",
			nil,
			nil,
		),
		lruCrawlerMovesToWarm: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "moves_to_warm_total"),
			"Total number of items moved from COLD to WARM LRU.",
			nil,
			nil,
		),
		lruCrawlerMovesWithinLru: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemLruCrawler, "moves_within_lru_total"),
			"Total number of items reshuffled within HOT or WARM LRU's.",
			nil,
			nil,
		),
		malloced: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "malloced_bytes"),
			"Number of bytes of memory allocated to slab pages.",
			nil,
			nil,
		),
		itemsNumber: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "current_items"),
			"Number of items currently stored in this slab class.",
			[]string{"slab"},
			nil,
		),
		itemsAge: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_age_seconds"),
			"Number of seconds the oldest item has been in the slab class.",
			[]string{"slab"},
			nil,
		),
		itemsCrawlerReclaimed: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_crawler_reclaimed_total"),
			"Number of items freed by the LRU Crawler.",
			[]string{"slab"},
			nil,
		),
		itemsEvicted: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_evicted_total"),
			"Total number of times an item had to be evicted from the LRU before it expired.",
			[]string{"slab"},
			nil,
		),
		itemsEvictedNonzero: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_evicted_nonzero_total"),
			"Total number of times an item which had an explicit expire time set had to be evicted from the LRU before it expired.",
			[]string{"slab"},
			nil,
		),
		itemsEvictedTime: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_evicted_time_seconds"),
			"Seconds since the last access for the most recent item evicted from this class.",
			[]string{"slab"},
			nil,
		),
		itemsEvictedUnfetched: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_evicted_unfetched_total"),
			"Total nmber of items evicted and never fetched.",
			[]string{"slab"},
			nil,
		),
		itemsExpiredUnfetched: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_expired_unfetched_total"),
			"Total number of valid items evicted from the LRU which were never touched after being set.",
			[]string{"slab"},
			nil,
		),
		itemsOutofmemory: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_outofmemory_total"),
			"Total number of items for this slab class that have triggered an out of memory error.",
			[]string{"slab"},
			nil,
		),
		itemsReclaimed: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_reclaimed_total"),
			"Total number of items reclaimed.",
			[]string{"slab"},
			nil,
		),
		itemsTailrepairs: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_tailrepairs_total"),
			"Total number of times the entries for a particular ID need repairing.",
			[]string{"slab"},
			nil,
		),
		itemsMovesToCold: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_moves_to_cold"),
			"Number of items moved from HOT or WARM into COLD.",
			[]string{"slab"},
			nil,
		),
		itemsMovesToWarm: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_moves_to_warm"),
			"Number of items moves from COLD into WARM.",
			[]string{"slab"},
			nil,
		),
		itemsMovesWithinLru: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "items_moves_within_lru"),
			"Number of times active items were bumped within HOT or WARM.",
			[]string{"slab"},
			nil,
		),
		itemsHot: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "hot_items"),
			"Number of items presently stored in the HOT LRU.",
			[]string{"slab"},
			nil,
		),
		itemsWarm: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "warm_items"),
			"Number of items presently stored in the WARM LRU.",
			[]string{"slab"},
			nil,
		),
		itemsCold: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "cold_items"),
			"Number of items presently stored in the COLD LRU.",
			[]string{"slab"},
			nil,
		),
		itemsTemporary: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "temporary_items"),
			"Number of items presently stored in the TEMPORARY LRU.",
			[]string{"slab"},
			nil,
		),
		itemsAgeOldestHot: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "hot_age_seconds"),
			"Age of the oldest item in HOT LRU.",
			[]string{"slab"},
			nil,
		),
		itemsAgeOldestWarm: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "warm_age_seconds"),
			"Age of the oldest item in HOT LRU.",
			[]string{"slab"},
			nil,
		),
		itemsLruHits: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "lru_hits_total"),
			"Number of get_hits to the LRU.",
			[]string{"slab", "lru"},
			nil,
		),
		slabsChunkSize: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "chunk_size_bytes"),
			"Number of bytes allocated to each chunk within this slab class.",
			[]string{"slab"},
			nil,
		),
		slabsChunksPerPage: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "chunks_per_page"),
			"Number of chunks within a single page for this slab class.",
			[]string{"slab"},
			nil,
		),
		slabsCurrentPages: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "current_pages"),
			"Number of pages allocated to this slab class.",
			[]string{"slab"},
			nil,
		),
		slabsCurrentChunks: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "current_chunks"),
			"Number of chunks allocated to this slab class.",
			[]string{"slab"},
			nil,
		),
		slabsChunksUsed: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "chunks_used"),
			"Number of chunks allocated to an item.",
			[]string{"slab"},
			nil,
		),
		slabsChunksFree: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "chunks_free"),
			"Number of chunks not yet allocated items.",
			[]string{"slab"},
			nil,
		),
		slabsChunksFreeEnd: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "chunks_free_end"),
			"Number of free chunks at the end of the last allocated page.",
			[]string{"slab"},
			nil,
		),
		slabsMemRequested: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "mem_requested_bytes"),
			"Number of bytes of memory actual items take up within a slab.",
			[]string{"slab"},
			nil,
		),
		slabsCommands: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystemSlab, "commands_total"),
			"Total number of all requests broken down by command (get, set, etc.) and status per slab.",
			[]string{"slab", "command", "status"},
			nil,
		),
		extstoreCompactLost: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_compact_lost_total"),
			"Total number of items lost because they were locked during extstore compaction.",
			nil,
			nil,
		),
		extstoreCompactRescues: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_compact_rescued_total"),
			"Total number of items moved to a new page during extstore compaction,",
			nil,
			nil,
		),
		extstoreCompactSkipped: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_compact_skipped_total"),
			"Total number of items dropped due to inactivity during extstore compaction.",
			nil,
			nil,
		),
		extstorePageAllocs: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_pages_allocated_total"),
			"Total number of times a page was allocated in extstore.",
			nil,
			nil,
		),
		extstorePageEvictions: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_pages_evicted_total"),
			"Total number of times a page was evicted from extstore.",
			nil,
			nil,
		),
		extstorePageReclaims: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_pages_reclaimed_total"),
			"Total number of times an empty extstore page was freed.",
			nil,
			nil,
		),
		extstorePagesFree: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_pages_free"),
			"Number of extstore pages not yet containing any items.",
			nil,
			nil,
		),
		extstorePagesUsed: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_pages_used"),
			"Number of extstore pages containing at least one item.",
			nil,
			nil,
		),
		extstoreObjectsEvicted: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_objects_evicted_total"),
			"Total number of items evicted from extstore to free up space.",
			nil,
			nil,
		),
		extstoreObjectsRead: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_objects_read_total"),
			"Total number of items read from extstore.",
			nil,
			nil,
		),
		extstoreObjectsWritten: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_objects_written_total"),
			"Total number of items written to extstore.",
			nil,
			nil,
		),
		extstoreObjectsUsed: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_objects_used"),
			"Number of items stored in extstore.",
			nil,
			nil,
		),
		extstoreBytesEvicted: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_bytes_evicted_total"),
			"Total number of bytes evicted from extstore to free up space.",
			nil,
			nil,
		),
		extstoreBytesWritten: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_bytes_written_total"),
			"Total number of bytes written to extstore.",
			nil,
			nil,
		),
		extstoreBytesRead: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_bytes_read_total"),
			"Total number of bytes read from extstore.",
			nil,
			nil,
		),
		extstoreBytesUsed: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_bytes_used"),
			"Current number of bytes used to store items in extstore.",
			nil,
			nil,
		),
		extstoreBytesFragmented: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_bytes_fragmented"),
			"Current number of bytes in extstore pages allocated but not used to store an object.",
			nil,
			nil,
		),
		extstoreBytesLimit: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_bytes_limit"),
			"Number of bytes of external storage allocated for this server.",
			nil,
			nil,
		),
		extstoreIOQueueDepth: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "extstore_io_queue_depth"),
			"Number of items in the I/O queue waiting to be processed.",
			nil,
			nil,
		),
		acceptingConnections: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "accepting_connections"),
			"The Memcached server is currently accepting new connections.",
			nil,
			nil,
		),
	}
}

// Describe describes all the metrics exported by the memcached exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up
	ch <- e.uptime
	ch <- e.time
	ch <- e.version
	ch <- e.rusageUser
	ch <- e.rusageSystem
	ch <- e.bytesRead
	ch <- e.bytesWritten
	ch <- e.currentConnections
	ch <- e.maxConnections
	ch <- e.connectionsTotal
	ch <- e.rejectedConnections
	ch <- e.connsYieldedTotal
	ch <- e.listenerDisabledTotal
	ch <- e.currentBytes
	ch <- e.limitBytes
	ch <- e.commands
	ch <- e.items
	ch <- e.itemsTotal
	ch <- e.evictions
	ch <- e.reclaimed
	ch <- e.itemStoreTooLarge
	ch <- e.itemStoreNoMemory
	ch <- e.lruCrawlerEnabled
	ch <- e.lruCrawlerSleep
	ch <- e.lruCrawlerMaxItems
	ch <- e.lruMaintainerThread
	ch <- e.lruHotPercent
	ch <- e.lruWarmPercent
	ch <- e.lruHotMaxAgeFactor
	ch <- e.lruWarmMaxAgeFactor
	ch <- e.lruCrawlerStarts
	ch <- e.directReclaims
	ch <- e.lruCrawlerReclaimed
	ch <- e.lruCrawlerItemsChecked
	ch <- e.lruCrawlerMovesToCold
	ch <- e.lruCrawlerMovesToWarm
	ch <- e.lruCrawlerMovesWithinLru
	ch <- e.itemsLruHits
	ch <- e.malloced
	ch <- e.itemsNumber
	ch <- e.itemsAge
	ch <- e.itemsCrawlerReclaimed
	ch <- e.itemsEvicted
	ch <- e.itemsEvictedNonzero
	ch <- e.itemsEvictedTime
	ch <- e.itemsEvictedUnfetched
	ch <- e.itemsExpiredUnfetched
	ch <- e.itemsOutofmemory
	ch <- e.itemsReclaimed
	ch <- e.itemsTailrepairs
	ch <- e.itemsExpiredUnfetched
	ch <- e.itemsMovesToCold
	ch <- e.itemsMovesToWarm
	ch <- e.itemsMovesWithinLru
	ch <- e.itemsHot
	ch <- e.itemsWarm
	ch <- e.itemsCold
	ch <- e.itemsTemporary
	ch <- e.itemsAgeOldestHot
	ch <- e.itemsAgeOldestWarm
	ch <- e.slabsChunkSize
	ch <- e.slabsChunksPerPage
	ch <- e.slabsCurrentPages
	ch <- e.slabsCurrentChunks
	ch <- e.slabsChunksUsed
	ch <- e.slabsChunksFree
	ch <- e.slabsChunksFreeEnd
	ch <- e.slabsMemRequested
	ch <- e.slabsCommands
	ch <- e.extstoreCompactLost
	ch <- e.extstoreCompactRescues
	ch <- e.extstoreCompactSkipped
	ch <- e.extstorePageAllocs
	ch <- e.extstorePageEvictions
	ch <- e.extstorePageReclaims
	ch <- e.extstorePagesFree
	ch <- e.extstorePagesUsed
	ch <- e.extstoreObjectsEvicted
	ch <- e.extstoreObjectsRead
	ch <- e.extstoreObjectsWritten
	ch <- e.extstoreObjectsUsed
	ch <- e.extstoreBytesEvicted
	ch <- e.extstoreBytesWritten
	ch <- e.extstoreBytesRead
	ch <- e.extstoreBytesUsed
	ch <- e.extstoreBytesFragmented
	ch <- e.extstoreBytesLimit
	ch <- e.extstoreIOQueueDepth
	ch <- e.acceptingConnections
}

// Collect fetches the statistics from the configured memcached server, and
// delivers them as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	c := helpmemcache.New(e.address)
	c.Timeout = e.timeout
	c.TlsConfig = e.tlsConfig
	if e.username != "" || e.password != "" {
		c.SetAuth(e.username, e.password)
	}

	up := float64(1)
	stats, err := c.Stats()
	if err != nil {
		e.logger.Error("Failed to collect stats from memcached", "err", err)
		up = 0
	}
	statsSettings, err := c.StatsSettings()
	if err != nil {
		e.logger.Error("Could not query stats settings", "err", err)
		up = 0
	}

	if err := e.parseStats(ch, stats); err != nil {
		up = 0
	}
	if err := e.parseStatsSettings(ch, statsSettings); err != nil {
		up = 0
	}

	ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, up)
}

func (e *Exporter) parseStats(ch chan<- prometheus.Metric, stats map[net.Addr]memcache.Stats) error {
	// TODO(ts): Clean up and consolidate metric mappings.
	itemsCounterMetrics := map[string]*prometheus.Desc{
		"crawler_reclaimed": e.itemsCrawlerReclaimed,
		"evicted":           e.itemsEvicted,
		"evicted_nonzero":   e.itemsEvictedNonzero,
		"evicted_time":      e.itemsEvictedTime,
		"evicted_unfetched": e.itemsEvictedUnfetched,
		"expired_unfetched": e.itemsExpiredUnfetched,
		"outofmemory":       e.itemsOutofmemory,
		"reclaimed":         e.itemsReclaimed,
		"store_too_large":   e.itemStoreTooLarge,
		"store_no_memory":   e.itemStoreNoMemory,
		"tailrepairs":       e.itemsTailrepairs,
		"mem_requested":     e.slabsMemRequested,
		"moves_to_cold":     e.itemsMovesToCold,
		"moves_to_warm":     e.itemsMovesToWarm,
		"moves_within_lru":  e.itemsMovesWithinLru,
	}

	itemsGaugeMetrics := map[string]*prometheus.Desc{
		"number_hot":  e.itemsHot,
		"number_warm": e.itemsWarm,
		"number_cold": e.itemsCold,
		"number_temp": e.itemsTemporary,
		"age_hot":     e.itemsAgeOldestHot,
		"age_warm":    e.itemsAgeOldestWarm,
	}

	var parseError error
	for _, t := range stats {
		s := t.Stats
		ch <- prometheus.MustNewConstMetric(e.version, prometheus.GaugeValue, 1, s["version"])

		for _, op := range []string{"get", "delete", "incr", "decr", "cas", "touch"} {
			err := firstError(
				e.parseAndNewMetric(ch, e.commands, prometheus.CounterValue, s, op+"_hits", op, "hit"),
				e.parseAndNewMetric(ch, e.commands, prometheus.CounterValue, s, op+"_misses", op, "miss"),
			)
			if err != nil {
				parseError = err
			}
		}
		err := firstError(
			e.parseAndNewMetric(ch, e.uptime, prometheus.CounterValue, s, "uptime"),
			e.parseAndNewMetric(ch, e.time, prometheus.GaugeValue, s, "time"),
			e.parseAndNewMetric(ch, e.commands, prometheus.CounterValue, s, "cas_badval", "cas", "badval"),
			e.parseAndNewMetric(ch, e.commands, prometheus.CounterValue, s, "cmd_flush", "flush", "hit"),
		)
		if err != nil {
			parseError = err
		}

		// memcached includes cas operations again in cmd_set.
		setCmd, err := parse(s, "cmd_set", e.logger)
		if err == nil {
			if cas, casErr := sum(s, "cas_misses", "cas_hits", "cas_badval"); casErr == nil {
				ch <- prometheus.MustNewConstMetric(e.commands, prometheus.CounterValue, setCmd-cas, "set", "hit")
			} else {
				e.logger.Error("Failed to parse cas", "err", casErr)
				parseError = casErr
			}
		} else {
			e.logger.Error("Failed to parse set", "err", err)
			parseError = err
		}

		// extstore stats are only included if extstore is actually active. Take the presence of the
		// maxbytes key as a signal that they all should be there and do the parsing
		if _, ok := s["extstore_limit_maxbytes"]; ok {
			err = firstError(
				e.parseAndNewMetric(ch, e.extstoreCompactLost, prometheus.CounterValue, s, "extstore_compact_lost"),
				e.parseAndNewMetric(ch, e.extstoreCompactRescues, prometheus.CounterValue, s, "extstore_compact_rescues"),
				e.parseAndNewMetric(ch, e.extstoreCompactSkipped, prometheus.CounterValue, s, "extstore_compact_skipped"),
				e.parseAndNewMetric(ch, e.extstorePageAllocs, prometheus.CounterValue, s, "extstore_page_allocs"),
				e.parseAndNewMetric(ch, e.extstorePageEvictions, prometheus.CounterValue, s, "extstore_page_evictions"),
				e.parseAndNewMetric(ch, e.extstorePageReclaims, prometheus.CounterValue, s, "extstore_page_reclaims"),
				e.parseAndNewMetric(ch, e.extstorePagesFree, prometheus.GaugeValue, s, "extstore_pages_free"),
				e.parseAndNewMetric(ch, e.extstorePagesUsed, prometheus.GaugeValue, s, "extstore_pages_used"),
				e.parseAndNewMetric(ch, e.extstoreObjectsEvicted, prometheus.CounterValue, s, "extstore_objects_evicted"),
				e.parseAndNewMetric(ch, e.extstoreObjectsRead, prometheus.CounterValue, s, "extstore_objects_read"),
				e.parseAndNewMetric(ch, e.extstoreObjectsWritten, prometheus.CounterValue, s, "extstore_objects_written"),
				e.parseAndNewMetric(ch, e.extstoreObjectsUsed, prometheus.GaugeValue, s, "extstore_objects_used"),
				e.parseAndNewMetric(ch, e.extstoreBytesEvicted, prometheus.CounterValue, s, "extstore_bytes_evicted"),
				e.parseAndNewMetric(ch, e.extstoreBytesWritten, prometheus.CounterValue, s, "extstore_bytes_written"),
				e.parseAndNewMetric(ch, e.extstoreBytesRead, prometheus.CounterValue, s, "extstore_bytes_read"),
				e.parseAndNewMetric(ch, e.extstoreBytesUsed, prometheus.CounterValue, s, "extstore_bytes_used"),
				e.parseAndNewMetric(ch, e.extstoreBytesFragmented, prometheus.GaugeValue, s, "extstore_bytes_fragmented"),
				e.parseAndNewMetric(ch, e.extstoreBytesLimit, prometheus.GaugeValue, s, "extstore_limit_maxbytes"),
				e.parseAndNewMetric(ch, e.extstoreIOQueueDepth, prometheus.GaugeValue, s, "extstore_io_queue"),
			)
			if err != nil {
				parseError = err
			}
		}

		err = firstError(
			e.parseTimevalAndNewMetric(ch, e.rusageUser, prometheus.CounterValue, s, "rusage_user"),
			e.parseTimevalAndNewMetric(ch, e.rusageSystem, prometheus.CounterValue, s, "rusage_system"),
			e.parseAndNewMetric(ch, e.currentBytes, prometheus.GaugeValue, s, "bytes"),
			e.parseAndNewMetric(ch, e.limitBytes, prometheus.GaugeValue, s, "limit_maxbytes"),
			e.parseAndNewMetric(ch, e.items, prometheus.GaugeValue, s, "curr_items"),
			e.parseAndNewMetric(ch, e.itemsTotal, prometheus.CounterValue, s, "total_items"),
			e.parseAndNewMetric(ch, e.bytesRead, prometheus.CounterValue, s, "bytes_read"),
			e.parseAndNewMetric(ch, e.bytesWritten, prometheus.CounterValue, s, "bytes_written"),
			e.parseAndNewMetric(ch, e.currentConnections, prometheus.GaugeValue, s, "curr_connections"),
			e.parseAndNewMetric(ch, e.connectionsTotal, prometheus.CounterValue, s, "total_connections"),
			e.parseAndNewMetric(ch, e.rejectedConnections, prometheus.CounterValue, s, "rejected_connections"),
			e.parseAndNewMetric(ch, e.connsYieldedTotal, prometheus.CounterValue, s, "conn_yields"),
			e.parseAndNewMetric(ch, e.listenerDisabledTotal, prometheus.CounterValue, s, "listen_disabled_num"),
			e.parseAndNewMetric(ch, e.evictions, prometheus.CounterValue, s, "evictions"),
			e.parseAndNewMetric(ch, e.reclaimed, prometheus.CounterValue, s, "reclaimed"),
			e.parseAndNewMetric(ch, e.itemStoreTooLarge, prometheus.CounterValue, s, "store_too_large"),
			e.parseAndNewMetric(ch, e.itemStoreNoMemory, prometheus.CounterValue, s, "store_no_memory"),
			e.parseAndNewMetric(ch, e.lruCrawlerStarts, prometheus.CounterValue, s, "lru_crawler_starts"),
			e.parseAndNewMetric(ch, e.directReclaims, prometheus.CounterValue, s, "direct_reclaims"),
			e.parseAndNewMetric(ch, e.lruCrawlerItemsChecked, prometheus.CounterValue, s, "crawler_items_checked"),
			e.parseAndNewMetric(ch, e.lruCrawlerReclaimed, prometheus.CounterValue, s, "crawler_reclaimed"),
			e.parseAndNewMetric(ch, e.lruCrawlerMovesToCold, prometheus.CounterValue, s, "moves_to_cold"),
			e.parseAndNewMetric(ch, e.lruCrawlerMovesToWarm, prometheus.CounterValue, s, "moves_to_warm"),
			e.parseAndNewMetric(ch, e.lruCrawlerMovesWithinLru, prometheus.CounterValue, s, "moves_within_lru"),
			e.parseAndNewMetric(ch, e.malloced, prometheus.GaugeValue, s, "total_malloced"),
			e.parseAndNewMetric(ch, e.acceptingConnections, prometheus.GaugeValue, s, "accepting_conns"),
		)
		if err != nil {
			parseError = err
		}

		for slab, u := range t.Items {
			slab := strconv.Itoa(slab)
			err := firstError(
				e.parseAndNewMetric(ch, e.itemsNumber, prometheus.GaugeValue, u, "number", slab),
				e.parseAndNewMetric(ch, e.itemsAge, prometheus.GaugeValue, u, "age", slab),
				e.parseAndNewMetric(ch, e.itemsLruHits, prometheus.CounterValue, u, "hits_to_hot", slab, "hot"),
				e.parseAndNewMetric(ch, e.itemsLruHits, prometheus.CounterValue, u, "hits_to_warm", slab, "warm"),
				e.parseAndNewMetric(ch, e.itemsLruHits, prometheus.CounterValue, u, "hits_to_cold", slab, "cold"),
				e.parseAndNewMetric(ch, e.itemsLruHits, prometheus.CounterValue, u, "hits_to_temp", slab, "temporary"),
			)
			if err != nil {
				parseError = err
			}
			for m, d := range itemsCounterMetrics {
				if _, ok := u[m]; !ok {
					continue
				}
				if err := e.parseAndNewMetric(ch, d, prometheus.CounterValue, u, m, slab); err != nil {
					parseError = err
				}
			}
			for m, d := range itemsGaugeMetrics {
				if _, ok := u[m]; !ok {
					continue
				}
				if err := e.parseAndNewMetric(ch, d, prometheus.GaugeValue, u, m, slab); err != nil {
					parseError = err
				}
			}
		}

		for slab, v := range t.Slabs {
			slab := strconv.Itoa(slab)

			for _, op := range []string{"get", "delete", "incr", "decr", "cas", "touch"} {
				if err := e.parseAndNewMetric(ch, e.slabsCommands, prometheus.CounterValue, v, op+"_hits", slab, op, "hit"); err != nil {
					parseError = err
				}
			}
			if err := e.parseAndNewMetric(ch, e.slabsCommands, prometheus.CounterValue, v, "cas_badval", slab, "cas", "badval"); err != nil {
				parseError = err
			}

			slabSetCmd, err := parse(v, "cmd_set", e.logger)
			if err == nil {
				if slabCas, slabCasErr := sum(v, "cas_hits", "cas_badval"); slabCasErr == nil {
					ch <- prometheus.MustNewConstMetric(e.slabsCommands, prometheus.CounterValue, slabSetCmd-slabCas, slab, "set", "hit")
				} else {
					e.logger.Error("Failed to parse cas", "err", slabCasErr)
					parseError = slabCasErr
				}
			} else {
				e.logger.Error("Failed to parse set", "err", err)
				parseError = err
			}

			err = firstError(
				e.parseAndNewMetric(ch, e.slabsChunkSize, prometheus.GaugeValue, v, "chunk_size", slab),
				e.parseAndNewMetric(ch, e.slabsChunksPerPage, prometheus.GaugeValue, v, "chunks_per_page", slab),
				e.parseAndNewMetric(ch, e.slabsCurrentPages, prometheus.GaugeValue, v, "total_pages", slab),
				e.parseAndNewMetric(ch, e.slabsCurrentChunks, prometheus.GaugeValue, v, "total_chunks", slab),
				e.parseAndNewMetric(ch, e.slabsChunksUsed, prometheus.GaugeValue, v, "used_chunks", slab),
				e.parseAndNewMetric(ch, e.slabsChunksFree, prometheus.GaugeValue, v, "free_chunks", slab),
				e.parseAndNewMetric(ch, e.slabsChunksFreeEnd, prometheus.GaugeValue, v, "free_chunks_end", slab),
				e.parseAndNewMetric(ch, e.slabsMemRequested, prometheus.GaugeValue, v, "mem_requested", slab),
			)
			if err != nil {
				parseError = err
			}
		}
	}

	return parseError
}

func (e *Exporter) parseStatsSettings(ch chan<- prometheus.Metric, statsSettings map[net.Addr]map[string]string) error {
	var parseError error
	for _, settings := range statsSettings {
		if err := e.parseAndNewMetric(ch, e.maxConnections, prometheus.GaugeValue, settings, "maxconns"); err != nil {
			parseError = err
		}

		if v, ok := settings["lru_crawler"]; ok && v == "yes" {
			err := firstError(
				e.parseBoolAndNewMetric(ch, e.lruCrawlerEnabled, prometheus.GaugeValue, settings, "lru_crawler"),
				e.parseAndNewMetric(ch, e.lruCrawlerSleep, prometheus.GaugeValue, settings, "lru_crawler_sleep"),
				e.parseAndNewMetric(ch, e.lruCrawlerMaxItems, prometheus.GaugeValue, settings, "lru_crawler_tocrawl"),
				e.parseBoolAndNewMetric(ch, e.lruMaintainerThread, prometheus.GaugeValue, settings, "lru_maintainer_thread"),
				e.parseAndNewMetric(ch, e.lruHotPercent, prometheus.GaugeValue, settings, "hot_lru_pct"),
				e.parseAndNewMetric(ch, e.lruWarmPercent, prometheus.GaugeValue, settings, "warm_lru_pct"),
				e.parseAndNewMetric(ch, e.lruHotMaxAgeFactor, prometheus.GaugeValue, settings, "hot_max_factor"),
				e.parseAndNewMetric(ch, e.lruWarmMaxAgeFactor, prometheus.GaugeValue, settings, "warm_max_factor"),
			)
			if err != nil {
				parseError = err
			}
		}
	}
	return parseError
}

func (e *Exporter) parseAndNewMetric(ch chan<- prometheus.Metric, desc *prometheus.Desc, valueType prometheus.ValueType, stats map[string]string, key string, labelValues ...string) error {
	return e.extractValueAndNewMetric(ch, desc, valueType, parse, stats, key, labelValues...)
}

func (e *Exporter) parseBoolAndNewMetric(ch chan<- prometheus.Metric, desc *prometheus.Desc, valueType prometheus.ValueType, stats map[string]string, key string, labelValues ...string) error {
	return e.extractValueAndNewMetric(ch, desc, valueType, parseBool, stats, key, labelValues...)
}

func (e *Exporter) parseTimevalAndNewMetric(ch chan<- prometheus.Metric, desc *prometheus.Desc, valueType prometheus.ValueType, stats map[string]string, key string, labelValues ...string) error {
	return e.extractValueAndNewMetric(ch, desc, valueType, parseTimeval, stats, key, labelValues...)
}

func (e *Exporter) extractValueAndNewMetric(ch chan<- prometheus.Metric, desc *prometheus.Desc, valueType prometheus.ValueType, f func(map[string]string, string, *slog.Logger) (float64, error), stats map[string]string, key string, labelValues ...string) error {
	v, err := f(stats, key, e.logger)
	if err == errKeyNotFound {
		return nil
	}
	if err != nil {
		return err
	}

	ch <- prometheus.MustNewConstMetric(desc, valueType, v, labelValues...)
	return nil
}

func parse(stats map[string]string, key string, logger *slog.Logger) (float64, error) {
	value, ok := stats[key]
	if !ok {
		logger.Debug("Key not found", "key", key)
		return 0, errKeyNotFound
	}

	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		logger.Error("Failed to parse", "key", key, "value", value, "err", err)
		return 0, err
	}
	return v, nil
}

func parseBool(stats map[string]string, key string, logger *slog.Logger) (float64, error) {
	value, ok := stats[key]
	if !ok {
		logger.Debug("Key not found", "key", key)
		return 0, errKeyNotFound
	}

	switch value {
	case "yes":
		return 1, nil
	case "no":
		return 0, nil
	default:
		logger.Error("Failed to parse", "key", key, "value", value)
		return 0, errors.New("failed parse a bool value")
	}
}

func parseTimeval(stats map[string]string, key string, logger *slog.Logger) (float64, error) {
	value, ok := stats[key]
	if !ok {
		logger.Debug("Key not found", "key", key)
		return 0, errKeyNotFound
	}
	values := strings.Split(value, ".")

	if len(values) != 2 {
		logger.Error("Failed to parse", "key", key, "value", value)
		return 0, errors.New("failed parse a timeval value")
	}

	seconds, err := strconv.ParseFloat(values[0], 64)
	if err != nil {
		logger.Error("Failed to parse", "key", key, "value", value, "err", err)
		return 0, errors.New("failed parse a timeval value")
	}

	microseconds, err := strconv.ParseFloat(values[1], 64)
	if err != nil {
		logger.Error("Failed to parse", "key", key, "value", value, "err", err)
		return 0, errors.New("failed parse a timeval value")
	}

	return (seconds + microseconds/(1000.0*1000.0)), nil
}

func sum(stats map[string]string, keys ...string) (float64, error) {
	s := 0.
	for _, key := range keys {
		if _, ok := stats[key]; !ok {
			return 0, errKeyNotFound
		}
		v, err := strconv.ParseFloat(stats[key], 64)
		if err != nil {
			return 0, err
		}
		s += v
	}
	return s, nil
}

func firstError(errors ...error) error {
	for _, v := range errors {
		if v != nil {
			return v
		}
	}
	return nil
}
