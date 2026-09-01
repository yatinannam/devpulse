package doctor

import (
 "testing"
 "time"
 "github.com/yatinannam/devpulse/internal/traffic"
)

func TestAnalyzeDuplicateRequests(t *testing.T) {
 now:=time.Now()
 entries:=[]traffic.Request{
  {Time:now,Method:"GET",Path:"/api/users",Status:200},
  {Time:now.Add(time.Second),Method:"GET",Path:"/api/users",Status:200},
  {Time:now.Add(2*time.Second),Method:"GET",Path:"/api/users",Status:200},
 }
 findings:=Analyze(entries)
 if len(findings)!=1 || findings[0].Title!="Duplicate requests" { t.Fatalf("unexpected findings: %#v",findings) }
}
