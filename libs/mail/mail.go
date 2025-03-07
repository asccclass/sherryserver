package main

import (
   "os"
   "fmt"
   "bytes"
   "strings"
   "net/smtp"
   "path/filepath"
   "encoding/base64"
)

// MIME 圖片
type Image struct {
   Data        string // base64 編碼後的圖片資料
   ContentType string // MIME 類型
   ContentID   string // 用於 HTML 中的 cid
}

type SMTPMail struct {
   Host		string		`json:host"`
   Port		string		`json:"port"`
   From		string
   Password	string
}

// 根據檔案副檔名判斷 MIME 類型
func(app *SMTPMail) getMineType(imageName string)(string) {
   ext := strings.ToLower(filepath.Ext(imageName))
   contentType := ""
   switch ext {
      case ".jpg", ".jpeg":
         contentType = "image/jpeg"
      case ".png":
         contentType = "image/png"
      case ".gif":
         contentType = "image/gif"
      default:
         fmt.Println("不支援的圖片格式:", ext)
   }
   return contentType
}

// SMTP 認證 && 寄信
func(app *SMTPMail) send(to string, msg bytes.Buffer)(error) {
   auth := smtp.PlainAuth("", app.From, app.Password, app.Host)
   // 寄送郵件 fix)使用 msg.Bytes() 獲取 []byte
   if err := smtp.SendMail(app.Host + ":" + app.Port, auth, app.From, []string{to}, msg.Bytes() ); err != nil {
      return fmt.Errorf("寄送郵件失敗:", err)
   }
   return nil
}

// 加入檔案
func(app *SMTPMail) AddImages(img string, images []Image)([]Image, error) {
   // read file
   imageData, err := os.ReadFile(img)
   if err != nil {
      return images, fmt.Errorf("讀取圖片失敗:", err)
   }
   // 將圖片轉為 base64 編碼
   imageBase64 := base64.StdEncoding.EncodeToString(imageData)
   // 為每張圖片生成唯一的 Content-ID
   contentID := fmt.Sprintf("image-%d", len(images)) // 給1～1000 亂數
   images = append(images, Image{
      Data:        imageBase64,
      ContentType: app.getMineType(img),
      ContentID:   contentID,
   })
   return images, nil
}

func NewSMTPMail()(*SMTPMail, error) {
   smtpEmail := os.Getenv("smtpEmail")
   password := os.Getenv("smtpPassword")
   if password == "" || smtpEmail == "" {
      return nil, fmt.Errorf("email sender email or password not set")
   }
   smtpHost := os.Getenv("mailHost")
   if smtpHost == "" {
      smtpHost = "smtp.gmail.com"
   }
   smtpPort := os.Getenv("smtpPort")
   if smtpPort == "" {
      smtpPort = "587"
   }
   return &SMTPMail {
      Host: smtpHost,
      Port: smtpPort,
      From: smtpEmail,
      Password: password,
   }, nil
}

/*
func main() {
   app, _ := NewSMTPMail()

   // read file
   img := "logo_v2.png"
   images, err := app.AddImages(img, []Image{})
   if err != nil {
      fmt.Println(err.Error())
      return
   }

   subject := "【活動提醒及入場QR Code】114年度國中會考趨勢分析與複習策略專題講座入場通知"
   body := fmt.Sprintf(`
      <!DOCTYPE html>
      <html>
         <body>
	 <p>%s,您好<br /><br />感謝您報名參加114年度國中會考趨勢分析與複習策略專題講座。以下為本次活動相關訊息供您參考。<br /><br />活動時間：2 月 22 日（星期六）13:00～17:50<br>活動地點：中正國中活動中心（臺北市中正區愛國東路 158 號）<br><br>注意事項：<br>
	 1.會議室內禁止錄影音、飲食。<br>
	 2.報到時請出示下方QR Code 供主辦方確認後，方得入場。<br>
	 3.附近較難停車，請提早10分鐘到場並盡量使用大眾交通工具。<br><br>
	 下方為您專屬的報到 QR code:</p>
	 <img src="cid:image-0" alt="內嵌圖片">
	 請您於規定時間內出席<br><br>
	 祝安康,</p>
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
   if err := app.send(app.From, msg); err != nil {
      fmt.Println("send err:", err.Error())
      return
   }
   fmt.Println("send ok")
}
*/
