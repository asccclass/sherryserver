package SherryPages

import(
   "os"
   "fmt"
   "embed"
   "net/http"
   "html/template"
   "github.com/asccclass/sherryserver"
   "github.com/asccclass/sherryserver/libs/pages"
)

type Page struct {
   Srv          *SherryServer.Server
   Path         string    // 樣板路徑
   files        embed.FS          // 樣板內容
   funcs        template.FuncMap  // 樣板函數
}

// Web Router
func(app *Page) AddRouter(router *http.ServeMux) {
   router.HandleFunc("/www/{pageName}", app.LoadPageFromWeb)			// 讀取網頁
}

// Initial
func PageService(srv *SherryServer.Server)(*Page, error) {
   path := os.Getenv("TemplateRoot")
   if path == "" {
      return nil, fmt.Errorf("envfile's params TemplateRoot not set.")
   }

   fin := &Page {
      Srv: srv,
      Path: path,
   }

   fin.funcs = template.FuncMap{
      "toFloat64": fin.toFloat64,
      "sub": fin.minus,
      "calPercentage": fin.calPercentage,
      "toString": fin.toString,
      "toInt": fin.toInt,
      "strSum": fin.strSum,
      "multiply": fin.multiply,
      "getFieldValue": fin.getFieldValue,
      "getArrayValueByMonth": fin.getArrayValueByMonth,
      "getArrayValueByIndex": fin.getArrayValueByIndex ,
   }

   srv.Logger.Info("Page service created.")
   return fin, nil
}
