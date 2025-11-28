package MailService

import (
   "regexp"
)

// 判斷Email格式是否正確
func(app *SMTPMail)  IsValidEmail(email string)(bool) {
   re := regexp.MustCompile(`^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}\$`)
   return re.MatchString(email)
}
