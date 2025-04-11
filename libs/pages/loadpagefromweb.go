package SherryPages

import(
   "os"
   "fmt"
   "bytes"
   "embed"
   "strings"
   "net/http"
   "html/template"
   "path/filepath"
   "encoding/json"
   "github.com/Masterminds/sprig/v3"
   "github.com/asccclass/sherrytime"
)

//將jsonData string 轉成 data interface
func(app *Page) convertString2Interface(jsonData, templateName string)(interface{}, error) {
   var data interface{} // 解析 JSON 數據
   if jsonData != "" {
      if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
         return data, fmt.Errorf("String to JSON 解析錯誤: %v", err)
      }
      // 確保 data 是一個 map[string]interface{}
      if dataMap, ok := data.(map[string]interface{}); ok {
         newMap := map[string]string{"menu": templateName} // 新增的 map
         for key, value := range newMap { // 將 newMap 中的元素追加到 dataMap 中
            dataMap[key] = value
         }
         data = dataMap
      } else {  // [{...}]
         data = map[string]string{"menu": templateName, "vals": jsonData}
         app.Srv.Logger.Info("JSON 解析錯誤type")
      }
   } else {
      data = map[string]string{"menu": templateName}
   }
   return data, nil
}

// 輸出Element元件
func(app *Page) ProcessElementTemplate(jsonData string, tpls []string) (string, error) {
   data, err := app.convertString2Interface(jsonData, tpls[0]) // 解析 JSON 數據
   paths := make([]string, len(tpls))
   for i, name := range tpls {
      paths[i] = app.Path + name
   }
   // 創建並解析模板
   tmpl := template.New(tpls[0]).
      Funcs(sprig.FuncMap()).Funcs(app.funcs)
      tmpl, err = tmpl.ParseFiles(paths...)
   if err != nil {
      return "", fmt.Errorf("模板解析錯誤: %v", err)
   }

   // 執行模板
   var buf bytes.Buffer
   if err := tmpl.Execute(&buf, data); err != nil {
      return "", fmt.Errorf("模板執行錯誤: %s", err.Error())
   }
   return buf.String(), nil
}

// 輸出畫面
func(app *Page) PrintPage(p, pageName string, w http.ResponseWriter) {
   pages := []string{pageName + ".tpl"}
   // 此函數只處理預設的樣板：sidebar.tpl
   _, err := os.Stat(app.Path + pages[0] + "-sidebar.tpl")
   if os.IsNotExist(err) {
      pages = append(pages, "sidebar.tpl")
   } else {
      pages = append(pages, pages[0] + "-sidebar.tpl")
   }
   // 檢查目錄下是否有其他-*檔案
   var result []string
   pattern := pages[0] + "-*.tpl"
   // 遍歷目錄並匹配檔案
   err = filepath.Walk(app.Path, func(path string, info os.FileInfo, err error)(error){
      if err != nil {
         return err
      }
      // 檢查是否為檔案以及是否符合 pattern
      if !info.IsDir() && strings.HasPrefix(info.Name(), strings.TrimSuffix(pattern, "*.tpl")) && strings.HasSuffix(info.Name(), ".tpl") {
	 for _, v := range pages {  // 如果檔案不在 existingFiles 中，則添加到結果
            if v != info.Name() {
               result = append(result, info.Name())
	    }
	 }
      }
      return nil
   })
   if err == nil {
      pages = append(pages, result...)
   }

   s, err := app.ProcessElementTemplate(p, pages)
   if err != nil {
      w.WriteHeader(http.StatusNotFound)
      fmt.Fprintf(w, "Page " + pageName + " not found(" + err.Error() + ")")
      return
   }
   fmt.Fprintf(w, s)
}

// /www/{pageName}
func(app *Page) LoadPageFromWeb(w http.ResponseWriter, r *http.Request) {
   st := sherrytime.NewSherryTime("Asia/Taipei", "-")  // Initial
   page := r.PathValue("pageName")
   if page == "" {
      w.WriteHeader(http.StatusNotFound)
      fmt.Fprintf(w, "Page name is empty or not found(" + st.Now() + ")")
      return
   }
   var p []byte
   _, err := os.Stat(app.Path + page + ".json")
   if err != nil {
      // 不存在就不做任何事
   } else { // 讀取資料內容 *.json
      p, err = os.ReadFile(app.Path + page + ".json")  // 任務區塊
      if err != nil {
         fmt.Fprintf(w, "Content error (" + err.Error() + ")")
      }
   }
   app.PrintPage(string(p), page, w)
}
