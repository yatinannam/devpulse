package discovery

import (
 "fmt"
 "net"
 "net/http"
 "strings"
 "time"
 "github.com/yatinannam/devpulse/internal/ports"
)

type Service struct {
 Port int
 Process string
 PID string
 State string
 HTTP bool
 URL string
}

func Discover(timeout time.Duration) ([]Service,error) {
 entries,err:=ports.List(); if err!=nil{return nil,err}
 out:=make([]Service,0,len(entries))
 for _,e:=range entries {
  s:=Service{Port:e.Port,Process:e.Process,PID:e.PID,State:e.State}
  s.HTTP=probe(e.Port,timeout)
  if s.HTTP{s.URL=fmt.Sprintf("http://127.0.0.1:%d",e.Port)}
  out=append(out,s)
 }
 return out,nil
}

func probe(port int, timeout time.Duration) bool {
 addr:=fmt.Sprintf("127.0.0.1:%d",port)
 conn,err:=net.DialTimeout("tcp",addr,timeout);if err!=nil{return false}
 _=conn.Close()
 client:=http.Client{Timeout:timeout,CheckRedirect:func(req *http.Request,via []*http.Request)error{return http.ErrUseLastResponse}}
 resp,err:=client.Get("http://"+addr+"/")
 if err==nil {resp.Body.Close();return resp.StatusCode>0}
 return strings.HasPrefix(err.Error(),"http:")
}
