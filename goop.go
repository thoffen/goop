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
	linesCopy := make([]string, 0)
	linesCopy = append(linesCopy, lines[:]...)

	// set up our regexps
	//  rClassStart = class declaration
	rClassStart := regexp.MustCompile("^\\s*class\\s+(" + VALID_NAME + ")\\s*\\{\\s*$")
	//  rVariableDecl = variable declaration
	rVariableDecl := regexp.MustCompile("^\\s*var\\s+(.+)$")
	//  rMethod = method declaration (also captures constructors)
	rMethodDecl := regexp.MustCompile("^\\s*func (" + VALID_NAME + ")\\s*\\((.*)\\)(.*){\\s*$")
	//  rNewObjectDecl = creation of a new object
	rNewObjectDecl := regexp.MustCompile("(.*\\s+)new\\s+(" + VALID_NAME + ")\\((.*)\\)(.*)")
	//  rSlashSlashComment = lines with comments of style //
	rSlashSlashComment := regexp.MustCompile("^(.*)//(.*)$")
	//  rSlashStarCommentStart = lines with a start of comment of style /*
	rSlashStarCommentStart := regexp.MustCompile("^(.*)/\\*(.*)$")
	//  rSlashStarCommentEnd = lines that finish a comment of style */
	rSlashStarCommentEnd := regexp.MustCompile("^(.*)\\*/(.*)$")
	//  rTab = lines that begin with a tab
	rTab := regexp.MustCompile("^\t(.*)$")

	var totalBrackets, classLineStart, classLineEnd int
	var className, constructorArgs string
	classDecl := make([]string, 0)

	// strip our comments...problematic when // or /* are within quotes :(
	startSlashStarComment := 0
	for i := 1; i < len(lines); i++ {
		// strip all of our lines matching //
		if sm := rSlashSlashComment.FindStringSubmatch(lines[i]); sm != nil {
			lines[i] = sm[1]
		}
		// strip our /*
		if startSlashStarComment != 0 {
			if sm := rSlashStarCommentEnd.FindStringSubmatch(lines[i]); sm != nil {
				lines[i] = sm[2]
				startSlashStarComment = 0
			} else {
				lines[i] = ""
			}
		} else if rSlashStarCommentStart.MatchString(lines[i]) {
			if rSlashStarCommentEnd.MatchString(lines[i]) {
				sm := rSlashStarCommentEnd.FindStringSubmatch(lines[i])
				lines[i] = sm[2]
			} else {
				sm := rSlashStarCommentStart.FindStringSubmatch(lines[i])
				lines[i] = sm[1]
				startSlashStarComment = i
			}
		}
	}

	for i := 1; i < len(lines); i++ {
		countBrackets := strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
		totalBrackets += countBrackets

		// match class start...need to be in global scope & end w/ open bracket
		if sm := rClassStart.FindStringSubmatch(lines[i]); sm != nil {
			classLineStart = i
			className = sm[1]
			lines[i] = ""
		}
		if className != "" {
			if sm := rVariableDecl.FindStringSubmatch(lines[i]); sm != nil {
				classDecl = append(classDecl, "\t" + sm[1])
				lines = append(lines[0:i], lines[i+1:]...)
				i--
			} else if sm := rMethodDecl.FindStringSubmatch(lines[i]); sm != nil {
				// constructor --> save our args
				if sm[1] == className {
					constructorArgs = sm[2]
				}
				// rewrite our line as method on struct with (this *ClassName)
				lines[i] = "func (this *" + className + ") " + sm[1] + "(" + sm[2] + ")" + sm[3] + "{"
			} else if (totalBrackets == 0) {
				lines = append(lines[0:i], lines[i+1:]...)
				// now we insert our class struct & NewXyz method...
				stub := make([]string, 0)
				stub = append(stub, "type " + className + " struct {");
				stub = append(stub, classDecl[:]...)
				stub = append(stub, "}", "")
				stub = append(stub, "func New" + className + "(" + constructorArgs + ") *" + className + "{")
				stub = append(stub, "\tthis := " + className + "{}")
				rStripTypes := regexp.MustCompile("(" + VALID_NAME + ")\\s+" + VALID_NAME + "\\s*(,|\\z)")
				stub = append(stub, "\tthis." + className + "(" + rStripTypes.ReplaceAllString(constructorArgs, "$1") + ")")
				stub = append(stub, "\treturn &this")
				stub = append(stub, "}")
				// append our stub and reset i
				newLines := make([]string, 0)
				newLines = append(newLines, lines[classLineEnd:classLineStart]...)
				newLines = append(newLines, stub[:]...)
				newLines = append(newLines, lines[classLineStart + 1:]...)
				lines = newLines
				i += len(stub) - 1
				// reset our context
				classLineEnd = i
				className = ""
			} else {
				lines[i] = rTab.ReplaceAllString(lines[i], "$1")
			}
		}
		for rNewObjectDecl.MatchString(lines[i]) {
			sm := rNewObjectDecl.FindStringSubmatch(lines[i])
			lines[i] = sm[1] + "New" + sm[2] + "(" + sm[3] + ")" + sm[4]
		}
	}

	gofile := strings.Join(lines[1:], "\n")
	// write out our gofile
	err = ioutil.WriteFile(dir + "/" + base[:len(base) - 1], []byte(gofile), 0644)
	if err != nil {
		fmt.Println("Could not write Go file: " + dir + "/" + base[:len(base) - 1])
	}
}

func visit(path string, f os.FileInfo, err error) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	found, _ := regexp.MatchString("^[0-9a-zA-Z '\"!@#~\\$%^&*()-_{}\\[\\]]+\\.[Gg][Oo][Oo]$", base)
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