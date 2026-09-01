package status

import (
 "fmt"
 "os"
 "sort"
 "time"
 "github.com/yatinannam/devpulse/internal/traffic"
)

type Summary struct {
 Total, Errors, Slow int
 Average time.Duration
 Findings int
}

func Build(entries []traffic.Request) Summary {
 var s Summary
 s.Total=len(entries)
 if len(entries)==0{return s}
 var total time.Duration
 for _,e:=range entries {
  total+=e.Latency
  if e.Status>=400{s.Errors++}
  if e.Latency>=500*time.Millisecond{s.Slow++}
 }
 s.Average=total/time.Duration(len(entries))
 return s
}

type Endpoint struct{ Method, Path string; Count int; Errors int; Average time.Duration }

func Endpoints(entries []traffic.Request) []Endpoint {
 m:=map[string]*Endpoint{}
 for _,e:=range entries {
  key:=e.Method+" "+e.Path
  x:=m[key];if x==nil{x=&Endpoint{Method:e.Method,Path:e.Path};m[key]=x}
  x.Count++;if e.Status>=400{x.Errors++};x.Average+=(e.Latency)
 }
 out:=make([]Endpoint,0,len(m))
 for _,x:=range m{x.Average/=time.Duration(x.Count);out=append(out,*x)}
 sort.Slice(out,func(i,j int)bool{return out[i].Count>out[j].Count})
 return out
}

func Print(s Summary, eps []Endpoint, session string) {
 fmt.Println("DEVPULSE")
 fmt.Println("────────────────────────────────────────────────")
 fmt.Printf("Session   %s\n",session)
 fmt.Printf("Requests  %d\nErrors    %d\nAverage   %s\nSlow      %d\n",s.Total,s.Errors,s.Average.Round(time.Millisecond),s.Slow)
 fmt.Println()
 if len(eps)==0 {fmt.Println("No captured HTTP traffic.");return}
 fmt.Println("ENDPOINTS")
 for _,e:=range eps {
  marker:=" "
  if e.Errors>0 || e.Average>=500*time.Millisecond {marker="!"}
  fmt.Printf("%s %-6s %-32s %3d calls  %3d errors  %s avg\n",marker,e.Method,e.Path,e.Count,e.Errors,e.Average.Round(time.Millisecond))
 }
 if s.Errors==0&&s.Slow==0 {fmt.Println("\n✓ Service looks healthy from captured traffic.")}
 _=os.Stdout
}
