package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	flagEntryDir  = flag.String("entry", "", "directory where to start scan")
	flagGoModName = flag.String("gomod", "github.com/sequix/baobab", "go mod name")
)

var (
	edges      = map[string]struct{}{}
	dirsParsed = map[string]struct{}{}
)

func main() {
	flag.Parse()
	if err := parseDir(*flagEntryDir); err != nil {
		log.Fatal(err)
	}
	fmt.Println("digraph G {")
	for e := range edges {
		e = strings.ReplaceAll(e, string(os.PathSeparator), "_")
		e = strings.ReplaceAll(e, "-", "_")
		e = strings.ReplaceAll(e, " _> ", " -> ")
		fmt.Println(e)
	}
	fmt.Println("}")
}

func parseDir(dir string) error {
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read dir %s: %s", dir, err)
	}
	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}
		if !strings.HasSuffix(fi.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(fi.Name(), "_test.go") {
			continue
		}
		file := filepath.Join(dir, fi.Name())
		imports, err := parseFile(file)
		if err != nil {
			return fmt.Errorf("failed to parse file %s: %s", file, err)
		}
		for _, imp := range imports {
			if !strings.HasPrefix(imp, *flagGoModName) {
				continue
			}
			nextDir := strings.TrimPrefix(imp, *flagGoModName)
			nextDir = strings.TrimPrefix(nextDir, "/")
			if nextDir == dir {
				continue
			}
			edges[fmt.Sprintf("%s -> %s", dir, nextDir)] = struct{}{}
			if _, parsed := dirsParsed[nextDir]; !parsed {
				if err := parseDir(nextDir); err != nil {
					return err
				}
			}
		}
	}
	dirsParsed[dir] = struct{}{}
	return nil
}

func parseFile(file string) ([]string, error) {
	fileReader, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %s", file, err)
	}
	defer fileReader.Close()
	var (
		result []string
		scan   = NewScanner(bufio.NewReader(fileReader))
	)
	for {
		token := scan.Next()
		switch token.Type {
		case EOF:
			return result, nil
		case Error:
			return nil, fmt.Errorf("scan file %s error: %s", file, token)
		case Word:
			switch token.Text {
			case "package":
				nextToken := scan.Next()
				if nextToken.Type != Word {
					return nil, fmt.Errorf("expected a word after 'package' got %s", token)
				}
			case "import":
				partial, err := parseImport(scan)
				if err != nil {
					return nil, err
				}
				result = append(result, partial...)
			case "var", "const", "func", "type":
				return result, nil
			}
		default:
			return nil, fmt.Errorf("unexpected token %s", token)
		}
	}
}

func parseImport(scan *Scanner) ([]string, error) {
	token := scan.Next()
	switch token.Type {
	case EOF:
		return nil, fmt.Errorf("unexpected EOF after 'import'")
	case Error:
		return nil, fmt.Errorf("scan element after 'import' error: %s", token)
	case Word:
		nextToken := scan.Next()
		if nextToken.Type != String {
			return nil, fmt.Errorf("expected string after import alias: %s", token)
		}
		return []string{strings.Trim(nextToken.Text, "`\"")}, nil
	case String:
		return []string{strings.Trim(token.Text, "`\"")}, nil
	case LeftParen:
		return parseImportParen(scan)
	default:
		return nil, fmt.Errorf("unexpected token while scanning 'import' %s", token)
	}
}

func parseImportParen(scan *Scanner) ([]string, error) {
	var result []string
	for {
		token := scan.Next()
		switch token.Type {
		case EOF:
			return nil, fmt.Errorf("unexpected EOF after 'import ('")
		case Error:
			return nil, fmt.Errorf("scan element after 'import (' error: %s", token)
		case Word:
			nextToken := scan.Next()
			if nextToken.Type != String {
				return nil, fmt.Errorf("expected string after import alias: %s", token)
			}
			result = append(result, strings.Trim(nextToken.Text, "`\""))
		case String:
			result = append(result, strings.Trim(token.Text, "`\""))
		case RightParen:
			return result, nil
		default:
			return nil, fmt.Errorf("unexpected token while scanning 'import (' %s", token)
		}
	}
}
