# Avatar Query Routing

這份文件用來統一記錄每個 Avatar 的角色名稱、模型檔、查詢來源與特殊規則，方便後續新增角色時集中管理。

## Avatar 統一設定表

| Avatar ID | 角色名稱 | 狀態 | 模型檔 | 查詢來源 | 連線位址 / 端點 | 特殊規則 |
| --- | --- | --- | --- | --- | --- | --- |
| `friday` | Friday | 預設 | `sherryserver/www/html/avatar/friday.vrm` | MCP | `http://localhost:8000/mcp/sse` | 收到使用者問題後，直接透過 MCP 查詢；明確使用工具 `query_brain_knowledge`，參數欄位為 `question` |

## 欄位說明

- `Avatar ID`：程式內部或設定檔中辨識 Avatar 的固定代號。
- `角色名稱`：對外顯示的人設名稱，可使用中文或英文。
- `狀態`：例如 `預設`、`啟用中`、`停用中`、`測試中`。
- `模型檔`：對應的 VRM 檔案路徑。
- `查詢來源`：例如 `MCP`、`OpenAI-compatible API`、`資料庫`、`混合流程`。
- `連線位址 / 端點`：實際查詢服務位置。
- `特殊規則`：記錄路由條件、前處理、fallback 規則、角色限制等。

## 文件與程式設定對照表

這一節用來對照「文件中的欄位名稱」與「程式實際使用的設定位置」，方便後續把多 Avatar 設定正式落到程式裡。

| 文件欄位 | 目前程式對應 | 型別 / 來源 | 目前狀態 | 說明 |
| --- | --- | --- | --- | --- |
| `Avatar ID` | `LIVETALKING_AVATAR_ID` / `config.LiveTalkingConfig.AvatarID` | env / Go struct | 已存在，但目前未用於 `friday.vrm` 載入 | 目前比較接近 LiveTalking 服務端 Avatar 代號，不是前端 VRM 檔選擇機制 |
| `角色名稱` | `sherryserver/avatars.json` 的 `displayName` | JSON 設定 | 已存在並已程式化 | 首頁標題會跟著目前選定 Avatar 的 `displayName` 同步更新 |
| `狀態` | 無直接欄位 | 文件欄位 | 尚未程式化 | 目前只有文件管理，程式沒有 `default/enabled/disabled` 狀態欄位 |
| `模型檔` | `sherryserver/www/html/avatar.js` 搭配 `sherryserver/avatars.json` 的 `avatarModel` | 前端 JS + JSON 設定 | 已存在 | 預設先載入 `/avatar/friday.vrm`，切換 Avatar 後會改用對應 `avatarModel` |
| `查詢來源` | `config.MCP`, `config.LLM`, `config.LiveTalking` | Go struct | 部分存在 | 查詢管道已存在，但還沒有依 Avatar 做路由切換 |
| `連線位址 / 端點` | `MCP_SSE_URL`, `CUSTOM_LLM_API_URL`, `OLLAMA_URL`, `LIVETALKING_BASE_URL` | env | 已存在 | 不同查詢來源已各自有 env 設定 |
| `特殊規則` | `setup.systemInstruction`, `CUSTOM_LLM_SYSTEM_PROMPT`, 後端 orchestrator 流程 | 前端 JSON / env / Go 流程 | 部分存在 | 可描述角色提示詞、特定問題改走不同來源、fallback 規則等 |

## 目前可直接對照的程式欄位

### 1. Env 與 Go Config

| 用途 | Env 欄位 | Go struct 欄位 |
| --- | --- | --- |
| MCP SSE 端點 | `MCP_SSE_URL` | `config.Config.MCP.SSEURL` |
| 自訂 LLM API | `CUSTOM_LLM_API_URL` | `config.Config.LLM.APIURL` |
| 自訂 LLM Model | `CUSTOM_LLM_MODEL` | `config.Config.LLM.Model` |
| 自訂 LLM System Prompt | `CUSTOM_LLM_SYSTEM_PROMPT` | `config.Config.LLM.SystemPrompt` |
| 處理中訊息 | `CUSTOM_LLM_PROCESSING_MESSAGE` | `config.Config.LLM.ProcessingMessage` |
| 錯誤訊息 | `CUSTOM_LLM_ERROR_MESSAGE` | `config.Config.LLM.ErrorMessage` |
| LiveTalking Base URL | `LIVETALKING_BASE_URL` | `config.Config.LiveTalking.BaseURL` |
| LiveTalking Avatar ID | `LIVETALKING_AVATAR_ID` | `config.Config.LiveTalking.AvatarID` |

