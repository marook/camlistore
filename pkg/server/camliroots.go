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

/*
Registers the "camli-roots" handler for delivering content from the
roots directory structure.

Example low-level config:

    "/myRoots/": {
        "handler": "camli-roots",
        "handlerArgs": {}
    }

*/

package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"perkeep.org/pkg/auth"
	"perkeep.org/pkg/blob"
	"perkeep.org/pkg/blobserver"
	"perkeep.org/internal/httputil"
	"perkeep.org/internal/magic"
	"perkeep.org/pkg/schema"
	"perkeep.org/pkg/search"

	"go4.org/jsonconfig"
	"golang.org/x/net/context"
)

type CamliRootsHandler struct {
	Fetcher blob.Fetcher

	// Search is optional. If present, it's used to map a fileref
	// to a wholeref, if the Fetcher is of a type that knows how
	// to get at a wholeref more efficiently. (e.g. blobpacked)
	search *search.Handler
}

func init() {
	blobserver.RegisterHandlerConstructor("camli-roots", camliRootsFromConfig)
}

func camliRootsFromConfig(ld blobserver.Loader, conf jsonconfig.Obj) (h http.Handler, err error) {
	camliRoots := &CamliRootsHandler{
		search: nil,                // initialized by InitHandler(...)
	}

	if err = conf.Validate(); err != nil {
		return
	}

	rootPrefix, _, err := ld.FindHandlerByType("root")
	if err != nil {
		return nil, errors.New("No root handler configured, which is necessary for the camli-roots handler")
	}
	if h, err := ld.GetHandler(rootPrefix); err == nil {
		camliRoots.Fetcher = h.(*RootHandler).Storage
	} else {
		return nil, errors.New("failed to find the 'root' handler")
	}

	return camliRoots, nil
}

func (camliRoots *CamliRootsHandler) InitHandler(hl blobserver.FindHandlerByTyper) error {
	// InitHandler is called after all handlers have been setup, so the bootstrap
	// of the camliRoot node for publishers in dev-mode is already done.
	searchPrefix, _, err := hl.FindHandlerByType("search")
	if err != nil {
		return errors.New("No search handler configured, which is necessary for the ui handler")
	}
	var sh *search.Handler
	_, hi := hl.AllHandlers()
	if h, ok := hi[searchPrefix]; !ok {
		return errors.New("failed to find the \"search\" handler")
	} else {
		sh = h.(*search.Handler)
		camliRoots.search = sh
	}
	return nil
}

func (camliRoots *CamliRootsHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !auth.Allowed(req, auth.OpGet) {
		auth.SendUnauthorized(rw, req)
		return
	}

	if req.Method != "GET" {
		http.Error(rw, "Invalid method", http.StatusBadRequest)
		return
	}

	pathSegments := strings.Split(httputil.PathSuffix(req), "/")
	if len(pathSegments) < 1 {
		http.Error(rw, "Not found.", http.StatusNotFound)
		return
	}

	ctx := req.Context()
	camliRootDescribe, err := camliRoots.FindCamliRoot(ctx, rw, pathSegments[0])
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
			log.Printf("Failed to parse ref '%s'", *nextBlobRefStr)
			http.Error(rw, "Server error", http.StatusInternalServerError)
			return
		}

		constraint := search.Constraint{
			CamliType: "permanode",
			BlobRefPrefix: *nextBlobRefStr,
		}
		describeRule := search.DescribeRule{
			Attrs: []string{ "camliPath:*", "camliContent" },
		}
		describe := search.DescribeRequest{
			Depth: 1,
			BlobRefs: []blob.Ref{ nextBlobRef },
			Rules: []*search.DescribeRule{ &describeRule },
		}
		query := search.SearchQuery{
			Constraint: &constraint,
			Limit: 100,
			Describe: &describe,
		}
		results, err := camliRoots.search.Query(ctx, &query)
		if err != nil {
			log.Printf("child query failure: %v", err)
			http.Error(rw, "Server error", http.StatusInternalServerError)
			return
		}
		if len(results.Blobs) == 0 {
			http.Error(rw, fmt.Sprintf("child %v not found.", pathSegment), http.StatusNotFound)
			return
		}
		currentPermanodeDescribe = results.Describe.Meta[*nextBlobRefStr]
		if currentPermanodeDescribe == nil {
			log.Printf("Failed to query permanode %v (below %v)", *nextBlobRefStr, pathSegment)
			http.Error(rw, "Server error", http.StatusInternalServerError)
			return
		}
	}

	camliRoots.ServePermanodeContent(rw, req, currentPermanodeDescribe)
}

