package status

import (
 "fmt"
 "sort"
 "time"
 "github.com/yatinannam/devpulse/internal/discovery"
 "github.com/yatinannam/devpulse/internal/traffic"
)

type Summary struct { Total, Errors, Slow int; Average time.Duration; Findings int }

func Build(entries []traffic.Request) Summary {
 var s Summary; s.Total=len(entries); if len(entries)==0{return s}
 var total time.Duration
 for _,e:=range entries {total+=e.Latency;if e.Status>=400{s.Errors++};if e.Latency>=500*time.Millisecond{s.Slow++}}
 s.Average=total/time.Duration(len(entries));return s
}
type Endpoint struct { Method,Path string; Count,Errors int; Average time.Duration }
func Endpoints(entries []traffic.Request) []Endpoint {
 m:=map[string]*Endpoint{}
 for _,e:=range entries {key:=e.Method+" "+e.Path;x:=m[key];if x==nil{x=&Endpoint{Method:e.Method,Path:e.Path};m[key]=x};x.Count++;if e.Status>=400{x.Errors++};x.Average+=e.Latency}
 out:=make([]Endpoint,0,len(m));for _,x:=range m{x.Average/=time.Duration(x.Count);out=append(out,*x)}
 sort.Slice(out,func(i,j int)bool{if out[i].Errors!=out[j].Errors{return out[i].Errors>out[j].Errors};if out[i].Average!=out[j].Average{return out[i].Average>out[j].Average};return out[i].Count>out[j].Count});return out
}
type ServiceTraffic struct { Service discovery.Service; Requests []Endpoint; Total,Errors,Slow int; Average time.Duration }
func GroupByService(services []discovery.Service, entries []traffic.Request) []ServiceTraffic {
 groups:=map[int][]traffic.Request{};for _,e:=range entries{groups[e.TargetPort]=append(groups[e.TargetPort],e)}
 out:=make([]ServiceTraffic,0,len(services));for _,s:=range services{es:=groups[s.Port];summary:=Build(es);out=append(out,ServiceTraffic{Service:s,Requests:Endpoints(es),Total:summary.Total,Errors:summary.Errors,Slow:summary.Slow,Average:summary.Average})};return out
}
func Health(g ServiceTraffic) string {if !g.Service.HTTP{return "running"};if g.Errors>0{return "error"};if g.Slow>0{return "warning"};return "healthy"}
func PrintServices(groups []ServiceTraffic) {
 fmt.Println("SERVICES")
 for _,g:=range groups {
  kind:=g.Service.Kind;if kind==""{kind="TCP service"}
  fmt.Printf("%s :%-5d %-18s %3d req  %2d err  %s avg  [%s]\n",healthMarker(Health(g)),g.Service.Port,kind,g.Total,g.Errors,g.Average.Round(time.Millisecond),Health(g))
  for _,e:=range g.Requests {
   if e.Errors==0&&e.Average<500*time.Millisecond {continue}
   fmt.Printf("  ! %-6s %-28s %3d×  %s avg  (%d errors)\n",e.Method,e.Path,e.Count,e.Average.Round(time.Millisecond),e.Errors)
  }
 }
}
func healthMarker(h string) string {switch h{case "healthy":return "●";case "warning":return "●";case "error":return "●";default:return "○"}}
