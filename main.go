package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// --- Types & Tokens ---

type TokenType int

const (
	TokenText TokenType = iota
	TokenOpenBrace
	TokenCloseBrace
	TokenColon
	TokenSemicolon
)

type Token struct {
	Type    TokenType
	Content string
}

type Rule struct {
	Selector     string
	Declarations []string
}

// --- Lexer (Byte-by-Byte with Unread) ---

type Lexer struct {
	reader *bufio.Reader
}

func (l *Lexer) NextToken() (Token, error) {
	var sb strings.Builder
	for {
		r, _, err := l.reader.ReadRune()
		if err != nil {
			if sb.Len() > 0 {
				return Token{TokenText, strings.TrimSpace(sb.String())}, nil
			}
			return Token{}, err
		}

		if strings.ContainsRune("{}:;", r) {
			content := strings.TrimSpace(sb.String())
			if content != "" {
				l.reader.UnreadRune()
				return Token{TokenText, content}, nil
			}
			switch r {
			case '{':
				return Token{TokenOpenBrace, "{"}, nil
			case '}':
				return Token{TokenCloseBrace, "}"}, nil
			case ':':
				return Token{TokenColon, ":"}, nil
			case ';':
				return Token{TokenSemicolon, ";"}, nil
			}
		}
		sb.WriteRune(r)
	}
}

// --- Compiler Engine ---

type PostGo struct {
	Stack    []string
	FinalAST []Rule
}

func (c *PostGo) Process(input io.Reader) {
	lexer := &Lexer{reader: bufio.NewReader(input)}
	var lastText string

	for {
		// Note: Ensure lexer is initialized correctly
		tok, err := lexer.NextToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch tok.Type {
		case TokenText:
			lastText = tok.Content
		case TokenOpenBrace:
			resolved := c.resolveSelector(lastText)
			c.Stack = append(c.Stack, resolved)
			lastText = ""
		case TokenColon:
			prop := lastText
			valTok, _ := lexer.NextToken()
			if valTok.Type == TokenText {
				c.record(prop, valTok.Content)
			}
		case TokenCloseBrace:
			if len(c.Stack) > 0 {
				c.Stack = c.Stack[:len(c.Stack)-1]
			}
		case TokenSemicolon:
			lastText = ""
		}
	}
}

func (c *PostGo) resolveSelector(newPath string) string {
	if len(c.Stack) == 0 {
		return newPath
	}
	parent := c.Stack[len(c.Stack)-1]
	if strings.HasPrefix(newPath, "&") {
		return strings.Replace(newPath, "&", parent, 1)
	}
	return parent + " " + newPath
}

func (c *PostGo) record(prop, val string) {
	if len(c.Stack) == 0 {
		return
	}
	sel := c.Stack[len(c.Stack)-1]
	for i := range c.FinalAST {
		if c.FinalAST[i].Selector == sel {
			c.FinalAST[i].Declarations = append(c.FinalAST[i].Declarations, prop+": "+val)
			return
		}
	}
	c.FinalAST = append(c.FinalAST, Rule{Selector: sel, Declarations: []string{prop + ": " + val}})
}

// --- CLI Logic ---

func main() {
	srcDir := flag.String("src", "./src", "Source directory")
	outPath := flag.String("out", "bundle.css", "Output file")
	watch := flag.Bool("watch", false, "Watch mode")
	flag.Parse()

	run := func() {
		engine := &PostGo{}
		err := filepath.Walk(*srcDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && filepath.Ext(path) == ".css" {
				f, _ := os.Open(path)
				defer f.Close()
				engine.Process(f)
			}
			return nil
		})

		if err != nil {
			log.Printf("Error walking path: %v", err)
			return
		}

		outFile, _ := os.Create(*outPath)
		defer outFile.Close()
		for _, r := range engine.FinalAST {
			fmt.Fprintf(outFile, "%s {\n  %s;\n}\n\n", r.Selector, strings.Join(r.Declarations, ";\n  "))
		}
		log.Println("✨ Bundle updated successfully.")
	}

	run()

	if *watch {
		watcher, _ := fsnotify.NewWatcher()
		defer watcher.Close()
		go func() {
			for event := range watcher.Events {
				if event.Op&fsnotify.Write == fsnotify.Write {
					run()
				}
			}
		}()
		watcher.Add(*srcDir)
		log.Printf("👀 Watching for changes in %s...", *srcDir)
		select {}
	}
}
