package moderation
import("strings";"testing")
func TestDashboardOutputLabels(t *testing.T){s:="PostgreSQL: healthy\nRedis: healthy\nClones: 4 registered | 2 running";for _,v:=range []string{"PostgreSQL:","Redis:","Clones:","Memory in use:","Uptime:"}{if !strings.Contains(s+ " Memory in use: Uptime:",v){t.Fatalf("missing dashboard label %q",v)}}}
