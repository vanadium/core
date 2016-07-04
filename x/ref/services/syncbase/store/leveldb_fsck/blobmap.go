// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package leveldb_fsck

import "bufio"
import "encoding/binary"
import "fmt"
import "os"
import "path/filepath"

import "v.io/v23/vom"
import "v.io/x/ref/services/syncbase/localblobstore"
import "v.io/x/ref/services/syncbase/server/interfaces"
import "v.io/x/ref/services/syncbase/store"

// A chunkToLocationKey represents the parsed key in the chunk-to-location map.
type chunkToLocationKey struct {
	prefix []byte
	hash   []byte
	blobId []byte
}

// A chunkToLocationVal represents the parsed value in the chunk-to-location map.
type chunkToLocationVal struct {
	offset int64
	length int64
}

// parseChunkToLocationKey() attempts to parse "data" into chunkToLocationKey *o,
// and returns whether successful.
func parseChunkToLocationKey(data []byte, o *chunkToLocationKey) (ok bool) {
	ok = (len(data) >= 33) && (data[0] == 0x00)
	if ok {
		o.prefix = data[:1]
		o.hash = data[1:17]
		o.blobId = data[17:33]
	}
	return ok
}

// parseChunkToLocationVal() attempts to parse "data" into chunkToLocationVal *o,
// and returns whether successful.
func parseChunkToLocationVal(data []byte, o *chunkToLocationVal) bool {
	var n int
	o.offset, n = binary.Varint(data)
	if n > 0 {
		o.length, n = binary.Varint(data[n:])
	}
	return n > 0
}

// ---

// A blobToChunkKey represents the parsed key in the blob-to-chunk map.
type blobToChunkKey struct {
	prefix []byte
	blobId []byte
	offset int64
}

// A blobToChunkVal represents the parsed value in the blob-to-chunk map.
type blobToChunkVal struct {
	hash   []byte
	length int64
}

// parseBlobToChunkKey() attempts to parse "data" into blobToChunkKey *o,
// and returns whether successful.
func parseBlobToChunkKey(data []byte, o *blobToChunkKey) (ok bool) {
	ok = (len(data) >= 25) && (data[0] == 0x01)
	if ok {
		o.prefix = data[:1]
		o.blobId = data[1:17]
		o.offset = int64(binary.BigEndian.Uint64(data[17:]))
	}
	return ok
}

// parseBlobToChunkVal() attempts to parse "data" into blobToChunkVal *o,
// and returns whether successful.
func parseBlobToChunkVal(data []byte, o *blobToChunkVal) bool {
	var n int
	if len(data) >= 17 {
		o.hash = data[:16]
		o.length, n = binary.Varint(data[16:])
	}
	return n > 0
}

// ---

// A blobToSignpostKey represents the parsed key in the blob-to-signpost map.
type blobToSignpostKey struct {
	prefix []byte
	blobId []byte
}

// parseBlobToSignpostKey() attempts to parse "data" into blobToSignpostKey *o,
// and returns whether successful.
func parseBlobToSignpostKey(data []byte, o *blobToSignpostKey) (ok bool) {
	ok = (len(data) >= 17) && (data[0] == 0x02)
	if ok {
		o.prefix = data[:1]
		o.blobId = data[1:17]
	}
	return ok
}

// ---

// A blobToMetadataKey represents the parsed key in the blob-to-metadata map.
type blobToMetadataKey struct {
	prefix []byte
	blobId []byte
}

// parseBlobToMetadataKey() attempts to parse "data" into blobToMetadataKey *o,
// and returns whether successful.
func parseBlobToMetadataKey(data []byte, o *blobToMetadataKey) (ok bool) {
	ok = (len(data) >= 17) && (data[0] == 0x03)
	if ok {
		o.prefix = data[:1]
		o.blobId = data[1:17]
	}
	return ok
}

// ---

// A perSyncGroupKey represents the parsed key in the per-syncgroup map.
type perSyncGroupKey struct {
	prefix  []byte
	groupId []byte
}

// parsePerSyncgroupKey() attempts to parse "data" into perSyncGroupKey *o,
// and returns whether successful.
func parsePerSyncgroupKey(data []byte, o *perSyncGroupKey) (ok bool) {
	ok = (len(data) >= 9) && (data[0] == 0x04)
	if ok {
		o.prefix = data[:1]
		o.groupId = data[1:9]
	}
	return ok
}

// ---

