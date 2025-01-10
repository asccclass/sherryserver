package SherryRegister

import (
   "os"
   "fmt"
   "net/http"
   "net/smtp"
   "crypto/rand"
   "database/sql"
   _ "modernc.org/sqlite"
)

// 發送驗證郵件
func(app *Register) sendVerificationEmail(to, code string)( error) {
   subject := "驗證您的電子郵件"
   body := fmt.Sprintf(`
       <html>
           <body>
               <h2>歡迎註冊！</h2>
               <p>您的驗證碼是：<strong>%s</strong></p>
               <p>請點擊以下連結進行驗證：</p>
               <a href="%s/verify?email=%s&code=%s">驗證電子郵件</a>
           </body>
       </html>
   `, code, os.Getenv("baseURL"), to, code)

   auth := smtp.PlainAuth("", app.Info.EmailFrom, app.Info.EmailPassword, app.Info.SMTPHost)
   mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
   msg := fmt.Sprintf("Subject: %s\n%s\n%s", subject, mime, body)

   return smtp.SendMail(
       app.Info.SMTPHost + ":" + app.Info.SMTPPort,
       auth,
       app.Info.EmailFrom,
       []string{to},
       []byte(msg),
   )
}

// 生成驗證碼
func(app *Register) generateVerificationCode() string {
   code := make([]byte, 6)
   rand.Read(code)
   return fmt.Sprintf("%x", code)[:6]
}

func(app *Register) register(w http.ResponseWriter, r *http.Request) {
   if r.Method != http.MethodPost {
       http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
       return
   }

   // 解析表單數據
   name := r.FormValue("name")
   email := r.FormValue("email")
   organization := r.FormValue("organization")

   // 基本驗證
   if len(name) < 2 || len(organization) < 2 {
       http.Error(w, "Invalid input", http.StatusBadRequest)
       return
   }

   // 生成驗證碼
   verifyCode := app.generateVerificationCode()

   // 保存用戶信息
   _, err := app.DB.Exec(`
       INSERT INTO users (name, email, organization, verify_code)
       VALUES (?, ?, ?, ?)
   `, name, email, organization, verifyCode)

   if err != nil {
       fmt.Printf("保存用戶錯誤: %v", err)
       w.Write([]byte(`<div class="error">註冊失敗，請稍後重試</div>`))
       return
   }

   // 發送驗證郵件
   err = app.sendVerificationEmail(email, verifyCode)
   if err != nil {
       fmt.Printf("發送驗證郵件錯誤: %v", err)
       w.Write([]byte(`<div class="error">驗證郵件發送失敗，請檢查郵箱地址</div>`))
       return
   }

   w.Write([]byte(`<div class="success">註冊成功！請查收驗證郵件完成註冊。</div>`))
}

func(app *Register) verifyEmail(w http.ResponseWriter, r *http.Request) {
   email := r.URL.Query().Get("email")
   code := r.URL.Query().Get("code")

   if email == "" || code == "" {
       http.Error(w, "Missing parameters", http.StatusBadRequest)
       return
   }

   // 驗證碼檢查
   var dbCode string
   err := app.DB.QueryRow("SELECT verify_code FROM users WHERE email = ?", email).Scan(&dbCode)
   if err != nil {
       http.Error(w, "Invalid verification", http.StatusBadRequest)
       return
   }

   if code != dbCode {
       http.Error(w, "Invalid verification code", http.StatusBadRequest)
       return
   }

   // 更新驗證狀態
   _, err = app.DB.Exec("UPDATE users SET is_verified = TRUE WHERE email = ?", email)
   if err != nil {
       http.Error(w, "Verification failed", http.StatusInternalServerError)
       return
   }

   // 返回成功頁面
   w.Write([]byte(`
       <html>
           <body>
               <h1>驗證成功！</h1>
               <p>您的郵箱已驗證成功，現在可以登入系統了。</p>
           </body>
       </html>
   `))
}
