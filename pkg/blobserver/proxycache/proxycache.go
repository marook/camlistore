/*
Copyright 2014 The Camlistore Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Package proxycache registers the "proxycache" blobserver storage type,
which uses a provided blobserver as a cache for a second origin
blobserver.

The proxycache blobserver type also takes a sorted.KeyValue reference
which it uses as the LRU for which old items to evict from the cache.

Be aware that maxCacheBytes is only the upper limit for the blob's
content. The cache blob storage might actually grow bigger because of
some storage overhead.

Example config:

      "/cache/": {
          "handler": "storage-proxycache",
          "handlerArgs": {
		"origin": "/bs-remote-origin/",
		"cache": "/bs-local-cache/",
		"meta": {
		    "file": "/home/myUser/var/camlistore/proxycache.leveldb",
		    "type": "leveldb"
		},
		"maxCacheBytes": 25165824
          }
      },

In case you are never ever removing blobs from origin you can enable
basic temporary offline support from origin. You have to add the
property "I_AGREE" just like in the following config example:

      "/cache/": {
          "handler": "storage-proxycache",
          "handlerArgs": {
		"origin": "/bs-remote-origin/",
		"cache": "/bs-local-cache/",
                "I_AGREE": "that blobs are never removed from origin",
		"meta": {
		    "file": "/home/myUser/var/camlistore/proxycache.leveldb",
		    "type": "leveldb"
		},
		"maxCacheBytes": 25165824
          }
      },

*/
package proxycache // import "camlistore.org/pkg/blobserver/proxycache"

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/sorted"
	"go4.org/jsonconfig"
	"golang.org/x/net/context"
)

type BlobAccess struct {
	ref           blob.Ref
	blobSize      uint32
	access        int64 // unix timestamp of the last access
	contentCached bool
	heapIndex     int // index of the BlobAccess in the heap. used to speed up finding it
}

type BlobAccessHeap []*BlobAccess

func (h BlobAccessHeap) Len() int           { return len(h) }
func (h BlobAccessHeap) Less(i, j int) bool { return h[i].access < h[j].access }

func (h BlobAccessHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].heapIndex = i
	h[j].heapIndex = j
}

func (h *BlobAccessHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(*BlobAccess))
}

