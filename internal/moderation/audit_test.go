package moderation

import "testing"

func TestParseAuditLimit(t *testing.T) {
	for _,tc:=range []struct{args []string; want int; wantErr bool}{
		{nil,10,false},{[]string{"1"},1,false},{[]string{"20"},20,false},
		{[]string{"0"},0,true},{[]string{"21"},0,true},
		{[]string{"abc"},0,true},{[]string{"1","2"},0,true},
	} {
		got,err:=parseAuditLimit(tc.args)
		if (err!=nil)!=tc.wantErr { t.Fatalf("parseAuditLimit(%v) err=%v wantErr=%v",tc.args,err,tc.wantErr) }
		if err==nil && got!=tc.want { t.Fatalf("parseAuditLimit(%v)=%d want %d",tc.args,got,tc.want) }
	}
}
