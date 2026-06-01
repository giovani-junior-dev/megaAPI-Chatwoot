package web

import (
	"encoding/json"
	"html/template"
)

const defaultLocale = "pt-BR"

func loadLocale(name string) (map[string]string, error) {
	raw, err := i18nFS.ReadFile("i18n/" + name + ".json")
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func newTFunc(strings map[string]string) func(string) string {
	return func(key string) string {
		if v, ok := strings[key]; ok {
			return v
		}
		return key
	}
}

// dictMap builds a map[string]any from alternating key/value args for use
// in template sub-renderings like {{template "badge" (dict "Variant" "success" ...)}}.
func dictMap(values ...any) map[string]any {
	if len(values)%2 != 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, _ := values[i].(string)
		out[key] = values[i+1]
	}
	return out
}

func i18nFuncMap(strings map[string]string) template.FuncMap {
	return template.FuncMap{
		"t":    newTFunc(strings),
		"dict": dictMap,
	}
}
