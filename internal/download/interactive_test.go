package download

import (
 "encoding/json"
 "strings"
 "testing"
)
func TestInteractiveDownloadFormats(t *testing.T){
 quiz:=`<html><div data-app-data="{&quot;quiz&quot;:[{&quot;question&quot;:&quot;Q?&quot;,&quot;answerOptions&quot;:[{&quot;text&quot;:&quot;Yes&quot;,&quot;isCorrect&quot;:true}]}]}"></div></html>`
 data,err:=FormatInteractive(quiz,"Title","quiz","json");if err!=nil{t.Fatal(err)};var decoded map[string]any;if err:=json.Unmarshal(data,&decoded);err!=nil{t.Fatal(err)};if len(decoded["questions"].([]any))!=1{t.Fatal("lost questions")}
 md,err:=FormatInteractive(quiz,"Title","quiz","markdown");if err!=nil||!strings.Contains(string(md),"- [x] Yes"){t.Fatalf("%s %v",md,err)}
 cards:=`<script type="application/json">{"flashcards":[{"f":"Front","b":"Back"}]}</script>`;data,err=FormatInteractive(cards,"Cards","flashcards","json");if err!=nil||!strings.Contains(string(data),`"front": "Front"`){t.Fatalf("%s %v",data,err)}
 if _,err:=FormatInteractive("<html>No data</html>","","quiz","json");err==nil{t.Fatal("missing data reported success")}
}
