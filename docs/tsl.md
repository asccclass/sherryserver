# TLS 憑證動態取得與自動續期規劃

狀態：功能已實作，本地自動化驗收通過；公開網域的 ACME staging 部署驗收尚待執行。驗收紀錄日期：2026-11-03。

參考文章：[Acquiring TLS certificates at runtime](https://dushed.com/blog/inband-tls-certificates)。文中以 Go 的 `autocert.Manager` 搭配 TLS 憑證回呼，在程式執行期間取得、快取及續期憑證。

## 功能目標與範圍

新增預設關閉的自動 TLS 功能，開啟後由本程式透過 ACME 管理 HTTPS 憑證，無須因憑證續期重啟服務。

- 開關在啟動時讀取；修改開關或網域設定後須重新啟動服務。
- 憑證取得與續期在執行期間自動進行，更新後供後續 TLS 握手使用，既有連線不因更新而主動中斷。
- 使用 `golang.org/x/crypto/acme/autocert`；專案目前已直接依賴 `golang.org/x/crypto`。
- 第一版以單實例部署及 `TLS-ALPN-01` 驗證為範圍，不額外啟動 port 80 listener。
- 第一版不包含固定 PEM 檔案熱重載、DNS-01、萬用字元憑證、執行期間切換開關、多實例協調或 `SO_REUSEPORT` 零停機部署。

## 現有行為與模式選擇

目前 `server.go` 的 `Start()` 讀取 `sslCertification` 與 `sslKeyfile`：兩者皆有設定時呼叫 `ListenAndServeTLS(certfile, keyfile)`，否則使用 HTTP。

| 設定 | 預期行為 |
| --- | --- |
| 自動 TLS 關閉，未完整提供固定憑證路徑 | 維持既有 HTTP 行為 |
| 自動 TLS 關閉，完整提供固定憑證路徑 | 維持既有固定憑證 HTTPS 行為 |
| 自動 TLS 開啟，未設定固定憑證路徑 | 使用 ACME 自動 TLS |
| 自動 TLS 開啟，任一固定憑證路徑有設定 | 回報設定衝突，拒絕啟動 |
| 自動 TLS 開啟，必要設定不合法 | 回報設定錯誤，拒絕啟動，不降級為 HTTP |

保留 `NewServer()` 與 `Start()` 的公開呼叫方式，將必要設定驗證放在可回傳錯誤的初始化流程。

## 環境設定

沿用專案的 `envfile` 與環境變數讀取方式。

```dotenv
# 預設關閉
TLSAutoEnabled=false

# 開啟時必填，以逗號分隔明確允許的網域
TLSAutoDomains=example.com,www.example.com

# 開啟時必填，保存憑證、私鑰與 ACME 帳號資料
TLSAutoCacheDir=/app/data/autocert

# 選填：ACME 聯絡信箱
TLSAutoEmail=admin@example.com

# 預設 false；開啟自動 TLS 時須明確設為 true
TLSAutoAcceptTOS=false

# 選填：未設定時使用 autocert 預設的 Let's Encrypt 正式環境
# 測試部署時指定 ACME staging directory URL
# TLSAutoDirectoryURL=...
```

設定規則：

- 布林值採嚴格解析；不合法的值須回報變數名稱與錯誤原因。
- 關閉自動 TLS 時，不因未填自動 TLS 的其他設定而阻擋既有模式。
- 開啟時，網域清單須去除前後空白、正規化及去重，拒絕空清單、無效網域、URL、含 port 的字串與萬用字元。
- 快取目錄須可建立及讀寫；目錄及檔案權限應限制於服務帳號，Windows 部署以檔案系統 ACL 管理存取。
- 使用者須以 `TLSAutoAcceptTOS=true` 明確同意 ACME CA 服務條款。
- 自訂 ACME directory URL 須驗證格式；正式部署使用 HTTPS。
- 不由套件強制改寫應用程式傳入的 listen address；文件須說明外部 TCP 443 必須導向該 listener。

## 實作設計

### 設定與初始化

新增 `tlsconfig.go`，集中處理設定解析、模式判斷、設定驗證與 `autocert.Manager` 建立。

在 `NewServer()` 已載入環境設定後進行初始化，讓設定錯誤能透過既有的 error 回傳給呼叫端。關閉自動 TLS 時不建立 ACME manager，也不建立快取目錄。

### 憑證管理與 HTTPS 啟動

- 建立單一 `autocert.Manager`，設定 `HostPolicy`、`DirCache`、聯絡信箱、條款同意回呼與選用的 ACME client。
- 使用 `manager.TLSConfig()`，保留 HTTP/2 與 `acme-tls/1` 所需的 ALPN 設定。
- 在 `GetCertificate` 回呼入口檢查 SNI 是否位於允許清單，再委派給 manager；不能只依賴 `HostPolicy`，因為該策略不限制已快取的憑證。
- 啟動時先建立 TCP listener，再依模式呼叫 `ServeTLS(listener, "", "")`、帶入固定憑證路徑的 `ServeTLS` 或 HTTP `Serve`。此方式與 `ListenAndServeTLS` 使用相同的 TLS 設定機制，也讓三種模式能共用本地 listener 驗收。
- 憑證取得、快取、續期與同時握手的協調交由 `autocert` 處理，不自行實作 ACME 協定或每次握手讀取 PEM。
- 呼叫端的 `TLSConfig` 會先複製，保留相容的版本限制、cipher suites、session tickets 與自訂 ALPN，並補上 HTTP/2、HTTP/1.1 及 ACME ALPN。預設最低 TLS 1.2；明確指定過低或矛盾的版本範圍會拒絕啟動。自訂憑證、`NameToCertificate`、`GetCertificate`、`GetConfigForClient` 與非預設 `ClientAuth` 均回報衝突；自訂驗證回呼仍須允許 ACME 驗證握手。
- 快取依 ACME directory URL 的 SHA-256 建立子目錄，隔離 staging 與正式 CA 的帳號與憑證。

### 錯誤與日誌

- 記錄啟動模式、允許網域及快取位置，不記錄私鑰或 ACME 帳號機密。
- 將初始化錯誤與執行期間握手／簽發錯誤分開處理。
- 首次簽發失敗時，該次 TLS 握手失敗，服務繼續接受後續連線，重試遵循 `autocert` 的退避機制。
- 續期失敗時，既有尚有效的憑證繼續使用；憑證過期後不保證客戶端仍能連線。
- 延用既有 graceful shutdown，不因本功能新增額外 HTTP listener。

## 部署條件

- 使用可公開驗證的網域，DNS 須正確導向服務。
- 外部 TCP 443 的 TLS 握手及 ACME ALPN 驗證須能直達本程式；內部可透過 port mapping 對應至其他 listen port。
- 程式須能對外連線至 ACME CA。
- Docker 或其他容器部署須將快取目錄掛載為持久化儲存，重新部署後保留憑證及帳號資料。
- 若反向代理已終止 TLS，優先由代理管理憑證並關閉本功能；單純 HTTP 轉送無法傳遞 TLS-ALPN-01 驗證。
- 首次未有快取時，第一個握手可能因簽發流程而延遲或失敗；listener 啟動不等同所有網域已取得憑證。
- 正式上線前使用 ACME staging 驗證，不以反覆正式簽發作為測試手段。

## 待辦事項

- [x] 新增自動 TLS 設定結構、嚴格解析與模式判斷。
- [x] 實作網域正規化、允許清單、SNI 檢查及錯誤訊息。
- [x] 實作快取目錄初始化及可讀寫檢查。
- [x] 建立 `autocert.Manager`，設定持久化快取、服務條款、信箱與 ACME directory。
- [x] 在 `NewServer()` 整合初始化及錯誤回傳，保留既有公開 API。
- [x] 在 `Start()` 整合 HTTP、固定憑證 HTTPS、自動 TLS 三種分支。
- [x] 定義並測試呼叫端自訂 `TLSConfig` 的相容與衝突處理。
- [x] 加入啟動與憑證回呼錯誤日誌，確認不輸出機密。
- [x] 加入設定與模式相容性測試。
- [x] 加入本地 TLS 握手、SNI 限制及快取重用測試。
- [x] 使用模擬 ACME 驗證首次簽發、失敗處理與續期流程。
- [x] 更新 `docs/README.md` 與範例 `.env.example`，提供開啟、關閉、固定憑證與容器部署範例。
- [x] 執行格式化、`go test ./...` 及支援環境下的 `go test -race ./...`。
- [ ] 完成 staging 部署驗收並記錄結果，再安排正式環境啟用。

## 驗收條件

### 設定與相容性

- [x] 未設定 `TLSAutoEnabled` 或設為 `false` 時，既有 HTTP 與固定憑證 HTTPS 行為不變，且不發出 ACME 請求。
- [x] 自動 TLS 開啟且必要設定完整時，伺服器採用動態憑證回呼，不要求固定憑證檔案。
- [x] 無效布林值、空白或無效網域清單、未同意條款、不可用快取目錄均產生可辨識錯誤。
- [x] 自動 TLS 與任一固定憑證路徑同時設定時，初始化失敗，且不啟動 HTTP fallback。
- [x] 現有呼叫端無須修改 `NewServer()`／`Start()` 的呼叫方式。

### 憑證與連線行為

- [x] 允許的網域可透過本地測試或 staging 完成首次簽發與 HTTPS 握手。
- [x] 未允許或未提供 SNI 的握手遭拒，且不替該名稱發出簽發請求。
- [x] 即使快取已有憑證，未允許的網域仍無法繞過入口檢查。
- [x] 同時多個握手不產生資料競爭，憑證取得可正確協調。
- [x] 程式重新啟動後能重用持久化快取，不因單純重啟而重新簽發有效憑證。
- [x] 模擬續期成功後，新的完整 TLS 握手可取得更新憑證，伺服器程序無須重啟，既有連線不被主動中斷。
- [x] 首次簽發失敗僅使相關握手失敗，不造成程序退出或改用 HTTP；後續可依重試機制恢復。
- [x] 續期暫時失敗時，仍能使用既有尚有效的憑證。
- [x] 自動模式保留 HTTP/2 與 ACME 驗證 ALPN，既有 graceful shutdown 可正常完成。

### 驗證與部署文件

- [x] 自動化測試以本地憑證、模擬 ACME 或測試快取執行，不依賴正式 CA。
- [x] 更新憑證的握手測試停用 session resumption，確認實際收到新憑證。
- [x] `go test ./...` 通過；在支援 race detector 的環境執行 `go test -race ./...` 並通過。
- [ ] staging 實際部署確認 DNS、外部 443、ALPN、快取掛載及重啟後重用均正常。
- [x] 文件清楚說明首次簽發延遲、代理部署限制，以及改回 `TLSAutoEnabled=false` 並設定固定憑證的操作方式。

## 本地驗收紀錄

環境：Windows amd64、Go 1.27.2，使用本機 GCC 執行 race detector。以下測試使用暫存目錄、loopback listener、本地測試 CA；不連線至正式 CA，也不簽發公開網域憑證。

| 驗收項目 | 測試證據 |
| --- | --- |
| 開關、舊模式、衝突、必要欄位 | `TestTLSSettings`、`TestAutoTLSSettings` |
| 網域正規化、IDN、無效輸入 | `TestTLSDomains`、`TestAutoTLSSettings` |
| 快取路徑錯誤、CA 環境隔離 | `TestTLSCacheErrors`、`TestAutoTLSSettings`；Unix 目錄權限分支須於 Unix 環境執行 |
| 自訂 TLSConfig 保留與衝突 | `TestAutoTLSMerge` |
| 初始化錯誤回傳、設定快照、listen address 不變 | `TestNewServerTLSValidationAndSnapshot` |
| HTTP 與固定檔案 HTTPS 實際回應，不使用 ACME | `TestLegacyModesServe` |
| 首次簽發、TLS-ALPN-01 真實 socket 驗證、並行協調、HTTP/2、SNI 拒絕 | `TestAutoTLSACMEIssuanceConcurrentRestartAndSNI` |
| 程序重啟後讀取原快取，不重新簽發 | 同上，透過 `TestAutoTLSProcessHelper` 啟動獨立子程序重用同一目錄 |
| 背景續期成功、新完整握手取得新憑證、既有連線保留 | `TestAutoTLSACMERenewal/failure=false` |
| 背景續期失敗時仍使用尚有效憑證 | `TestAutoTLSACMERenewal/failure=true` |
| 首次簽發失敗，等待實際退避後恢復，不輸出 CA 回應機密 | `TestAutoTLSACMEInitialFailureRetries` |
| 關閉 listener 時等待進行中的 HTTPS 請求完成 | `TestAutoTLSGracefulShutdown` |

驗證命令：

```bash
go test ./... -count=1
go test -race ./... -count=1
```

兩者均已通過。重試測試會等待 `autocert` v0.55.0 的實際一分鐘失敗狀態保留時間，因此完整測試約需一分鐘以上。續期測試以短效本地憑證與 manager 公開的 `RenewBefore` 觸發真正的背景續期，未更改套件內部時鐘或計時器。更新檢查不使用 session cache，並確認 `DidResume=false`。

## 待執行的公開 staging 驗收

使用者已確認目前沒有公開 staging 驗收所需的網域與部署環境，因此下列實際部署項目保留待驗，不能視為已通過。此項也包含部署者檢查實際快取目錄擁有者／ACL。單機模擬 CA 測試無法替代外部 DNS 與網路驗證。

1. 準備專用測試網域，確認 A／AAAA 記錄均導向能到達服務的位址，外部 TCP 443 能直達程式，且允許程式對外存取 CA。
2. 依 `docs/README.md` 設定自動 TLS、持久化快取，以及 `TLSAutoDirectoryURL=https://acme-staging-v02.api.letsencrypt.org/directory`；閱讀並同意 CA 條款後啟用。
3. 啟動服務，檢查日誌的 `mode=auto-tls`、網域與快取位置。從外部主機連線至該網域；以 `curl --insecure https://測試網域/` 取得應用程式預期回應。此處略過信任檢查僅限 staging，因 staging 根憑證不受一般客戶端信任，不能用於正式環境的驗收。
4. 使用 `openssl s_client -connect 測試網域:443 -servername 測試網域 -alpn h2` 檢查協商結果，確認憑證 SAN 與 staging 簽發者符合預期；成功簽發亦須已完成 CA 的 TLS-ALPN-01 驗證。
5. 記錄憑證序號或指紋與有效期，重啟程序或重建容器並保留快取掛載。再次連線，確認使用相同有效憑證，且未因重啟重新簽發；確認快取不在容器暫存層。
6. 記錄實際網域、部署版本、測試時間、憑證序號／指紋、ALPN、重啟結果及快取權限；完成後勾選 staging 驗收項目。
7. 正式啟用時移除 staging directory 設定，保留快取根目錄；正式 CA 會使用獨立快取並首次簽發。以一般客戶端的正常信任驗證確認 HTTPS 回應，不使用 `--insecure`。

## 技術參考

- [autocert.Manager：自動取得與更新憑證](https://pkg.go.dev/golang.org/x/crypto/acme/autocert#Manager)
- [Manager.TLSConfig：HTTP/2 與 TLS-ALPN-01 設定](https://pkg.go.dev/golang.org/x/crypto/acme/autocert#Manager.TLSConfig)
- [Manager.GetCertificate：憑證回呼及快取策略說明](https://pkg.go.dev/golang.org/x/crypto/acme/autocert#Manager.GetCertificate)
- [Manager.HTTPHandler：HTTP-01 與預設驗證行為](https://pkg.go.dev/golang.org/x/crypto/acme/autocert#Manager.HTTPHandler)

## 參考資料
* [Acquiring TLS certificates at runtime](https://dushed.com/blog/inband-tls-certificates?via=dailydev)
