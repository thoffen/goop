package main

import (
	"fmt"
	"path/filepath"
	"flag"
	"os"
	"regexp"
	"io/ioutil"
	"bufio"
	"strings"
)

const (
	VALID_NAME = "[0-9a-zA-Z_]+"
)

func parseGoo(dir, base string) {
	// open the goo file for reading
	goofile, err := os.Open(dir + "/" + base)
	if err != nil {
		fmt.Println("Could not open Goo file: " + dir + "/" + base)
		return
	}
	defer goofile.Close()
	// create a slice of our go file lines & scan it in
	lines := make([]string, 1)
	scanner := bufio.NewScanner(goofile)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// set up our regexps
	//  rClassStart = class declaration
	rClassStart := regexp.MustCompile("^\\s*class\\s+(" + VALID_NAME + ")\\s*\\{\\s*$")
	//  rVariableDecl = variable declaration
	rVariableDecl := regexp.MustCompile("^\\s*var\\s+(.+)$")
	//  rMethod = method declaration (also captures constructors)
	rMethodDecl := regexp.MustCompile("^\\s*func (" + VALID_NAME + ")\\s*(\\(.*)\\)\\s*(.+)?\\s*{\\s*$")
	//  rNewObjectDecl = creation of a new object
	rNewObjectDecl := regexp.MustCompile("new\\s+(" + VALID_NAME + ")\\((.*)\\)")

	var totalBrackets, classLineStart, classLineEnd int
	var inClass bool

	for i := 1; i < len(lines); i++ {
		countBrackets := strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
		// match class start...need to be in global scope & end w/ open bracket
		if rClassStart.MatchString(lines[i]) {
			classLineStart = i
			inClass = true
			fmt.Printf("class  %v: %s\n", i, lines[i])
		}
		if rVariableDecl.MatchString(lines[i]) {
			fmt.Printf("var    %v: %s\n", i, lines[i])
		}
		if rMethodDecl.MatchString(lines[i]) {
			fmt.Printf("method %v: %s\n", i, lines[i])
		}
		if rNewObjectDecl.MatchString(lines[i]) {
			fmt.Printf("newobj %v: %s\n", i, lines[i])
		}

		totalBrackets += countBrackets
		if inClass && totalBrackets == 0 {
			classLineEnd = i
			inClass = false
		}
	}

	fmt.Printf("Class start: %v, Class end: %v, Total Brackets: %v\n", classLineStart, classLineEnd, totalBrackets)

	gofile := []byte{'G', 'o', '!', '\n'}
	// write out our gofile
	err = ioutil.WriteFile(dir + "/" + base[:len(base) - 1], gofile, 0644)
	if err != nil {
		fmt.Println("Could not write Go file: " + dir + "/" + base[:len(base) - 1])
	}
}

func visit(path string, f os.FileInfo, err error) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	found, _ := regexp.MatchString("^[0-9a-zA-Z '\"!@#~$%^&*()-_{}\\[\\]]+\\.[Gg][Oo][Oo]$", base)
	if found {
		// found a goo file!
		fmt.Println("Matched: ", path)
		parseGoo(dir, base)
	}
	return nil
}

func main() {
	flag.Parse()
	root := flag.Arg(0)
	if err := filepath.Walk(root, visit); err != nil {
		fmt.Println("Error: ", err)
	}
}