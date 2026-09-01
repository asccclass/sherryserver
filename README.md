# SherryServer Gemini Live Proxy

`sherryserver` 上的 Gemini Live WebSocket 代理範例。這個版本已從原本的瀏覽器直連 Gemini，改成由 Go 後端持有 Gemini Live 連線，前端靜態頁面統一放在 `www/html`，瀏覽器只連本地的 `/ws/live`。

目前流程是：

1. 使用者說話給 Gemini Live
2. Gemini Live 只負責語音辨識與語音輸出
3. 後端收到辨識結果後，轉交給自訂 LLM
4. 後端先讓 Gemini Live 口播一段「處理中」
5. 自訂 LLM 完成後，後端再把正式答案交給 Gemini Live 發聲

## 專案重點

- Go 後端代理 Gemini Live WebSocket
- Gemini API Key 只留在 server 端
- 預設使用 Gemini Live `v1beta`
- 支援 OpenAI-compatible 自訂 LLM API
- 精簡前端，只保留:
  - session 設定
  - 獨立的 status / user transcript / assistant transcript 顯示區
  - 文字對話
  - 麥克風音訊串流
  - 模型音訊回放

## 結構

- [cmd/gemini-live-server/main.go](/D:/myprograms/myavatar/sherryserver/cmd/gemini-live-server/main.go)
  - 啟動 `sherryserver`
  - 提供 `GET /ws/live`
  - 將前端訊息轉接到 `google.golang.org/genai`
  - 串接自訂 LLM orchestration
- [www/html/index.html](/D:/myprograms/myavatar/sherryserver/www/html/index.html)
  - 精簡版單頁介面
- [www/html/script.js](/D:/myprograms/myavatar/sherryserver/www/html/script.js)
  - WebSocket client
  - 文字送出
  - 麥克風 PCM 串流
  - 音訊播放
- [www/html/avatar.js](/D:/myprograms/myavatar/sherryserver/www/html/avatar.js)
  - VRM 載入與嘴型同步
- [envfile](/D:/myprograms/myavatar/sherryserver/envfile)
  - 本地執行所需環境變數

## 環境設定

請先編輯 `envfile`:

```env
SystemName=GeminiLiveSherryServer
OriginAllowList=http://127.0.0.1:8080;http://localhost:8080
AllowMethods=GET;POST;OPTIONS
GEMINI_API_KEY=your_real_api_key
LISTEN_ADDR=:8080
GEMINI_API_VERSION=v1beta
DOCUMENT_ROOT=www/html
WEB_INDEX_PATH=index.html
CUSTOM_LLM_API_URL=http://localhost:11434/v1/chat/completions
CUSTOM_LLM_MODEL=your-model
CUSTOM_LLM_API_KEY=
CUSTOM_LLM_PROCESSING_MESSAGE=請稍候，我正在處理您的需求。
CUSTOM_LLM_ERROR_MESSAGE=抱歉，我剛剛處理失敗了，請再說一次。
```

如果你使用目前 `envfile` 內現成的 Ollama/OpenAI-compatible 服務，也可以只設定：

```env
OLLAMA_URL=http://10.109.190.13:8003/v1
OLLAMA_MODEL=gemma-4-31b-it-fp8
```

後端會自動補成 `/chat/completions` 端點。

## 啟動

```bash
go run ./cmd/gemini-live-server
```

啟動後開啟:

```text
http://localhost:8080
```

## 前後端協定

前端送到 `/ws/live` 的訊息格式:

```json
{ "type": "setup", "setup": { "model": "gemini-2.5-flash-native-audio-preview-12-2025", "systemInstruction": "...", "voice": "Puck", "temperature": 0.8, "enableInputTranscription": true, "enableOutputTranscription": true } }
{ "type": "text", "text": "Hello" }
{ "type": "audio_chunk", "audio": "BASE64_PCM16_16KHZ" }
{ "type": "audio_end" }
```

`systemInstruction` 在這一版代表「給自訂 LLM 的提示」，不是給 Gemini Live 的 system prompt。

後端回傳:

```json
{ "type": "ready", "message": { "model": "...", "voice": "...", "version": "v1beta" } }
{ "type": "status", "message": "Custom LLM response ready." }
{ "type": "live", "message": { "...raw Gemini Live server message..." } }
{ "type": "error", "error": "..." }
```

## 開發註記

- 這版已移除舊的 Gemini 直連前端與 audio worklet demo 檔案
- 瀏覽器不再需要 ephemeral token
- 若之後要擴充:
  - video frame streaming
  - tool calling UI
  - session resumption
  都可以在目前的 proxy 架構上繼續加
