package SherryErrorExecuter

import(
   "fmt"
)

// A KnownError is one that returns a specific error code to the client such that
// it can be handled explicitly.  For example, the unique job feature will return
// a NOTUNIQUE error when the client tries to push() a job that already exists.
type KnownError interface {
   error
   Code() string
}

// Unexpected errors will always use "ERR" as their code, for instance any
// malformed data, network errors, IO errors, etc.  Clients are expected to
// raise an exception for any ERR response.
type codedError struct {
   code string
   msg  string
}

func(t *codedError) Error()(string) {
   return fmt.Sprintf("%s %s", t.code, t.msg)
}

func(t *codedError) Code() (string) {
   return t.code
}

// 已知的錯誤
func ExpectedError(code string, msg string)(error) {
   return &codedError{code: code, msg: msg}
}
