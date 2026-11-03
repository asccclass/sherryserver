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

# Cookie session signing key. Generate your own value; do not reuse examples.
# Example: openssl rand -base64 32
SessionCookieAuthKey=replace-with-at-least-32-random-bytes
# Optional cookie encryption key. Must be exactly 16, 24, or 32 bytes if set.
# SessionCookieEncryptionKey=replace-with-32-random-bytes

DocumentRoot=www/html
TemplateRoot=www/template/
TempRoot=www/temp
QRCodePath=www/temp

datafiles=/app/data/
socketiologfile=logs/register.log

ServerURL=https://www.justdrink.com.tw/apigateway/
```

## 使用範例

### HTTPS 與自動 TLS 憑證

預設關閉自動 TLS；`NewServer()`／`Start()` 的呼叫方式不變。設定在 `NewServer()` 載入 `envfile` 後讀取，系統環境變數優先於檔案；修改設定須重新啟動服務。

| 模式 | 設定 |
| --- | --- |
| HTTP | `TLSAutoEnabled=false`，不提供完整固定憑證路徑 |
| 固定憑證 HTTPS | `TLSAutoEnabled=false`，提供 `sslCertification` 與 `sslKeyfile` |
| 自動取得與續期 HTTPS | `TLSAutoEnabled=true`，填妥以下自動 TLS 設定，移除兩個固定憑證變數 |

固定憑證範例：

```dotenv
TLSAutoEnabled=false
sslCertification=/app/certs/fullchain.pem
sslKeyfile=/app/certs/privkey.pem
```

自動 TLS 範例（以自己的網域取代範例值）：

```dotenv
PORT=443
TLSAutoEnabled=true
TLSAutoDomains=example.com,www.example.com
TLSAutoCacheDir=/app/data/autocert
TLSAutoEmail=admin@example.com
# 閱讀並同意所選 ACME CA 的服務條款後設為 true
TLSAutoAcceptTOS=true
# 首次部署使用 staging，驗收後移除此行切換正式 CA
TLSAutoDirectoryURL=https://acme-staging-v02.api.letsencrypt.org/directory
```

程式透過 `autocert.Manager` 取得、快取及續期憑證，更新後供後續 TLS 握手使用，不需因續期重啟。只接受 `TLSAutoDomains` 列出的 SNI，支援大小寫、尾端句點與 IDN 正規化；不支援萬用字元憑證。`TLSAutoEmail` 選填，網域、快取位置與條款同意必填。自動模式與任一固定憑證路徑同時設定會拒絕啟動；設定錯誤不會降級至 HTTP。

第一版使用 TLS-ALPN-01，須讓公開網域 DNS 導向本服務，並將外部 TCP 443 的 TLS 握手直接送至程式。內部 listen port 可以透過映射調整，套件不會改寫傳入 `NewServer()` 的 listen address。無須額外開啟 port 80，也不提供 HTTP 自動轉址。若代理已終止 TLS，請關閉本功能，由代理管理憑證；L4 TCP passthrough 須保留 SNI 與 ACME ALPN。

Docker 部署可使用既有 Dockerfile，將內部服務設定為 `PORT=8443`：

```bash
docker volume create sherry-autocert
docker run --name sherryserver -p 443:8443 --env-file ./envfile \
  --mount type=bind,source="$(pwd)/envfile",target=/app/envfile,readonly \
  --mount type=volume,source=sherry-autocert,target=/app/data \
  your-sherryserver-image
```

快取目錄包含私鑰及 ACME 帳號，須持久化並限制服務帳號存取。Unix 新目錄以 `0700` 建立；既有快取目錄若允許群組或其他使用者存取，初始化會回報錯誤，請由管理者調整權限。Windows 由部署者設定目錄 ACL，只授權服務帳號與必要管理者，程式只檢查讀寫，不自動更改 ACL。容器以非 root 帳號執行時，也須預先設定掛載目錄的擁有者與權限。

快取依 ACME directory URL 的 SHA-256 分目錄保存，staging 與正式 CA 不會共用憑證或帳號。切換正式 CA 後須完成首次正式簽發。啟動日誌的 `Server listener started` 只表示 socket 已開啟；首次握手可能因尚未完成簽發而延遲或失敗，後續重試依 `autocert` 機制執行。續期暫時失敗時仍使用尚有效的憑證，過期後客戶端可能拒絕連線。

呼叫端可在 `Start()` 前設定 `srv.Server.TLSConfig`：版本限制、cipher suites、session tickets、ALPN 及其他不影響憑證管理的欄位會複製保留，並加入 HTTP/2、HTTP/1.1、ACME 所需 ALPN。預設最低 TLS 1.2；明確指定過低或矛盾的版本範圍會拒絕啟動。自行提供憑證、`GetCertificate`、`GetConfigForClient`、`NameToCertificate` 或非預設 `ClientAuth` 會與自動模式衝突。自訂驗證回呼仍由呼叫端負責，須允許 CA 的驗證握手；啟動後不可修改 TLS 設定。

回復固定憑證模式時，設定 `TLSAutoEnabled=false`，填入有效的 `sslCertification`／`sslKeyfile` 並重啟。只有其中一個檔案路徑時，依舊有行為使用 HTTP，因此切換前須確認兩者皆完整。保留自動 TLS 快取供日後重新啟用。

完整設計、測試對照與 staging 驗收步驟見 [TLS 功能與驗收紀錄](tsl.md)。

### 完整範例：YouTube 直播留言分句轉送

已新增可執行範例於：

- `examples/youtube-live-chat/`

範例內容：
- Web UI 輸入 `videoId` / `apiKey` 開始與停止抓取留言
- 輪詢 YouTube Data API v3（`liveBroadcasts.list` / `liveChatMessages.list`）
- 留言分句後透過 WebSocket 傳送到外部 server
- UI 即時顯示 sent/status/error/external 事件

快速啟動：

```bash
cd /home/runner/work/sherryserver/sherryserver/examples/youtube-live-chat
cp .env.example envfile
# 編輯 envfile：至少填入 SessionCookieAuthKey、YOUTUBE_API_KEY、EXTERNAL_WS_URL
go run .
```

更多設定、測試與 mock external websocket 手動驗證流程，請見：

- `examples/youtube-live-chat/README.md`

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
