package traytext

import (
	"encoding/json"
	"testing"
)

func TestForLocaleComplete(t *testing.T) {
	ru := ForLocale("ru")
	en := ForLocale("en")
	if !ru.Complete() || !en.Complete() {
		t.Fatalf("incomplete catalogs ru=%v en=%v", ru, en)
	}
	if ru.ShowMainTitle == en.ShowMainTitle {
		t.Fatal("russian and english show-window labels must differ")
	}
	if en.HeaderNone != "Norka · no tunnels" {
		t.Fatalf("english empty header = %q", en.HeaderNone)
	}
	if ru.HeaderNone != "Norka · нет туннелей" {
		t.Fatalf("russian empty header = %q", ru.HeaderNone)
	}
}

func TestTrayJSONKeysMatch(t *testing.T) {
	data, err := trayFS.ReadFile("tray.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalogs map[string]map[string]string
	if err := json.Unmarshal(data, &catalogs); err != nil {
		t.Fatal(err)
	}
	ru, en := catalogs["ru"], catalogs["en"]
	if len(ru) == 0 || len(en) == 0 {
		t.Fatal("both ru and en catalogs are required")
	}
	for key, value := range ru {
		other, ok := en[key]
		if !ok {
			t.Errorf("en is missing %s", key)
			continue
		}
		if value == "" || other == "" {
			t.Errorf("empty string for %s", key)
		}
	}
	for key := range en {
		if _, ok := ru[key]; !ok {
			t.Errorf("ru is missing %s", key)
		}
	}
}
