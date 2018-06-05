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

maxCacheBytes is only the upper limit for the blob's content. The cache
blob storage might actually grow bigger on disk because of storage
overhead.

Example config:

      "/bs/": {
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

*/
package proxycache // import "perkeep.org/pkg/blobserver/proxycache"

import (
	"bytes"
	"container/heap"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"perkeep.org/pkg/blob"
	"perkeep.org/pkg/blobserver"
	"perkeep.org/pkg/sorted"
	"go4.org/jsonconfig"
)

type BlobAccess struct {
	Ref           blob.Ref
	BlobSize      uint32
	Access        int64 // unix timestamp of the last access
	ContentCached bool
	HeapIndex     int // index of the BlobAccess in the heap. used to speed up finding it
}

type BlobAccessHeap []*BlobAccess

func (h BlobAccessHeap) Len() int           { return len(h) }
func (h BlobAccessHeap) Less(i, j int) bool { return h[i].Access < h[j].Access }

func (h BlobAccessHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].HeapIndex = i
	h[j].HeapIndex = j
}

func (h *BlobAccessHeap) Push(x interface{}) {
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
	origin                  blobserver.Storage
	cache                   blobserver.Storage
	kv                      sorted.KeyValue
	maxCacheBytes           int64

	mu             sync.Mutex // guards cacheBytes, kv and blobAccess* mutations
	cacheBytes     int64
	blobAccessHeap BlobAccessHeap // min heap of the blob's last access timestamps
	blobAccessMap  map[blob.Ref]*BlobAccess
}

var (
	_ blobserver.Storage = (*sto)(nil)
)

func init() {
	blobserver.RegisterStorageConstructor("proxycache", blobserver.StorageConstructor(newFromConfig))
}

func newFromConfig(ld blobserver.Loader, config jsonconfig.Obj) (storage blobserver.Storage, err error) {
	var (
		origin                  = config.RequiredString("origin")
		cache                   = config.RequiredString("cache")
		kvConf                  = config.RequiredObject("meta")
		maxCacheBytes           = config.OptionalInt64("maxCacheBytes", 512<<20)
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
	kv, err := sorted.NewKeyValueMaybeWipe(kvConf)
	if err != nil {
		return nil, err
	}
	kvEmpty := !kv.Find("", "").Next()
	if kvEmpty {
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
	log.Printf("proxycache uses %v of %v bytes", cacheBytes, maxCacheBytes)

	s := &sto{
		origin:                  originSto,
		cache:                   cacheSto,
		cacheBytes:              cacheBytes,
		maxCacheBytes:           maxCacheBytes,
		kv:                      kv,
		blobAccessHeap:          blobAccessHeap,
		blobAccessMap:           blobAccessMap,
	}
	s.EnforceCacheLimits(context.TODO())
	return s, nil
}

func New(maxCacheBytes int64, cache blobserver.Storage, origin blobserver.Storage) (storage blobserver.Storage){
	/*
	 * at the time of writing this function was required by the s3
	 * blobstore.
	 */
	panic("proxycache without kv storage is not supported")
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
			Ref:           ref,
			BlobSize:      blobSize,
			Access:        entryAccessTimestamp,
			ContentCached: contentCachedStr == "1",
		}
	}
	return blobAccessMap, nil
}

func calcCacheBytesFromBlobAccessMap(blobAccessMap *map[blob.Ref]*BlobAccess) (cacheBytes int64) {
	cacheBytes = int64(0)
	for _, blobAccess := range *blobAccessMap {
		cacheBytes += int64(blobAccess.BlobSize)
	}
	return cacheBytes
}

func builbBlobAccessHeapFromBlobAccessMap(blobAccessMap *map[blob.Ref]*BlobAccess) (blobAccessHeap BlobAccessHeap) {
	blobAccessHeap = make(BlobAccessHeap, len(*blobAccessMap))
	i := 0
	for _, blobAccess := range *blobAccessMap {
		blobAccess.HeapIndex = i
		blobAccessHeap[i] = blobAccess
		i += 1
	}
	heap.Init(&blobAccessHeap)
	return blobAccessHeap
}

