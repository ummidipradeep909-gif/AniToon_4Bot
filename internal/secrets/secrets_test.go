package secrets

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestLoadDirectAndFileValues(t *testing.T) {
 t.Setenv("TEST_SECRET_VALUE","legacy-value")
 t.Setenv("TEST_SECRET_VALUE_FILE","")
 got,err:=Load("TEST_SECRET_VALUE",true)
 if err!=nil || got!="legacy-value" { t.Fatalf("direct load: %q %v",got,err) }
 path:=filepath.Join(t.TempDir(),"secret")
 if err:=os.WriteFile(path,[]byte("file-value\n"),0600); err!=nil { t.Fatal(err) }
 t.Setenv("TEST_SECRET_VALUE","")
 t.Setenv("TEST_SECRET_VALUE_FILE",path)
 got,err=Load("TEST_SECRET_VALUE",true)
 if err!=nil || got!="file-value" { t.Fatalf("file load: %q %v",got,err) }
}

func TestLoadRejectsAmbiguousMissingAndEmptySecrets(t *testing.T) {
 t.Setenv("TEST_SECRET_VALUE","direct")
 t.Setenv("TEST_SECRET_VALUE_FILE","/tmp/not-used")
 if _,err:=Load("TEST_SECRET_VALUE",true); err==nil { t.Fatal("ambiguous secret configuration accepted") }
 t.Setenv("TEST_SECRET_VALUE","")
 t.Setenv("TEST_SECRET_VALUE_FILE","")
 if _,err:=Load("TEST_SECRET_VALUE",true); err==nil { t.Fatal("missing required secret accepted") }
 path:=filepath.Join(t.TempDir(),"empty")
 if err:=os.WriteFile(path,nil,0600); err!=nil { t.Fatal(err) }
 t.Setenv("TEST_SECRET_VALUE_FILE",path)
 if _,err:=Load("TEST_SECRET_VALUE",true); err==nil || strings.Contains(err.Error(),path) { t.Fatalf("empty or path-leaking error: %v",err) }
}

func TestLoadFileErrorDoesNotLeakPath(t *testing.T) {
 path:=filepath.Join(t.TempDir(),"private-secret")
 t.Setenv("TEST_SECRET_VALUE","")
 t.Setenv("TEST_SECRET_VALUE_FILE",path)
 _,err:=Load("TEST_SECRET_VALUE",true)
 if err==nil || strings.Contains(err.Error(),path) { t.Fatalf("secret path leaked or error missing: %v",err) }
}
