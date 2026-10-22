# YouTube 直播留言分句轉送範例（sherryserver）

此範例示範如何使用 `github.com/asccclass/sherryserver` 建立 Web server，做到：

1. 在 Web UI 輸入 YouTube 直播 `videoId`，開始 / 停止抓取留言。
2. 使用 YouTube Data API v3（`liveBroadcasts.list` + `liveChatMessages.list`）輪詢直播聊天室。
3. 將留言依中英文句末標點分句後，以 JSON 透過 WebSocket 送到外部 server。
4. 前端即時接收後端事件（sent/status/error）與外部 server 回傳內容（external）。

---

## 1) 建立 YouTube Data API v3 API key（Google Cloud）

1. 到 Google Cloud Console 建立或選擇專案。
2. 在「API 和服務 > 程式庫」啟用 **YouTube Data API v3**。
3. 在「API 和服務 > 憑證」建立 **API key**。
4. 建議在 API key 加上限制：
   - API 限制：只允許 YouTube Data API v3
   - 應用程式限制：依你的部署環境設定（IP / HTTP referrer 等）

> ⚠️ **請勿把 API key 提交到 Git**。請只放在本機 `envfile` 或環境變數。

---

## 2) 啟動方式

```bash
cd /home/runner/work/sherryserver/sherryserver/examples/youtube-live-chat
cp .env.example envfile
# 編輯 envfile，至少填入 SessionCookieAuthKey、YOUTUBE_API_KEY、EXTERNAL_WS_URL

go run .
```

預設會在 `http://127.0.0.1:8080` 啟動。

---

## 3) 環境變數

- `PORT`：服務埠（預設 8080）
- `YOUTUBE_API_KEY`：YouTube Data API v3 金鑰
- `YOUTUBE_VIDEO_ID`：預設直播影片 ID（可在 UI 覆蓋）
- `EXTERNAL_WS_URL`：外部 WebSocket server URL（由後端環境變數提供）
- `EXTERNAL_WS_ALLOWED_HOSTS`：可選，允許的外部 WebSocket host 清單（逗號分隔）；未設定時預設只允許 `EXTERNAL_WS_URL` 的 host
- `SessionCookieAuthKey`：sherryserver 必要設定（至少 32 bytes）
- `SessionCookieEncryptionKey`：可選（16/24/32 bytes）

---

## 4) UI 使用方式

1. 開啟首頁。
2. 輸入 `videoId`、`apiKey`（若 envfile 已設定可只填 videoId）。
3. 按「開始」啟動工作；按「停止」中止。
4. 下方事件區會看到：
   - `status`：狀態事件
   - `sent`：本地已送出到外部 WS 的分句 JSON（即使外部沒有回資料也可看到）
   - `external`：外部 WS server 回傳資料（若是 JSON 會直接顯示 JSON）
   - `error`：錯誤事件

---

## 5) 對外 WebSocket 訊息格式

每句會送出一筆 JSON：

```json
{
  "text": "今天直播開始了",
  "author": "userA",
  "videoId": "abc123",
  "messageId": "LCC.xxx",
  "publishedAt": "2026-10-22T00:00:00Z"
}
```

---

## 6) 測試（不需 YouTube 憑證）

此範例的單元測試不會呼叫真實 YouTube API，也不會連外部 WebSocket。

```bash
cd /home/runner/work/sherryserver/sherryserver
go test ./examples/youtube-live-chat/...
```

---

## 7) 本地 mock external websocket server（手動驗證流程指令）

可用以下指令先在本機啟一個簡單的外部 WebSocket server（收到訊息後回傳 JSON）：

```bash
cat > /tmp/mock-external-ws.go <<'EOF'
package main

import (
  "context"
  "encoding/json"
  "log"
  "net/http"
  "time"

  "github.com/coder/websocket"
)

func main() {
  http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
    c, err := websocket.Accept(w, r, nil)
    if err != nil {
      log.Println("accept error:", err)
      return
    }
    defer c.CloseNow()

    ctx := c.CloseRead(r.Context())
    for {
      _, data, err := c.Read(ctx)
      if err != nil {
        log.Println("read end:", err)
        return
      }
      log.Printf("recv: %s\n", string(data))

      resp, _ := json.Marshal(map[string]string{
        "type": "ack",
        "echo": string(data),
        "at":   time.Now().UTC().Format(time.RFC3339),
      })
      if err := c.Write(context.Background(), websocket.MessageText, resp); err != nil {
        log.Println("write error:", err)
        return
      }
    }
  })

  log.Println("mock external ws server on :9000/ws")
  log.Fatal(http.ListenAndServe(":9000", nil))
}
EOF

cd /home/runner/work/sherryserver/sherryserver/examples/youtube-live-chat
go run /tmp/mock-external-ws.go
```

再開另一個終端機啟動本範例：

```bash
cd /home/runner/work/sherryserver/sherryserver/examples/youtube-live-chat
cp .env.example envfile

# 確認至少以下設定（其餘可沿用預設）
# SessionCookieAuthKey=至少32 bytes
# EXTERNAL_WS_URL=ws://127.0.0.1:9000/ws
# EXTERNAL_WS_ALLOWED_HOSTS=127.0.0.1

go run .
```

最後在瀏覽器開 `http://127.0.0.1:8080`，輸入 `videoId` + `apiKey` 後按「開始」：
- mock server 終端機會看到 `recv: ...`
- UI 事件區會看到 `sent`（本地送出）與 `external`（mock ack 回傳）

---

## 8) 限制與注意事項

- YouTube API 有 quota 限制，輪詢頻率依 `pollingIntervalMillis`。
- 本範例假設外部 WebSocket server 接收文字 JSON frame。
- 若外部 WS 連線失敗或讀寫錯誤，工作會停止並回報錯誤事件。