### 2. 前端 WebSocket Setup JSON

前端目前送到 `/ws/live` 的 `setup` JSON 欄位如下，可視為未來 Avatar 行為設定的第一層入口：

| JSON 欄位 | 目前來源 | 說明 |
| --- | --- | --- |
| `setup.model` | `www/html/index.html` 表單欄位 | Gemini Live model |
| `setup.systemInstruction` | `www/html/index.html` 表單欄位 | 傳給後端作為自訂 LLM prompt 的一部分 |
| `setup.voice` | `www/html/index.html` 表單欄位 | 語音輸出 voice |
| `setup.temperature` | `www/html/index.html` 表單欄位 | 生成溫度 |
| `setup.enableInputTranscription` | `www/html/index.html` 表單欄位 | 是否啟用輸入轉寫 |
| `setup.enableOutputTranscription` | `www/html/index.html` 表單欄位 | 是否啟用輸出轉寫 |

### 3. 目前 `friday` 的實際落點

| 文件欄位 | 目前實作位置 | 目前值 |
| --- | --- | --- |
| `Avatar ID` | 文件定義 | `friday` |
| `角色名稱` | `sherryserver/avatars.json -> displayName` | `Friday` |
| `模型檔` | `sherryserver/www/html/avatar.js` + `sherryserver/avatars.json -> avatarModel` | `/avatar/friday.vrm` |
| `查詢來源` | 後端 MCP 設定 | `MCP` |
| `連線位址 / 端點` | `sherryserver/avatars.json` | `endpoint=http://localhost:8000/mcp/sse` |
| `MCP Tool Name` | `sherryserver/avatars.json` | `query_brain_knowledge` |
| `MCP Tool Argument` | `sherryserver/avatars.json` | `question` |
| `特殊規則` | 文件定義 | 收到使用者問題後直接走 MCP |

## 建議中的未來設定格式

如果之後要正式支援多 Avatar，建議把文件欄位落成一份 JSON 或 Go 設定，至少包含以下欄位：

```json
{
  "id": "friday",
  "displayName": "Friday",
  "enabled": true,
  "isDefault": true,
  "avatarModel": "/avatar/friday.vrm",
  "querySource": "mcp",
  "endpoint": "http://localhost:8000/mcp/sse",
  "mcpToolName": "query_brain_knowledge",
  "mcpToolArgument": "question",
  "systemInstruction": "",
  "specialRules": [
    "收到使用者問題後直接透過 MCP 查詢"
  ]
}
```

這樣文件中的統一設定表就能和實際程式設定一一對應，不需要再人工比對。

## 目前已知補充

- 目前 `friday` 是預設 Avatar。
- 首頁角色名稱會跟著目前選定 Avatar 的 `displayName` 同步更新。
- 前端目前預設載入 `friday.vrm`。
- `friday` 目前以 `sherryserver/avatars.json` 為主，使用的 MCP SSE 端點是 `http://localhost:8000/mcp/sse`。

## 新增 Avatar 時請一併補上

| Avatar ID | 角色名稱 | 狀態 | 模型檔 | 查詢來源 | 連線位址 / 端點 | 特殊規則 |
| --- | --- | --- | --- | --- | --- | --- |
| `example-avatar` | Example Avatar | 測試中 | `sherryserver/www/html/avatar/example-avatar.vrm` | MCP / API / DB / 混合流程 | `http://localhost:9000` | 例如特定問題走 MCP，其餘走 LLM API；或指定專屬 system prompt / tool set |
