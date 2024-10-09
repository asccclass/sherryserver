package SherryServer
/*
  func)用途：處理靜態網頁部分
*/

import(
   "os"
   "regexp"
   "strings"
   "reflect"
   "net/http"
   "path/filepath"
)

type StaticFileServer struct {
   StaticPath string
   IndexPath  string
}

// Header struct
type Register_Header struct{
   Method string `header:"x-api-key" validate:"len=10"`
   Agent string `header:"User-Agent"`
}

func(h StaticFileServer) FixPrefix(prefix string)(string) {
   prefix = regexp.MustCompile(`/*$`).ReplaceAllString(prefix, "")
   if !strings.HasPrefix(prefix, "/") {
      prefix = "/" + prefix
   }
   if prefix == "/" {
      prefix = ""
   }
   return prefix
}

// 取得Header參數
func(h StaticFileServer) GetHeader(r *http.Request, data interface{}) {
   //ValueOf returns a new Value initialized to the concrete value stored in the interface i.
   //.elem dereference
   val:=reflect.ValueOf(data).Elem()
   //TypeOf returns the reflection Type that represents the dynamic type of interface i.
   //basically it is used to access the metadata of variables inside struct.
   data_type:=reflect.TypeOf(data).Elem()
   header:=r.Header
   //now I am iterating over all the fields in the passed struct
   for i:=0 ;i<val.NumField(); i++ {
      fld:=val.Field(i)
      tag:=data_type.Field(i).Tag.Get("header")
      //for example for the first field the above line will return x-api-key and in second iteration it wil return User-agent.
      header_data,ok:= header[tag]
      if ok{
         fld.SetString(header_data[0])
      }
   }
}

func(h StaticFileServer)  ServeHTTP(w http.ResponseWriter, r *http.Request) {
   path, err := filepath.Abs(r.URL.Path)
   if err != nil {
      http.Error(w, err.Error(), http.StatusBadRequest)  // 400 bad request
      return
   }
   path = filepath.Join(h.StaticPath, path)
   _, err = os.Stat(path)
   if os.IsNotExist(err) {
      http.ServeFile(w, r, filepath.Join(h.StaticPath, h.IndexPath))
      return
   } else if err != nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)  // 500 internal server error
      return
   }
   fs := http.FileServer(http.Dir(h.StaticPath))   // .ServeHTTP(w, r)  // return Handler
	http.Handle("/", http.StripPrefix("/", fs))
}

func(app *StaticFileServer) AddRouter(router *http.ServeMux) {	
   fs := http.FileServer(http.Dir(app.StaticPath))   // .ServeHTTP(w, r)  // return Handler
	router.Handle("/", http.StripPrefix("/", fs))
}
