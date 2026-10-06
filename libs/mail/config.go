package MailService

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"
)

type Config struct {
	SMTPAccounts []SMTPAccount
	BaseURL      string
}

func (app *SMTPMail) getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func (app *SMTPMail) getEnvInt(key string, fallback int) int {
	value := app.getEnv(key, "")
	if value == "" {
		return fallback
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return i
}

// SelectMinCntConfig 函式：選出 cnt 最小的組態
func (app *SMTPMail) SelectMinCntConfig() (*SMTPAccount, error) {
	if len(app.CFG.SMTPAccounts) == 0 {
		return nil, fmt.Errorf("未設定 SMTP 帳號")
	}

	minCnt := app.CFG.SMTPAccounts[0].Cnt
	for _, config := range app.CFG.SMTPAccounts {
		if config.Cnt < minCnt {
			minCnt = config.Cnt
		}
	}
	// 收集所有 Cnt 等於最小值的組態
	var minCntConfigs []*SMTPAccount
	for i := range app.CFG.SMTPAccounts {
		if app.CFG.SMTPAccounts[i].Cnt == minCnt {
			minCntConfigs = append(minCntConfigs, &app.CFG.SMTPAccounts[i])
		}
	}
	// 從最小組態列表中隨機挑選一個
	numMinConfigs := len(minCntConfigs)
	if numMinConfigs == 0 { // 理論上不會發生，除非數據在尋找最小和收集最小之間被修改
		return nil, fmt.Errorf("未找到任何最小 Cnt 組態")
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomIndex := r.Intn(numMinConfigs)   // 隨機選擇一個索引
	return minCntConfigs[randomIndex], nil // 返回選中的組態
}

func (app *SMTPMail) LoadConfig() (*Config, error) {
	cfg := Config{
		BaseURL: app.getEnv("ServerURL", "http://localhost:8080"),
	}
	// Try to load from smtp_accounts.json
	file, err := os.Open("smtp_accounts.json")
	if err == nil {
		defer file.Close()
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&cfg.SMTPAccounts); err != nil {
			fmt.Printf("Error decoding smtp_accounts.json: %v\n", err)
			return nil, err
		}
	}

	// Fallback to .env if no accounts loaded
	if len(cfg.SMTPAccounts) == 0 {
		fmt.Println("No SMTP accounts found in config/smtp_accounts.json, using .envfile")
		cfg.SMTPAccounts = append(cfg.SMTPAccounts, SMTPAccount{
			Host:     app.getEnv("mailHost", "smtp.gmail.com"),
			Port:     app.getEnvInt("smtpPort", 587),
			User:     app.getEnv("smtpEmail", ""),
			Password: app.getEnv("smtpPassword", ""),
			Cnt:      0,
		})
	}
	return &cfg, nil
}
