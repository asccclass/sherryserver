package DBLoginService

import(
   "crypto/rand"
   "encoding/base64"
   "golang.org/x/crypto/bcrypt"
)

// 將密碼加密
func(app *DBLoginService) hashPassword(password string)(string, error) {
   xpass, err := bcrypt.GenerateFromPassword([]byte(password), 10)
   return (string)xpass, err
}

// 檢查 Hash 密碼是否正確
func(app *DBLoninService) checkPasswordHash(password, hash string)(bool) {
   err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
   return err == nil
}

// 產生 token
func(app *DBLoginService) generateToken(length int)(string) {
   xpass := make([]byte, length)
   if _, err := rand.Read(xpass); err != nil {
      fmt.Println(err.Error())
      return ""
   }
   return base64.URLEncoding.EncodeToString(xpass)
}
