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
	rVariableDecl := regexp.MustCompile("^\\s*(" + VALID_NAME + ")\\s+(.+)$")
	//  rMethodDecl = method declaration (also captures constructors)
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
	//  rGenericInterface = generic interfaces ($ by itself)
	rGenericInterface := regexp.MustCompile("\\$(,|;|\\s|{|\\)|\\z)")
	//  rInterface = non-generic interfaces e.g. $ClassName
	rInterface := regexp.MustCompile("\\$(" + VALID_NAME + ")")
	//  rEmpty = line that contains only whitespace
	rEmpty := regexp.MustCompile("^\\s*$")

	var totalBrackets, classLineStart int
	var className, constructorArgs string
	classDecl := make([]string, 0)
	methodDecl := make([]string, 0)

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
		} else if className != "" {
			if totalBrackets == 1 && countBrackets == 0 && !rEmpty.MatchString(lines[i]) {
				// must be a variable declaration if it's not starting/within a block
				classDecl = append(classDecl, lines[i])
				lines = append(lines[0:i], lines[i+1:]...)
				i--
			} else if sm := rMethodDecl.FindStringSubmatch(lines[i]); sm != nil {
				// constructor --> save our args
				if sm[1] == className {
					constructorArgs = sm[2]
				}
				// save our prototype for putting in our interface
				methodDecl = append(methodDecl, lines[i])
				// rewrite our line as method on struct with (this *ClassName)
				lines[i] = "func (this *" + className + ") " + sm[1] + "(" + sm[2] + ")" + sm[3] + "{"
			} else if (totalBrackets == 0) {
				lines = append(lines[0:i], lines[i+1:]...)
				// construct our class struct
				classStruct := make([]string, 0)
				classStruct = append(classStruct, "type " + className + " struct {")
				classStruct = append(classStruct, classDecl[:]...)
				classStruct = append(classStruct, "}", "")
				// build our New__Class function
				newClass := make([]string, 0)
				newClass = append(newClass, "func New__" + className + "(" + constructorArgs + ") *" + className + " {")
				newClass = append(newClass, "\tthis := &" + className + "{}")
				rStripTypes := regexp.MustCompile("(" + VALID_NAME + ")\\s+" + VALID_NAME + "\\s*(,|\\z)")
				newClass = append(newClass, "\tthis." + className + "(" + rStripTypes.ReplaceAllString(constructorArgs, "$1") + ")")
				newClass = append(newClass, "\treturn this")
				newClass = append(newClass, "}", "")
				// build our getters/setters
				getSet := make([]string, 0)
				for _, decl := range classDecl {
					if sm := rVariableDecl.FindStringSubmatch(decl); sm != nil {
						// don't provide getters/setters for private declarations
						if sm[1][0] < 'A' || sm[1][0] > 'Z' {
							continue
						}
						// getter has same method name as Variable
						// if that function exists within our methods then don't provide it
						funcFound := false
						for _, method := range methodDecl {
							sm2 := rMethodDecl.FindStringSubmatch(method)
							if sm2 != nil && sm2[1] == sm[1] {
								funcFound = true
								break
							}
						}
						if !funcFound {
							// add our method to the code stub
							getSet = append(getSet, "func (this *" + className + ") " + sm[1] + "() " + sm[2] + " {")
							getSet = append(getSet, "\treturn this." + sm[1], "}", "")
							// add our method to the method declarations
							methodDecl = append(methodDecl, "\tfunc " + sm[1] + "() " + sm[2] + " {")
						}
						// if setter exists within our methods, then don't provide it
						setterName := "Set" + sm[1]
						funcFound = false
						for _, method := range methodDecl {
							sm2 := rMethodDecl.FindStringSubmatch(method)
							if sm2 != nil && sm2[1] == setterName {
								funcFound = true
								break
							}
						}
						if !funcFound {
							// add our method to the code stub
							getSet = append(getSet, "func (this *" + className + ") " + setterName + "(" + sm[1] + " " + sm[2] + ") {")
							getSet = append(getSet, "\tthis." + sm[1] + " = " + sm[1], "}", "")
							// add our method to the method declarations
							methodDecl = append(methodDecl, "\tfunc " + setterName + "(" + sm[1] + " " + sm[2] + ") {")
						}
					}
				}
				// construct our class interface
				classInterface := make([]string, 0)
				classInterface = append(classInterface, "type Interface__" + className + " interface {")
				for _, decl := range methodDecl {
					classInterface = append(classInterface, decl[0:strings.LastIndex(decl, "{")])
				}
				classInterface = append(classInterface, "}", "")
				// append our class code and reset i
				newLines := make([]string, 0)
				newLines = append(newLines, lines[0:classLineStart]...)
				newLines = append(newLines, classStruct...)
				newLines = append(newLines, classInterface...)
				newLines = append(newLines, newClass...)
				newLines = append(newLines, getSet...)
				// strip trailing blank line
				newLines = newLines[:len(newLines)-1]
				newLines = append(newLines, lines[classLineStart + 1:]...)
				lines = newLines
				i += len(classStruct) + len(classInterface) + len(newClass) + len(getSet) - 2
				// reset our context
				classDecl = make([]string, 0)
				methodDecl = make([]string, 0)
				className = ""
			} else {
				lines[i] = rTab.ReplaceAllString(lines[i], "$1")
			}
		}
		// for each instance of "new Xyz(args)" convert to "NewXyz(args)"
		for rNewObjectDecl.MatchString(lines[i]) {
			sm := rNewObjectDecl.FindStringSubmatch(lines[i])
			lines[i] = sm[1] + "New__" + sm[2] + "(" + sm[3] + ")" + sm[4]
		}
		// convert "$" to "interface{}"
		lines[i] = rGenericInterface.ReplaceAllString(lines[i], "interface{}$1")
		// convert "$ClassName" to Interface__ClassName
		lines[i] = rInterface.ReplaceAllString(lines[i], "Interface__$1")
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