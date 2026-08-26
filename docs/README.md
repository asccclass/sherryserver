# HTTP Server with tools

此套件可用來快速建立 Go HTTP 伺服器。新版已移除對舊有 grolla 寫法的依賴，改以標準 `net/http` 為主。

## 需要環境
* Go 版本：1.25 以上版本
  - 可搭配使用 [http.CrossOriginProtection](https://pkg.go.dev/net/http#CrossOriginProtection) 進行 CORS / CSRF 相關保護

## 環境建置
### Dockerfile

```
FROM alpine

WORKDIR /app
COPY ./app /app
ENTRYPOINT ["/app/app"]
```

### `envfile` 範例
```
SystemName=APIGateway
PORT=80

DBMSType=MySQL
DBSERVER=MySQLx
DBNAME=todo.db
DBLOGIN=root
DBPASSWORD=oooxxx
DBPath=/app/data/

OriginAllowList=https://www.justdrink.com.tw;http://www.justdrink.com.tw;https://finance.justdrink.com.tw;https://www.jhupat.org.tw
AllowMethods=POST;GET;DELETE;PUT

DocumentRoot=www/html
TemplateRoot=www/template/
TempRoot=www/temp
QRCodePath=www/temp

datafiles=/app/data/
socketiologfile=logs/register.log

ServerURL=https://www.justdrink.com.tw/apigateway/
```

## 使用範例

### `main.go`

```go
package main

import (
   "fmt"
   "os"

   "github.com/joho/godotenv"
   SherryServer "github.com/asccclass/sherryserver"
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
      documentRoot = "www/html"
   }
   templateRoot := os.Getenv("TemplateRoot")
   if templateRoot == "" {
      templateRoot = "www/template"
   }

   srv, err := SherryServer.NewServer(":" + port, documentRoot, templateRoot)
   if err != nil {
      panic(err)
   }
   router := NewRouter(srv, documentRoot)
   if router == nil {
      fmt.Println("router is nil")
      return
   }
   srv.Server.Handler = router // 若要自訂跨來源保護，可在此包裝 handler
   srv.Start()
}
```

### `router.go`

```go
// router.go
package main

import(
   "net/http"

   SherryServer "github.com/asccclass/sherryserver"
)

func NewRouter(srv *SherryServer.Server, documentRoot string)(*http.ServeMux) {
   router := http.NewServeMux()

   // Static assets
   staticfileserver := SherryServer.StaticFileServer{StaticPath: documentRoot}
   staticfileserver.AddRouter(router)

/*
   // API routes
   router.HandleFunc("GET /api/notes", GetAll)
   router.HandleFunc("POST /api/notes", Post)

   // Go template + HTMX pages
   router.HandleFunc("GET /", HomeTemplate)
   router.HandleFunc("GET /partials/page/{page}", PagePartial)
*/	
   return router
}
```

補充：若專案已改為 server-side template + HTMX，`/` 通常會由應用程式路由接手輸出模板頁面；`StaticFileServer` 只負責 CSS、圖片、文件等靜態資源即可。

## 記錄錯誤
```go
app.Srv.Logger.Info("Server stopped")
app.Srv.Logger.Fatal(err.Error(), zap.String("addr", app.Server.Addr))
```

## DB Login Service

```go
// router.go

import(
   DBLoginService "github.com/asccclass/sherryserver/libs/dblogin"
)

loginService, err := DBLoginService.NewDBLoginService(srv)
if err == nil {
   loginService.AddRouter(router)
}
```

## Mail 範例

```go
func main() {
   app, _ := NewSMTPMail()

   // 讀取要內嵌的圖片
   img := "logo_v2.png"
   images, err := app.AddImages(img, []Image{})
   if err != nil {
      fmt.Println(err.Error())
      return
   }

   subject := "【活動提醒及入場 QR Code】114 年度國中會考趨勢分析與複習策略專題講座入場通知"
   body := fmt.Sprintf(`
      <!DOCTYPE html>
      <html>
         <body>
         <p>%s，您好<br /><br />感謝您報名參加 114 年度國中會考趨勢分析與複習策略專題講座。以下為本次活動相關訊息供您參考。<br /><br />活動時間：2 月 22 日（星期六）13:00～17:50<br />活動地點：中正國中活動中心（臺北市中正區愛國東路 158 號）<br /><br />注意事項：<br />
         1. 會議室內禁止錄影音與飲食。<br />
         2. 報到時請出示下方 QR Code，供主辦方確認後入場。<br />
         3. 附近較難停車，請提早 10 分鐘到場並盡量使用大眾交通工具。<br /><br />
         下方為您專屬的報到 QR Code：</p>
         <img src="cid:image-0" alt="內嵌圖片">
         <p>請您於規定時間內出席。<br /><br />
         敬祝 平安順心</p>
         <p>臺北市國中學生家長會聯合會敬上</p>
         </body>
      </html>
   `, "Jii 哥")

   boundary := "boundary_" + fmt.Sprintf("%d", os.Getpid()) // 動態生成 boundary

   // 完整的 MIME 訊息
   var msg bytes.Buffer
   msg.WriteString(fmt.Sprintf("To: %s\r\n", app.From))
   msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
   msg.WriteString("MIME-Version: 1.0\r\n")
   msg.WriteString(fmt.Sprintf("Content-Type: multipart/related; boundary=%s\r\n", boundary))
   msg.WriteString("\r\n")

   // HTML 部分
   msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
   msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
   msg.WriteString("\r\n")
   msg.WriteString(body)
   msg.WriteString("\r\n")

   // 圖片部分：動態添加每張圖片
   for _, img := range images {
      msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
      msg.WriteString(fmt.Sprintf("Content-Type: %s\r\n", img.ContentType))
      msg.WriteString("Content-Transfer-Encoding: base64\r\n")
      msg.WriteString(fmt.Sprintf("Content-ID: <%s>\r\n", img.ContentID))
      msg.WriteString("\r\n")
      msg.WriteString(img.Data)
      msg.WriteString("\r\n")
   }

   // MIME 結束
   msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
   // SMTP 認證
   if err := app.Send(app.From, msg, true); err != nil {
      fmt.Println("send error:", err.Error())
      return
   }
   fmt.Println("send success")
}
```

### 內建功能
* [websocket](websocket.md)

## 公用程式
### `clean.sh` 清除無用的 Docker container

```sh
#!/bin/sh

docker rmi $(docker images | grep "none" | awk '{print $3}')
clear
docker ps -a
```

## 參考資料
* https://github.com/EsotericTech/chatapp/tree/main
* https://www.alexedwards.net/blog/preventing-csrf-in-go#summary
