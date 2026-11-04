package moderation

import "testing"

func TestParseRaidConfig(t *testing.T) {
	tests := []struct {
		args []string
		want raidConfig
		wantErr bool
	}{
		{[]string{"5","10"}, raidConfig{JoinLimit:5,WindowSeconds:10,QuarantineMinutes:10}, false},
		{[]string{"20","60","15"}, raidConfig{JoinLimit:20,WindowSeconds:60,QuarantineMinutes:15}, false},
		{[]string{"3","5","1"}, raidConfig{JoinLimit:3,WindowSeconds:5,QuarantineMinutes:1}, false},
		{[]string{"2","10"}, raidConfig{}, true},
		{[]string{"101","10"}, raidConfig{}, true},
		{[]string{"5","4"}, raidConfig{}, true},
		{[]string{"5","301"}, raidConfig{}, true},
		{[]string{"5","10","0"}, raidConfig{}, true},
		{[]string{"5","10","61"}, raidConfig{}, true},
		{[]string{"5"}, raidConfig{}, true},
		{[]string{"5","10","10","extra"}, raidConfig{}, true},
	}
	for _,tc := range tests {
		got,err:=parseRaidConfig(tc.args)
		if (err!=nil)!=tc.wantErr { t.Fatalf("parseRaidConfig(%v) err=%v wantErr=%v",tc.args,err,tc.wantErr) }
		if err==nil && got!=tc.want { t.Fatalf("parseRaidConfig(%v)=%+v want %+v",tc.args,got,tc.want) }
	}
}

func TestIsNewMemberJoin(t *testing.T) {
	for _,tc:=range []struct{old,new string; want bool}{
		{"left","member",true},{"kicked","member",true},
		{"member","member",false},{"member","left",false},
		{"administrator","member",false},{"restricted","member",false},
	} {
		if got:=isNewMemberJoin(tc.old,tc.new);got!=tc.want { t.Fatalf("isNewMemberJoin(%q,%q)=%v want %v",tc.old,tc.new,got,tc.want) }
	}
}

func TestRaidKeysAreScopedToBotAndChat(t *testing.T) {
	base:=raidCounterKey(1,2)
	if base==raidCounterKey(3,2) || base==raidCounterKey(1,4) { t.Fatal("raid counters must be isolated by bot and chat") }
	if raidAlertKey(1,2)==raidAlertKey(1,3) { t.Fatal("alert cooldown must be isolated by chat") }
	if raidAIReviewKey(1,2)==raidAIReviewKey(1,3) { t.Fatal("AI review cooldown must be isolated by chat") }
	if raidAIReviewKey(1,2)==raidAIReviewKey(2,2) { t.Fatal("AI review cooldown must be isolated by bot") }
	if raidQuarantineKey(1,2,3)==raidQuarantineKey(1,2,4) { t.Fatal("quarantine markers must be isolated by member") }
}

func TestRaidQuarantinePermissionsExplicitlyDenyAllMemberActions(t *testing.T) {
	for key,value := range raidQuarantinePermissions() {
		allowed,ok:=value.(bool)
		if !ok || allowed { t.Fatalf("quarantine permission %s must be explicitly false",key) }
	}
	if len(raidQuarantinePermissions()) < 10 { t.Fatal("quarantine payload is missing Telegram permission fields") }
}
