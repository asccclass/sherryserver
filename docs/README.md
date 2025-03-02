# HTTP Server with tools

主要用來作為伺服器使用。移除 grolla 函數，改用內建的 net/http 函數，
並重構相關函數。

## 使用範例

* server.go

```
package main

import (
   "os"
   "fmt"
   "github.com/joho/godotenv"
   "github.com/asccclass/sherryserver"
)

func main() {
   if err := godotenv.Load("envfile"); err != nil {
	   fmt.Println(err.Error())
	   return
   }
   port := os.Getenv("PORT")
   if port == "" {
      port = "80"
   }
   documentRoot := os.Getenv("DocumentRoot")
   if documentRoot == "" {
      documentRoot = "www"
   }
   templateRoot := os.Getenv("TemplateRoot")
   if templateRoot == "" {
      templateRoot = "www/html"
   }

   server, err := SherryServer.NewServer(":" + port, documentRoot, templateRoot)
   if err != nil {
      panic(err)
   }
   router := NewRouter(server, documentRoot)
   if router == nil {
      fmt.Println("router return nil")
      return
   }
   server.Server.Handler = router  // server.CheckCROS(router)  // 需要自行implement, overwrite 預設的
   server.Start()
}
```

* router.go

```
// router.go
package main

import(
   "fmt"
   "net/http"
   "github.com/asccclass/sherryserver"
)

func NewRouter(srv *SherryServer.Server, documentRoot string)(*http.ServeMux) {
   router := http.NewServeMux()

   // Static File server
   staticfileserver := SherryServer.StaticFileServer{documentRoot, "index.html"}
   staticfileserver.AddRouter(router)

/*
   // App router
   router.HandleFunc("GET /api/notes", GetAll)
   router.HandleFunc("POST /api/notes", Post)

   router.Handle("/homepage", oauth.Protect(http.HandlerFunc(Home)))
   router.Handle("/upload", oauth.Protect(http.HandlerFunc(Upload)))
*/	
   return router
}
```

## 輸出錯誤方式
```
app.Srv.Logger.Info("Server stopped")
app.Srv.Logger.Fatal(err.Error(), zap.String("addr", app.Server.Addr))
```

### 內建函數
* [websocket](websocket.md)

### 參考資料
* https://github.com/EsotericTech/chatapp/tree/main
