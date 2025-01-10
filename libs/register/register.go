package SherryRegister

import (
   "os"
   "fmt"
   "time"
   "regexp"
   "net/http"
   // "net/smtp"
   // "crypto/rand"
   "database/sql"
   // "encoding/json"
   _ "modernc.org/sqlite"
)

type User struct {
   ID            int    `json:"id"`
   Name          string `json:"name"`
   Email         string `json:"email"`
   Password      string `json:"password"`
   Organization  string `json:"organization"`
   VerifyCode    string `json:"verify_code"`
   IsVerified    bool   `json:"is_verified"`
}

type Response struct {
   Status  string `json:"status"`
   Message string `json:"message"`
   HTML    string `json:"html,omitempty"`
}

type Config struct {
   EmailFrom     string
   EmailPassword string
   SMTPHost      string
   SMTPPort      string
   DBPath        string
}

type Register struct {
   DB		*sql.DB
   Info		Config
   CsrfTokens	map[string]time.Time
}

func(app *Register) initDB()(error) {
   var err error
   app.DB, err = sql.Open("sqlite", app.Info.DBPath)
   if err != nil {
      return err
   }

   // 創建用戶表
   _, err = app.DB.Exec(`
       CREATE TABLE IF NOT EXISTS users (
           id INTEGER PRIMARY KEY AUTOINCREMENT,
           name TEXT NOT NULL,
           email TEXT UNIQUE NOT NULL,
           organization TEXT NOT NULL,
           verify_code TEXT NOT NULL,
           is_verified BOOLEAN DEFAULT FALSE
       )
   `)
   if err != nil {
      return err
   }
   return nil
}

// 驗證處理函數
func(app *Register) validateOrganization(w http.ResponseWriter, r *http.Request) {
   org := r.FormValue("organization")
   if len(org) < 2 {
      w.Write([]byte(`<div class="error">服務單位至少需要 2 個字元</div>`))
      return
   }
   w.Write([]byte(`<div class="success">服務單位格式正確</div>`))
}

// 驗證 email
func(app *Register) validateEmail(w http.ResponseWriter, r *http.Request) {
   email := r.FormValue("email")
   emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
   if !emailRegex.MatchString(email) {
      w.Write([]byte(`<div class="error">請輸入有效的電子郵件地址</div>`))
      return
   }
   // 檢查郵箱是否已存在
   var count int
   err := app.DB.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", email).Scan(&count)
   if err != nil {
      fmt.Printf("檢查郵箱錯誤: %v", err)
      w.Write([]byte(`<div class="error">系統錯誤</div>`))
      return
   }
   if count > 0 {
      w.Write([]byte(`<div class="error">此電子郵件已被註冊</div>`))
      return
   }
   w.Write([]byte(`<div class="success">電子郵件格式正確</div>`))
}

func(app *Register) validateName(w http.ResponseWriter, r *http.Request) {
   name := r.FormValue("name")
   if len(name) < 2 {
       w.Write([]byte(`<div class="error">姓名至少需要 2 個字元</div>`))
       return
   }
   w.Write([]byte(`<div class="success">姓名格式正確</div>`))
}

// Router
func(app *Register) AddRouter(router *http.ServeMux) {
   router.HandleFunc("/validate/name", csrfMiddleware(app.validateName))
   router.HandleFunc("/validate/email", csrfMiddleware(app.validateEmail))
   router.HandleFunc("/validate/organization", csrfMiddleware(app.validateOrganization))
   router.HandleFunc("/register", csrfMiddleware(app.register))
   router.HandleFunc("/verify", app.verifyEmail)
}

func NewRegister()(*Register, error) {
   config := Config {  // 配置信息
       EmailFrom:     os.Getenv("EmailFrom"),
       EmailPassword: os.Getenv("EmailPassword"),
       SMTPHost:      os.Getenv("SMTPHost"),
       SMTPPort:      os.Getenv("SMTPPort"),
       DBPath:        os.Getenv("USERDB"),
   }
   rg := &Register {
      Info: config,
      CsrfTokens: make(map[string]time.Time),
   }
   if err := rg.initDB(); err != nil {  // 初始化數據庫
      return nil, err
   }
   return rg, nil
}

