/*
Copyright 2014 The Camlistore Authors.

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

// Package serverconfig provides types related to the server configuration file.
package serverconfig // import "camlistore.org/pkg/types/serverconfig"

import (
	"encoding/json"
)

// Config holds the values from the JSON (high-level) server config
// file that is exposed to users (and is by default at
// osutil.UserServerConfigPath). From this simpler configuration, a
// complete, low-level one, is generated by
// serverinit.genLowLevelConfig, and used to configure the various
// Camlistore components.
type Config struct {
	Auth               string `json:"auth"`               // auth scheme and values (ex: userpass:foo:bar).
	BaseURL            string `json:"baseURL,omitempty"`  // Base URL the server advertizes. For when behind a proxy.
	Listen             string `json:"listen"`             // address (of the form host|ip:port) on which the server will listen on.
	Identity           string `json:"identity"`           // GPG identity.
	IdentitySecretRing string `json:"identitySecretRing"` // path to the secret ring file.
	// alternative source tree, to override the embedded ui and/or closure resources.
	// If non empty, the ui files will be expected at
	// sourceRoot + "/server/camlistored/ui" and the closure library at
	// sourceRoot + "/vendor/embed/closure/lib"
	// Also used by the publish handler.
	SourceRoot string `json:"sourceRoot,omitempty"`
	OwnerName  string `json:"ownerName,omitempty"`

	// Blob storage.
	MemoryStorage      bool   `json:"memoryStorage,omitempty"`      // do not store anything (blobs or queues) on localdisk, use memory instead.
	BlobPath           string `json:"blobPath,omitempty"`           // path to the directory containing the blobs.
	PackBlobs          bool   `json:"packBlobs,omitempty"`          // use "diskpacked" instead of the default filestorage. (exclusive with PackRelated)
	PackRelated        bool   `json:"packRelated,omitempty"`        // use "blobpacked" instead of the default storage (exclusive with PackBlobs)
	S3                 string `json:"s3,omitempty"`                 // Amazon S3 credentials: access_key_id:secret_access_key:bucket[/optional/dir][:hostname].
	GoogleCloudStorage string `json:"googlecloudstorage,omitempty"` // Google Cloud credentials: clientId:clientSecret:refreshToken:bucket[/optional/dir] or ":bucket[/optional/dir/]" for auto on GCE
	GoogleDrive        string `json:"googledrive,omitempty"`        // Google Drive credentials: clientId:clientSecret:refreshToken:parentId.
	ShareHandler       bool   `json:"shareHandler,omitempty"`       // enable the share handler. If true, and shareHandlerPath is empty then shareHandlerPath will default to "/share/" when generating the low-level config.
	ShareHandlerPath   string `json:"shareHandlerPath,omitempty"`   // URL prefix for the share handler. If set, overrides shareHandler.

	// HTTPS.
	HTTPS     bool   `json:"https,omitempty"`     // enable HTTPS.
	HTTPSCert string `json:"httpsCert,omitempty"` // path to the HTTPS certificate file.
	HTTPSKey  string `json:"httpsKey,omitempty"`  // path to the HTTPS key file.

	// Index.
	RunIndex          invertedBool `json:"runIndex,omitempty"`          // if logically false: no search, no UI, etc.
	CopyIndexToMemory invertedBool `json:"copyIndexToMemory,omitempty"` // copy disk-based index to memory on start-up.
	MemoryIndex       bool         `json:"memoryIndex,omitempty"`       // use memory-only indexer.
	DBName            string       `json:"dbname,omitempty"`            // name of the database for mysql, postgres, mongo.
	LevelDB           string       `json:"levelDB,omitempty"`           // path to the levelDB directory, for indexing with github.com/syndtr/goleveldb.
	KVFile            string       `json:"kvIndexFile,omitempty"`       // path to the kv file, for indexing with github.com/cznic/kv.
	MySQL             string       `json:"mysql,omitempty"`             // MySQL credentials (username@host:password), for indexing with MySQL.
	Mongo             string       `json:"mongo,omitempty"`             // MongoDB credentials ([username:password@]host), for indexing with MongoDB.
	PostgreSQL        string       `json:"postgres,omitempty"`          // PostgreSQL credentials (username@host:password), for indexing with PostgreSQL.
	SQLite            string       `json:"sqlite,omitempty"`            // path to the SQLite file, for indexing with SQLite.

	// DBNames lists which database names to use for various types of key/value stores. The keys may be:
	//    "index"               (overrides 'dbname' key above)
	//    "queue-sync-to-index" (the sync queue to index things)
	//    "queue-sync-to-s3"    (the sync queue to replicate to s3)
	//    "blobpacked_index"    (the index for blobpacked, the 'packRelated' option. Defaults to "blobpacked_index".)
	//    "ui_thumbcache"
	DBNames map[string]string `json:"dbNames"`

	ReplicateTo []interface{} `json:"replicateTo,omitempty"` // NOOP for now.
	// Publish maps a URL prefix path used as a root for published paths (a.k.a. a camliRoot path), to the configuration of the publish handler that serves all the published paths under this root.
	Publish map[string]*Publish `json:"publish,omitempty"`

	// TODO(mpl): map of importers instead?
	Flickr string `json:"flickr,omitempty"` // flicker importer.
	Picasa string `json:"picasa,omitempty"` // picasa importer.
}

// App holds the common configuration values for apps and the app handler.
// See https://camlistore.org/doc/app-environment
type App struct {
	// Listen is the address (of the form host|ip:port) on which the app
	// will listen on. It defines CAMLI_APP_LISTEN.
	// If empty, the default is the concatenation of the Camlistore server's
	// Listen host part, and a random port.
	Listen string `json:"listen,omitempty"`

	// BackendURL is the URL of the application's process, always ending in a
	// trailing slash. It is the URL that the app handler will proxy to when
	// getting requests for the concerned app.
	// If empty, the default is the concatenation of the Camlistore server's BaseURL
	// scheme, the Camlistore server's BaseURL host part, and the port of Listen.
	BackendURL string `json:"backendURL,omitempty"`

	// APIHost is URL prefix of the Camlistore server which the app should
	// use to make API calls. It defines CAMLI_API_HOST.
	// If empty, the default is the Camlistore server's BaseURL, with a
	// trailing slash appended.
	APIHost string `json:"apiHost,omitempty"`

	HTTPSCert string `json:"httpsCert,omitempty"` // path to the HTTPS certificate file.
	HTTPSKey  string `json:"httpsKey,omitempty"`  // path to the HTTPS key file.
}

// Publish holds the server configuration values specific to a publisher, i.e. to a publish prefix.
type Publish struct {
	// Program is the server app program to run as the publisher.
	// Defaults to "publisher".
	Program string `json:"program"`

	*App // Common apps and app handler configuration.

	// CamliRoot value that defines our root permanode for this
	// publisher. The root permanode is used as the root for all the
	// paths served by this publisher.
	CamliRoot string `json:"camliRoot"`

	// GoTemplate is the name of the Go template file used by this
	// publisher to represent the data. This file should live in
	// app/publisher/.
	GoTemplate string `json:"goTemplate"`

	// CacheRoot is the path that will be used as the root for the
	// caching blobserver (for images). No caching if empty.
	// An example value is Config.BlobPath + "/cache".
	CacheRoot string `json:"cacheRoot,omitempty"`
}

// invertedBool is a bool that marshals to and from JSON with the opposite of its in-memory value.
type invertedBool bool

func (ib invertedBool) MarshalJSON() ([]byte, error) {
	return json.Marshal(!bool(ib))
}

func (ib *invertedBool) UnmarshalJSON(b []byte) error {
	var bo bool
	if err := json.Unmarshal(b, &bo); err != nil {
		return err
	}
	*ib = invertedBool(!bo)
	return nil
}

// Get returns the logical value of ib.
func (ib invertedBool) Get() bool {
	return !bool(ib)
}
