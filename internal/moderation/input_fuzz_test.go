package moderation

import (
	"strings"
	"testing"
)

func FuzzParseFloodConfigNeverPanics(f *testing.F) {
	for _,seed:=range [][]string{{"5","10"},{"2","3","ban"},{""},{"999999999999999999999999","10"},{"5","-1"},{"5","10","delete","extra"}} { f.Add(strings.Join(seed,"\x00")) }
	f.Fuzz(func(t *testing.T,raw string){
		if len(raw)>4096 { t.Skip() }
		_ = raw
		parts:=strings.Split(raw,"\x00")
		_,_ = parseFloodConfig(parts)
	})
}

func FuzzParseRaidConfigNeverPanics(f *testing.F) {
	for _,seed:=range [][]string{{"5","10"},{"3","5","1"},{""},{"999999999999999999999999","10"},{"5","-1"},{"5","10","10","extra"}} { f.Add(strings.Join(seed,"\x00")) }
	f.Fuzz(func(t *testing.T,raw string){
		if len(raw)>4096 { t.Skip() }
		_,_ = parseRaidConfig(strings.Split(raw,"\x00"))
	})
}

func FuzzNormalizeBlacklistPhraseNeverAcceptsOversizedInput(f *testing.F) {
	for _,seed:=range []string{"spam","  bad   word ","","\x00","🙂","a\nb"} { f.Add(seed) }
	f.Fuzz(func(t *testing.T,raw string){
		if len(raw)>8192 { t.Skip() }
		got,err:=normalizeBlacklistPhrase(raw)
		if err==nil {
			if len([]rune(got))<2 || len([]rune(got))>maxBlacklistPhraseLength {
				t.Fatalf("accepted phrase with invalid rune length %d",len([]rune(got)))
			}
			for _,r:=range got { if r=='\x00' { t.Fatal("accepted control character") } }
		}
	})
}
