package scheme

import "testing"

func TestOpenCommandQuotesExecutable(t *testing.T) {
	got, err := OpenCommand(`C:\Program Files\Norka\norka.exe`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `"C:\Program Files\Norka\norka.exe" "%1"` {
		t.Fatalf("command %s", got)
	}
	if _, err := OpenCommand(`C:\bad"norka.exe`); err == nil {
		t.Fatal("quote in the path must be rejected")
	}
}
