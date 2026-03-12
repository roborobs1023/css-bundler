package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"

	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// --- Types ---

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

// --- Lexer ---

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

		switch r {
		case '{':
			return Token{TokenOpenBrace, "{"}, nil
		case '}':
			return Token{TokenCloseBrace, "}"}, nil
		case ':':
			return Token{TokenColon, ":"}, nil
		case ';':
			return Token{TokenSemicolon, ";"}, nil
		case ' ', '\n', '\t', '\r':
			if sb.Len() > 0 {
				sb.WriteRune(r)
			}
		default:
			sb.WriteRune(r)
			next, _ := l.reader.Peek(1)
			if len(next) > 0 && strings.ContainsAny(string(next), "{}:;") {
				return Token{TokenText, strings.TrimSpace(sb.String())}, nil
			}
		}
	}
}

// --- Compiler Engine ---

type PostGo struct {
	Stack    []string
	FinalAST []Rule
}

func (c *PostGo) Process(input io.Reader) {
	lexer := &Lexer{reader: bufio.NewReader(input)}
	var currentText string
	var currentProp string

	for {
		tok, err := lexer.NextToken()
		if err == io.EOF {
			break
		}

		switch tok.Type {
		case TokenText:
			currentText = tok.Content
		case TokenOpenBrace:
			resolved := c.resolveSelector(currentText)
			c.Stack = append(c.Stack, resolved)
			currentText = ""
		case TokenColon:
			currentProp = currentText
		case TokenSemicolon:
			c.record(currentProp, currentText)
		case TokenCloseBrace:
			if len(c.Stack) > 0 {
				c.Stack = c.Stack[:len(c.Stack)-1]
			}
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

// --- CLI & Runner ---

func main() {
	srcDir := flag.String("src", "./src", "Source directory")
	outPath := flag.String("out", "bundle.css", "Output file")
	watch := flag.Bool("watch", false, "Watch mode")
	flag.Parse()

	run := func() {
		engine := &PostGo{}
		filepath.Walk(*srcDir, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() && filepath.Ext(path) == ".css" {
				f, _ := os.Open(path)
				defer f.Close()
				engine.Process(f)
			}
			return nil
		})

		outFile, _ := os.Create(*outPath)
		defer outFile.Close()
		for _, r := range engine.FinalAST {
			fmt.Fprintf(outFile, "%s {\n  %s;\n}\n\n", r.Selector, strings.Join(r.Declarations, ";\n  "))
		}
		fmt.Println("🚀 Bundle updated!")
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
		fmt.Println("👀 Watching for changes...")
		select {}
	}
}
