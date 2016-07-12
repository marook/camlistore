/*
Copyright 2016 Markus Peroebner

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

package server

import (
	"log"
	"net/http"
	"strings"

	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/client"
	"camlistore.org/pkg/httputil"
	"camlistore.org/pkg/search"

	"go4.org/jsonconfig"
)

type SearchRootsHandler struct {
	client  *client.Client
}

func init() {
	blobserver.RegisterHandlerConstructor("search-roots",  searchRootsFromConfig)
}

func searchRootsFromConfig(ld blobserver.Loader, conf jsonconfig.Obj) (h http.Handler, err error) {
	searchRoots := &SearchRootsHandler{
		client: client.NewOrFail(), // automatic from flags
	}

	if err = conf.Validate(); err != nil {
		return
	}

	return searchRoots, nil	
}

func (searchRoots *SearchRootsHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	suffix := httputil.PathSuffix(req)

	switch {
	default:
		http.Error(rw, "Illegal URL.", http.StatusNotFound)
		return
	case strings.HasPrefix(suffix, "root/"):
		searchRoots.serveRoot(rw, req)
	}
}

func (searchRoots *SearchRootsHandler) serveRoot(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(rw, "Invalid method", http.StatusBadRequest)
		return
	}
	
	suffix := httputil.PathSuffix(req)
	log.Printf("suffix: %s", suffix)
	// TODO parse path segments (root name, dir names, ...) from suffix
	
	// TODO ui.go#serveDownload(...)
	
	var rootRes, err = searchRoots.client.GetPermanodesWithAttr(&search.WithAttrRequest{N: 100, Attr: "camliRoot"})
	if err != nil {
		log.Printf("Get permanodes failure: %s", err)
		// TODO check if this is a good response HTTP code
		http.Error(rw, "Server error", http.StatusInternalServerError)
		return
	}

	dr := &search.DescribeRequest{
		Depth: 1,
	}
	for _, wi := range rootRes.WithAttr {
		dr.BlobRefs = append(dr.BlobRefs, wi.Permanode)
	}
	if len(dr.BlobRefs) == 0 {
		http.Error(rw, "Not found.", http.StatusNotFound)
		return
	}

	dres, err := searchRoots.client.Describe(dr)
	if err != nil {
		log.Printf("Describe failure: %s", err)
		http.Error(rw, "Server error", http.StatusInternalServerError)
		return
	}

	for _, wi := range rootRes.WithAttr {
		pn := wi.Permanode
		db := dres.Meta[pn.String()]
		if db != nil && db.Permanode != nil {
			name := db.Permanode.Attr.Get("camliRoot")
			if name != "" {
				log.Printf("root: %s %s", name, pn)
			}
		}
	}

	//h := rw.Header()
	//h.Set("X-Root", // TODO
}
