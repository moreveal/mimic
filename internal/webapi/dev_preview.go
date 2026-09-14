package webapi

import (
	_ "embed"
	"strings"
)

//go:embed dev_preview.js
var devPreviewSource string

// WithDevPreview is applied before the bootstrap cache key is computed. Normal
// realms neither parse this source nor install its callback or form tracking.
func WithDevPreview(source string) string {
	source = strings.Replace(source, "/* dev_preview_state */", "let devPreviewFormRevision=0;", 1)
	source = strings.Replace(source, "/* dev_preview_sources */", "previewSources(root){return Array.from(ownerCollection(root)).concat(adopted.get(root)||[]).map(sheet=>({text:sourceText(sheet),base:requireSheet(sheet).href||host.location()}))},", 1)
	source = strings.Replace(source, "/* dev_preview_form_revision */", "if(descriptor.set){const set=descriptor.set;descriptor={...descriptor,set(value){try{return Reflect.apply(set,this,[value])}finally{devPreviewFormRevision++}}}}", 1)
	return strings.Replace(source, "/* dev_preview_install */", devPreviewSource, 1)
}
