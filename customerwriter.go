/*
   reference: https://anto.pt/articles/go-http-responsewriter?ref=dailydev
*/
package SherryServer

import(
   "net/http"
)

// it's actually a good idea to implement a Flusher version for our writer as well
type HttpWriterFlusher struct {
   *HttpWriter   // wrap our "normal" writer
   http.Flusher  // keep a ref to the wrapped Flusher
}

type HttpWriter struct {
   w http.ResponseWriter // wrap an existing writer
   headerWritten bool
}


// implement http.ResponseWriter
func(w *HttpWriter) Header()(http.Header) {
   return w.w.Header()
}

func(w *HttpWriter) Write(data []byte) (int, error) {
   return w.w.Write(data)
}

func(w *HttpWriter) WriteHeader(statusCode int) {
   w.w.WriteHeader(statusCode)
}

func(w *HttpWriterFlusher) Flush() {
    w.Flusher.Flush()
}

// modify the constructor to either return HttpWriter or HttpWriterFlusher depending on the writer being wrapped
func NewHttpWriter(w http.ResponseWriter)(http.ResponseWriter) {
   httpWriter := &HttpWriter {
      w: w,
   }
   if flusher, ok := w.(http.Flusher); ok {
      return &HttpWriterFlusher{
         HttpWriter: httpWriter,
         Flusher:    flusher,
      }
   }
   return httpWriter
}
