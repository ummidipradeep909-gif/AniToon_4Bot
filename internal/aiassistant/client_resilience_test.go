package aiassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const validAssistResponse = `{"mode":"assist","enforcement_enabled":false,"analysis":{"category":"spam","confidence":0.8,"recommendation":"admin_review","rationale":"review"}}`

func TestAnalyzeTruncatesOversizedUnicodeMessageByRunes(t *testing.T) {
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		var payload struct{Text string `json:"text"`}
		if err:=json.NewDecoder(r.Body).Decode(&payload);err!=nil{t.Error(err);http.Error(w,"bad request",400);return}
		if got:=len([]rune(payload.Text));got!=4000{t.Errorf("sent %d runes, want 4000",got)}
		w.Header().Set("Content-Type","application/json")
		_,_=w.Write([]byte(validAssistResponse))
	}))
	defer server.Close()
	client:=NewClient(server.URL,"test-service-token")
	if _,err:=client.Analyze(context.Background(),1,2,3,"anti_flood",strings.Repeat("🙂",4001));err!=nil{t.Fatal(err)}
}

func TestAnalyzeHonorsCallerTimeout(t *testing.T) {
	releaseHandler := make(chan struct{})
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		<-releaseHandler
	}))
	defer func() {
		close(releaseHandler)
		server.Close()
	}()
	client:=NewClient(server.URL,"test-service-token")
	ctx,cancel:=context.WithCancel(context.Background())
	result:=make(chan error,1)
	go func() {
		_,err:=client.Analyze(ctx,1,2,3,"anti_flood","burst")
		result<-err
	}()
	cancel()
	select {
	case err:=<-result:
		if err==nil { t.Fatal("expected caller cancellation to abort AI request") }
	case <-time.After(time.Second):
		t.Fatal("AI request did not return promptly after caller cancellation")
	}
}

func TestAnalyzeRejectsMalformedAndNonSuccessResponses(t *testing.T) {
	for _,tc:=range []struct{name string;status int;body string}{
		{"malformed JSON",http.StatusOK,"not-json"},
		{"provider unavailable",http.StatusServiceUnavailable,`{"error":"unavailable"}`},
		{"punitive recommendation",http.StatusOK,`{"mode":"assist","enforcement_enabled":false,"analysis":{"category":"spam","confidence":1,"recommendation":"ban","rationale":"no"}}`},
	} {
		t.Run(tc.name,func(t *testing.T){
			server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(tc.status);_,_=w.Write([]byte(tc.body))}))
			defer server.Close()
			if _,err:=NewClient(server.URL,"test-token").Analyze(context.Background(),1,2,3,"anti_flood","test");err==nil{t.Fatal("expected invalid response to fail safely")}
		})
	}
}

func TestAnalyzeRejectsOversizedResponseBody(t *testing.T) {
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		w.Header().Set("Content-Type","application/json")
		_,_=w.Write([]byte(`{"mode":"assist","enforcement_enabled":false,"analysis":{"category":"spam","confidence":0.8,"recommendation":"allow","rationale":"`+strings.Repeat("x",70*1024)+`"}}`))
	}))
	defer server.Close()
	if _,err:=NewClient(server.URL,"test-token").Analyze(context.Background(),1,2,3,"anti_flood","test");err==nil{t.Fatal("oversized AI response must be rejected")}
}