func (h *BlobAccessHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type sto struct {
	origin        blobserver.Storage
	cache         blobserver.Storage
	kv            sorted.KeyValue
	maxCacheBytes int64
	offlineSupport   bool

	mu             sync.Mutex // guards cacheBytes, kv and blobAccess* mutations
	cacheBytes     int64
	blobAccessHeap BlobAccessHeap // min heap of the blob's last access timestamps
	blobAccessMap  map[blob.Ref]*BlobAccess
}

func init() {
	blobserver.RegisterStorageConstructor("proxycache", blobserver.StorageConstructor(newFromConfig))
}

func newFromConfig(ld blobserver.Loader, config jsonconfig.Obj) (storage blobserver.Storage, err error) {
	var (
		origin        = config.RequiredString("origin")
		cache         = config.RequiredString("cache")
		kvConf        = config.RequiredObject("meta")
		maxCacheBytes = config.OptionalInt64("maxCacheBytes", 512<<20)
		agreement     = config.OptionalString("I_AGREE", "")
	)
	if err := config.Validate(); err != nil {
		return nil, err
	}
	cacheSto, err := ld.GetStorage(cache)
	if err != nil {
		return nil, err
	}
	originSto, err := ld.GetStorage(origin)
	if err != nil {
		return nil, err
	}
	kv, err := sorted.NewKeyValue(kvConf)
	if err != nil {
		return nil, err
	}
	kvEmpty := !kv.Find("", "").Next()
	if(kvEmpty){
		err := rebuildKvFromCache(kv, cacheSto)
		if err != nil {
			return nil, err
		}
	}
	blobAccessMap, err := buildBlobAccessMapFromKv(kv)
	if err != nil {
		log.Printf("Error in proxycache metadata => rebuilding it: %v", err)
		err := rebuildKvFromCache(kv, cacheSto)
		if err != nil {
			return nil, err
		}
		blobAccessMap, err = buildBlobAccessMapFromKv(kv)
		if err != nil {
			return nil, err
		}
	}
	cacheBytes := calcCacheBytesFromBlobAccessMap(&blobAccessMap)
	blobAccessHeap := builbBlobAccessHeapFromBlobAccessMap(&blobAccessMap)

        // TODO we might should ensure that no entry is newer than 'now'... otherwise the blobAccessHeap will break when appending new entries

	s := &sto{
		origin:         originSto,
		cache:          cacheSto,
		cacheBytes:     cacheBytes,
		maxCacheBytes:  maxCacheBytes,
		kv:             kv,
		blobAccessHeap: blobAccessHeap,
		blobAccessMap:  blobAccessMap,
		offlineSupport: agreement == "that blobs are never removed from origin",
	}
	s.EnforceCacheLimits()
	return s, nil
}

func buildBlobAccessMapFromKv(kv sorted.KeyValue) (blobAccessMap map[blob.Ref]*BlobAccess, err error) {
	blobAccessMap = make(map[blob.Ref]*BlobAccess)
	kvIt := kv.Find("", "")
	for kvIt.Next() {
		val := kvIt.Value()
		var blobSize uint32
		var entryAccessTimestamp int64
		var contentCachedStr string
		nTokens, err := fmt.Sscanf(val, "%x:%x:%s", &blobSize, &entryAccessTimestamp, &contentCachedStr)
		if err != nil {
			return nil, err
		}
		if nTokens != 3 {
			return nil, errors.New(fmt.Sprintf("Can't parse kv entry '%s'", val))
		}
		ref, ok := blob.Parse(kvIt.Key())
		if !ok {
			return nil, errors.New(fmt.Sprintf("Failed to parse blob ref '%s'", kvIt.Key()))
		}
		blobAccessMap[ref] = &BlobAccess{
			ref: ref,
			blobSize: blobSize,
			access: entryAccessTimestamp,
			contentCached: contentCachedStr == "1",
		}
	}
	return blobAccessMap, nil
}

func calcCacheBytesFromBlobAccessMap(blobAccessMap *map[blob.Ref]*BlobAccess) (cacheBytes int64) {
	cacheBytes = int64(0)
	for _, blobAccess := range *blobAccessMap {
		cacheBytes += int64(blobAccess.blobSize)
	}
	return cacheBytes
}

func builbBlobAccessHeapFromBlobAccessMap(blobAccessMap *map[blob.Ref]*BlobAccess) (blobAccessHeap BlobAccessHeap) {
	blobAccessHeap = make(BlobAccessHeap, len(*blobAccessMap))
	i := 0
	for _, blobAccess := range *blobAccessMap {
		blobAccess.heapIndex = i
		blobAccessHeap[i] = blobAccess
		i += 1
	}
	heap.Init(&blobAccessHeap)
	return blobAccessHeap
}

func rebuildKvFromCache(kv sorted.KeyValue, cache blobserver.Storage) (err error){
	log.Printf("Rebuilding proxycache metadata...")
	
	delBatch := kv.BeginBatch()
	kvIt := kv.Find("", "")
	for kvIt.Next() {
		delBatch.Delete(kvIt.Key())
	}
	kv.CommitBatch(delBatch)

	setBatch := kv.BeginBatch()
	ch := make(chan blob.SizedRef)
	errCh := make(chan error)
	go func() {
		errCh <- cache.EnumerateBlobs(context.TODO(), ch, "", -1)
	}()
	for {
		sr, ok := <-ch
		if !ok {
			break
		}
		val := fmt.Sprintf("%x:%x:1", sr.Size, 0)
		kv.Set(sr.Ref.String(), val)
	}
	err = <-errCh
	if err != nil {
		return err
	}
	kv.CommitBatch(setBatch)

	return nil
}

func (sto *sto) touchBlob(sb blob.SizedRef, contentCached bool) {
	sto.mu.Lock()
	defer sto.mu.Unlock()
	sto.touchBlobs([]blob.SizedRef{sb}, contentCached)
}

func (sto *sto) touchBlobs(blobs []blob.SizedRef, contentCached bool) {
	now := time.Now().Unix()
	kvBatch := sto.kv.BeginBatch()
	for _, sb := range blobs {
		blobAccess, exists := sto.blobAccessMap[sb.Ref]
		if exists {
			blobAccess.access = now
			blobAccess.contentCached = blobAccess.contentCached || contentCached
			heap.Fix(&sto.blobAccessHeap, blobAccess.heapIndex)
		} else {
			sto.cacheBytes += int64(sb.Size)
			
			blobAccess := BlobAccess{
				ref: sb.Ref,
				blobSize: sb.Size,
				access: now,
				contentCached: contentCached,
				heapIndex: len(sto.blobAccessHeap),
			}
			sto.blobAccessMap[sb.Ref] = &blobAccess
			sto.blobAccessHeap = append(sto.blobAccessHeap, &blobAccess)
		}

		var contentCachedStr string
		if contentCached {
			contentCachedStr = "1"
		} else {
			contentCachedStr = "0"
		}
		val := fmt.Sprintf("%x:%x:%s", sb.Size, now, contentCachedStr)
		kvBatch.Set(sb.Ref.String(), val)
	}
	sto.kv.CommitBatch(kvBatch)
	sto.EnforceCacheLimits()
}

func (sto *sto) EnforceCacheLimits() {
	if sto.cacheBytes > sto.maxCacheBytes {
		deletedRefs := []blob.Ref{}
		kvBatch := sto.kv.BeginBatch()
		for sto.cacheBytes > sto.maxCacheBytes {
			droppedBlobAccess := heap.Pop(&sto.blobAccessHeap).(*BlobAccess)
			delete(sto.blobAccessMap, droppedBlobAccess.ref)
			kvBatch.Delete(droppedBlobAccess.ref.String())
			if droppedBlobAccess.contentCached {
				sto.cacheBytes -= int64(droppedBlobAccess.blobSize)
				log.Printf(">>>>>> dropping %v (ac: %d, size: %d, mx: %d > %d)", droppedBlobAccess.ref, int(droppedBlobAccess.access), droppedBlobAccess.blobSize, int(sto.cacheBytes), int(sto.maxCacheBytes))
				deletedRefs = append(deletedRefs, droppedBlobAccess.ref)
			}
		}
		sto.cache.RemoveBlobs(deletedRefs)
		sto.kv.CommitBatch(kvBatch)
	}
}

func (sto *sto) Fetch(b blob.Ref) (rc io.ReadCloser, size uint32, err error) {
	rc, size, err = sto.cache.Fetch(b)
	if err == nil {
		sto.touchBlob(blob.SizedRef{Ref: b, Size: size}, true)
		log.Printf(">>>>>> cache fetch %v", b)
		return
	}
	if err != os.ErrNotExist {
		log.Printf("warning: proxycache cache fetch error for %v: %v", b, err)
	}
	rc, size, err = sto.origin.Fetch(b)
	if err != nil {
		return
	}
	all, err := ioutil.ReadAll(rc)
	if err != nil {
		return
	}
	go func() {
		if _, err := blobserver.Receive(sto.cache, b, bytes.NewReader(all)); err != nil {
			log.Printf("populating proxycache cache for %v: %v", b, err)
			return
		}
		sto.touchBlob(blob.SizedRef{Ref: b, Size: size}, true)
		log.Printf(">>>>>> origin fetch %v", b)
	}()
	return ioutil.NopCloser(bytes.NewReader(all)), size, nil
}

func (sto *sto) StatBlobs(dest chan<- blob.SizedRef, blobs []blob.Ref) error {
	if(sto.offlineSupport){
		sto.mu.Lock()
		defer sto.mu.Unlock()

		notCachedRefs := []blob.Ref{}
		for _, ref := range blobs {
			blobAccess, ok := sto.blobAccessMap[ref]
			if ok {
				dest <- blob.SizedRef{Ref: ref, Size: blobAccess.blobSize}
				log.Printf(">>>>>> cache stat %v", ref)
			} else {
				notCachedRefs = append(notCachedRefs, ref)
			}
		}

		originStats := []blob.SizedRef{}
		if(len(notCachedRefs) > 0){
			originStatsCh := make(chan blob.SizedRef)
			errCh := make(chan error)
			sto.mu.Unlock()
			go func() {
				errCh <- sto.origin.StatBlobs(originStatsCh, notCachedRefs)
			}()
			for {
				originRef, ok := <-originStatsCh
				if !ok {
					break
				}
				originStats = append(originStats, originRef)
				dest <- originRef
				log.Printf(">>>>>> origin stat %v", originRef.Ref)
			}
			sto.mu.Lock()
			err := <-errCh
			if err != nil {
				return err
			}
		}

		sto.touchBlobs(originStats, false)

		return nil
	} else {
		return sto.origin.StatBlobs(dest, blobs)
	}
}

func (sto *sto) ReceiveBlob(br blob.Ref, src io.Reader) (sb blob.SizedRef, err error) {
	// Slurp the whole blob before replicating. Bounded by 16 MB anyway.
	var buf bytes.Buffer
	if _, err = io.Copy(&buf, src); err != nil {
		return
	}

	if _, err = sto.cache.ReceiveBlob(br, bytes.NewReader(buf.Bytes())); err != nil {
		return
	}
	sto.touchBlob(sb, true)
	return sto.origin.ReceiveBlob(br, bytes.NewReader(buf.Bytes()))
}

func (sto *sto) RemoveBlobs(blobs []blob.Ref) error {
	sto.RemoveFromCacheMetadata(blobs)
	// Ignore result of cache removal
	go sto.cache.RemoveBlobs(blobs)
	return sto.origin.RemoveBlobs(blobs)
}

func (sto *sto) RemoveFromCacheMetadata(blobs []blob.Ref) {
	sto.mu.Lock()
	defer sto.mu.Unlock()
	kvBatch := sto.kv.BeginBatch()
	for _, ref := range blobs {
		kvBatch.Delete(ref.String())
		
		blobAccess, ok := sto.blobAccessMap[ref]
		if ok {
			delete(sto.blobAccessMap, blobAccess.ref)
			heap.Remove(&sto.blobAccessHeap, blobAccess.heapIndex)
		}
	}
	sto.kv.CommitBatch(kvBatch)
}

func (sto *sto) EnumerateBlobs(ctx context.Context, dest chan<- blob.SizedRef, after string, limit int) error {
	return sto.origin.EnumerateBlobs(ctx, dest, after, limit)
}
