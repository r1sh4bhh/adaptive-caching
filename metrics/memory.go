package metrics

import (
	"runtime"
	"unsafe"

	"github.com/r1sh4bhh/adaptive-caching/types"
)

// entryStructBytes is the fixed cost of one types.Entry header, excluding the
// bytes its Key string and Value slice point at.
var entryStructBytes = int64(unsafe.Sizeof(types.Entry{}))

// mapEntryOverheadBytes is the approximate per-key cost of Go's map machinery
// for a map[string]*Entry: the string key header plus the pointer value plus
// bucket bookkeeping (top-hash byte, and the slack from buckets being at most
// ~81% full before growth). This is an estimate; it is the only estimated term
// in the accounting and it is deliberately conservative (i.e. it over-reports
// our own overhead rather than flattering it).
const mapEntryOverheadBytes = int64(unsafe.Sizeof("") + unsafe.Sizeof((*types.Entry)(nil)) + 8)

// EntryStructBytes returns the size of the Entry header in bytes.
func EntryStructBytes() int64 { return entryStructBytes }

// RuntimeMemory is a snapshot of process-level memory from the Go runtime.
type RuntimeMemory struct {
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapSysBytes   uint64 `json:"heap_sys_bytes"`
	HeapObjects    uint64 `json:"heap_objects"`
	TotalAllocated uint64 `json:"total_allocated_bytes"`
	SysBytes       uint64 `json:"sys_bytes"`
	NumGC          uint32 `json:"num_gc"`
}

// ReadRuntimeMemory samples runtime.ReadMemStats. It stops the world briefly,
// so call it on the frame tick (10 Hz), never in the request path.
func ReadRuntimeMemory() RuntimeMemory {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return RuntimeMemory{
		HeapAllocBytes: ms.HeapAlloc,
		HeapSysBytes:   ms.HeapSys,
		HeapObjects:    ms.HeapObjects,
		TotalAllocated: ms.TotalAlloc,
		SysBytes:       ms.Sys,
		NumGC:          ms.NumGC,
	}
}

// MemoryBreakdown separates what the cache stores on the user's behalf from
// what it spends on itself.
//
// HOW OVERHEAD IS COMPUTED — this is an evaluation metric, so the arithmetic is
// stated explicitly rather than left implicit in code:
//
//	PayloadBytes  = sum over entries of len(Entry.Value)
//	                (the object bytes a user actually asked us to cache)
//	KeyBytes      = sum over entries of len(Entry.Key)
//	EntryBytes    = objectCount * unsafe.Sizeof(types.Entry{})
//	                (the fixed struct header: size, two timestamps, access
//	                 count, string/slice headers, PolicyMeta interface)
//	IndexBytes    = objectCount * mapEntryOverheadBytes
//	                (the store's map[string]*Entry machinery — estimated)
//	PolicyBytes   = EvictionPolicy.MetadataBytes()
//	                (the policy's own structures: lists, heaps, sketches)
//
//	MetadataBytes = KeyBytes + EntryBytes + IndexBytes + PolicyBytes
//	OverheadRatio = MetadataBytes / (PayloadBytes + MetadataBytes)
//
// Target: OverheadRatio < 0.05, acceptable < 0.10 — and it must be MEASURED,
// never estimated from a formula alone, which is why RuntimeMemory is reported
// alongside it as a cross-check.
type MemoryBreakdown struct {
	ObjectCount   int   `json:"object_count"`
	PayloadBytes  int64 `json:"payload_bytes"`
	KeyBytes      int64 `json:"key_bytes"`
	EntryBytes    int64 `json:"entry_bytes"`
	IndexBytes    int64 `json:"index_bytes"`
	PolicyBytes   int64 `json:"policy_bytes"`
	MetadataBytes int64 `json:"metadata_bytes"`
	// OverheadRatio is MetadataBytes / (PayloadBytes + MetadataBytes), in 0..1.
	OverheadRatio float64       `json:"overhead_ratio"`
	Runtime       RuntimeMemory `json:"runtime"`
}

// AccountMemory computes the breakdown described on MemoryBreakdown.
// payloadBytes and keyBytes are supplied by the store, which tracks them
// exactly; policyBytes comes from EvictionPolicy.MetadataBytes().
func AccountMemory(objectCount int, payloadBytes, keyBytes, policyBytes int64) MemoryBreakdown {
	b := MemoryBreakdown{
		ObjectCount:  objectCount,
		PayloadBytes: payloadBytes,
		KeyBytes:     keyBytes,
		EntryBytes:   int64(objectCount) * entryStructBytes,
		IndexBytes:   int64(objectCount) * mapEntryOverheadBytes,
		PolicyBytes:  policyBytes,
	}
	b.MetadataBytes = b.KeyBytes + b.EntryBytes + b.IndexBytes + b.PolicyBytes
	if total := b.PayloadBytes + b.MetadataBytes; total > 0 {
		b.OverheadRatio = float64(b.MetadataBytes) / float64(total)
	}
	b.Runtime = ReadRuntimeMemory()
	return b
}
