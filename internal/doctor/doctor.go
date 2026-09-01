package doctor

import (
 "fmt"
 "sort"
 "time"
 "github.com/yatinannam/devpulse/internal/traffic"
)

type Finding struct { Level, Title, Detail, Advice string }

func Analyze(entries []traffic.Request) []Finding {
 var findings []Finding
 if len(entries)==0{return findings}
 var errors,slow int
 for _,e:=range entries {if e.Status>=400{errors++};if e.Latency>=500*time.Millisecond{slow++}}
 if errors>0 {findings=append(findings,Finding{"ERROR","HTTP errors",fmt.Sprintf("%d of %d requests returned 4xx/5xx",errors,len(entries)),"Inspect the failing endpoints and response status; repeated 401/403 responses usually indicate auth/session problems."})}
 if slow>0 {findings=append(findings,Finding{"WARN","Slow requests",fmt.Sprintf("%d request(s) took 500ms or more",slow),"Check backend logs and downstream calls for the slow endpoint before optimizing the client."})}

 type key struct{method,path string}; groups:=map[key][]traffic.Request{}
 for _,e:=range entries{k:=key{e.Method,e.Path};groups[k]=append(groups[k],e)}
 type dup struct{k key;count int;first,last time.Time};var ds []dup
 for k,g:=range groups {
  if len(g)<3{continue}
  sort.Slice(g,func(i,j int)bool{return g[i].Time.Before(g[j].Time)})
  if g[len(g)-1].Time.Sub(g[0].Time)<=3*time.Second{ds=append(ds,dup{k,len(g),g[0].Time,g[len(g)-1].Time})}
 }
 sort.Slice(ds,func(i,j int)bool{return ds[i].count>ds[j].count})
 for _,d:=range ds{findings=append(findings,Finding{"WARN","Duplicate requests",fmt.Sprintf("%s %s called %d times in %.1fs",d.k.method,d.k.path,d.count,d.last.Sub(d.first).Seconds()),"Check frontend effects, event handlers, retries, and data-fetching logic for an unintended request loop."})}
 return findings
}

func Print(findings []Finding) {
 fmt.Println("DEVPULSE DOCTOR")
 fmt.Println("────────────────────────────────────────────────")
 if len(findings)==0{fmt.Println("✓ No obvious HTTP issues detected.");return}
 for _,f:=range findings{fmt.Printf("%s  %s — %s\n",f.Level,f.Title,f.Detail);fmt.Printf("       → %s\n",f.Advice)}
}
