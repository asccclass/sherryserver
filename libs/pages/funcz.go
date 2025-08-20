package SherryPages

import(
   "fmt"
   "time"
   "math"
   "strings"
   "strconv"
)

// 01.Sum function to split the string and sum the values
func(app *Page) strSum(input string)(int){
   values := strings.Split(input, ",")
   total := 0
   for _, v := range values {
      num, err := strconv.Atoi(strings.TrimSpace(v))
      if err == nil {
         total += num
      }
   }
   return total
}

// 02計算百分比
func(app *Page) calPercentage(part, whole float64)(float64){
   if whole == 0 {
      return 0 // 防止除以零錯誤
   }
   p := (part / whole) * 100
   return math.Round(p*100)/100
}

// 03.轉成 int
func(app *Page) toInt(s any)(int) {
   switch v := s.(type) {
       case int:
          return v
       case string:
          y := strings.Replace(v, ",", "", -1)
          i, err := strconv.Atoi(y)
	  if err != nil {
             return 0
	  }
          return i
      case int64:
         return int(v)
      case float64:
         str := strconv.FormatFloat(v, 'f', 0, 64) // 'f' 表示以小數點格式，0 表示不保留小數部分，64 表示輸入類型是 float64
	 return app.toInt(str)
      default:
         fmt.Println(s, v)
         return 0 
   }
}

// 04.轉成字串
func(app *Page) toString(s any)(string) {
   switch v := s.(type) {
       case int:
          return strconv.Itoa(v)
       case string:
          return v
      case float64:
         return strconv.FormatFloat(v, 'f', 6, 64) 
      case bool:
          return strconv.FormatBool(v)
      default:
         fmt.Println(s, v)
         return "" 
   }
}

// 05.定義一個將字串轉換為浮點數的函數
func(app *Page) toFloat64(s any) (float64) {
   switch v := s.(type) {
       case int:
          return float64(v)
       case string:
          y := strings.Replace(v, ",", "", -1)
	  x, err := strconv.ParseFloat(y, 64)
          if err != nil {
             fmt.Println(err.Error())
             return 0
          }
          return x
      case int64:
         return float64(v)
      case float64:
         return v
      default:
         fmt.Println("不在switch中", s, v)
	 return 0
   }
}

// 06.兩數相乘
func(app *Page) multiply(a, b any)(float64) {
   return app.toFloat64(a) * app.toFloat64(b)
}

// 07.兩數相減
func(app *Page) minus(a, b float64)(float64) {
   return a - b
}

// 08.取得月份
func(app *Page) getArrayValueByMonth(a []any)(any) {
   if len(a) == 0 {
      return 0
   }
   i := int(time.Now().Month()) - 1
   return a[i]
}

// 09.getFieldValue .stocks  "no"  "00919"  "yield"
func(app *Page) getFieldValue(data []any, fieldName, fieldValue, valName string)(any) {
   for _, item := range data { // 檢查 item 是否為 map[string]any
      if record, ok := item.(map[string]any); ok { // 比對 fieldName 對應的值是否等於 fieldValue
         if record[fieldName] == fieldValue { // 回傳 valName 對應的值
            return record[valName]
         }
      }
   }
   return nil // 如果未找到符合條件的資料，回傳 nil
}

// 10.透過 Index 取得陣列內容
func(app *Page) getArrayValueByIndex(a []any, idx int)(any) {
   return a[idx]
}
