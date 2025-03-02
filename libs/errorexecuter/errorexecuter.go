package SherryErrorExecuter

import(
   "os"
   "fmt"
   "time"
   "bytes"
   // "io/ioutil"
   "net/http"
   "encoding/json"
   "crypto/tls"
   "mime/multipart"
)

type ErrorExecuter struct {
   LineNotifyURL	string
   LineNotifyToken	string
   ActionScriptURL	string
}

// 輸出訊息to Web
func(ee *ErrorExecuter) Message2Web(w http.ResponseWriter, title string, err error) {
   w.Header().Set("Content-Type", "application/json;charset=UTF-8")
   w.Header().Set("Access-Control-Allow-Origin", "*")
   w.WriteHeader(http.StatusOK)
   fmt.Fprintf(w, "{\"" + title + "\": \"%s\"}", err.Error())
}

// 輸出錯誤訊息to Web
func(ee *ErrorExecuter) Error2Web(w http.ResponseWriter, err error) {
   ee.Message2Web(w, "errMsg", err)
}

// 送出訊息給SSE Servive
func(ee *ErrorExecuter) Error2SSE(url, msg string) {
   if msg == "" {
      return
   }
   if url == "" {
      url := os.Getenv("SSEServerURL")
      if url == "" {
         return
      }
   }
   payload := bytes.NewBuffer([]byte(msg))
   timeout := time.Duration(10 * time.Second) //超时时间50ms
   client := &http.Client{
      Timeout: timeout,
      Transport: &http.Transport{
         TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
      },
   }
   req, err := http.NewRequest("POST", url, payload)
   if err != nil {
      return
   }
   req.Header.Set("Content-Type", "application/json")
   _, _ = client.Do(req)
}

// 透過GoogleAS 輸出錯誤訊息給LINE。可避開Container內無SSL憑證問題
func(ee *ErrorExecuter) Error2AS(systemName string, err error) {
   if ee.ActionScriptURL == "" {
      return
   }
   jsonData := map[string]string{"lineNotifyToken": ee.LineNotifyToken, "systemName": systemName, "status": err.Error()}
   jsonValue, err := json.Marshal(jsonData)
   if err != nil {
      fmt.Println(err.Error())
      return
   }
   payload := bytes.NewBuffer(jsonValue)
   timeout := time.Duration(10 * time.Second) //超时时间50ms
   client := &http.Client{
      Timeout: timeout,
      Transport: &http.Transport{
         TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
      },
   }
   req, err := http.NewRequest("POST", ee.ActionScriptURL, payload)
   if err != nil {
      fmt.Println("NewRequest:" + err.Error())
      return
   }
   req.Header.Add("Authorization", "Bearer " + ee.LineNotifyToken)
   req.Header.Set("Content-Type", "application/json")
   _, err = client.Do(req)
   if err != nil {
      fmt.Println(ee.ActionScriptURL + " " +err.Error())
   }
}

// 輸出錯誤訊息 to Line
func(ee *ErrorExecuter) Error2Line(systemName string, err error) {
   payload := &bytes.Buffer{}
   writer := multipart.NewWriter(payload)
   _ = writer.WriteField("message", systemName + "\n" + err.Error())
   _ = writer.Close()

   timeout := time.Duration(10 * time.Second) //超时时间50ms
   client := &http.Client{
      Timeout: timeout,
      Transport: &http.Transport{
         TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
      },
   }
   req, err := http.NewRequest("POST", ee.LineNotifyURL, payload)
   if err != nil {
      return   
   }
   req.Header.Add("Authorization", "Bearer " + ee.LineNotifyToken) 
   req.Header.Set("Content-Type", "application/json")
   req.Header.Set("Content-Type", writer.FormDataContentType())
   _, err = client.Do(req)

   if err != nil {
      fmt.Println(err.Error())
   }
/*
   defer res.Body.Close()
   body, err := ioutil.ReadAll(res.Body)
   if err != nil {
      fmt.Println(err.Error())
   }
   fmt.Println(string(body))
*/
}

func NewErrorExecuter()(*ErrorExecuter, error) {
   var lineNotifyToken, lineNotifyURL string
   lineNotifyToken = os.Getenv("ErrorLineNotifyToken")
   lineNotifyURL = os.Getenv("ErrorLineNotifyURL")
   if lineNotifyToken == "" {
      lineNotifyToken = "fD4Dj6x2Ujjjt8IPWwsXtlf5GAKbaNkYuicxS13e4lL"
   }
   if lineNotifyURL == "" {
      lineNotifyURL = "https://notify-api.line.me/api/notify"
      // return nil, fmt.Errorf("No ErrorLineNotifyToken or ErrorLineNotifyURL")
   }

   return &ErrorExecuter {
      LineNotifyURL: lineNotifyURL,
      LineNotifyToken: lineNotifyToken,
      ActionScriptURL: os.Getenv("ActionScriptURL"),
   }, nil
}

/*
func main() {
   os.Setenv("ActionScriptURL", "https://script.google.com/macros/s/AKfycbygFuH_hX2kqYl1NWKWB0CzbWLPgjwkmCSyZG7AY6kjp5YH0Plu/exec")
   x, _ := NewErrorExecuter()
   x.Error2AS("測試系統", fmt.Errorf("錯誤訊息測試"))
}
*/
