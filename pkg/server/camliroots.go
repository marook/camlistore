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
	"errors"
	"log"
	"net/http"
	"strings"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/client"
	"camlistore.org/pkg/httputil"
	"camlistore.org/pkg/search"

	"go4.org/jsonconfig"
)

type CamliRootsHandler struct {
	client  *client.Client
}

func init() {
	blobserver.RegisterHandlerConstructor("camli-roots",  camliRootsFromConfig)
}

func camliRootsFromConfig(ld blobserver.Loader, conf jsonconfig.Obj) (h http.Handler, err error) {
	camliRoots := &CamliRootsHandler{
		client: client.NewOrFail(), // automatic from flags
	}

	if err = conf.Validate(); err != nil {
		return
	}

	return camliRoots, nil	
}

func (camliRoots *CamliRootsHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(rw, "Invalid method", http.StatusBadRequest)
		return
	}
	
	pathSegments := strings.Split(httputil.PathSuffix(req), "/")
	if len(pathSegments) < 1 {
		http.Error(rw, "Not found.", http.StatusNotFound)
		return
	}

	camliRootDescribe, err := camliRoots.FindCamliRoot(rw, pathSegments[0])
	if err != nil {
		return
	}

	currentPermanodeDescribe := camliRootDescribe
	for _, pathSegment := range pathSegments[1:] {
		pathAttrKey := "camliPath:" + pathSegment
		var nextBlobRefStr *string = nil

		for attrKey, attrValues := range currentPermanodeDescribe.Permanode.Attr {
			if attrKey == pathAttrKey {
				nextBlobRefStr = &(attrValues[0])
				break
			}
		}

		if nextBlobRefStr == nil {
			http.Error(rw, "Not found.", http.StatusNotFound)
			return
		}

		nextBlobRef, ok := blob.Parse(*nextBlobRefStr)
		if !ok {
			log.Printf("Failed to parse ref %s", *nextBlobRefStr)
			http.Error(rw, "Server error", http.StatusInternalServerError)
			return
		}

		dr := &search.DescribeRequest{
			Depth: 1,
			BlobRefs: []blob.Ref{nextBlobRef},
		}
		dres, err := camliRoots.client.Describe(dr)
		if err != nil {
			log.Printf("Describe failure: %s", err)
			http.Error(rw, "Server error", http.StatusInternalServerError)
			return
		}

		db := dres.Meta[*nextBlobRefStr]
		if db == nil || db.Permanode == nil {
			log.Printf("Expected permanode: %s", *nextBlobRefStr)
			http.Error(rw, "Server error", http.StatusInternalServerError)
			return
		}

		currentPermanodeDescribe = db
	}

	log.Printf("last permanode: %s", currentPermanodeDescribe)

	// TODO download.go#ServeHTTP(...)
}

func (camliRoots *CamliRootsHandler) FindCamliRoot(rw http.ResponseWriter, camliRootName string) (*search.DescribedBlob, error) {
	rootRes, err := camliRoots.client.GetPermanodesWithAttr(&search.WithAttrRequest{N: 100, Attr: "camliRoot"})
	if err != nil {
		http.Error(rw, "Server error", http.StatusInternalServerError)
		return nil, err
	}

	dr := &search.DescribeRequest{
		Depth: 1,
	}
	for _, wi := range rootRes.WithAttr {
		dr.BlobRefs = append(dr.BlobRefs, wi.Permanode)
	}
	if len(dr.BlobRefs) == 0 {
		http.Error(rw, "Not found.", http.StatusNotFound)
		return nil, err
	}

	dres, err := camliRoots.client.Describe(dr)
	if err != nil {
		log.Printf("Describe failure: %s", err)
		http.Error(rw, "Server error", http.StatusInternalServerError)
		return nil, err
	}

	for _, wi := range rootRes.WithAttr {
		pn := wi.Permanode
		db := dres.Meta[pn.String()]
		if db != nil && db.Permanode != nil {
			name := db.Permanode.Attr.Get("camliRoot")
			if name == camliRootName {
				return db, nil
			}
		}
	}
	
	http.Error(rw, "Not found.", http.StatusNotFound)
	return nil, errors.New("No camliRoot found with that name")
}
