package moderation

import "testing"

func TestNormalizeBlacklistPhrase(t *testing.T) {
	tests:=[]struct{in,want string;wantErr bool}{
		{"  Bad   WORD  ","bad word",false},
		{"ab","ab",false},
		{"x","",true},
		{"","",true},
		{"a\nb","a b",false},
	}
	for _,tc:=range tests {
		got,err:=normalizeBlacklistPhrase(tc.in)
		if (err!=nil)!=tc.wantErr {t.Errorf("normalize(%q) err=%v",tc.in,err);continue}
		if err==nil && got!=tc.want {t.Errorf("normalize(%q)=%q want %q",tc.in,got,tc.want)}
	}
}

func TestContainsBlacklistPhrase(t *testing.T) {
	for _,tc:=range []struct{text,phrase string;want bool}{
		{"THIS is BAD WORD here","bad word",true},
		{"badly word","bad word",false},
		{"clean message","spam",false},
		{"spam in caption","spam",true},
	} {
		if got:=containsBlacklistPhrase(tc.text,tc.phrase);got!=tc.want {t.Errorf("contains(%q,%q)=%v want %v",tc.text,tc.phrase,got,tc.want)}
	}
}
