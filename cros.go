/*
   StaticServer CROS 公用程式
*/
package SherryServer

import (
   "strings"
   "net/http"
)

// 新版
// 檢查CROS問題
func(srv *Server) CheckCROSNew(next http.HandlerFunc)(http.HandlerFunc) {
   return func(w http.ResponseWriter, r *http.Request) {
      if srv.OriginAllow.PatternNum != 0 {  // 有設定CROS參數
         origin := r.Header.Get("Origin")
         if _, ok := srv.OriginAllow.Search(origin); ok {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Add("Vary", "Origin")
         }
      }
      if srv.MethodAllow.PatternNum != 0 {  // 有限制 Methods
         method := strings.ToUpper(r.Header.Get("Access-Control-Request-Method"))
         if _, ok := srv.OriginAllow.Search(method); ok {
            if methodx := srv.MethodAllow.GetPattern(", "); methodx != "" {
               w.Header().Set("Access-Control-Allow-Methods", methodx)
            }
         }
      }
      next(w, r)
   }
}

// 檢查CROS問題
func(srv *Server) CheckCROS(next http.Handler)(http.Handler) {
   return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      if srv.OriginAllow.PatternNum != 0 {  // 有設定CROS參數
         origin := r.Header.Get("Origin")
         if _, ok := srv.OriginAllow.Search(origin); ok {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Add("Vary", "Origin")
         }
      }
      if srv.MethodAllow.PatternNum != 0 {  // 有限制 Methods
         method := strings.ToUpper(r.Header.Get("Access-Control-Request-Method"))
         if _, ok := srv.OriginAllow.Search(method); ok {
            if methodx := srv.MethodAllow.GetPattern(", "); methodx != "" {
               w.Header().Set("Access-Control-Allow-Methods", methodx)
            }
         }
      }
      next.ServeHTTP(w, r)
   })
}