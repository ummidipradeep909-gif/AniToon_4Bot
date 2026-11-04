package moderation

import (
	"testing"
	"time"
)

func TestParseReportWindow(t *testing.T) {
	for _, tc := range []struct{ args []string; want time.Duration; label string; wantErr bool }{
		{nil,24*time.Hour,"24-hour",false},
		{[]string{"day"},24*time.Hour,"24-hour",false},
		{[]string{"DAY"},24*time.Hour,"24-hour",false},
		{[]string{"week"},7*24*time.Hour,"7-day",false},
		{[]string{"WEEK"},7*24*time.Hour,"7-day",false},
		{[]string{"month"},0,"",true},
		{[]string{"day","week"},0,"",true},
	} {
		got,label,err:=parseReportWindow(tc.args)
		if (err!=nil)!=tc.wantErr { t.Fatalf("parseReportWindow(%v) err=%v wantErr=%v",tc.args,err,tc.wantErr) }
		if err==nil&&(got!=tc.want||label!=tc.label) { t.Errorf("parseReportWindow(%v)=(%v,%q), want (%v,%q)",tc.args,got,label,tc.want,tc.label) }
	}
}
