package moderation

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestGreetingTemplateValidation(t *testing.T) {
	if _,err:=validateGreetingTemplate("   "); err==nil {t.Fatal("empty template accepted")}
	if _,err:=validateGreetingTemplate(strings.Repeat("x",1001)); err==nil {t.Fatal("oversized template accepted")}
	if _,err:=validateGreetingTemplate("hello\x00"); err==nil {t.Fatal("control character accepted")}
	if got,err:=validateGreetingTemplate(" Hello\n{first} "); err!=nil || got!="Hello\n{first}" {t.Fatalf("unexpected validation: %q %v",got,err)}
}

func TestGreetingMembershipTransitions(t *testing.T) {
	if !isGreetingJoin("left","member") || !isGreetingJoin("kicked","member") {t.Fatal("valid join transition missed")}
	if isGreetingJoin("member","administrator") {t.Fatal("promotion must not trigger welcome")}
	if !isGreetingLeave("member","left") || !isGreetingLeave("administrator","kicked") {t.Fatal("valid leave transition missed")}
	if isGreetingLeave("member","restricted") {t.Fatal("restriction must not trigger goodbye")}
}

func TestRenderGreetingEscapesUserAndChatContent(t *testing.T) {
	user:=gotgbot.User{Id:42,FirstName:"<Alice>",LastName:"&Co",Username:"alice"}
	chat:=gotgbot.Chat{Title:"<group>"}
	got:=renderGreeting("Hi {mention} ({first}) {username} {id} in {chatname}",user,chat)
	for _,want:=range []string{"&lt;Alice&gt;","&amp;Co","@alice","42","&lt;group&gt;","<a href=\"tg://user?id=42\">"} {
		if !strings.Contains(got,want) {t.Fatalf("rendered greeting missing %q: %s",want,got)}
	}
	if strings.Contains(got,"<Alice>") || strings.Contains(got,"<group>") {t.Fatalf("unsafe unescaped content: %s",got)}
}