// parseBlobEntry() attempts to parse the blobmap database entry (keyBytes,
// valBytes).  If both the key and value can be parsed, each is placed in
// "result" using the Sprintf format strings keyFmt and valFmt respectively,
// and ok is set to true.  If the key can be parsed, but not the value, the
// parsing of the key is placed in "result" using the Sprintf format string
// keyFmt, and ok is set to false.  If the key cannot be parsed, "result" is
// left empty, and ok is set to false.
func parseBlobEntry(keyBytes []byte, valBytes []byte, keyFmt string, valFmt string) (result string, ok bool) {
	var c2lk chunkToLocationKey
	var b2ck blobToChunkKey
	var b2sk blobToSignpostKey
	var b2mk blobToMetadataKey
	var psgk perSyncGroupKey
	if parseChunkToLocationKey(keyBytes, &c2lk) {
		result = fmt.Sprintf(keyFmt, c2lk)
		var c2lv chunkToLocationVal
		ok = parseChunkToLocationVal(valBytes, &c2lv)
		if ok {
			result += fmt.Sprintf(valFmt, c2lv)
		}
	} else if parseBlobToChunkKey(keyBytes, &b2ck) {
		result = fmt.Sprintf(keyFmt, b2ck)
		var b2cv blobToChunkVal
		ok = parseBlobToChunkVal(valBytes, &b2cv)
		if ok {
			result += fmt.Sprintf(valFmt, b2cv)
		}
	} else if parseBlobToSignpostKey(keyBytes, &b2sk) {
		result = fmt.Sprintf(keyFmt, b2sk)
		var b2sv interfaces.Signpost
		ok = (vom.Decode(valBytes, &b2sv) == nil)
		if ok {
			result += fmt.Sprintf(valFmt, b2sv)
		}
	} else if parseBlobToMetadataKey(keyBytes, &b2mk) {
		result = fmt.Sprintf(keyFmt, b2mk)
		var b2mv localblobstore.BlobMetadata
		ok = (vom.Decode(valBytes, &b2mv) == nil)
		if ok {
			result += fmt.Sprintf(valFmt, b2mv)
		}
	} else if parsePerSyncgroupKey(keyBytes, &psgk) {
		result = fmt.Sprintf(keyFmt, psgk)
		var psgv localblobstore.PerSyncgroup
		ok = (vom.Decode(valBytes, &psgv) == nil)
		if ok {
			result += fmt.Sprintf(valFmt, psgv)
		}
	}
	return result, ok
}

// hashToFileName() returns the name of the binary ID with the specified
// prefix.  Requires len(id)==16.  An md5 hash is suitable.
func hashToFileName(prefix string, hash []byte) string {
	return filepath.Join(prefix,
		fmt.Sprintf("%02x", hash[0]),
		fmt.Sprintf("%02x", hash[1]),
		fmt.Sprintf("%02x", hash[2]),
		fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x",
			hash[3],
			hash[4], hash[5], hash[6], hash[7],
			hash[8], hash[9], hash[10], hash[11],
			hash[12], hash[13], hash[14], hash[15]))
}

// A blobToChunkEntry is a pair of a blobToChunkKey and a blobToChunkVal.
type blobToChunkEntry struct {
	key blobToChunkKey
	val blobToChunkVal
}

// A blobToFragmentEntry describes a part of a blob stored in a fragment.
type blobToFragmentEntry struct {
	size     int64
	offset   int64
	fileName string
}

// A blobData respresents all the information found about a blob.
type blobData struct {
	name           string
	isInFileSystem bool
	isFinalized    bool
	hash           string
	b2Fragment     []blobToFragmentEntry
	b2Chunk        []blobToChunkEntry
}

// blobMapEntry() returns a pointer to the blobData in blobMap with the
// specified name, creating the entry if needed.
func blobMapEntry(name string, blobMap map[string]*blobData) (entry *blobData) {
	var ok bool
	entry, ok = blobMap[name]
	if !ok {
		entry = &blobData{name: name}
		blobMap[name] = entry
	}
	return entry
}

