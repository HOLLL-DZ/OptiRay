package main
import "testing"
func TestVlessJSONConversion(t *testing.T){
 raw:=`{"outbounds":[{"protocol":"vless","tag":"Example","settings":{"vnext":[{"address":"example.com","port":443,"users":[{"id":"test-id","flow":"xtls-rprx-vision","encryption":"none"}]}]},"streamSettings":{"network":"tcp","security":"reality","realitySettings":{"publicKey":"public","shortId":"abcd","serverName":"example.com","fingerprint":"firefox"}}}]}`
 links,err:=vlessJSONLinks(raw);if err!=nil{t.Fatal(err)};if len(links)!=1{t.Fatal("wrong server count")}
 server,err:=parseLink(links[0]);if err!=nil{t.Fatal(err)}
 if str(server["name"])!="Example" || str(obj(obj(server["outbound"])["streamSettings"])["security"])!="reality"{t.Fatal("lost metadata")}
 d:=Object{"subscriptions":[]any{Object{"id":1,"url":raw}}};if err:=collectStandaloneConfigs(d);err!=nil{t.Fatal(err)}
 if !boolean(obj(list(d["subscriptions"])[0])["standalone_group"]){t.Fatal("JSON not grouped")}
}