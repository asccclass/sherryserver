package SherryCalendar

import (
   "log"
   "time"
   "strconv"
   "net/http"
   "encoding/json"
   // "github.com/asccclass/sherrytime"
)

// Calendar 定義日曆 API 的回應結構
type Calendar struct {
   Year        int       `json:"year"`
   Month       string    `json:"month"`
   MonthNumber int       `json:"monthNumber"`
   Dates       []Date    `json:"dates"`
   Navigation  Navigation `json:"navigation"`
}

// Date 定義日期結構
type Date struct {
   Day         int    `json:"day"`
   IsOtherMonth bool  `json:"isOtherMonth"`
   IsAvailable bool   `json:"isAvailable"`
   IsToday     bool   `json:"isToday"`
}

// Navigation 定義導航信息
type Navigation struct {
   PrevMonth string `json:"prevMonth"`
   PrevYear  int    `json:"prevYear"`
   NextMonth string `json:"nextMonth"`
   NextYear  int    `json:"nextYear"`
}

// AvailableTime 定義可預約時間結構
type AvailableTime struct {
   Year  int
   Month time.Month
   Day   int
}

// 模擬數據庫中的可預約時間
var availableTimes = []AvailableTime{
   // {2025, 1, 23}, {2025, 1, 28}, {2025, 1, 30},
}

func(app *Calendar)  generateCalendarData(year int, month time.Month) Calendar {
   currentDate := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)  // 創建指定年月的時間
   // 獲取上個月和下個月
   prevMonth := currentDate.AddDate(0, -1, 0)
   nextMonth := currentDate.AddDate(0, 1, 0)

   firstDay := currentDate.Weekday()  // 獲取當月第一天是星期幾（0=星期日）
   daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day() // 獲取當月天數

   var dates []Date   // 生成日期數組
   prevMonthDays := time.Date(prevMonth.Year(), prevMonth.Month()+1, 0, 0, 0, 0, 0, time.Local).Day() // 添加上個月的日期
   for i := int(firstDay) - 1; i >= 0; i-- {
       dates = append(dates, Date{
           Day:         prevMonthDays - i,
           IsOtherMonth: true,
       })
   }

   today := time.Now()  // 添加當月的日期
   for day := 1; day <= daysInMonth; day++ {
       isAvailable := true  // false
/*
       for _, at := range availableTimes { // 檢查是否是可預約時間
           if at.Year == year && at.Month == month && at.Day == day {
               isAvailable = true
               break
           }
       }
*/

       dates = append(dates, Date{
           Day:         day,
           IsOtherMonth: false,
           IsAvailable: isAvailable,
           IsToday:     today.Year() == year && today.Month() == month && today.Day() == day,
       })
   }

   // 添加下個月的日期（補齊到 6 行日曆）
   remainingDays := 42 - len(dates)
   for day := 1; day <= remainingDays; day++ {
       dates = append(dates, Date{
           Day:         day,
           IsOtherMonth: true,
       })
   }

   return Calendar{
       Year:        year,
       Month:       month.String(),
       MonthNumber: int(month),
       Dates:       dates,
       Navigation: Navigation{
           PrevMonth: prevMonth.Month().String(),
           PrevYear:  prevMonth.Year(),
           NextMonth: nextMonth.Month().String(),
           NextYear:  nextMonth.Year(),
       },
   }
}

func(app *Calendar) handleCalendar(w http.ResponseWriter, r *http.Request) {
   w.Header().Set("Access-Control-Allow-Origin", "*")  // 設置 CORS 頭
   w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
   w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
   if r.Method == "OPTIONS" {  // 處理 OPTIONS 請求
       w.WriteHeader(http.StatusOK)
       return
   }

   // 獲取查詢參數
   queryYear := r.URL.Query().Get("year")
   queryMonth := r.URL.Query().Get("month")

   // 設置默認時間為當前時間
   now := time.Now()
   year := now.Year()
   month := now.Month()

   // 如果提供了年份參數
   if queryYear != "" {
       if y, err := strconv.Atoi(queryYear); err == nil {
           year = y
       }
   }
   calendar := app.generateCalendarData(year, month)  // 生成日曆數據
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(calendar)  // 返回 JSON 響應
}

// 設置路由
func(app *Calendar) AddRouter(router *http.ServeMux) {
   http.Handle("GET /calendar", http.HandlerFunc(app.handleCalendar))
}

// 初始化SrySocketio
func NewSryCalendar()(*Calendar) {
   // st := sherrytime.NewSherryTime("Asia/Taipei", "-")  // Initial
   return &Calendar {
   }
}
