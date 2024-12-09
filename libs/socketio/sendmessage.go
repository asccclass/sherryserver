package SherrySocketIO

import(
   "fmt"
   "net/http"
   "io/ioutil" 
   "encoding/json"
   "github.com/asccclass/sherrytime"
)

type JsonMsg struct {
   From		string		`json:"from"`
   To		string		`json:"to"`
   Channel	string		`json:"channel"`
   Action	string		`json:"action"`
   Message	string		`json:"msg"`
   TimeStamp	string		`json:"timestamp"`
}

// 輸出訊息to Web
func(io *SrySocketio) Message2Web(w http.ResponseWriter, title string, err error) {
   w.Header().Set("Content-Type", "application/json;charset=UTF-8")
   w.WriteHeader(http.StatusOK)
   fmt.Fprintf(w, "{\"" + title + "\": \"%s\"}", err.Error())
}

// 處理 /sendmessage 
func(io *SrySocketio) SendMessageInString(w http.ResponseWriter, r *http.Request) {
   b, err := ioutil.ReadAll(r.Body)
   defer r.Body.Close()
   if err != nil {
      io.Message2Web(w, "response", err)
      return
   }
   io.Hub.Broadcast <-b  // b)[]byte
}

// 處理 /sendmessage 
func(io *SrySocketio) SendMessageInJson(w http.ResponseWriter, r *http.Request) {
   b, err := ioutil.ReadAll(r.Body)
   defer r.Body.Close()
   if err != nil {
      io.Message2Web(w, "response", err)
      return
   }
   var jdata JsonMsg
   if err := json.Unmarshal(b, &jdata); err != nil {
      io.Message2Web(w, "response", err)
      return
   }
   st := sherrytime.NewSherryTime("Asia/Taipei", "-")  // Initial
   jdata.TimeStamp = st.Now()

   if jdata.To != "" {
      io.ToSpecificMessage(jdata)
   }  else {
      s, err := json.Marshal(jdata)
      if err != nil {
         io.Message2Web(w, "response", err)
         return
      }
      io.BroadCastMessageWs(string(s))
   }
}
