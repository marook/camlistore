/*
Copyright 2017 The Perkeep Authors.

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

// The goui program inserts the javascript code generated by gopherjs for the
// web UI into the existing web UI javascript codebase.
package main

import (
	"context"

	"perkeep.org/pkg/blob"
	"perkeep.org/server/perkeepd/ui/goui/aboutdialog"
	"perkeep.org/server/perkeepd/ui/goui/dirchildren"
	"perkeep.org/server/perkeepd/ui/goui/downloadbutton"
	"perkeep.org/server/perkeepd/ui/goui/importshare"
	"perkeep.org/server/perkeepd/ui/goui/mapquery"
	"perkeep.org/server/perkeepd/ui/goui/selectallbutton"
	"perkeep.org/server/perkeepd/ui/goui/sharebutton"

	"github.com/gopherjs/gopherjs/js"
)

func main() {
	js.Global.Set("goreact", map[string]interface{}{
		"AboutMenuItem":    aboutdialog.New,
		"DownloadItemsBtn": downloadbutton.New,
		"ShareItemsBtn":    sharebutton.New,
		"SelectAllBtn":     selectallbutton.New,
		"NewDirChildren":   dirchildren.New,
		// TODO: we want to investigate integrating the share importer with the other
		// importers. But if we instead end up keeping it tied to a dialog, we need to add
		// a cancel button to the dialog, that triggers the context cancellation.
		"ImportShare": func(cfg map[string]string, shareURL string, updateDialogFunc func(message string, importedBlobRef string)) {
			importshare.Import(context.TODO(), cfg, shareURL, updateDialogFunc)
		},
		"NewMapQuery":      mapquery.New,
		"DeleteMapZoom":    mapquery.DeleteZoomPredicate,
		"ShiftMapZoom":     mapquery.ShiftZoomPredicate,
		"HasZoomParameter": mapquery.HasZoomParameter,
		"IsBlobRef": func(ref string) bool {
			_, ok := blob.Parse(ref)
			return ok
		},
	})
}
