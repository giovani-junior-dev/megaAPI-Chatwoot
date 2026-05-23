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

func i18nFuncMap(strings map[string]string) template.FuncMap {
	return template.FuncMap{"t": newTFunc(strings)}
}
