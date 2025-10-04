package main

import (
	"flag"
	"fmt"
	"os"
)

func fail(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, msg+": %v\n", err)
		os.Exit(1)
	}
}

func main() {

	mailRoot := flag.String("root", "", "root of the received echomail tree")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "echomail-index will generate Gemini index files for an echomail groups directory as received by echomail-mailer.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [options]\n\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}
	flag.Parse()

	info, err := os.Stat(*mailRoot)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "mail root must be a directory: %v\n", err)
		flag.Usage()
	}

	err = indexMail(*mailRoot)
	fail(err, "index generation error")
	os.Exit(0)
}
