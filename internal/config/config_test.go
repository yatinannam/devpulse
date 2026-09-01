package config

import (
 "path/filepath"
 "testing"
)

func TestSaveLoad(t *testing.T) {
 p:=filepath.Join(t.TempDir(),"config.json");t.Setenv("DEVPULSE_CONFIG",p)
 c:=Default();c.Target="http://localhost:8080"
 if err:=Save(c);err!=nil{t.Fatal(err)}
 got,err:=Load();if err!=nil{t.Fatal(err)}
 if got.Target!=c.Target{t.Fatalf("target=%q",got.Target)}
}
