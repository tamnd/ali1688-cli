package ali1688

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "ali1688" {
		t.Errorf("Scheme = %q, want ali1688", info.Scheme)
	}
	if len(info.Hosts) == 0 {
		t.Error("Hosts is empty")
	}
	if info.Identity.Binary != "ali1688" {
		t.Errorf("Identity.Binary = %q, want ali1688", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in      string
		wantTyp string
		wantID  string
		wantErr bool
	}{
		{"789123456", "product", "789123456", false},
		{"https://detail.1688.com/offer/789123456.html", "product", "789123456", false},
		{"", "", "", true},
		{"not-a-number", "", "", true},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Classify(%q): want error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Classify(%q): unexpected error %v", tc.in, err)
			continue
		}
		if typ != tc.wantTyp || id != tc.wantID {
			t.Errorf("Classify(%q) = (%q, %q), want (%q, %q)",
				tc.in, typ, id, tc.wantTyp, tc.wantID)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("product", "123456")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://detail.1688.com/offer/123456.html"
	if got != want {
		t.Errorf("Locate = %q, want %q", got, want)
	}

	_, err = Domain{}.Locate("unknown", "123")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestDomainRegistered(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Domain("ali1688"); !ok {
		t.Fatal("ali1688 domain not registered")
	}
}
