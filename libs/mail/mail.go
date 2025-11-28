package MailService

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

// smtp 帳號（可多組）
type SMTPAccount struct {
   Host		string	`json:"host"`
   Port		int	`json:"port"`
   User		string	`json:"user"`
   Password	string	`json:"password"`
   Cnt		int	`json:"count"`
}

type SMTPMail struct {
   CFG		Config
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
func(app *SMTPMail) Send(to string, msg bytes.Buffer, needAuth bool)(error) {
   var auth smtp.Auth
   acc, err := app.SelectMinCntConfig()
   if err != nil {
      return err
   }
   if needAuth {
      auth = smtp.PlainAuth("", acc.User, acc.Password, acc.Host)
   }
   // 寄送郵件 fix)使用 msg.Bytes() 獲取 []byte
   if err := smtp.SendMail(acc.Host + ":" + acc.Port, auth, acc.User, []string{to}, msg.Bytes() ); err != nil {
      return fmt.Errorf("寄送郵件失敗:", err.Error())
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
   sm := &SMTPMail {
   }
   cfg, err := sm.LoadConfig()  // 取得SMTP資訊
   if err != nil {
      return nil, err
   }
   sm.CFG = cfg
   return sm, nil
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
	 <p>%s,您好<br /><br />注意事項：<br>
	 1.會議室內禁止錄影音、飲食。<br>
	 下方為您專屬的報到 QR code:</p>
	 <img src="cid:image-0" alt="內嵌圖片">
	 祝安康,</p>
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