func rebuildKvFromCache(kv sorted.KeyValue, cache blobserver.Storage) (err error) {
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

func (sto *sto) touchBlob(ctx context.Context, sb blob.SizedRef, contentCached bool) {
	sto.mu.Lock()
	defer sto.mu.Unlock()
	sto.touchBlobs(ctx, []blob.SizedRef{sb}, contentCached)
}

func (sto *sto) touchBlobs(ctx context.Context, blobs []blob.SizedRef, contentCached bool) {
	now := time.Now().Unix()
	kvBatch := sto.kv.BeginBatch()
	for _, sb := range blobs {
		blobAccess, exists := sto.blobAccessMap[sb.Ref]
		if exists {
			blobAccess.Access = now
			if !blobAccess.ContentCached && contentCached {
				blobAccess.ContentCached = true
				sto.cacheBytes += int64(sb.Size)
			}
			heap.Fix(&sto.blobAccessHeap, blobAccess.HeapIndex)
		} else {
			sto.cacheBytes += int64(sb.Size)

			blobAccess := BlobAccess{
				Ref:           sb.Ref,
				BlobSize:      sb.Size,
				Access:        now,
				ContentCached: contentCached,
				HeapIndex:     len(sto.blobAccessHeap),
			}
			sto.blobAccessMap[sb.Ref] = &blobAccess
			// TODO we might should ensure that no entry is
			// newer than 'now'... otherwise the
			// blobAccessHeap will break when appending new
			// entries. should be checked during heap
			// initialization.
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
	sto.EnforceCacheLimits(ctx)
}

func (sto *sto) EnforceCacheLimits(ctx context.Context) {
	if sto.cacheBytes > sto.maxCacheBytes {
		deletedRefs := []blob.Ref{}
		kvBatch := sto.kv.BeginBatch()
		for sto.cacheBytes > sto.maxCacheBytes {
			droppedBlobAccess := heap.Pop(&sto.blobAccessHeap).(*BlobAccess)
			delete(sto.blobAccessMap, droppedBlobAccess.Ref)
			kvBatch.Delete(droppedBlobAccess.Ref.String())
			if droppedBlobAccess.ContentCached {
				sto.cacheBytes -= int64(droppedBlobAccess.BlobSize)
				deletedRefs = append(deletedRefs, droppedBlobAccess.Ref)
			}
		}
		sto.cache.RemoveBlobs(ctx, deletedRefs)
		sto.kv.CommitBatch(kvBatch)
	}
}

func (sto *sto) Fetch(ctx context.Context, b blob.Ref) (rc io.ReadCloser, size uint32, err error) {
	rc, size, err = sto.cache.Fetch(ctx, b)
	if err == nil {
		sto.touchBlob(ctx, blob.SizedRef{Ref: b, Size: size}, true)
		return
	}
	if err != os.ErrNotExist {
		log.Printf("warning: proxycache cache fetch error for %v (type %T): %v", b, err, err)
	}
	rc, size, err = sto.origin.Fetch(ctx, b)
	if err != nil {
		return
	}
	all, err := ioutil.ReadAll(rc)
	if err != nil {
		return
	}
	go func() {
		if _, err := blobserver.Receive(ctx, sto.cache, b, bytes.NewReader(all)); err != nil {
			log.Printf("populating proxycache cache failed for %v: %v", b, err)
			return
		}
		sto.touchBlob(ctx, blob.SizedRef{Ref: b, Size: size}, true)
	}()
	return ioutil.NopCloser(bytes.NewReader(all)), size, nil
}

func (sto *sto) StatBlobs(ctx context.Context, blobs []blob.Ref, fn func(blob.SizedRef) error) error {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	notCachedRefs := []blob.Ref{}
	for _, ref := range blobs {
		blobAccess, ok := sto.blobAccessMap[ref]
		if ok {
			sb := blob.SizedRef{Ref: ref, Size: blobAccess.BlobSize}
			if err := fn(sb); err != nil {
				return err
			}
		} else {
			notCachedRefs = append(notCachedRefs, ref)
		}
	}

	originStats := []blob.SizedRef{}
	if len(notCachedRefs) > 0 {
		sto.mu.Unlock()
		fnproxy := func (sr blob.SizedRef) error {
			originStats = append(originStats, sr)
			return fn(sr)
		}

		if err := sto.origin.StatBlobs(ctx, notCachedRefs, fnproxy); err != nil {
			sto.mu.Lock()
			return err
		}
		sto.mu.Lock()
	}

	sto.touchBlobs(ctx, originStats, false)

	return nil
}

func (sto *sto) ReceiveBlob(ctx context.Context, br blob.Ref, src io.Reader) (sb blob.SizedRef, err error) {
	// Slurp the whole blob before replicating. Bounded by 16 MB anyway.
	var buf bytes.Buffer
	if _, err = io.Copy(&buf, src); err != nil {
		return
	}
	if sb, err = sto.cache.ReceiveBlob(ctx, br, bytes.NewReader(buf.Bytes())); err != nil {
		return
	}
	sto.touchBlob(ctx, sb, true)
	return sto.origin.ReceiveBlob(ctx, br, bytes.NewReader(buf.Bytes()))
}

func (sto *sto) RemoveBlobs(ctx context.Context, blobs []blob.Ref) error {
	panic("proxycache does not support removing blobs")
}

func (sto *sto) RemoveFromCacheMetadata(blobs []blob.Ref) {
	sto.mu.Lock()
	defer sto.mu.Unlock()
	kvBatch := sto.kv.BeginBatch()
	for _, ref := range blobs {
		kvBatch.Delete(ref.String())

		blobAccess, ok := sto.blobAccessMap[ref]
		if ok {
			delete(sto.blobAccessMap, blobAccess.Ref)
			heap.Remove(&sto.blobAccessHeap, blobAccess.HeapIndex)
			if blobAccess.ContentCached {
				sto.cacheBytes -= int64(blobAccess.BlobSize)
			}
		}
	}
	sto.kv.CommitBatch(kvBatch)
}

func (sto *sto) EnumerateBlobs(ctx context.Context, dest chan<- blob.SizedRef, after string, limit int) error {
	return sto.origin.EnumerateBlobs(ctx, dest, after, limit)
}
