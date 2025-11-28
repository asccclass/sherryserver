package MailService

import (
   "os"
   "fmt"
   "encoding/json"
)

type Config struct {
   SMTPAccounts []SMTPAccount
   BaseURL      string
}

func(app *SMTPMail) getEnv(key, fallback string)(string) {
   if value, ok := os.LookupEnv(key); ok {
      return value
   }
   return fallback
}

// SelectMinCntConfig 函式：選出 cnt 最小的組態
func(app *SMTPMail) SelectMinCntConfig() (*SMTPAccount, error) {
   minCnt := app.CFG[0].SMTPAccounts.Cnt
   for _, config := range app.CFG.SMTPAccounts {
      if config.Cnt < minCnt {
         minCnt = config.Cnt
      }
   }
   // 收集所有 Cnt 等於最小值的組態
   var minCntConfigs []*SMTPAccount
   for i := range configs {
      if configs[i].Cnt == minCnt {
         minCntConfigs = append(minCntConfigs, &configs[i]) // 注意：這裡使用 &configs[i] 取得元素的指標，而不是直接使用迴圈變數的指標
      }
   }
   // 從最小組態列表中隨機挑選一個
   numMinConfigs := len(minCntConfigs)
   if numMinConfigs == 0 { // 理論上不會發生，除非數據在尋找最小和收集最小之間被修改
      return nil, fmt.Errorf("未找到任何最小 Cnt 組態")
   }
   // 初始化 rand 源，確保運行能得到隨機數（Go 1.20 及以後版本可省略）由於程式碼可能在舊版 Go 或需要確定性/更好的隨機性時，保留此步驟是安全
   r := rand.New(rand.NewSource(time.Now().UnixNano()))
   randomIndex := r.Intn(numMinConfigs)  // 隨機選擇一個索引
   return minCntConfigs[randomIndex], nil // 返回選中的組態
}

func(app *SMTPMail) LoadConfig()(*Config, error) {
   cfg := &Config{
      BaseURL: getEnv("ServerURL", "http://localhost:8080"),
   }
   // Try to load from smtp_accounts.json
   file, err := os.Open("smtp_accounts.json")
   if err == nil {
      defer file.Close()
      decoder := json.NewDecoder(file)
      if err := decoder.Decode(&cfg.SMTPAccounts); err != nil {
         fmt.Println("Error decoding smtp_accounts.json: %v", err)
         return nil, err
      }
   }

   // Fallback to .env if no accounts loaded
   if len(cfg.SMTPAccounts) == 0 {
      fmt.Println("No SMTP accounts found in config/smtp_accounts.json, using .envfile")
      cfg.SMTPAccounts = append(cfg.SMTPAccounts, SMTPAccount{
         Host:     getEnv("mailHost", "smtp.gmail.com"),
         Port:     getEnv("smtpPort", 587),
         User:     getEnv("smtpEmail", ""),
         Password: getEnv("smtpPassword", ""),
	 Cnt: 0,
      })
   }
   return cfg
}
