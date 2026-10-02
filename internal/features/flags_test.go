package features

import "testing"

func TestCatalogDefaults(t *testing.T) {
	if Default(Profiles) {
		t.Fatal("profiles default must be off")
	}
	if !Default(QuickSearch) || !Default(AutoUpdate) || !Default(TrafficMonitor) || !Default(Mascot) {
		t.Fatal("expected on by default")
	}
	if Default(Notifications) {
		t.Fatal("notifications default must stay off")
	}
	if Default(ID("language")) {
		t.Fatal("unknown flag must be off")
	}
}

func TestSetRejectsUnknownAndKeepsExplicitFalse(t *testing.T) {
	var flags Flags
	if err := flags.Set(ID("language"), false); err == nil {
		t.Fatal("expected unknown feature")
	}
	if flags.Enabled(QuickSearch) != true {
		t.Fatal("missing key must use the default")
	}
	if err := flags.Set(QuickSearch, false); err != nil {
		t.Fatal(err)
	}
	if flags.Enabled(QuickSearch) {
		t.Fatal("explicit false was lost")
	}
	if _, ok := flags.Explicit(QuickSearch); !ok {
		t.Fatal("explicit false must stay stored")
	}
	if err := flags.Set(Profiles, true); err != nil {
		t.Fatal(err)
	}
	cloned := flags.Clone()
	if err := flags.Set(Profiles, false); err != nil {
		t.Fatal(err)
	}
	if !cloned.Enabled(Profiles) || flags.Enabled(Profiles) {
		t.Fatal("clone aliased the stored bool")
	}
}

func TestMigrateLeavesExplicitChoice(t *testing.T) {
	off := false
	var flags Flags
	if err := flags.Set(QuickSearch, false); err != nil {
		t.Fatal(err)
	}
	on := true
	flags.MigrateLegacy(Legacy{QuickSearch: &on, NotificationsOn: true, TrafficExplicitOff: false})
	if flags.Enabled(QuickSearch) {
		t.Fatal("legacy true overwrote explicit false")
	}
	flags.MigrateLegacy(Legacy{QuickSearch: &off, NotificationsOn: true, TrafficExplicitOff: true})
	if !flags.Enabled(Notifications) || flags.Enabled(TrafficMonitor) {
		t.Fatalf("legacy migration: %+v", flags)
	}
	if flags.Enabled(Profiles) {
		t.Fatal("profiles must stay at the default")
	}
	_ = off
}

func TestViewsUseCatalogOrder(t *testing.T) {
	views := Flags{}.Views()
	if len(views) != len(All()) {
		t.Fatalf("views %d catalog %d", len(views), len(All()))
	}
	if views[0].ID != string(Profiles) || views[0].Enabled || views[0].TitleKey != "features.profiles" {
		t.Fatalf("first view: %+v", views[0])
	}
	found := map[string]bool{}
	for _, view := range views {
		found[view.TitleKey] = true
		found[view.DescriptionKey] = true
	}
	for _, flag := range All() {
		if !found[flag.TitleKey] || !found[flag.DescriptionKey] {
			t.Fatalf("missing i18n key for %s", flag.ID)
		}
	}
}
