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
- `EXTERNAL_WS_URL`：外部 WebSocket server URL（可在 UI 覆蓋）
- `EXTERNAL_WS_ALLOWED_HOSTS`：可選，允許的外部 WebSocket host 清單（逗號分隔）；未設定時預設只允許 `EXTERNAL_WS_URL` 的 host
- `SessionCookieAuthKey`：sherryserver 必要設定（至少 32 bytes）
- `SessionCookieEncryptionKey`：可選（16/24/32 bytes）

---

## 4) UI 使用方式

1. 開啟首頁。
2. 輸入 `videoId`、`apiKey`、`externalWsUrl`（若 envfile 已設定可只填 videoId）。
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

## 7) 限制與注意事項

- YouTube API 有 quota 限制，輪詢頻率依 `pollingIntervalMillis`。
- 本範例假設外部 WebSocket server 接收文字 JSON frame。
- 若外部 WS 連線失敗或讀寫錯誤，工作會停止並回報錯誤事件。