func (camliRoots *CamliRootsHandler) FindCamliRoot(context context.Context, rw http.ResponseWriter, camliRootName string) (*search.DescribedBlob, error) {
	permanodeConstraint := search.PermanodeConstraint{
		Attr: "camliRoot",
		Value: camliRootName,
	}
	constraint := search.Constraint{
		Permanode: &permanodeConstraint,
	}
	describeRule := search.DescribeRule{
		IfResultRoot: true,
		Attrs: []string{ "camliPath:*" },
	}
	describe := search.DescribeRequest{
		Rules: []*search.DescribeRule{ &describeRule },
	}
	query := search.SearchQuery{
		Constraint: &constraint,
		Limit: 100,
		Describe: &describe,
	}
	results, err := camliRoots.search.Query(context, &query)
	if err != nil {
		log.Printf("camliRoot query failure: %v", err)
		http.Error(rw, "Server error", http.StatusInternalServerError)
		return nil, err
	}
	if len(results.Blobs) == 0 {
		msg := fmt.Sprintf("camliRoot '%s' not found.", camliRootName)
		http.Error(rw, msg, http.StatusNotFound)
		return nil, errors.New(msg)
	}
	camliRootRef := results.Blobs[0].Blob
	return results.Describe.Meta[camliRootRef.String()], nil
}

func (camliRoots *CamliRootsHandler) ServePermanodeContent(rw http.ResponseWriter, req *http.Request, permanodeDescribe *search.DescribedBlob) {
	// TODO large parts of this function are copied from download.go#ServeHTTP(...). should be refactored to reduce duplication.
	// TODO make sure that we actually got ONE camliContent attribute
	contentRefStr := permanodeDescribe.Permanode.Attr.Get("camliContent")
	file, ok := blob.Parse(contentRefStr)
	if !ok {
		log.Printf("Failed to parse ref '%s'", contentRefStr)
		http.Error(rw, "Server error", http.StatusInternalServerError)
		return
	}

	if req.Header.Get("If-None-Match") == contentRefStr {
		rw.WriteHeader(http.StatusNotModified)
		return
	}

	fi, err := camliRoots.fileInfo(req, file)
	if err != nil {
		http.Error(rw, "Can't serve file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer fi.close()

	h := rw.Header()
	h.Set("Content-Length", fmt.Sprint(fi.size))
	h.Set("Expires", time.Now().Add(60*time.Second).Format(http.TimeFormat))
	h.Set("Content-Type", fi.mime)
	h.Set("ETag", contentRefStr)

	if fi.mime == "application/octet-stream" {
		// Chrome seems to silently do nothing on
		// application/octet-stream unless this is set.
		// Maybe it's confused by lack of URL it recognizes
		// along with lack of mime type?
		fileName := fi.name
		if fileName == "" {
			fileName = "file-" + file.String() + ".dat"
		}
		rw.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	}

	for _, headerSpec := range permanodeDescribe.Permanode.Attr["per:httpHeader"] {
		separatorIndex := strings.Index(headerSpec, "=")
		if separatorIndex == -1 {
			log.Printf("Missign separator in per:httpHeader \"%v\" with permanode %v", headerSpec, permanodeDescribe.BlobRef)
			continue
		}
		headerName := headerSpec[:separatorIndex]
		headerValue := headerSpec[separatorIndex + 1:]
		rw.Header().Set(headerName, headerValue)
	}

	http.ServeContent(rw, req, "", time.Now(), fi.rs)
}

func (camliRootsHandler *CamliRootsHandler) fileInfo(req *http.Request, file blob.Ref) (fi fileInfo, err error) {
	// TODO this function was copied from download.go... should be refactored to be unique in one place

	// Fast path for blobpacked.
	fi, ok := fileInfoPacked(context.TODO(), camliRootsHandler.search, camliRootsHandler.Fetcher, req, file)
	if ok {
		return fi, nil
	}
	fr, err := schema.NewFileReader(context.TODO(), camliRootsHandler.Fetcher, file)
	if err != nil {
		return
	}
	mime := magic.MIMETypeFromReaderAt(fr)
	if mime == "" {
		name := fr.FileName()
		if strings.HasSuffix(name, ".css") {
			mime = "text/css"
		} else if strings.HasSuffix(name, ".json") {
			mime = "application/json"
		} else if strings.HasSuffix(name, ".js") {
			mime = "application/javascript"
		}
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	return fileInfo{
		mime:  mime,
		name:  fr.FileName(),
		size:  fr.Size(),
		rs:    fr,
		close: fr.Close,
	}, nil
}