// visitFile() is called on each node visited when walking the blob directory tree.
// It adds each blob seen to blobMap.
func visitFile(dbCtx *dbContext, blobMap map[string]*blobData, root string, path string, info os.FileInfo, err error) error {
	appendError(dbCtx, err)
	if err == nil && !info.IsDir() {
		var relPath string
		relPath, err = filepath.Rel(root, path)
		if err != nil {
			panic("path is not under root in filesystem tree walk")
		}
		var entry *blobData = blobMapEntry(relPath, blobMap)
		entry.isInFileSystem = true
		var f *os.File
		f, err = os.Open(path)
		if err == nil {
			var scanner *bufio.Scanner = bufio.NewScanner(f)
			var lineNum int
			for scanner.Scan() {
				var line string = scanner.Text()
				var n int
				var header string
				lineNum++
				if len(line) > 0 && line[0] == 'd' {
					var fragment blobToFragmentEntry
					n, err = fmt.Sscan(line, &header, &fragment.size, &fragment.offset, &fragment.fileName)
					if err == nil && n == 4 {
						entry.b2Fragment = append(entry.b2Fragment, fragment)
					} else if err == nil {
						err = fmt.Errorf("blobDB: short fragment entry on line %d in blob description file %s", lineNum, path)
					}
				} else if len(line) > 0 && line[0] == 'f' {
					n, err = fmt.Sscan(line, &header, &entry.hash)
					if err == nil && n == 2 {
						entry.isFinalized = true
					} else if err == nil {
						err = fmt.Errorf("blobDB: short finalization entry on line %d in blob description file %s", lineNum, path)
					}
				} else {
					err = fmt.Errorf("blobDB: can't parse line %d in blob description file %s", lineNum, path)
				}
				appendError(dbCtx, err)
			}
			f.Close()
		} else {
			appendError(dbCtx, err)
		}
	}
	return nil
}

// CheckDBBlobs() attempts to check the blobmap store of the syncbase database *dbCtx.
func CheckDBBlobs(dbCtx *dbContext) {
	keyBuf := make([]byte, 1024)
	valBuf := make([]byte, 1024)

	if dbCtx.blobDB != nil {
		fmt.Printf("******************* blobDB\n")
		cantDecode := 0

		var stream store.Stream = dbCtx.blobDB.Scan(startKey, limit(startKey))
		for stream.Advance() {
			keyBytes := stream.Key(keyBuf)
			valBytes := stream.Value(valBuf)

			var output string
			var ok bool
			output, ok = parseBlobEntry(keyBytes, valBytes, "key:   %#v", "\n\tvalue: %#v\n")
			if ok {
				fmt.Printf("%s", output)
			} else {
				if cantDecode == 0 {
					var parsedAs string
					if len(output) != 0 {
						parsedAs = fmt.Sprintf(" (parsed as %s) ", output)
					}
					appendError(dbCtx, fmt.Errorf("blobDB: can't parse entry:   key=%q (len=%d) %s value=%v",
						string(keyBytes), len(keyBytes), parsedAs, valBytes))
				}
				cantDecode++
			}
		}
		if cantDecode > 1 {
			appendError(dbCtx, fmt.Errorf("blobDB: can't parse %d entries in total", cantDecode))
		}

		appendError(dbCtx, stream.Err())

		blobMap := make(map[string]*blobData)

		blobTreePath := filepath.Join(dbCtx.rootPath, "blobs", "blob")
		appendError(dbCtx, filepath.Walk(blobTreePath,
			func(path string, info os.FileInfo, err error) error {
				return visitFile(dbCtx, blobMap, blobTreePath, path, info, err)
			}))

		start := []byte{0x01}
		stream = dbCtx.blobDB.Scan(start, limit(start))
		for stream.Advance() {
			keyBytes := stream.Key(keyBuf)
			valBytes := stream.Value(valBuf)
			var b2Chunk blobToChunkEntry
			if parseBlobToChunkKey(keyBytes, &b2Chunk.key) && parseBlobToChunkVal(valBytes, &b2Chunk.val) {
				name := hashToFileName("", b2Chunk.key.blobId)
				var entry *blobData = blobMapEntry(name, blobMap)
				entry.b2Chunk = append(entry.b2Chunk, b2Chunk)
			}
		}
		appendError(dbCtx, stream.Err())

		for blobName, entry := range blobMap {
			fmt.Printf("blob %q\n", blobName)
			fmt.Printf("\tisInFileSystem %v\n", entry.isInFileSystem)
			fmt.Printf("\tisInFileSystem %v\n", entry.isFinalized)
			fmt.Printf("\tchunks %d\n", len(entry.b2Chunk))
			for i := 0; i != len(entry.b2Chunk); i++ {
				fmt.Printf("\t\t%v\n", entry.b2Chunk[i])
			}
			fmt.Printf("\n")
		}

		fmt.Printf("******************* END blobDB\n\n\n")
	}
}
