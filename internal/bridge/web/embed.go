package web

import "embed"

//go:embed templates/*.html templates/partials/*.html
var templatesFS embed.FS

//go:embed i18n/*.json
var i18nFS embed.FS
