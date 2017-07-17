/*
Copyright 2017 The Camlistore Authors.

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
	"camlistore.org/server/camlistored/ui/goui/aboutdialog"
	"camlistore.org/server/camlistored/ui/goui/downloadbutton"
	"camlistore.org/server/camlistored/ui/goui/geo"
	"camlistore.org/server/camlistored/ui/goui/sharebutton"

	"github.com/gopherjs/gopherjs/js"
)

func main() {
	js.Global.Set("goreact", map[string]interface{}{
		"AboutMenuItem":      aboutdialog.New,
		"DownloadItemsBtn":   downloadbutton.New,
		"ShareItemsBtn":      sharebutton.New,
		"Geocode":            geo.Lookup,
		"IsLocPredicate":     geo.IsLocPredicate,
		"LocPredicatePrefix": geo.LocPredicatePrefix,
		"LocationCenter":     geo.LocationCenter,
		"WrapAntimeridian":   geo.WrapAntimeridian,
	})
}
