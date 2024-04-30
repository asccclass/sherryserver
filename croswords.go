package SherryServer

import (
   "strings"
)

type TrieNode struct {
   children map[rune]*TrieNode
   fail     *TrieNode
   output   []string
}

type AhoCorasick struct {
   root         *TrieNode
   PatternNum   int
   Sep          string
}

func(ac *AhoCorasick) AddPattern(pattern string) {
   current := ac.root

   for _, ch := range pattern {
      if current.children[ch] == nil {
         current.children[ch] = &TrieNode{
            children: make(map[rune]*TrieNode),
            fail:     nil,
            output:   nil,
         }
      }
      current = current.children[ch]
   }
   current.output = append(current.output, pattern)
   ac.PatternNum = ac.PatternNum + 1
}

// 增加資料傳入字串
func(ac *AhoCorasick) AddPatterns(s, sep string) {
   if len(s) == 0 {
      ac.PatternNum = 0
      return
   }
   elems := strings.Split(s, sep)
   for _, elem := range elems {
      ac.AddPattern(elem)
   }
   ac.PatternNum = len(elems)
   ac.Sep = sep
   ac.buildFailTransitions()
}

func(ac *AhoCorasick) buildFailTransitions() {
   queue := make([]*TrieNode, 0)

   // Set the fail pointer of all level-1 nodes to the root node
   for _, child := range ac.root.children {
      child.fail = ac.root
      queue = append(queue, child)
   }

   for len(queue) > 0 {
      current := queue[0]
      queue = queue[1:]

      for ch, child := range current.children {
         failNode := current.fail

         for failNode != nil && failNode.children[ch] == nil {
            failNode = failNode.fail
         }

         if failNode != nil {
            child.fail = failNode.children[ch]
         } else {
            child.fail = ac.root
         }

         child.output = append(child.output, child.fail.output...)
         queue = append(queue, child)
      }
   }
}

// 取得 output 資料
func(ac *AhoCorasick) GetPattern(sep string)(string) {
   if len(ac.root.output) == 0 {
      return ""
   }
   if sep == "" {
      sep = ac.Sep
   }
   return strings.Join(ac.root.output, sep)
}

func(ac *AhoCorasick) Search(text string)([]string, bool) {
   result := make([]string, 0)
   current := ac.root

   for _, ch := range text {
      for current.children[ch] == nil && current != ac.root {
         current = current.fail
      }
      if current.children[ch] != nil {
         current = current.children[ch]
      }
      result = append(result, current.output...)
   }
   return result, len(result) > 0
}


func NewAhoCorasick()(*AhoCorasick) {
   return &AhoCorasick {
      root: &TrieNode{
         children: make(map[rune]*TrieNode),
         fail:     nil,
         output:   nil,
      },
      PatternNum: 0,
   }
}